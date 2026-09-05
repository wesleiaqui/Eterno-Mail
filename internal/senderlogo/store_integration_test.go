//go:build integration

package senderlogo

import "testing"

func TestFetchKnownDomainLogos(t *testing.T) {
	store := NewStore(nil)
	for _, domain := range []string{
		"google.com",
		"accounts.google.com",
		"dekudeals.com",
		"carrefour.com",
		"gigamaisfibra.com.br",
		"email.openai.com",
	} {
		result := fetchDomainLogo(store.strictClient, store.normalClient, domain)
		t.Logf("domain=%s found=%t source=%s media_type=%s data_length=%d status=%d", domain, result.ok, result.source, result.mediaType, len(result.data), result.status)
		if result.source == "" {
			t.Errorf("domain=%s returned no resolver source", domain)
		}
		if result.ok && (result.mediaType == "" || len(result.data) == 0) {
			t.Errorf("domain=%s returned an incomplete logo", domain)
		}
	}
}
