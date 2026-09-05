package oauth2

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCallbackServerSupportsLocalhostRedirect(t *testing.T) {
	server := NewCallbackServer()
	port, err := server.Start(context.Background(), "localhost")
	if err != nil {
		t.Fatal(err)
	}
	defer server.Stop()

	redirectURI := loopbackRedirectURI("localhost", port)
	response, err := http.Get(redirectURI)
	if err != nil {
		t.Fatalf("localhost callback listener is unreachable: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("callback listener status = %d, want %d", response.StatusCode, http.StatusOK)
	}
}

func TestCallbackServerImmediateStopDoesNotRaceServe(t *testing.T) {
	for i := 0; i < 100; i++ {
		server := NewCallbackServer()
		if _, err := server.Start(context.Background(), "localhost"); err != nil {
			t.Fatalf("iteration %d: Start: %v", i, err)
		}
		server.Stop()
		server.Stop()
	}
}

func TestCallbackServerCannotRestartAfterStop(t *testing.T) {
	server := NewCallbackServer()
	if _, err := server.Start(context.Background(), "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	server.Stop()
	if _, err := server.Start(context.Background(), "127.0.0.1"); err == nil {
		t.Fatal("Start after Stop succeeded")
	}
}

func TestCallbackServerCallbackThenStop(t *testing.T) {
	server := NewCallbackServer()
	port, err := server.Start(context.Background(), "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.Get(loopbackRedirectURI("127.0.0.1", port) + "?code=code&state=state")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	server.Stop()
	server.Stop()

	select {
	case result := <-server.resultCh:
		if result.Code != "code" || result.State != "state" {
			t.Fatalf("unexpected callback result: %#v", result)
		}
	default:
		t.Fatal("callback result was not delivered")
	}
}

func TestCallbackAfterStopIsRejected(t *testing.T) {
	server := NewCallbackServer()
	if _, err := server.Start(context.Background(), "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	server.Stop()

	request := httptest.NewRequest(http.MethodGet, "/callback?code=late&state=late", nil)
	response := httptest.NewRecorder()
	server.handleCallback(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("callback after Stop status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
	select {
	case <-server.resultCh:
		t.Fatal("callback after Stop was delivered")
	default:
	}
}

func TestCallbackServerStopCancelsWait(t *testing.T) {
	server := NewCallbackServer()
	if _, err := server.Start(context.Background(), "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() {
		_, err := server.WaitForCallback(context.Background())
		result <- err
	}()
	server.Stop()
	if err := <-result; !errors.Is(err, ErrAuthorizationCancelled) {
		t.Fatalf("WaitForCallback error = %v, want cancellation", err)
	}
}

func TestCallbackServerLocalhostFallsBackToIPv4WithoutIPv6(t *testing.T) {
	server := NewCallbackServer()
	server.listen = func(network, address string) (net.Listener, error) {
		if network == "tcp6" {
			return nil, fmt.Errorf("IPv6 unavailable")
		}
		return net.Listen(network, address)
	}

	port, err := server.Start(context.Background(), "localhost")
	if err != nil {
		t.Fatal(err)
	}
	defer server.Stop()
	response, err := http.Get(loopbackRedirectURI("127.0.0.1", port))
	if err != nil {
		t.Fatalf("IPv4 callback listener is unreachable: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("callback listener status = %d, want %d", response.StatusCode, http.StatusOK)
	}
}

func TestCallbackServerGoogleUsesIPv4Only(t *testing.T) {
	server := NewCallbackServer()
	var sawIPv6 bool
	server.listen = func(network, address string) (net.Listener, error) {
		if network == "tcp6" {
			sawIPv6 = true
		}
		return net.Listen(network, address)
	}
	if _, err := server.Start(context.Background(), "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	defer server.Stop()
	if sawIPv6 {
		t.Fatal("Google IPv4 callback unexpectedly opened an IPv6 listener")
	}
}
