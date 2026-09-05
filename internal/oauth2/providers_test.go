package oauth2

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestGoogleProvider(t *testing.T) {
	p := GoogleProvider()

	if p.Name != "google" {
		t.Errorf("Name = %q, want %q", p.Name, "google")
	}
	if !strings.Contains(p.AuthURL, "google") {
		t.Errorf("AuthURL = %q, expected it to contain 'google'", p.AuthURL)
	}
	if len(p.Scopes) == 0 {
		t.Error("Scopes is empty, want at least one scope")
	}
}

func TestOfficialProvidersArePublicClients(t *testing.T) {
	origGoogle, origGoogleSecret, origMicrosoft := GoogleClientID, GoogleClientSecret, MicrosoftClientID
	t.Cleanup(func() {
		GoogleClientID, GoogleClientSecret, MicrosoftClientID = origGoogle, origGoogleSecret, origMicrosoft
	})

	GoogleClientID = "google-public-id"
	GoogleClientSecret = "google-desktop-configuration"
	MicrosoftClientID = "microsoft-public-id"

	google := GoogleProvider()
	if google.ClientSecret == "" || !IsGoogleConfigured() {
		t.Fatal("Google Desktop client must be configured with ID and configuration value")
	}
	microsoft := MicrosoftProvider()
	if microsoft.ClientSecret != "" || !IsMicrosoftConfigured() {
		t.Fatal("Microsoft public client must be configured with an ID and no secret")
	}
	if google.LoopbackHost != "127.0.0.1" || microsoft.LoopbackHost != "localhost" {
		t.Fatalf("unexpected loopback hosts: Google=%q Microsoft=%q", google.LoopbackHost, microsoft.LoopbackHost)
	}
}

func TestGoogleWithoutSecretIsUnavailable(t *testing.T) {
	origGoogle, origSecret, origOverride := GoogleClientID, GoogleClientSecret, UserOverrideLookup
	t.Cleanup(func() { GoogleClientID, GoogleClientSecret, UserOverrideLookup = origGoogle, origSecret, origOverride })
	GoogleClientID = "google-public-id"
	GoogleClientSecret = ""
	UserOverrideLookup = nil
	if IsGoogleConfigured() {
		t.Fatal("Google must be unavailable without required Desktop configuration")
	}
}

func TestGoogleWithoutClientIDIsUnavailable(t *testing.T) {
	origGoogle, origOverride := GoogleClientID, UserOverrideLookup
	t.Cleanup(func() { GoogleClientID, UserOverrideLookup = origGoogle, origOverride })
	GoogleClientID = ""
	UserOverrideLookup = nil
	if IsGoogleConfigured() {
		t.Fatal("Google must be unavailable without a client ID")
	}
}

func TestAuthorizationAndExchangeUseSameRedirectURI(t *testing.T) {
	provider := GoogleProvider()
	redirectURI := loopbackRedirectURI(provider.LoopbackHost, 4242)
	authURL := buildAuthURL(provider, "state", "challenge", redirectURI)
	parsed, err := url.Parse(authURL)
	if err != nil {
		t.Fatal(err)
	}
	if got := parsed.Query().Get("redirect_uri"); got != redirectURI {
		t.Fatalf("authorization redirect URI = %q, want %q", got, redirectURI)
	}
}

func TestPublicClientExchangeOmitsSecretAndKeepsRedirectURI(t *testing.T) {
	redirectURI := loopbackRedirectURI("127.0.0.1", 4242)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if got := r.Form.Get("client_secret"); got != "" {
			http.Error(w, "public client sent client_secret", http.StatusBadRequest)
			return
		}
		if got := r.Form.Get("redirect_uri"); got != redirectURI {
			http.Error(w, "redirect URI mismatch", http.StatusBadRequest)
			return
		}
		_, _ = io.WriteString(w, `{"access_token":"access","refresh_token":"refresh","expires_in":3600}`)
	}))
	defer server.Close()

	provider := ProviderConfig{ClientID: "public-client", TokenURL: server.URL}
	tokens, err := NewManager().exchangeCode(provider, "code", "verifier", redirectURI)
	if err != nil {
		t.Fatal(err)
	}
	if tokens.AccessToken == "" || tokens.RefreshToken == "" {
		t.Fatal("expected token response")
	}
}

