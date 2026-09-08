package core

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	officialCatalogURL = "https://api.github.com/repos/MetaCubeX/mihomo/releases?per_page=10"
	maxCatalogBytes    = 8 << 20
	maxArchiveBytes    = 256 << 20
)

type githubRelease struct {
	TagName     string        `json:"tag_name"`
	PublishedAt string        `json:"published_at"`
	Draft       bool          `json:"draft"`
	Prerelease  bool          `json:"prerelease"`
	Assets      []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Digest             string `json:"digest"`
	Size               int64  `json:"size"`
}

func (m *VersionManager) fetchCatalog(ctx context.Context) ([]releaseAsset, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, m.catalogURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create mihomo release request: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	request.Header.Set("User-Agent", "Jeemi-mihomo-version-manager")

	response, err := m.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("request mihomo releases: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4<<10))
		return nil, fmt.Errorf("request mihomo releases: unexpected HTTP status %d", response.StatusCode)
	}

	limited := io.LimitReader(response.Body, maxCatalogBytes+1)
	contents, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read mihomo releases: %w", err)
	}
	if len(contents) > maxCatalogBytes {
		return nil, fmt.Errorf("mihomo release response exceeds size limit")
	}

	var releases []githubRelease
	if err := json.Unmarshal(contents, &releases); err != nil {
		return nil, fmt.Errorf("parse mihomo releases: %w", err)
	}

	assets := make([]releaseAsset, 0, len(releases))
	for _, release := range releases {
		if release.Draft || release.Prerelease || !stableVersionPattern.MatchString(release.TagName) {
			continue
		}
		expectedName, err := m.target.AssetName(release.TagName)
		if err != nil {
			continue
		}
		for _, asset := range release.Assets {
			if asset.Name != expectedName || asset.Size <= 0 || asset.Size > maxArchiveBytes {
				continue
			}
			digest, ok := strings.CutPrefix(strings.ToLower(asset.Digest), "sha256:")
			if !ok || !isSHA256(digest) {
				continue
			}
			if err := m.validateAssetURL(release.TagName, asset); err != nil {
				continue
			}
			assets = append(assets, releaseAsset{
				Version:     release.TagName,
				PublishedAt: release.PublishedAt,
				Name:        asset.Name,
				URL:         asset.BrowserDownloadURL,
				Size:        asset.Size,
				SHA256:      digest,
			})
			break
		}
	}
	if len(assets) == 0 {
		return nil, fmt.Errorf("no verified mihomo assets match target %s", m.target.ID)
	}
	return assets, nil
}

func (m *VersionManager) validateAssetURL(version string, asset githubAsset) error {
	parsed, err := url.Parse(asset.BrowserDownloadURL)
	if err != nil {
		return err
	}
	if m.catalogURL != officialCatalogURL {
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return fmt.Errorf("unsupported test asset URL scheme")
		}
		return nil
	}
	expected := "https://github.com/MetaCubeX/mihomo/releases/download/" + version + "/" + asset.Name
	if asset.BrowserDownloadURL != expected {
		return fmt.Errorf("asset URL is not the expected official release URL")
	}
	return nil
}

func isSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}
