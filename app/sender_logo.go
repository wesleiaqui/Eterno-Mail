package app

import "github.com/hkdb/aerion/internal/senderlogo"

// GetSenderLogos returns cached, best-effort brand logos keyed by email-domain.
// It is intentionally independent from contacts and account-profile photos.
func (a *App) GetSenderLogos(domains []string) ([]senderlogo.SenderLogo, error) {
	if a.db == nil {
		return []senderlogo.SenderLogo{}, nil
	}
	return senderlogo.NewStore(a.db.DB).GetLogos(domains), nil
}
