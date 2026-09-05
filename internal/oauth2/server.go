package oauth2

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hkdb/aerion/internal/logging"
	"github.com/rs/zerolog"
)

var ErrAuthorizationCancelled = errors.New("authorization cancelled")

// CallbackResult represents the result of an OAuth callback
type CallbackResult struct {
	Code             string // Authorization code
	State            string // State parameter for CSRF validation
	Error            string // Error code if authorization failed
	ErrorDescription string // Human-readable error description
}

// CallbackServer is a temporary HTTP server that handles OAuth callbacks
type CallbackServer struct {
	log       zerolog.Logger
	server    *http.Server
	listeners []net.Listener
	resultCh  chan CallbackResult
	done      chan struct{}
	mu        sync.Mutex
	started   bool
	stopped   bool
	accepting atomic.Bool
	listen    func(network, address string) (net.Listener, error)
}

// NewCallbackServer creates a new OAuth callback server
func NewCallbackServer() *CallbackServer {
	return &CallbackServer{
		log:      logging.WithComponent("oauth2-callback"),
		resultCh: make(chan CallbackResult, 1),
		done:     make(chan struct{}),
		listen:   net.Listen,
	}
}

// Start starts the callback server on an available port
// Returns the port number the server is listening on
func (s *CallbackServer) Start(_ context.Context, redirectHost string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return 0, fmt.Errorf("callback server already started")
	}
	if s.stopped {
		return 0, fmt.Errorf("callback server cannot be restarted after Stop")
	}

	// Always bind IPv4 first. For localhost redirects, also accept IPv6 on the
	// same port when available so a browser resolving localhost to ::1 reaches
	// the callback. Google uses an explicit IPv4 loopback redirect instead.
	listener, err := s.listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("failed to find available port: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	s.listeners = []net.Listener{listener}
	if redirectHost == "localhost" {
		if listener6, err := s.listen("tcp6", fmt.Sprintf("[::1]:%d", port)); err == nil {
			s.listeners = append(s.listeners, listener6)
		} else {
			s.log.Debug().Err(err).Msg("IPv6 loopback unavailable; using IPv4 localhost callback")
		}
	}

	// Create HTTP server with routes
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", s.handleCallback)
	mux.HandleFunc("/", s.handleRoot)

	server := &http.Server{
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	s.server = server
	s.started = true
	s.accepting.Store(true)

	// Start each listener with the immutable local server reference. Stop may
	// clear s.server immediately after Start returns, so goroutines must never
	// dereference that mutable field.
	for _, l := range s.listeners {
		go func(listener net.Listener) {
			s.log.Debug().Int("port", port).Msg("Starting OAuth callback server")
			if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
				s.log.Error().Err(err).Msg("Callback server error")
			}
		}(l)
	}

	return port, nil
}

// WaitForCallback waits for the OAuth callback to complete
// Returns the callback result or an error if cancelled/timed out
func (s *CallbackServer) WaitForCallback(ctx context.Context) (*CallbackResult, error) {
	// Set a timeout for the callback (5 minutes)
	timeout := time.After(5 * time.Minute)

	select {
	case result := <-s.resultCh:
		// Give the browser a moment to receive the response before shutting down
		time.Sleep(500 * time.Millisecond)
		s.Stop()
		return &result, nil

	case <-timeout:
		s.Stop()
		return nil, fmt.Errorf("OAuth callback timed out after 5 minutes")

	case <-ctx.Done():
		s.Stop()
		return nil, ctx.Err()

	case <-s.done:
		return nil, ErrAuthorizationCancelled
	}
}

// Stop stops the callback server
func (s *CallbackServer) Stop() {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return
	}

	s.log.Debug().Msg("Stopping OAuth callback server")
	server := s.server
	listeners := s.listeners
	s.server = nil
	s.listeners = nil
	s.started = false
	s.stopped = true
	s.accepting.Store(false)

	// Signal done
	select {
	case <-s.done:
		// Already closed
	default:
		close(s.done)
	}
	s.mu.Unlock()

	// Shutdown server
	if server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}

	// Shutdown closes listeners it has already accepted; explicit close also
	// covers the Start→Stop race before Serve begins. Closing twice is harmless.
	for _, listener := range listeners {
		_ = listener.Close()
	}
}

