package imap

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hkdb/aerion/internal/logging"
	"github.com/rs/zerolog"
)

// IsConnectionError checks if an error indicates a dead/broken connection.
// These errors warrant discarding the connection and getting a new one from the pool.
func IsConnectionError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	connectionErrors := []string{
		"use of closed network connection",
		"connection reset",
		"broken pipe",
		"EOF",
		"i/o timeout",
		"connection refused",
		"no such host",
		"network is unreachable",
	}
	for _, connErr := range connectionErrors {
		if strings.Contains(errStr, connErr) {
			return true
		}
	}
	return false
}

// PoolConfig configures the connection pool
type PoolConfig struct {
	// MaxConnections is the maximum number of connections per account
	MaxConnections int

	// IdleTimeout is how long a connection can be idle before being closed
	IdleTimeout time.Duration

	// ConnectTimeout is how long to wait for a connection to be established
	ConnectTimeout time.Duration

	// WaiterTimeout is max time to wait for a connection when pool is exhausted
	WaiterTimeout time.Duration
}

// DefaultPoolConfig returns sensible defaults for the pool
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MaxConnections: 3,
		IdleTimeout:    5 * time.Minute,
		ConnectTimeout: 30 * time.Second,
		WaiterTimeout:  2 * time.Minute, // Don't wait forever for a connection
	}
}

// PooledConnection wraps a Client with pool metadata
type PooledConnection struct {
	client          *Client
	accountID       string
	createdAt       time.Time
	lastUsed        time.Time
	lastHealthCheck time.Time
	inUse           bool
	mu              sync.Mutex
}

// healthCheckTTL is intentionally far shorter than the pool idle timeout. It
// avoids duplicate NOOP round trips around a just-completed operation while
// still periodically detecting a server-side closed connection before reuse.
const healthCheckTTL = 15 * time.Second

// IsHealthy checks if the connection is still usable (acquires lock)
func (pc *PooledConnection) IsHealthy() bool {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	healthy := pc.isHealthyLocked()
	if healthy {
		pc.lastHealthCheck = time.Now()
	}
	return healthy
}

// isHealthyLocked checks health without acquiring lock (caller must hold lock).
// Sends a NOOP to verify the connection is alive — catches closed sockets
// that still have non-nil client references (e.g., after server-side disconnect).
func (pc *PooledConnection) isHealthyLocked() bool {
	if pc.client == nil || pc.client.client == nil {
		return false
	}

	// Send NOOP to verify the connection is actually alive.
	// This catches closed sockets (e.g., Proton Bridge dropping idle connections).
	if err := pc.client.client.Noop().Wait(); err != nil {
		return false
	}
	return true
}

// Pool manages IMAP connections for multiple accounts
type Pool struct {
	config      PoolConfig
	connections map[string][]*PooledConnection // accountID -> connections
	creating    map[string]int                 // accountID -> connections being established
	waiters     map[string][]chan poolWaitResult
	waitCounts  map[string]int
	waitTotal   map[string]time.Duration
	waitMax     map[string]time.Duration
	mu          sync.Mutex
	log         zerolog.Logger

	// Credentials provider function
	getCredentials func(accountID string) (*ClientConfig, error)
}

type poolWaitResult struct {
	conn   *PooledConnection
	closed bool
}

// NewPool creates a new connection pool
func NewPool(config PoolConfig, getCredentials func(accountID string) (*ClientConfig, error)) *Pool {
	return &Pool{
		config:         config,
		connections:    make(map[string][]*PooledConnection),
		creating:       make(map[string]int),
		waiters:        make(map[string][]chan poolWaitResult),
		waitCounts:     make(map[string]int),
		waitTotal:      make(map[string]time.Duration),
		waitMax:        make(map[string]time.Duration),
		log:            logging.WithComponent("imap-pool"),
		getCredentials: getCredentials,
	}
}

