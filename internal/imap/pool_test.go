package imap

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

type testIMAPServer struct {
	listener net.Listener
	accepted chan struct{}
	gate     <-chan struct{}
	noopGate <-chan struct{}
	count    atomic.Int32
	noops    atomic.Int32
}

func newTestIMAPServer(t *testing.T, gate <-chan struct{}) *testIMAPServer {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := &testIMAPServer{
		listener: listener,
		accepted: make(chan struct{}, 16),
		gate:     gate,
	}
	t.Cleanup(func() { _ = listener.Close() })
	go server.serve()
	return server
}

func (s *testIMAPServer) serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		s.count.Add(1)
		s.accepted <- struct{}{}
		go s.handle(conn)
	}
}

func (s *testIMAPServer) handle(conn net.Conn) {
	defer conn.Close()
	_, _ = fmt.Fprint(conn, "* OK test IMAP server\r\n")
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		if strings.EqualFold(fields[1], "NOOP") {
			s.noops.Add(1)
			if s.noopGate != nil {
				<-s.noopGate
			}
		}
		if strings.EqualFold(fields[1], "LOGIN") && s.gate != nil {
			<-s.gate
		}
		_, _ = fmt.Fprintf(conn, "%s OK completed\r\n", fields[0])
	}
}

func (s *testIMAPServer) credentials(accountID string) (*ClientConfig, error) {
	host, portString, err := net.SplitHostPort(s.listener.Addr().String())
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(portString)
	if err != nil {
		return nil, err
	}
	return &ClientConfig{
		Host:           host,
		Port:           port,
		Security:       SecurityNone,
		Username:       accountID,
		Password:       "test",
		ConnectTimeout: time.Second,
		ReadTimeout:    time.Second,
		WriteTimeout:   time.Second,
	}, nil
}

func testPoolConfig(maxConnections int) PoolConfig {
	return PoolConfig{
		MaxConnections: maxConnections,
		IdleTimeout:    time.Minute,
		ConnectTimeout: time.Second,
		WaiterTimeout:  time.Second,
	}
}

func TestPoolConcurrentAcquireRespectsMaxConnections(t *testing.T) {
	loginGate := make(chan struct{})
	server := newTestIMAPServer(t, loginGate)
	pool := NewPool(testPoolConfig(3), server.credentials)
	defer pool.CloseAll()

	const requests = 5
	errors := make(chan error, requests)
	var wg sync.WaitGroup
	for range requests {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			conn, err := pool.GetConnection(ctx, "account")
			if err == nil {
				pool.Release(conn)
			}
			errors <- err
		}()
	}

	for range 3 {
		select {
		case <-server.accepted:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for reserved connection attempts")
		}
	}
	select {
	case <-server.accepted:
		t.Fatal("created more connections than the configured maximum")
	default:
	}
	close(loginGate)
	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatalf("GetConnection: %v", err)
		}
	}
	if got := server.count.Load(); got != 3 {
		t.Fatalf("created %d connections, want 3", got)
	}
}

func TestPoolReusesReleasedConnection(t *testing.T) {
	server := newTestIMAPServer(t, nil)
	pool := NewPool(testPoolConfig(1), server.credentials)
	defer pool.CloseAll()

	first, err := pool.GetConnection(context.Background(), "account")
	if err != nil {
		t.Fatalf("first GetConnection: %v", err)
	}
	pool.Release(first)
	second, err := pool.GetConnection(context.Background(), "account")
	if err != nil {
		t.Fatalf("second GetConnection: %v", err)
	}
	defer pool.Release(second)
	if first != second {
		t.Fatal("released connection was not reused")
	}
	if got := server.count.Load(); got != 1 {
		t.Fatalf("created %d connections, want 1", got)
	}
}

