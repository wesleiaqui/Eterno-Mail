package oauth2

// coreProvider is Aerion core's CredentialsProvider. It owns every slot:
//
//   - `google-mail`, `google-contacts`, `google-calendar` — the public
//     Google Desktop client (GoogleClientID). Whether its granted scopes are
//     sufficient for an extension is decided by the Google project, never by
//     silently distributing a separate testing client.
//   - `microsoft-mail` / `microsoft-contacts` / `microsoft-calendar` — all
//     three resolve to `MicrosoftClientID`. Microsoft Graph doesn't gate
//     scopes behind verification the way Google does, so a single Azure AD
//     app registration covers Mail + Contacts + Calendar.
//
// No per-extension OAuth credentials live in the extension packages — source
// defaults and optional development environment overrides consolidate here.
// Extensions stay focused on domain logic.
//
// Registered automatically at package init.
type coreProvider struct{}

func (coreProvider) Lookup(configID string) (ClientCredentials, bool) {
	switch configID {
	case "google-mail":
		if GoogleClientID == "" {
			return ClientCredentials{}, false
		}
		return ClientCredentials{ClientID: GoogleClientID, ClientSecret: GoogleClientSecret}, true
	case "google-contacts", "google-calendar":
		if GoogleClientID == "" {
			return ClientCredentials{}, false
		}
		return ClientCredentials{ClientID: GoogleClientID, ClientSecret: GoogleClientSecret}, true
	case "microsoft-mail", "microsoft-contacts", "microsoft-calendar":
		if MicrosoftClientID == "" {
			return ClientCredentials{}, false
		}
		// Microsoft desktop apps omit the client secret (uses PKCE).
		return ClientCredentials{ClientID: MicrosoftClientID, ClientSecret: ""}, true
	default:
		return ClientCredentials{}, false
	}
}

func init() {
	RegisterCredentialsProvider(coreProvider{})
}