// GetConnection gets or creates a connection for an account
func (p *Pool) GetConnection(ctx context.Context, accountID string) (*PooledConnection, error) {
	acquireStarted := time.Now()
	hasWaited := false
	waitStarted := time.Now()
	var poolWaitDuration time.Duration
	var poolLockWait time.Duration
	var connectionLockWait time.Duration
	var healthCheckDuration time.Duration
	recordWait := func() {
		if !hasWaited {
			return
		}
		waited := time.Since(waitStarted)
		poolWaitDuration = waited
		p.mu.Lock()
		p.waitCounts[accountID]++
		p.waitTotal[accountID] += waited
		if waited > p.waitMax[accountID] {
			p.waitMax[accountID] = waited
		}
		p.mu.Unlock()
	}
	defer recordWait()

	// validateReservedConnection runs after the connection was atomically
	// reserved under p.mu. It must never be called while holding p.mu: NOOP is
	// network I/O and one slow server response must not block other accounts or
	// other idle connections in this pool.
	validateReservedConnection := func(conn *PooledConnection, source string) (*PooledConnection, bool) {
		connectionLockStarted := time.Now()
		conn.mu.Lock()
		connectionLockWait += time.Since(connectionLockStarted)

		now := time.Now()
		healthCheckRequired := conn.lastHealthCheck.IsZero() || now.Sub(conn.lastHealthCheck) >= healthCheckTTL
		healthy := conn.client != nil && conn.client.client != nil
		if healthCheckRequired && healthy {
			healthCheckStarted := time.Now()
			healthy = conn.isHealthyLocked()
			healthCheckDuration += time.Since(healthCheckStarted)
			if healthy {
				conn.lastHealthCheck = time.Now()
			}
		}
		conn.mu.Unlock()

		if !healthy {
			p.log.Debug().
				Str("account", accountID).
				Str("connection_source", source).
				Bool("health_check_performed", healthCheckRequired).
				Msg("Reserved IMAP connection is unhealthy, discarding")
			p.Discard(conn)
			return nil, false
		}

		p.log.Debug().
			Str("account", accountID).
			Str("connection_source", source).
			Int64("pool_lock_wait_ms", poolLockWait.Milliseconds()).
			Int64("connection_lock_wait_ms", connectionLockWait.Milliseconds()).
			Int64("health_check_ms", healthCheckDuration.Milliseconds()).
			Int64("pool_wait_ms", poolWaitDuration.Milliseconds()).
			Int64("total_ms", time.Since(acquireStarted).Milliseconds()).
			Bool("health_check_performed", healthCheckRequired).
			Bool("health_check_skipped_recent", !healthCheckRequired).
			Msg("IMAP connection acquired")
		return conn, true
	}

	acquireLoop:
	for {
		poolLockStarted := time.Now()
		p.mu.Lock()
		poolLockWait += time.Since(poolLockStarted)

		// Reserve an available connection while holding p.mu. The subsequent
		// health check happens after p.mu is released.
		if conns, ok := p.connections[accountID]; ok {
			for _, conn := range conns {
				connectionLockStarted := time.Now()
				conn.mu.Lock()
				connectionLockWait += time.Since(connectionLockStarted)
				if !conn.inUse {
					conn.inUse = true
					conn.lastUsed = time.Now()
					conn.mu.Unlock()
					p.mu.Unlock()
					if acquired, ok := validateReservedConnection(conn, "reused"); ok {
						return acquired, nil
					}
					// A failed health check discarded the reserved connection; retry
					// the normal lookup/create path.
					continue acquireLoop
				}
				conn.mu.Unlock()
			}
		}

		openConnections := len(p.connections[accountID])
		creatingConnections := p.creating[accountID]
		if openConnections+creatingConnections < p.config.MaxConnections {
			p.creating[accountID]++
			p.mu.Unlock()
			return p.createConnection(ctx, accountID)
		}

		if !hasWaited {
			p.log.Debug().Str("account_id", accountID).Int("open_connections", openConnections).Int("creating_connections", creatingConnections).Int("max", p.config.MaxConnections).Msg("Connection pool exhausted, waiting")
			hasWaited = true
		}
		waiter := make(chan poolWaitResult, 1)
		p.waiters[accountID] = append(p.waiters[accountID], waiter)
		p.mu.Unlock()

		timer := time.NewTimer(p.config.WaiterTimeout)
		select {
		case result := <-waiter:
			timer.Stop()
			poolWaitDuration = time.Since(waitStarted)
			if result.closed {
				return nil, fmt.Errorf("connection pool closed")
			}
			if result.conn != nil {
				if acquired, ok := validateReservedConnection(result.conn, "reused_after_wait"); ok {
					return acquired, nil
				}
				// A waiter can receive a connection whose last health check expired
				// while it was in use. Discard it on failure and retry normally.
				continue acquireLoop
			}
			// A connection attempt failed and released a slot. Try again.
		case <-ctx.Done():
			timer.Stop()
			p.mu.Lock()
			p.removeWaiterLocked(accountID, waiter)
			p.mu.Unlock()
			return nil, ctx.Err()
		case <-timer.C:
			p.mu.Lock()
			p.removeWaiterLocked(accountID, waiter)
			p.mu.Unlock()
			p.log.Warn().Str("account", accountID).Dur("timeout", p.config.WaiterTimeout).Msg("Timed out waiting for connection from pool")
			return nil, fmt.Errorf("timed out waiting for connection from pool")
		}
	}
}

