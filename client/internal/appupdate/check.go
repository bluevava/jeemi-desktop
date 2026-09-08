// Package appupdate manages verified updates from Jeemi's public releases.
package appupdate

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	RepositoryURL    = "https://github.com/bluevava/jeemi-desktop"
	ReleasesURL      = RepositoryURL + "/releases/latest"
	latestReleaseURL = "https://api.github.com/repos/bluevava/jeemi-desktop/releases/latest"
	maxReleaseBytes  = 1 << 20
)

var (
	ErrCheckFailed    = errors.New("jeemi_update_check_failed")
	ErrNoRelease      = errors.New("jeemi_update_no_release")
	ErrRateLimited    = errors.New("jeemi_update_rate_limited")
	ErrInvalidRelease = errors.New("jeemi_update_invalid_release")
	ErrCurrentVersion = errors.New("jeemi_update_current_version_invalid")
	versionPattern    = regexp.MustCompile(`^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)
)

type Result struct {
	CurrentVersion  string `json:"currentVersion"`
	LatestVersion   string `json:"latestVersion"`
	UpdateAvailable bool   `json:"updateAvailable"`
	release         release
}

type asset struct {
	Name   string `json:"name"`
	URL    string `json:"browser_download_url"`
	Digest string `json:"digest"`
	Size   int64  `json:"size"`
}

type release struct {
	TagName    string  `json:"tag_name"`
	Draft      bool    `json:"draft"`
	Prerelease bool    `json:"prerelease"`
	Assets     []asset `json:"assets"`
}

func Check(ctx context.Context, currentVersion string) (Result, error) {
	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= 3 || request.URL.Scheme != "https" || request.URL.Host != "api.github.com" {
				return ErrCheckFailed
			}
			return nil
		},
	}
	return check(ctx, currentVersion, client)
}

func check(ctx context.Context, currentVersion string, client *http.Client) (Result, error) {
	current, ok := parseVersion(currentVersion)
	if !ok {
		return Result{}, ErrCurrentVersion
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, latestReleaseURL, nil)
	if err != nil {
		return Result{}, ErrCheckFailed
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2026-03-10")
	request.Header.Set("User-Agent", "Jeemi-update-check")
	response, err := client.Do(request)
	if err != nil {
		return Result{}, ErrCheckFailed
	}
	defer response.Body.Close()
	switch response.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return Result{}, ErrNoRelease
	case http.StatusForbidden, http.StatusTooManyRequests:
		return Result{}, ErrRateLimited
	default:
		return Result{}, ErrCheckFailed
	}
	contents, err := io.ReadAll(io.LimitReader(response.Body, maxReleaseBytes+1))
	if err != nil {
		return Result{}, ErrCheckFailed
	}
	if len(contents) > maxReleaseBytes {
		return Result{}, ErrInvalidRelease
	}
	var published release
	if err := json.Unmarshal(contents, &published); err != nil {
		return Result{}, ErrInvalidRelease
	}
	latest, ok := parseVersion(published.TagName)
	if !ok || published.Draft || published.Prerelease || latest.prerelease {
		return Result{}, ErrInvalidRelease
	}
	return Result{
		CurrentVersion: currentVersion, LatestVersion: published.TagName,
		UpdateAvailable: newerStableRelease(current, latest),
		release:         published,
	}, nil
}

func (r release) archive(target target) (asset, error) {
	name := "Jeemi-v" + strings.TrimPrefix(r.TagName, "v") + "-" + target.directory + target.extension
	var selected asset
	for _, item := range r.Assets {
		if item.Name != name {
			continue
		}
		if selected.Name != "" {
			return asset{}, ErrInvalidRelease
		}
		selected = item
	}
	if selected.Name == "" {
		return asset{}, ErrAssetUnavailable
	}
	u, err := url.Parse(selected.URL)
	if err != nil || u.Scheme != "https" || u.Host != "github.com" || u.User != nil || u.RawQuery != "" || u.Fragment != "" ||
		u.Path != "/bluevava/jeemi-desktop/releases/download/"+r.TagName+"/"+name ||
		!digestPattern.MatchString(selected.Digest) || selected.Size <= 0 || selected.Size > maxArchiveBytes {
		return asset{}, ErrInvalidRelease
	}
	return selected, nil
}

type version struct {
	parts      [3]uint64
	prerelease bool
}

func parseVersion(value string) (version, bool) {
	if len(value) > 128 {
		return version{}, false
	}
	match := versionPattern.FindStringSubmatch(value)
	if match == nil {
		return version{}, false
	}
	result := version{prerelease: match[4] != ""}
	for index := range result.parts {
		part, err := strconv.ParseUint(match[index+1], 10, 64)
		if err != nil {
			return version{}, false
		}
		result.parts[index] = part
	}
	return result, true
}

// The release endpoint supplies a stable version, so only the installed
// version may have a prerelease suffix. Build metadata does not affect order.
func newerStableRelease(current, latest version) bool {
	for index, part := range latest.parts {
		if part != current.parts[index] {
			return part > current.parts[index]
		}
	}
	return current.prerelease
}