// handleCallback handles the OAuth callback request
func (s *CallbackServer) handleCallback(w http.ResponseWriter, r *http.Request) {
	if !s.accepting.Load() {
		http.Error(w, "OAuth callback server is no longer active", http.StatusServiceUnavailable)
		return
	}

	query := r.URL.Query()

	result := CallbackResult{
		Code:             query.Get("code"),
		State:            query.Get("state"),
		Error:            query.Get("error"),
		ErrorDescription: query.Get("error_description"),
	}

	// Send result (non-blocking, only first result counts)
	select {
	case s.resultCh <- result:
	default:
	}

	// Respond to browser
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if result.Error != "" {
		s.log.Warn().
			Str("error", result.Error).
			Str("description", result.ErrorDescription).
			Msg("OAuth callback received error")

		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, errorPageHTML, result.Error, result.ErrorDescription)
		return
	}

	s.log.Debug().Msg("OAuth callback received authorization code")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, successPageHTML)
}

// handleRoot handles requests to the root path (redirect to callback info)
func (s *CallbackServer) handleRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, waitingPageHTML)
}

// HTML templates for callback responses
const successPageHTML = `<!DOCTYPE html>
<html>
<head>
    <title>Aerion - Authentication Successful</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif;
            display: flex;
            justify-content: center;
            align-items: center;
            min-height: 100vh;
            background: linear-gradient(135deg, #1a1a2e 0%, #16213e 100%);
            color: #eee;
        }
        .container {
            text-align: center;
            padding: 2rem;
            max-width: 400px;
        }
        .icon {
            width: 80px;
            height: 80px;
            margin: 0 auto 1.5rem;
            background: #4ade80;
            border-radius: 50%;
            display: flex;
            align-items: center;
            justify-content: center;
        }
        .icon svg {
            width: 40px;
            height: 40px;
            stroke: #1a1a2e;
            stroke-width: 3;
            fill: none;
        }
        h1 {
            color: #4ade80;
            font-size: 1.5rem;
            margin-bottom: 0.75rem;
        }
        p {
            color: #a0a0a0;
            line-height: 1.5;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="icon">
            <svg viewBox="0 0 24 24">
                <polyline points="20 6 9 17 4 12"></polyline>
            </svg>
        </div>
        <h1>Authentication Successful</h1>
        <p>You can close this window and return to Aerion.</p>
    </div>
</body>
</html>`

const errorPageHTML = `<!DOCTYPE html>
<html>
<head>
    <title>Aerion - Authentication Failed</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif;
            display: flex;
            justify-content: center;
            align-items: center;
            min-height: 100vh;
            background: linear-gradient(135deg, #1a1a2e 0%%, #16213e 100%%);
            color: #eee;
        }
        .container {
            text-align: center;
            padding: 2rem;
            max-width: 400px;
        }
        .icon {
            width: 80px;
            height: 80px;
            margin: 0 auto 1.5rem;
            background: #f87171;
            border-radius: 50%%;
            display: flex;
            align-items: center;
            justify-content: center;
        }
        .icon svg {
            width: 40px;
            height: 40px;
            stroke: #1a1a2e;
            stroke-width: 3;
            fill: none;
        }
        h1 {
            color: #f87171;
            font-size: 1.5rem;
            margin-bottom: 0.75rem;
        }
        p {
            color: #a0a0a0;
            line-height: 1.5;
            margin-bottom: 0.5rem;
        }
        .error-details {
            background: rgba(248, 113, 113, 0.1);
            border: 1px solid rgba(248, 113, 113, 0.2);
            border-radius: 8px;
            padding: 1rem;
            margin-top: 1rem;
            font-size: 0.875rem;
        }
        .error-code {
            color: #f87171;
            font-weight: 600;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="icon">
            <svg viewBox="0 0 24 24">
                <line x1="18" y1="6" x2="6" y2="18"></line>
                <line x1="6" y1="6" x2="18" y2="18"></line>
            </svg>
        </div>
        <h1>Authentication Failed</h1>
        <p>Please close this window and try again in Aerion.</p>
        <div class="error-details">
            <span class="error-code">%s</span>: %s
        </div>
    </div>
</body>
</html>`

const waitingPageHTML = `<!DOCTYPE html>
<html>
<head>
    <title>Aerion - OAuth Callback</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            display: flex;
            justify-content: center;
            align-items: center;
            min-height: 100vh;
            background: #1a1a2e;
            color: #eee;
        }
        .container { text-align: center; }
        h1 { color: #60a5fa; margin-bottom: 1rem; }
    </style>
</head>
<body>
    <div class="container">
        <h1>Aerion OAuth</h1>
        <p>Waiting for authentication...</p>
    </div>
</body>
</html>`