func (p *Pool) removeWaiterLocked(accountID string, waiter chan poolWaitResult) {
	waiters := p.waiters[accountID]
	for i, candidate := range waiters {
		if candidate == waiter {
			p.waiters[accountID] = append(waiters[:i], waiters[i+1:]...)
			return
		}
	}
}

// createConnection creates a new connection for an account
func (p *Pool) createConnection(ctx context.Context, accountID string) (*PooledConnection, error) {
	creationStarted := time.Now()
	conn, err := p.createConnectionWithRetry(ctx, accountID, 0)

	p.mu.Lock()
	p.creating[accountID]--
	if p.creating[accountID] == 0 {
		delete(p.creating, accountID)
	}
	if err == nil {
		p.connections[accountID] = append(p.connections[accountID], conn)
	} else if waiters := p.waiters[accountID]; len(waiters) > 0 {
		waiter := waiters[0]
		p.waiters[accountID] = waiters[1:]
		waiter <- poolWaitResult{}
	}
	p.mu.Unlock()
	if err != nil {
		return nil, err
	}

	p.log.Info().
		Str("account", accountID).
		Str("connection_source", "created").
		Int64("total_create_ms", time.Since(creationStarted).Milliseconds()).
		Msg("New connection created")
	return conn, nil
}

// createConnectionWithRetry creates a connection with retry logic for transient errors
// like "max connections exceeded" (server still has ghost connections after network change).
func (p *Pool) createConnectionWithRetry(ctx context.Context, accountID string, attempt int) (*PooledConnection, error) {
	creationStarted := time.Now()
	p.log.Debug().
		Str("account", accountID).
		Int("attempt", attempt+1).
		Msg("Creating new connection")

	// Get credentials for this account
	credentialsStarted := time.Now()
	config, err := p.getCredentials(accountID)
	credentialsDuration := time.Since(credentialsStarted)
	if err != nil {
		p.log.Error().
			Err(err).
			Str("account", accountID).
			Int64("credentials_ms", credentialsDuration.Milliseconds()).
			Int64("total_create_ms", time.Since(creationStarted).Milliseconds()).
			Msg("Failed to get credentials")
		return nil, fmt.Errorf("failed to get credentials: %w", err)
	}

	p.log.Debug().
		Str("account", accountID).
		Str("host", config.Host).
		Int("port", config.Port).
		Str("authType", string(config.AuthType)).
		Msg("Got credentials, connecting to IMAP server")

	// Create and connect the client
	client := NewClient(*config)

	// Use a goroutine with context for connection timeout
	done := make(chan error, 1)
	go func() {
		connectStarted := time.Now()
		if err := client.Connect(); err != nil {
			p.log.Error().
				Err(err).
				Str("account", accountID).
				Int64("credentials_ms", credentialsDuration.Milliseconds()).
				Int64("connect_ms", time.Since(connectStarted).Milliseconds()).
				Int64("total_create_ms", time.Since(creationStarted).Milliseconds()).
				Msg("IMAP Connect failed")
			done <- err
			return
		}
		p.log.Debug().Str("account", accountID).Msg("IMAP connected, logging in")
		authStarted := time.Now()
		if err := client.Login(); err != nil {
			p.log.Error().
				Err(err).
				Str("account", accountID).
				Int64("credentials_ms", credentialsDuration.Milliseconds()).
				Int64("connect_ms", authStarted.Sub(connectStarted).Milliseconds()).
				Int64("auth_ms", time.Since(authStarted).Milliseconds()).
				Int64("total_create_ms", time.Since(creationStarted).Milliseconds()).
				Msg("IMAP Login failed")
			client.ForceClose()
			done <- err
			return
		}
		p.log.Debug().
			Str("account", accountID).
			Int64("credentials_ms", credentialsDuration.Milliseconds()).
			Int64("connect_ms", authStarted.Sub(connectStarted).Milliseconds()).
			Int64("auth_ms", time.Since(authStarted).Milliseconds()).
			Int64("total_create_ms", time.Since(creationStarted).Milliseconds()).
			Msg("IMAP connection creation complete")
		done <- nil
	}()

	select {
	case err := <-done:
		if err != nil {
			// Retry once for "max connections" errors — the server may still
			// have ghost connections from force-closed sessions (network change).
			// Wait 15s to give the server time to drop stale connections.
			if attempt == 0 && strings.Contains(err.Error(), "Maximum number of connections") {
				p.log.Warn().Str("account", accountID).Msg("Max connections exceeded, retrying after 15s")
				select {
				case <-time.After(15 * time.Second):
					return p.createConnectionWithRetry(ctx, accountID, attempt+1)
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			p.log.Error().Err(err).Str("account", accountID).Msg("Connection failed")
			return nil, fmt.Errorf("failed to connect: %w", err)
		}
	case <-ctx.Done():
		// Try to close the client if it was created
		p.log.Warn().Str("account", accountID).Msg("Connection timed out (context cancelled)")
		go client.ForceClose()
		return nil, ctx.Err()
	}

	conn := &PooledConnection{
		client:          client,
		accountID:       accountID,
		createdAt:       time.Now(),
		lastUsed:        time.Now(),
		lastHealthCheck: time.Now(), // Connect + Login just completed successfully.
		inUse:           true,
	}

	return conn, nil
}

// Release returns a connection to the pool
func (p *Pool) Release(conn *PooledConnection) {
	if conn == nil {
		return
	}

	releaseStarted := time.Now()
	connectionLockStarted := time.Now()
	conn.mu.Lock()
	connectionLockWait := time.Since(connectionLockStarted)
	conn.inUse = false
	conn.lastUsed = time.Now()
	trackedClient := conn.client != nil && conn.client.client != nil
	healthCheckSkippedRecent := !conn.lastHealthCheck.IsZero() && time.Since(conn.lastHealthCheck) < healthCheckTTL
	conn.mu.Unlock()

	poolLockStarted := time.Now()
	p.mu.Lock()
	poolLockWait := time.Since(poolLockStarted)
	defer p.mu.Unlock()

	// Release deliberately avoids a network round trip. A subsequent acquire
	// validates stale connections outside p.mu; a recently created or checked
	// connection remains valid for healthCheckTTL. We still reject locally
	// closed clients so CloseAll/sleep cannot hand a nil client to a waiter.
	if !trackedClient {
		p.log.Debug().
			Str("account", conn.accountID).
			Msg("Released connection is locally closed, discarding")
		return
	}

	inPool := false
	if conns, ok := p.connections[conn.accountID]; ok {
		for _, c := range conns {
			if c == conn {
				inPool = true
				break
			}
		}
	}
	if !inPool {
		p.log.Debug().
			Str("account", conn.accountID).
			Msg("Released connection no longer in pool, discarding")
		return
	}

	// Check if anyone is waiting for a connection for this account
	if waiters, ok := p.waiters[conn.accountID]; ok && len(waiters) > 0 {
		waiter := waiters[0]
		p.waiters[conn.accountID] = waiters[1:]

		conn.mu.Lock()
		conn.inUse = true
		conn.mu.Unlock()

		waiter <- poolWaitResult{conn: conn}
		return
	}

	p.log.Debug().
		Str("account", conn.accountID).
		Int64("connection_lock_wait_ms", connectionLockWait.Milliseconds()).
		Int64("health_check_ms", 0).
		Int64("pool_lock_wait_ms", poolLockWait.Milliseconds()).
		Int64("total_ms", time.Since(releaseStarted).Milliseconds()).
		Bool("health_check_performed", false).
		Bool("health_check_skipped_recent", healthCheckSkippedRecent).
		Msg("Connection released to pool")
}

// Discard removes a connection from the pool without returning it for reuse.
// Use this when a connection is known to be dead/unhealthy (e.g., after connection errors).
// Uses ForceClose to avoid blocking on dead TCP sockets.
func (p *Pool) Discard(conn *PooledConnection) {
	if conn == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Force-close the connection (known dead, skip graceful logout)
	conn.mu.Lock()
	if conn.client != nil {
		conn.client.ForceClose()
		conn.client = nil
	}
	conn.mu.Unlock()

	// Remove from pool
	if conns, ok := p.connections[conn.accountID]; ok {
		for i, c := range conns {
			if c == conn {
				p.connections[conn.accountID] = append(conns[:i], conns[i+1:]...)
				break
			}
		}
		// Clean up empty account entry
		if len(p.connections[conn.accountID]) == 0 {
			delete(p.connections, conn.accountID)
		}
	}

	p.log.Debug().
		Str("account", conn.accountID).
		Msg("Discarded dead connection from pool")
}

// CloseAccount closes all connections for a specific account.
// Uses ForceClose to avoid blocking on dead TCP sockets (e.g., after network changes).
func (p *Pool) CloseAccount(accountID string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	conns, ok := p.connections[accountID]
	if ok {
		for _, conn := range conns {
			conn.mu.Lock()
			if conn.client != nil {
				conn.client.ForceClose()
				conn.client = nil
			}
			conn.mu.Unlock()
		}

		delete(p.connections, accountID)
	}

	// Notify any waiters that we're closing
	if waiters, ok := p.waiters[accountID]; ok {
		for _, w := range waiters {
			w <- poolWaitResult{closed: true}
		}
		delete(p.waiters, accountID)
	}
	if !ok {
		return
	}

	p.log.Info().
		Str("account", accountID).
		Int("closed", len(conns)).
		Msg("Closed all connections for account")
}

// CloseAll closes all connections in the pool
func (p *Pool) CloseAll() {
	p.mu.Lock()
	accountIDs := make([]string, 0, len(p.connections))
	for accountID := range p.connections {
		accountIDs = append(accountIDs, accountID)
	}
	p.mu.Unlock()

	for _, accountID := range accountIDs {
		p.CloseAccount(accountID)
	}

	p.log.Info().Msg("Closed all connections")
}

// CleanupIdle closes connections that have been idle too long
func (p *Pool) CleanupIdle() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	cleaned := 0

	for accountID, conns := range p.connections {
		var remaining []*PooledConnection

		for _, conn := range conns {
			conn.mu.Lock()
			idle := !conn.inUse && now.Sub(conn.lastUsed) > p.config.IdleTimeout
			conn.mu.Unlock()

			if idle {
				conn.mu.Lock()
				if conn.client != nil {
					conn.client.ForceClose()
				}
				conn.mu.Unlock()
				cleaned++
			} else {
				remaining = append(remaining, conn)
			}
		}

		if len(remaining) == 0 {
			delete(p.connections, accountID)
		} else {
			p.connections[accountID] = remaining
		}
	}

	if cleaned > 0 {
		p.log.Debug().
			Int("cleaned", cleaned).
			Msg("Cleaned up idle connections")
	}
}

// StartCleanupRoutine starts a background goroutine that periodically cleans up idle connections
func (p *Pool) StartCleanupRoutine(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				p.CleanupIdle()
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Stats returns pool statistics
type PoolStats struct {
	TotalConnections  int
	ActiveConnections int
	IdleConnections   int
	AccountCount      int
}

// GetStats returns current pool statistics
func (p *Pool) GetStats() PoolStats {
	p.mu.Lock()
	defer p.mu.Unlock()

	stats := PoolStats{
		AccountCount: len(p.connections),
	}

	for _, conns := range p.connections {
		for _, conn := range conns {
			stats.TotalConnections++
			conn.mu.Lock()
			if conn.inUse {
				stats.ActiveConnections++
			} else {
				stats.IdleConnections++
			}
			conn.mu.Unlock()
		}
	}

	return stats
}

// WaitMetrics aggregates connection-wait observations for an account.
type WaitMetrics struct {
	Count int
	Total time.Duration
	Max   time.Duration
}

// WaitMetricsSnapshot returns the pool wait metrics observed for an account.
func (p *Pool) WaitMetricsSnapshot(accountID string) WaitMetrics {
	p.mu.Lock()
	defer p.mu.Unlock()
	return WaitMetrics{
		Count: p.waitCounts[accountID],
		Total: p.waitTotal[accountID],
		Max:   p.waitMax[accountID],
	}
}

// Client returns the underlying IMAP client from a pooled connection
func (pc *PooledConnection) Client() *Client {
	return pc.client
}
