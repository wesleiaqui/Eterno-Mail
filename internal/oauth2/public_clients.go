package oauth2

// DefaultGoogleClientID, DefaultGoogleClientSecret and
// DefaultMicrosoftClientID configure official Eterno Mail OAuth clients.
// Google labels the Desktop client value a "client_secret", but a distributed
// desktop application cannot keep it confidential. It is not a server secret
// and must not be treated as a password or protected by obfuscation.
//
// Release preparation: replace the two empty values below with the official
// public IDs through a deliberate, reviewed source change. Do not place a
// client secret, user token, authorization code, or PKCE verifier here.
const (
	DefaultGoogleClientID     = "911569498826-15u7nc6c691p1lmm192h60e72tj26otf.apps.googleusercontent.com"
	DefaultGoogleClientSecret = ""
	DefaultMicrosoftClientID  = "3f65c3de-d9e3-4267-8da6-08f93b143547"
)