func TestGoogleExchangeAndRefreshIncludeConfiguredSecret(t *testing.T) {
	redirectURI := loopbackRedirectURI("127.0.0.1", 4242)
	requests := make(chan url.Values, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		requests <- r.Form
		_, _ = io.WriteString(w, `{"access_token":"access","refresh_token":"refresh","expires_in":3600}`)
	}))
	defer server.Close()
	provider := ProviderConfig{ClientID: "google-id", ClientSecret: "google-desktop-secret", TokenURL: server.URL}
	manager := NewManager()
	if _, err := manager.exchangeCode(provider, "code", "verifier", redirectURI); err != nil {
		t.Fatal(err)
	}
	if form := <-requests; form.Get("client_secret") == "" || form.Get("code_verifier") != "verifier" {
		t.Fatal("Google token exchange omitted required client_secret or PKCE verifier")
	}
	if _, err := manager.RefreshTokenWithProvider(provider, "refresh"); err != nil {
		t.Fatal(err)
	}
	if form := <-requests; form.Get("client_secret") == "" || form.Get("refresh_token") != "refresh" {
		t.Fatal("Google refresh omitted required client_secret")
	}
}

func TestPublicGoogleFlowUsesPKCEStateAndIPv4Redirect(t *testing.T) {
	origGoogle, origSecret, origOverride := GoogleClientID, GoogleClientSecret, UserOverrideLookup
	t.Cleanup(func() { GoogleClientID, GoogleClientSecret, UserOverrideLookup = origGoogle, origSecret, origOverride })
	GoogleClientID = "google-public-id"
	GoogleClientSecret = "google-desktop-configuration"
	UserOverrideLookup = nil

	manager := NewManager()
	defer manager.CancelAuthFlow()
	authURL, err := manager.StartAuthFlow(context.Background(), "google")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(authURL)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	if query.Get("code_challenge_method") != "S256" || query.Get("code_challenge") == "" {
		t.Fatal("Google authorization flow must use PKCE S256")
	}
	if query.Get("state") == "" {
		t.Fatal("Google authorization flow must include state")
	}
	if got := query.Get("redirect_uri"); !strings.HasPrefix(got, "http://127.0.0.1:") {
		t.Fatalf("Google redirect URI = %q, want IPv4 loopback", got)
	}
}

func TestMicrosoftProvider(t *testing.T) {
	p := MicrosoftProvider()

	if p.Name != "microsoft" {
		t.Errorf("Name = %q, want %q", p.Name, "microsoft")
	}
	if !strings.Contains(p.AuthURL, "microsoftonline") {
		t.Errorf("AuthURL = %q, expected it to contain 'microsoftonline'", p.AuthURL)
	}
}

func TestGoogleContactsOnlyProvider(t *testing.T) {
	p := GoogleContactsOnlyProvider()

	if p.Name != "google-contacts" {
		t.Errorf("Name = %q, want %q", p.Name, "google-contacts")
	}
}

func TestGetProvider(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		wantErr  bool
	}{
		{name: "google", provider: "google", wantErr: false},
		{name: "microsoft", provider: "microsoft", wantErr: false},
		{name: "unknown", provider: "unknown", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GetProvider(tt.provider)
			if tt.wantErr && err == nil {
				t.Errorf("GetProvider(%q) = nil error, want error", tt.provider)
				return
			}
			if !tt.wantErr && err != nil {
				t.Errorf("GetProvider(%q) returned error: %v", tt.provider, err)
			}
		})
	}
}

func TestSupportedProviders(t *testing.T) {
	providers := SupportedProviders()

	if len(providers) != 2 {
		t.Fatalf("SupportedProviders() returned %d providers, want 2", len(providers))
	}

	want := map[string]bool{"google": true, "microsoft": true}
	for _, p := range providers {
		if !want[p] {
			t.Errorf("unexpected provider %q in SupportedProviders()", p)
		}
	}
}
