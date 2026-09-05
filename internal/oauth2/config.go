// Package oauth2 provides OAuth2 authentication for email providers
package oauth2

// Public client IDs have source defaults for reproducible official builds.
// Development builds may override them through ldflags, for example:
//
//	go build -ldflags "-X 'github.com/hkdb/aerion/internal/oauth2.GoogleClientID=xxx'"
//
// Microsoft has no client secret. Google Desktop clients may require the
// value Google calls client_secret; it remains non-confidential desktop
// configuration. ProviderConfig also supports custom provider credentials.
var (
	// GoogleClientID is the OAuth2 client ID for Google/Gmail (Mail-scoped project).
	// Same client also backs first-party extensions' Google flows for any scopes
	// listed in the extension manifest's first_party_uses_core_for_scopes (today:
	// contacts.readonly). When that's not enough (write scopes, full Calendar),
	GoogleClientID     string
	GoogleClientSecret string

	// MicrosoftClientID is the OAuth2 client ID for Microsoft/Outlook
	// (Mail-scoped registration). Also serves microsoft-contacts and
	// microsoft-calendar — Microsoft Graph doesn't gate scopes behind
	// verification, so one app registration covers all three surfaces.
	MicrosoftClientID string
)

func init() {
	if GoogleClientID == "" {
		GoogleClientID = DefaultGoogleClientID
	}
	if GoogleClientSecret == "" {
		GoogleClientSecret = DefaultGoogleClientSecret
	}
	if MicrosoftClientID == "" {
		MicrosoftClientID = DefaultMicrosoftClientID
	}
}

// IsGoogleConfigured returns true if Google OAuth credentials are
// available from ANY configured source — user override (Settings → OAuth
// Credentials), a user-set slot alias, or the shipped build-time vars.
// Routed through the resolver so a from-source build with empty
// build-time creds but a user override saved in the UI still passes the
// pre-flight check at the start of the OAuth flow.
func IsGoogleConfigured() bool {
	creds, ok := ClientConfigForID("google-mail")
	return ok && creds.ClientID != "" && creds.ClientSecret != ""
}

// IsMicrosoftConfigured mirrors IsGoogleConfigured for Microsoft.
func IsMicrosoftConfigured() bool {
	creds, ok := ClientConfigForID("microsoft-mail")
	return ok && creds.ClientID != ""
}

// IsProviderConfigured returns true if the specified provider has OAuth credentials
func IsProviderConfigured(provider string) bool {
	switch provider {
	case "google":
		return IsGoogleConfigured()
	case "microsoft":
		return IsMicrosoftConfigured()
	default:
		return false
	}
}
