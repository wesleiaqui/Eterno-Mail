package app

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hkdb/aerion/internal/account"
	"github.com/hkdb/aerion/internal/contact"
	"github.com/hkdb/aerion/internal/logging"
)

// ============================================================================
// Contact API - Exposed to frontend via Wails bindings
// ============================================================================

// SearchContacts searches for contacts matching the query
// Returns contacts from multiple sources: local database, vCard files, CardDAV, and Google Contacts
func (a *App) SearchContacts(query string, limit int) ([]*contact.Contact, error) {
	log := logging.WithComponent("app")

	// First search local contacts (DB + vCard + CardDAV)
	contacts, err := a.contactStore.Search(query, limit)
	if err != nil {
		return nil, err
	}

	// For OAuth accounts, also search Google Contacts API
	accounts, _ := a.accountStore.List()
	for _, acc := range accounts {
		if acc.AuthType == account.AuthOAuth2 && strings.Contains(acc.IMAPHost, "gmail") {
			// Get valid OAuth token
			tokens, err := a.getValidOAuthToken(acc.ID)
			if err != nil {
				log.Warn().Err(err).Str("accountID", acc.ID).Msg("Failed to get OAuth token for Google Contacts search")
				continue
			}

			// Search Google Contacts
			googleContacts, err := a.googleContactsClient.Search(tokens.AccessToken, query, limit-len(contacts))
			if err != nil {
				log.Warn().Err(err).Str("accountID", acc.ID).Msg("Google Contacts search failed")
				continue
			}

			// Append to results (deduplicate by email)
			existingEmails := make(map[string]bool)
			for _, c := range contacts {
				existingEmails[strings.ToLower(c.Email)] = true
			}

			for _, gc := range googleContacts {
				if !existingEmails[strings.ToLower(gc.Email)] {
					contacts = append(contacts, gc)
					existingEmails[strings.ToLower(gc.Email)] = true
				}
			}
		}
	}

	// Limit results
	if len(contacts) > limit {
		contacts = contacts[:limit]
	}

	return contacts, nil
}

// GetContact returns a single contact by ID
func (a *App) GetContact(id string) (*contact.Contact, error) {
	return a.contactStore.Get(id)
}

// AddContact adds or updates a contact
func (a *App) AddContact(email, displayName string) error {
	return a.contactStore.AddOrUpdate(email, displayName)
}

// DeleteContact deletes a contact
func (a *App) DeleteContact(id string) error {
	return a.contactStore.Delete(id)
}

// ListContacts returns all contacts
func (a *App) ListContacts(limit int) ([]*contact.Contact, error) {
	return a.contactStore.List(limit)
}

// GetContactPhotos returns inline contact photos for the given emails, keyed by
// lowercased email. Used by the message list to render contact profile pictures
// in the avatar slot (opt-in setting). Resolves the whole batch in one query —
// the frontend batches per list load rather than calling per row.
func (a *App) GetContactPhotos(emails []string) ([]contact.ContactPhoto, error) {
	if len(emails) == 0 {
		return []contact.ContactPhoto{}, nil
	}
	return a.contactStore.GetPhotosByEmails(emails)
}

// GetAccountProfilePhotos returns profile photos for authenticated Google mail
// accounts. A Google account's own profile is not part of People connections,
// so it cannot be obtained through GetContactPhotos.
func (a *App) GetAccountProfilePhotos(emails []string) ([]contact.ContactPhoto, error) {
	log := logging.WithComponent("app.account-profile-photo")

	wanted := make(map[string]struct{}, len(emails))
	for _, email := range emails {
		if normalized := strings.ToLower(strings.TrimSpace(email)); normalized != "" {
			wanted[normalized] = struct{}{}
		}
	}
	if len(wanted) == 0 {
		return []contact.ContactPhoto{}, nil
	}

	accounts, err := a.accountStore.List()
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	photos := make([]contact.ContactPhoto, 0, len(wanted))
	seen := make(map[string]struct{}, len(wanted))

	for _, acc := range accounts {
		email := strings.ToLower(strings.TrimSpace(acc.Email))
		if _, ok := wanted[email]; !ok {
			continue
		}
		if _, alreadyAdded := seen[email]; alreadyAdded || acc.AuthType != account.AuthOAuth2 || !strings.Contains(strings.ToLower(acc.IMAPHost), "gmail") {
			continue
		}

		tokens, err := a.getValidOAuthToken(acc.ID)
		if err != nil || tokens == nil || tokens.AccessToken == "" {
			continue
		}

		profileReq, err := http.NewRequest(http.MethodGet, "https://people.googleapis.com/v1/people/me?personFields=photos", nil)
		if err != nil {
			continue
		}
		profileReq.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
		resp, err := client.Do(profileReq)
		if err != nil {
			continue
		}

		var profile struct {
			Photos []struct {
				URL     string `json:"url"`
				Default bool   `json:"default"`
			} `json:"photos"`
		}
		decodeErr := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&profile)
		statusCode := resp.StatusCode
		resp.Body.Close()
		log.Debug().
			Str("accountID", acc.ID).
			Str("email", email).
			Int("status", statusCode).
			Interface("photos", profile.Photos).
			Msg("Google account profile photo response")
		if statusCode != http.StatusOK || decodeErr != nil {
			continue
		}

		for _, profilePhoto := range profile.Photos {
			if profilePhoto.Default || profilePhoto.URL == "" {
				continue
			}
			data, mediaType, ok := contact.FetchInlinePhotoURL(client, profilePhoto.URL)
			if !ok {
				continue
			}
			photos = append(photos, contact.ContactPhoto{Email: email, Data: data, MediaType: mediaType})
			seen[email] = struct{}{}
			break
		}
	}

	return photos, nil
}
