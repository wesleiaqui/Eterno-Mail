package sync

import (
	"bytes"
	"net/mail"
	"strings"

	"github.com/hkdb/aerion/internal/message"
)

// classifyInboxCategory uses standard RFC 5322 and widely deployed mailing-list
// headers. It deliberately stores only the conclusion, never the header values.
// Header signals take precedence over the lightweight sender fallback in the UI.
func classifyInboxCategory(raw []byte, m *message.Message) string {
	if len(raw) == 0 {
		return ""
	}
	parsed, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return ""
	}
	h := parsed.Header
	has := func(keys ...string) bool {
		for _, key := range keys {
			if strings.TrimSpace(h.Get(key)) != "" {
				return true
			}
		}
		return false
	}
	lower := func(key string) string { return strings.ToLower(strings.TrimSpace(h.Get(key))) }

	// Mailing list headers are the strongest available newsletter signal.
	if has("List-Unsubscribe", "List-Id", "List-Post", "List-Help", "List-Archive", "Mailing-List", "X-Mailman-Version", "X-Campaign-ID", "X-Mailchimp-Campaign-ID", "X-SFMC-Job") {
		return "news"
	}
	if precedence := lower("Precedence"); precedence == "bulk" || precedence == "list" || precedence == "junk" {
		return "news"
	}

	// These headers identify generated messages and delivery/system notices.
	if auto := lower("Auto-Submitted"); auto != "" && auto != "no" {
		return "notifications"
	}
	if has("X-Auto-Response-Suppress", "X-Autoreply", "X-Auto-Reply", "X-Loop", "X-BeenThere", "X-Failed-Recipients", "X-Postfix-Queue-ID", "X-Mailer-Daemon") {
		return "notifications"
	}

	// A distinct Sender/Reply-To is common for transactional and campaign mail.
	// It is a deliberately lower-priority signal because legitimate aliases use it.
	if sender := strings.ToLower(strings.TrimSpace(h.Get("Sender"))); sender != "" && !strings.Contains(sender, strings.ToLower(m.FromEmail)) {
		return "commercial"
	}
	if replyTo := strings.ToLower(strings.TrimSpace(m.ReplyTo)); replyTo != "" && !strings.Contains(replyTo, strings.ToLower(m.FromEmail)) {
		return "commercial"
	}

	return "people"
}

// classifyBodyCategory handles common compliance footers when a sender omits
// list headers. It is intentionally narrow: these phrases are strong evidence
// of a commercial or transactional bulk message, not ordinary prose.
func classifyBodyCategory(bodyText, bodyHTML string) string {
	content := strings.ToLower(bodyText + "\n" + bodyHTML)
	commercialSignals := []string{
		"email preferences", "manage your preferences", "unsubscribe", "contact us",
		"you are receiving this email because", "terms of use", "privacy notice",
		"aviso de privacidade", "termo de uso", "termos de uso",
		"você está recebendo esse e-mail porque", "todos os direitos reservados",
	}
	for _, signal := range commercialSignals {
		if strings.Contains(content, signal) {
			return "commercial"
		}
	}
	return ""
}
