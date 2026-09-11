// Package externalui manages Jeemi's local zashboard releases. It never starts
// a core, changes system networking, or accepts a download URL from the UI.
package externalui

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

const RepositoryURL = "https://github.com/Zephyruso/zashboard"
const officialAPI = "https://api.github.com/repos/Zephyruso/zashboard/releases"
const RuntimeDirectory = "external-ui/zashboard"
const maxArchiveBytes int64 = 64 << 20

var versionPattern = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

func ValidVersion(version string) bool {
	return len(version) < 64 && versionPattern.MatchString(version)
}

func RuntimePath(version string) string { return RuntimeDirectory + "/" + version + "/dist" }
func ArchiveURL(version string) string {
	return RepositoryURL + "/releases/download/" + version + "/dist.zip"
}
func Newer(left, right string) bool {
	if !ValidVersion(left) {
		return false
	}
	if !ValidVersion(right) {
		return true
	}
	l, r := strings.Split(left[1:], "."), strings.Split(right[1:], ".")
	for i := range l {
		if len(l[i]) != len(r[i]) {
			return len(l[i]) > len(r[i])
		}
		if l[i] != r[i] {
			return l[i] > r[i]
		}
	}
	return false
}

type Release struct {
	Version     string `json:"version"`
	PublishedAt string `json:"publishedAt"`
	Size        int64  `json:"size"`
	SHA256      string `json:"sha256"`
	address     string
}

type githubRelease struct {
	Tag         string `json:"tag_name"`
	PublishedAt string `json:"published_at"`
	Draft       bool   `json:"draft"`
	Prerelease  bool   `json:"prerelease"`
	Assets      []struct {
		Name   string `json:"name"`
		Size   int64  `json:"size"`
		Digest string `json:"digest"`
		URL    string `json:"browser_download_url"`
	} `json:"assets"`
}

func (m *Manager) request(ctx context.Context, address string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, fmt.Errorf("invalid zashboard request")
	}
	req.Header.Set("User-Agent", "Jeemi-zashboard-manager")
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	response, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("zashboard request failed (check network or retry)")
	}
	if response.StatusCode != http.StatusOK {
		response.Body.Close()
		return nil, fmt.Errorf("zashboard request returned HTTP %d", response.StatusCode)
	}
	return response, nil
}

func (m *Manager) fetch(ctx context.Context, suffix string) ([]Release, error) {
	response, err := m.request(ctx, m.api+suffix)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, (4<<20)+1))
	if err != nil || len(data) > 4<<20 {
		return nil, fmt.Errorf("invalid zashboard release response")
	}
	var releases []githubRelease
	if strings.HasPrefix(suffix, "/") {
		var release githubRelease
		err = json.Unmarshal(data, &release)
		releases = append(releases, release)
	} else {
		err = json.Unmarshal(data, &releases)
	}
	if err != nil {
		return nil, fmt.Errorf("invalid zashboard release response")
	}
	result := []Release{}
	for _, release := range releases {
		if release.Draft || release.Prerelease || !ValidVersion(release.Tag) {
			continue
		}
		for _, asset := range release.Assets {
			digest, ok := strings.CutPrefix(asset.Digest, "sha256:")
			decoded, decodeErr := hex.DecodeString(digest)
			if asset.Name != "dist.zip" || asset.Size <= 0 || asset.Size > maxArchiveBytes || !ok || decodeErr != nil || len(decoded) != 32 {
				continue
			}
			if asset.URL != ArchiveURL(release.Tag) {
				continue
			}
			result = append(result, Release{Version: release.Tag, PublishedAt: release.PublishedAt, Size: asset.Size, SHA256: strings.ToLower(digest), address: asset.URL})
			break
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("no verified zashboard dist.zip release is available")
	}
	return result, nil
}
