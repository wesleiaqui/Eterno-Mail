// Package senderlogo resolves and caches brand logos for non-personal email senders.
package senderlogo

import (
	"context"
	"database/sql"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/hkdb/aerion/internal/logging"
)

const (
	cacheTTL       = 14 * 24 * time.Hour
	transientTTL   = 10 * time.Minute
	requestTimeout = 4 * time.Second
	maxLogoBytes   = 256 * 1024
	maxConcurrent  = 4
	requestSpacing = 75 * time.Millisecond
)

type failureType string

const (
	failureDefinitive failureType = "definitive"
	failureTransient  failureType = "transient"
)

type SenderLogo struct {
	Domain    string `json:"domain"`
	Data      string `json:"data"`
	MediaType string `json:"mediaType"`
}

type Store struct {
	db     *sql.DB
	client *http.Client
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db, client: &http.Client{
		Timeout: requestTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

// FetchDomainLogo performs the best-effort BIMI → favicon cascade without cache.
func FetchDomainLogo(domain string) (data, mediaType string, ok bool) {
	result := fetchDomainLogo(NewStore(nil).client, domain)
	return result.data, result.mediaType, result.ok
}

// GetLogos resolves a batch cache-first. Definitive misses last 14 days;
// transient failures last just 10 minutes. It never returns an error.
func (s *Store) GetLogos(domains []string) []SenderLogo {
	normalized := normalizeDomains(domains)
	if len(normalized) == 0 {
		return []SenderLogo{}
	}

	cached := s.loadFresh(normalized)
	missing := make([]string, 0, len(normalized))
	for _, domain := range normalized {
		if _, ok := cached[domain]; !ok {
			missing = append(missing, domain)
		}
	}
	resolved := s.resolveBatch(missing)
	s.save(resolved)
	for domain, result := range resolved {
		cached[domain] = result
	}

	logos := make([]SenderLogo, 0, len(normalized))
	for _, domain := range normalized {
		if result := cached[domain]; result.ok {
			logos = append(logos, SenderLogo{Domain: domain, Data: result.data, MediaType: result.mediaType})
		}
	}
	return logos
}

type cacheResult struct {
	data      string
	mediaType string
	ok        bool
	failure   failureType
	source    string
	status    int
}

func (s *Store) loadFresh(domains []string) map[string]cacheResult {
	results := make(map[string]cacheResult)
	if s.db == nil {
		return results
	}
	placeholders := make([]string, len(domains))
	args := make([]any, len(domains))
	for i, domain := range domains {
		placeholders[i], args[i] = "?", domain
	}
	query := `
		SELECT domain, data, media_type, failure_type, fetched_at
		FROM sender_logo_cache WHERE domain IN (` + strings.Join(placeholders, ",") + `)
	`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return results
	}
	defer rows.Close()

	now := time.Now()
	for rows.Next() {
		var domain, data, mediaType, storedFailure string
		var fetchedAt int64
		if rows.Scan(&domain, &data, &mediaType, &storedFailure, &fetchedAt) != nil {
			continue
		}
		failure := failureType(storedFailure)
		ttl := cacheTTL
		if failure == failureTransient {
			ttl = transientTTL
		}
		if time.Unix(fetchedAt, 0).Add(ttl).After(now) {
			results[domain] = cacheResult{data: data, mediaType: mediaType, ok: data != "" && mediaType != "", failure: failure}
		}
	}
	return results
}

func (s *Store) save(results map[string]cacheResult) {
	if s.db == nil || len(results) == 0 {
		return
	}
	tx, err := s.db.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`
		INSERT INTO sender_logo_cache (domain, data, media_type, failure_type, fetched_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(domain) DO UPDATE SET data=excluded.data, media_type=excluded.media_type,
			failure_type=excluded.failure_type, fetched_at=excluded.fetched_at
	`)
	if err != nil {
		return
	}
	defer stmt.Close()
	now := time.Now().Unix()
	for domain, result := range results {
		if _, err := stmt.Exec(domain, result.data, result.mediaType, result.failure, now); err != nil {
			return
		}
	}
	_ = tx.Commit()
}

func (s *Store) resolveBatch(domains []string) map[string]cacheResult {
	results := make(map[string]cacheResult, len(domains))
	if len(domains) == 0 {
		return results
	}

	jobs := make(chan string)
	log := logging.WithComponent("senderlogo")
	var wg sync.WaitGroup
	var mu sync.Mutex
	workers := maxConcurrent
	if len(domains) < workers {
		workers = len(domains)
	}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for domain := range jobs {
				result := fetchDomainLogo(s.client, domain)
				log.Debug().
					Str("domain", domain).Str("source", result.source).Int("status", result.status).
					Str("cache_outcome", cacheOutcome(result)).Msg("Sender logo lookup completed")
				mu.Lock()
				results[domain] = result
				mu.Unlock()
			}
		}()
	}

	ticker := time.NewTicker(requestSpacing)
	defer ticker.Stop()
	for i, domain := range domains {
		if i > 0 {
			<-ticker.C
		}
		jobs <- domain
	}
	close(jobs)
	wg.Wait()
	return results
}

