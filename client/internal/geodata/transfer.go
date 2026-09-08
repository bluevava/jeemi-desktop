package geodata

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

func defaultHTTPClient() HTTPClient {
	return &http.Client{
		Timeout: 15 * time.Minute,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if request.URL.Scheme != "https" || request.URL.Host == "" || request.URL.User != nil {
				return fmt.Errorf("GEO data download redirect must use a safe HTTPS URL")
			}
			if len(via) >= 10 {
				return fmt.Errorf("GEO data download exceeded redirect limit")
			}
			return nil
		},
	}
}

func (m *Manager) Check(ctx context.Context, preferences Preferences) (State, error) {
	preferences = Normalize(preferences)
	if err := ValidatePreferences(preferences); err != nil {
		return State{}, err
	}
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	document, err := m.loadManifest()
	if err != nil {
		return State{}, err
	}
	for _, kind := range SelectedKinds(preferences) {
		sourceURL, _ := preferences.URLFor(kind)
		sha, fetchErr := m.fetchSidecarSHA(ctx, sourceURL)
		if fetchErr != nil {
			if preferences.Source == SourceOfficial {
				return State{}, fetchErr
			}
			delete(document.AvailableSHA256, string(kind))
			delete(document.AvailableURLs, string(kind))
			continue
		}
		document.AvailableSHA256[string(kind)] = sha
		document.AvailableURLs[string(kind)] = sourceURL
	}
	document.LastCheckedAt = m.now().UTC().Format(time.RFC3339Nano)
	if err := m.saveManifest(document); err != nil {
		return State{}, err
	}
	return m.stateLocked(preferences)
}

func (m *Manager) Download(ctx context.Context, kind Kind, preferences Preferences, executablePath, coreVersion string) (State, error) {
	preferences = Normalize(preferences)
	if err := ValidatePreferences(preferences); err != nil {
		return State{}, err
	}
	if !validKind(kind) {
		return State{}, fmt.Errorf("GEO data kind is invalid")
	}
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	downloadContext, cancel := context.WithCancel(ctx)
	m.setDownload(string(kind), cancel)
	defer func() {
		cancel()
		m.setDownload("", nil)
	}()
	if err := m.downloadOne(downloadContext, kind, preferences, executablePath, coreVersion); err != nil {
		return State{}, err
	}
	return m.stateLocked(preferences)
}

func (m *Manager) DownloadAll(ctx context.Context, preferences Preferences, executablePath, coreVersion string) (State, error) {
	preferences = Normalize(preferences)
	if err := ValidatePreferences(preferences); err != nil {
		return State{}, err
	}
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	downloadContext, cancel := context.WithCancel(ctx)
	m.setDownload("all", cancel)
	defer func() {
		cancel()
		m.setDownload("", nil)
	}()
	for _, kind := range SelectedKinds(preferences) {
		m.setDownload(string(kind), cancel)
		if err := m.downloadOne(downloadContext, kind, preferences, executablePath, coreVersion); err != nil {
			return State{}, err
		}
	}
	return m.stateLocked(preferences)
}

func (m *Manager) CancelDownload() {
	m.stateMu.RLock()
	cancel := m.cancel
	m.stateMu.RUnlock()
	if cancel != nil {
		cancel()
	}
}

func (m *Manager) setDownload(kind string, cancel context.CancelFunc) {
	m.stateMu.Lock()
	m.downloading = kind
	m.cancel = cancel
	m.stateMu.Unlock()
}

