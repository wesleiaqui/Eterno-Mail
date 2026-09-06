package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	latestReleaseEndpoint     = "https://api.github.com/repos/wesleiaqui/EternoMail/releases/latest"
	updateAutoCheckKey        = "update_auto_check"
	updateSkippedVersionKey   = "update_skipped_version"
	updateLastCheckKey        = "update_last_check"
	updateRequestTimeout      = 8 * time.Second
	updateResponseBodyMaxSize = 1 << 20
)

type githubLatestRelease struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	HTMLURL     string `json:"html_url"`
	PublishedAt string `json:"published_at"`
}

type updateCheckPayload struct {
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	Available      bool   `json:"available"`
	ReleaseURL     string `json:"releaseUrl"`
	ReleaseName    string `json:"releaseName"`
	PublishedAt    string `json:"publishedAt"`
}

// CheckForUpdates checks the latest stable GitHub Release without blocking app
// startup. It returns JSON rather than a Go struct so Wails does not need to
// regenerate frontend model classes for this small transport object.
func (a *App) CheckForUpdates() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), updateRequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, latestReleaseEndpoint, nil)
	if err != nil {
		return "", fmt.Errorf("create update request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "Eterno-Mail/"+Version)

	client := &http.Client{Timeout: updateRequestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("check GitHub release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		detail := strings.TrimSpace(string(body))
		if detail == "" {
			detail = http.StatusText(resp.StatusCode)
		}
		return "", fmt.Errorf("GitHub releases returned %d: %s", resp.StatusCode, detail)
	}

	var release githubLatestRelease
	decoder := json.NewDecoder(io.LimitReader(resp.Body, updateResponseBodyMaxSize))
	if err := decoder.Decode(&release); err != nil {
		return "", fmt.Errorf("decode GitHub release: %w", err)
	}

	latestVersion := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(release.TagName, "v"), "V"))
	if latestVersion == "" {
		return "", fmt.Errorf("latest GitHub release has no tag")
	}

	comparison, err := compareReleaseVersions(latestVersion, Version)
	if err != nil {
		return "", err
	}

	payload := updateCheckPayload{
		CurrentVersion: Version,
		LatestVersion:  latestVersion,
		Available:      comparison > 0,
		ReleaseURL:     release.HTMLURL,
		ReleaseName:    release.Name,
		PublishedAt:    release.PublishedAt,
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode update result: %w", err)
	}
	return string(encoded), nil
}

func compareReleaseVersions(left, right string) (int, error) {
	a, err := parseReleaseVersion(left)
	if err != nil {
		return 0, fmt.Errorf("parse latest version %q: %w", left, err)
	}
	b, err := parseReleaseVersion(right)
	if err != nil {
		return 0, fmt.Errorf("parse current version %q: %w", right, err)
	}

	for i := range a {
		if a[i] > b[i] {
			return 1, nil
		}
		if a[i] < b[i] {
			return -1, nil
		}
	}
	return 0, nil
}

func parseReleaseVersion(version string) ([3]int, error) {
	var parsed [3]int
	version = strings.TrimSpace(version)
	version = strings.TrimPrefix(strings.TrimPrefix(version, "v"), "V")
	if index := strings.IndexAny(version, "-+"); index >= 0 {
		version = version[:index]
	}
	parts := strings.Split(version, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return parsed, fmt.Errorf("expected semantic version")
	}

	for i, part := range parts {
		if part == "" {
			return parsed, fmt.Errorf("empty version component")
		}
		value, err := strconv.Atoi(part)
		if err != nil || value < 0 {
			return parsed, fmt.Errorf("invalid version component %q", part)
		}
		parsed[i] = value
	}
	return parsed, nil
}

// Update-check preferences are deliberately stored in the existing settings
// table so they survive upgrades and work the same in native and Flatpak builds.
func (a *App) GetAutoCheckUpdates() (bool, error) {
	value, err := a.settingsStore.Get(updateAutoCheckKey)
	if err != nil {
		return true, err
	}
	if value == "" {
		return true, nil
	}
	return value == "true", nil
}

func (a *App) SetAutoCheckUpdates(enabled bool) error {
	return a.settingsStore.Set(updateAutoCheckKey, strconv.FormatBool(enabled))
}

func (a *App) GetSkippedUpdateVersion() (string, error) {
	return a.settingsStore.Get(updateSkippedVersionKey)
}

func (a *App) SetSkippedUpdateVersion(version string) error {
	return a.settingsStore.Set(updateSkippedVersionKey, strings.TrimSpace(version))
}

func (a *App) GetLastUpdateCheck() (string, error) {
	return a.settingsStore.Get(updateLastCheckKey)
}

func (a *App) SetLastUpdateCheck(checkedAt string) error {
	checkedAt = strings.TrimSpace(checkedAt)
	if checkedAt == "" {
		return a.settingsStore.Set(updateLastCheckKey, "")
	}
	if _, err := time.Parse(time.RFC3339Nano, checkedAt); err != nil {
		return fmt.Errorf("invalid update check timestamp: %w", err)
	}
	return a.settingsStore.Set(updateLastCheckKey, checkedAt)
}