func fetchDomainLogo(client *http.Client, domain string) cacheResult {
	domain = normalizeDomain(domain)
	if domain == "" {
		return cacheResult{failure: failureDefinitive, source: "invalid-domain"}
	}
	logoURL, bimiFailure := lookupBIMILogo(domain)
	if logoURL != "" {
		result := fetchImage(client, logoURL, true)
		result.source = "bimi"
		if result.ok {
			return result
		}
	}
	direct := fetchImage(client, "https://"+domain+"/favicon.ico", false)
	direct.source = "favicon-direct"
	if direct.ok {
		return direct
	}
	google := fetchGoogleFavicon(client, domain)
	if google.ok {
		return google
	}
	if google.failure == failureTransient || direct.failure == failureTransient || bimiFailure == failureTransient {
		google.failure = failureTransient
	} else {
		google.failure = failureDefinitive
	}
	return google
}

func lookupBIMILogo(domain string) (string, failureType) {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	records, err := net.DefaultResolver.LookupTXT(ctx, "default._bimi."+domain)
	if err != nil {
		if dnsErr, ok := err.(*net.DNSError); ok && dnsErr.IsNotFound {
			return "", failureDefinitive
		}
		return "", failureTransient
	}
	for _, record := range records {
		parts := strings.Split(record, ";")
		if len(parts) == 0 || !strings.EqualFold(strings.TrimSpace(parts[0]), "v=BIMI1") {
			continue
		}
		for _, part := range parts[1:] {
			key, value, found := strings.Cut(strings.TrimSpace(part), "=")
			if !found || !strings.EqualFold(strings.TrimSpace(key), "l") {
				continue
			}
			u, err := url.Parse(strings.TrimSpace(value))
			if err == nil && u.Scheme == "https" && u.Host != "" {
				return u.String(), failureDefinitive
			}
		}
	}
	return "", failureDefinitive
}

func fetchGoogleFavicon(client *http.Client, domain string) cacheResult {
	log := logging.WithComponent("senderlogo")
	var result cacheResult
	for attempt, delay := range []time.Duration{0, 200 * time.Millisecond, 600 * time.Millisecond} {
		if delay > 0 {
			time.Sleep(delay)
		}
		result = fetchImage(client, "https://www.google.com/s2/favicons?domain="+url.QueryEscape(domain)+"&sz=128", false)
		result.source = "favicon-google"
		if result.ok || result.failure != failureTransient {
			return result
		}
		if attempt < 2 {
			log.Debug().
				Str("domain", domain).Int("attempt", attempt+1).Int("status", result.status).
				Msg("Retrying transient Google favicon failure")
		}
	}
	return result
}

func fetchImage(client *http.Client, rawURL string, requireSVG bool) cacheResult {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return cacheResult{failure: failureDefinitive}
	}
	resp, err := client.Do(req)
	if err != nil {
		return cacheResult{failure: failureTransient}
	}
	defer resp.Body.Close()
	result := cacheResult{status: resp.StatusCode}
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusRequestTimeout || resp.StatusCode >= http.StatusInternalServerError {
			result.failure = failureTransient
		} else {
			result.failure = failureDefinitive
		}
		return result
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxLogoBytes+1))
	if err != nil || len(body) == 0 || len(body) > maxLogoBytes {
		result.failure = failureTransient
		return result
	}
	mediaType := strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0])
	if !strings.HasPrefix(mediaType, "image/") || (requireSVG && mediaType != "image/svg+xml") {
		result.failure = failureDefinitive
		return result
	}
	result.data, result.mediaType, result.ok = base64.StdEncoding.EncodeToString(body), mediaType, true
	return result
}

func cacheOutcome(result cacheResult) string {
	if result.ok {
		return "success"
	}
	return string(result.failure)
}

func normalizeDomains(domains []string) []string {
	seen := make(map[string]struct{}, len(domains))
	result := make([]string, 0, len(domains))
	for _, domain := range domains {
		domain = normalizeDomain(domain)
		if domain == "" {
			continue
		}
		if _, ok := seen[domain]; ok {
			continue
		}
		seen[domain] = struct{}{}
		result = append(result, domain)
	}
	return result
}

func normalizeDomain(domain string) string {
	domain = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(domain), "."))
	if domain == "" || len(domain) > 253 || net.ParseIP(domain) != nil || !strings.Contains(domain, ".") {
		return ""
	}
	for _, label := range strings.Split(domain, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return ""
		}
		for _, char := range label {
			if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '-' {
				return ""
			}
		}
	}
	return domain
}