func TestPoolSkipsHealthCheckForRecentlyValidatedConnection(t *testing.T) {
	server := newTestIMAPServer(t, nil)
	pool := NewPool(testPoolConfig(1), server.credentials)
	defer pool.CloseAll()

	conn, err := pool.GetConnection(context.Background(), "account")
	if err != nil {
		t.Fatalf("first GetConnection: %v", err)
	}
	pool.Release(conn)

	reused, err := pool.GetConnection(context.Background(), "account")
	if err != nil {
		t.Fatalf("reused GetConnection: %v", err)
	}
	defer pool.Release(reused)
	if reused != conn {
		t.Fatal("reused connection differs from the original")
	}
	if got := server.noops.Load(); got != 0 {
		t.Fatalf("NOOP count for recently validated connection = %d, want 0", got)
	}
}

func TestPoolHealthCheckRunsAfterTTL(t *testing.T) {
	server := newTestIMAPServer(t, nil)
	pool := NewPool(testPoolConfig(1), server.credentials)
	defer pool.CloseAll()

	conn, err := pool.GetConnection(context.Background(), "account")
	if err != nil {
		t.Fatalf("first GetConnection: %v", err)
	}
	pool.Release(conn)
	conn.mu.Lock()
	conn.lastHealthCheck = time.Now().Add(-healthCheckTTL)
	conn.mu.Unlock()

	reused, err := pool.GetConnection(context.Background(), "account")
	if err != nil {
		t.Fatalf("reused GetConnection after TTL: %v", err)
	}
	defer pool.Release(reused)
	if got := server.noops.Load(); got != 1 {
		t.Fatalf("NOOP count after health-check TTL = %d, want 1", got)
	}
}

func TestPoolDiscardsConnectionWhenExpiredHealthCheckFails(t *testing.T) {
	server := newTestIMAPServer(t, nil)
	pool := NewPool(testPoolConfig(1), server.credentials)
	defer pool.CloseAll()

	conn, err := pool.GetConnection(context.Background(), "account")
	if err != nil {
		t.Fatalf("first GetConnection: %v", err)
	}
	pool.Release(conn)
	conn.mu.Lock()
	conn.lastHealthCheck = time.Now().Add(-healthCheckTTL)
	conn.mu.Unlock()
	if err := conn.Client().ForceClose(); err != nil {
		t.Fatalf("ForceClose: %v", err)
	}

	replacement, err := pool.GetConnection(context.Background(), "account")
	if err != nil {
		t.Fatalf("GetConnection after failed health check: %v", err)
	}
	defer pool.Release(replacement)
	if replacement == conn {
		t.Fatal("failed health check returned the closed connection")
	}
	if got := server.count.Load(); got != 2 {
		t.Fatalf("created %d connections, want replacement connection", got)
	}
}

func TestPoolSlowHealthCheckDoesNotBlockOtherReservedConnection(t *testing.T) {
	server := newTestIMAPServer(t, nil)
	pool := NewPool(testPoolConfig(2), server.credentials)
	defer pool.CloseAll()

	first, err := pool.GetConnection(context.Background(), "account")
	if err != nil {
		t.Fatalf("first GetConnection: %v", err)
	}
	second, err := pool.GetConnection(context.Background(), "account")
	if err != nil {
		t.Fatalf("second GetConnection: %v", err)
	}
	pool.Release(first)
	pool.Release(second)
	for _, conn := range []*PooledConnection{first, second} {
		conn.mu.Lock()
		conn.lastHealthCheck = time.Now().Add(-healthCheckTTL)
		conn.mu.Unlock()
	}

	gate := make(chan struct{})
	server.noopGate = gate
	results := make(chan *PooledConnection, 2)
	errs := make(chan error, 2)
	acquire := func() {
		conn, err := pool.GetConnection(context.Background(), "account")
		errs <- err
		results <- conn
	}
	go acquire()

	waitForNoops := func(want int32) {
		t.Helper()
		deadline := time.Now().Add(time.Second)
		for time.Now().Before(deadline) {
			if server.noops.Load() >= want {
				return
			}
			time.Sleep(time.Millisecond)
		}
		t.Fatalf("NOOP count = %d, want at least %d", server.noops.Load(), want)
	}
	waitForNoops(1)
	go acquire()
	waitForNoops(2)
	close(gate)

	for range 2 {
		if err := <-errs; err != nil {
			t.Fatalf("GetConnection: %v", err)
		}
		pool.Release(<-results)
	}
}