func (m *Manager) downloadOne(ctx context.Context, kind Kind, preferences Preferences, executablePath, coreVersion string) error {
	sourceURL, err := preferences.URLFor(kind)
	if err != nil {
		return err
	}
	parsed, err := url.Parse(sourceURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return fmt.Errorf("GEO data source must be a safe HTTPS URL")
	}
	expectedSHA := ""
	publisherVerified := false
	if preferences.Source == SourceOfficial {
		expectedSHA, err = m.fetchSidecarSHA(ctx, sourceURL)
		if err != nil {
			return err
		}
		publisherVerified = true
	} else {
		// A custom endpoint may change independently from a previous update
		// check. Fetch its checksum again for this exact URL and use it only
		// when the publisher exposes a valid sidecar.
		if checksum, checksumErr := m.fetchSidecarSHA(ctx, sourceURL); checksumErr == nil {
			expectedSHA = checksum
			publisherVerified = true
		}
	}
	if err := os.MkdirAll(m.directory, 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(m.directory, ".download-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		temporary.Close()
		return err
	}
	request.Header.Set("Accept", "application/octet-stream")
	request.Header.Set("User-Agent", "Jeemi-GEO-data-manager")
	response, err := m.httpClient.Do(request)
	if err != nil {
		temporary.Close()
		return fmt.Errorf("download GEO data: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		temporary.Close()
		return fmt.Errorf("download GEO data: unexpected HTTP status %d", response.StatusCode)
	}
	if response.ContentLength == 0 || response.ContentLength > maxAssetBytes {
		temporary.Close()
		return fmt.Errorf("download GEO data: invalid content size")
	}
	hasher := sha256.New()
	written, copyErr := copyWithContext(ctx, io.MultiWriter(temporary, hasher), response.Body, maxAssetBytes)
	syncErr := temporary.Sync()
	closeErr := temporary.Close()
	if copyErr != nil {
		return fmt.Errorf("download GEO data: %w", copyErr)
	}
	if written <= 0 || syncErr != nil || closeErr != nil {
		return fmt.Errorf("download GEO data: incomplete file")
	}
	sha := hex.EncodeToString(hasher.Sum(nil))
	if expectedSHA != "" && sha != expectedSHA {
		return fmt.Errorf("download GEO data: SHA-256 mismatch")
	}
	if err := m.validator.Validate(ctx, executablePath, kind, temporaryPath); err != nil {
		return fmt.Errorf("validate GEO data with mihomo %s: %w", coreVersion, err)
	}
	revision := Revision{
		Kind: kind, SHA256: sha, Size: written, InstalledAt: m.now().UTC().Format(time.RFC3339Nano),
		Source: preferences.Source, SourceURL: sourceURL, SourceName: filepath.Base(parsed.Path),
		ValidatedCoreVersion: coreVersion, PublisherVerified: publisherVerified,
	}
	if err := m.installRevision(temporaryPath, revision); err != nil {
		return err
	}
	document, err := m.loadManifest()
	if err != nil {
		return err
	}
	document.Desired[string(kind)] = sha
	if publisherVerified {
		document.AvailableSHA256[string(kind)] = sha
		document.AvailableURLs[string(kind)] = sourceURL
	}
	return m.saveManifest(document)
}

func (m *Manager) Import(ctx context.Context, kind Kind, sourcePath, executablePath, coreVersion string, preferences Preferences) (State, error) {
	preferences = Normalize(preferences)
	if err := ValidatePreferences(preferences); err != nil {
		return State{}, err
	}
	if !validKind(kind) {
		return State{}, fmt.Errorf("GEO data kind is invalid")
	}
	clean := filepath.Clean(sourcePath)
	if sourcePath == "" || !filepath.IsAbs(clean) {
		return State{}, fmt.Errorf("GEO data import path must be absolute")
	}
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	info, err := os.Lstat(clean)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() <= 0 || info.Size() > maxAssetBytes {
		return State{}, fmt.Errorf("GEO data import must be a bounded regular file")
	}
	if err := os.MkdirAll(m.directory, 0o700); err != nil {
		return State{}, err
	}
	stagingDirectory, err := os.MkdirTemp(m.directory, ".import-")
	if err != nil {
		return State{}, err
	}
	defer os.RemoveAll(stagingDirectory)
	stagedPath := filepath.Join(stagingDirectory, "candidate")
	if err := copyFile(clean, stagedPath, maxAssetBytes); err != nil {
		return State{}, fmt.Errorf("stage imported GEO data: %w", err)
	}
	stagedInfo, err := os.Stat(stagedPath)
	if err != nil {
		return State{}, err
	}
	if err := m.validator.Validate(ctx, executablePath, kind, stagedPath); err != nil {
		return State{}, fmt.Errorf("validate imported GEO data with mihomo %s: %w", coreVersion, err)
	}
	sha, err := fileSHA256(stagedPath)
	if err != nil {
		return State{}, err
	}
	revision := Revision{
		Kind: kind, SHA256: sha, Size: stagedInfo.Size(), InstalledAt: m.now().UTC().Format(time.RFC3339Nano),
		Source: "import", SourceName: filepath.Base(clean), ValidatedCoreVersion: coreVersion,
	}
	if err := m.installRevision(stagedPath, revision); err != nil {
		return State{}, err
	}
	document, err := m.loadManifest()
	if err != nil {
		return State{}, err
	}
	document.Desired[string(kind)] = sha
	if err := m.saveManifest(document); err != nil {
		return State{}, err
	}
	return m.stateLocked(preferences)
}

func (m *Manager) fetchSidecarSHA(ctx context.Context, sourceURL string) (string, error) {
	checksumURL, err := sidecarURL(sourceURL)
	if err != nil {
		return "", err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, checksumURL, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("Accept", "text/plain")
	request.Header.Set("User-Agent", "Jeemi-GEO-data-manager")
	response, err := m.httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("request GEO data checksum: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("request GEO data checksum: unexpected HTTP status %d", response.StatusCode)
	}
	contents, err := io.ReadAll(io.LimitReader(response.Body, 4097))
	if err != nil {
		return "", err
	}
	if len(contents) > 4096 {
		return "", fmt.Errorf("GEO data checksum response exceeds size limit")
	}
	fields := strings.Fields(strings.ToLower(string(contents)))
	if len(fields) == 0 || !sha256Pattern.MatchString(fields[0]) {
		return "", fmt.Errorf("GEO data checksum is invalid")
	}
	return fields[0], nil
}

func sidecarURL(sourceURL string) (string, error) {
	parsed, err := url.Parse(sourceURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return "", fmt.Errorf("GEO data checksum source must be a safe HTTPS URL")
	}
	parsed.Path += ".sha256sum"
	parsed.RawPath = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func copyWithContext(ctx context.Context, destination io.Writer, source io.Reader, limit int64) (int64, error) {
	buffer := make([]byte, 128<<10)
	var written int64
	for {
		if err := ctx.Err(); err != nil {
			return written, err
		}
		count, readErr := source.Read(buffer)
		if count > 0 {
			written += int64(count)
			if written > limit {
				return written, fmt.Errorf("content exceeds size limit")
			}
			if _, err := destination.Write(buffer[:count]); err != nil {
				return written, err
			}
		}
		if errors.Is(readErr, io.EOF) {
			return written, nil
		}
		if readErr != nil {
			return written, readErr
		}
	}
}