func TestPoolCreationFailureReleasesSlot(t *testing.T) {
	server := newTestIMAPServer(t, nil)
	var attempts atomic.Int32
	pool := NewPool(testPoolConfig(1), func(accountID string) (*ClientConfig, error) {
		if attempts.Add(1) == 1 {
			return nil, fmt.Errorf("credentials unavailable")
		}
		return server.credentials(accountID)
	})
	defer pool.CloseAll()

	if _, err := pool.GetConnection(context.Background(), "account"); err == nil {
		t.Fatal("first GetConnection succeeded, want credential error")
	}
	conn, err := pool.GetConnection(context.Background(), "account")
	if err != nil {
		t.Fatalf("second GetConnection after creation failure: %v", err)
	}
	pool.Release(conn)
	if got := server.count.Load(); got != 1 {
		t.Fatalf("created %d connections after retry, want 1", got)
	}
}

func TestPoolCancelledWaiterIsRemoved(t *testing.T) {
	server := newTestIMAPServer(t, nil)
	pool := NewPool(testPoolConfig(1), server.credentials)
	defer pool.CloseAll()

	conn, err := pool.GetConnection(context.Background(), "account")
	if err != nil {
		t.Fatalf("initial GetConnection: %v", err)
	}
	defer pool.Release(conn)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := pool.GetConnection(ctx, "account"); err == nil {
		t.Fatal("cancelled GetConnection succeeded")
	}

	pool.mu.Lock()
	waiterCount := len(pool.waiters["account"])
	pool.mu.Unlock()
	if waiterCount != 0 {
		t.Fatalf("cancelled waiter count = %d, want 0", waiterCount)
	}
}

func TestPoolWaitLogsOncePerAcquire(t *testing.T) {
	server := newTestIMAPServer(t, nil)
	pool := NewPool(testPoolConfig(1), server.credentials)
	defer pool.CloseAll()

	var output bytes.Buffer
	previous := pool.log
	pool.log = zerolog.New(&output).Level(zerolog.DebugLevel)
	t.Cleanup(func() { pool.log = previous })

	conn, err := pool.GetConnection(context.Background(), "account")
	if err != nil {
		t.Fatalf("initial GetConnection: %v", err)
	}

	acquired := make(chan *PooledConnection, 1)
	acquireErr := make(chan error, 1)
	go func() {
		conn, err := pool.GetConnection(context.Background(), "account")
		acquireErr <- err
		acquired <- conn
	}()

	waitFor := func() chan poolWaitResult {
		t.Helper()
		deadline := time.Now().Add(time.Second)
		for time.Now().Before(deadline) {
			pool.mu.Lock()
			if len(pool.waiters["account"]) > 0 {
				waiter := pool.waiters["account"][0]
				pool.waiters["account"] = pool.waiters["account"][1:]
				pool.mu.Unlock()
				return waiter
			}
			pool.mu.Unlock()
			time.Sleep(time.Millisecond)
		}
		t.Fatal("timed out waiting for pool waiter")
		return nil
	}

	waitFor() <- poolWaitResult{}
	waitFor() <- poolWaitResult{}
	firstLogs := output.String()
	if got := strings.Count(firstLogs, "Connection pool exhausted, waiting"); got != 1 {
		t.Fatalf("waiting log count after two wakes = %d, want 1: %s", got, firstLogs)
	}

	pool.Release(conn)
	if err := <-acquireErr; err != nil {
		t.Fatalf("waiting GetConnection: %v", err)
	}
	second := <-acquired
	if second == nil {
		t.Fatal("waiting GetConnection returned nil connection")
	}
	pool.Release(second)
	if got := strings.Count(output.String(), "Connection pool exhausted, waiting"); got != 1 {
		t.Fatalf("final waiting log count = %d, want 1: %s", got, output.String())
	}
}
