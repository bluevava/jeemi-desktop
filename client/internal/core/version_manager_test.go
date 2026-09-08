package core

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestResolvePlatformTargetUsesExactCompatibleAssetNames(t *testing.T) {
	target, err := ResolvePlatformTarget("windows", "amd64")
	if err != nil {
		t.Fatalf("ResolvePlatformTarget() error = %v", err)
	}
	name, err := target.AssetName("v1.19.30")
	if err != nil {
		t.Fatalf("AssetName() error = %v", err)
	}
	if name != "mihomo-windows-amd64-compatible-v1.19.30.zip" {
		t.Fatalf("AssetName() = %q", name)
	}
	if _, err := target.AssetName("Alpha"); err == nil {
		t.Fatal("AssetName() accepted a non-stable version")
	}
}

func TestVersionManagerChecksDownloadsSelectsAndRemovesVerifiedVersions(t *testing.T) {
	archives := map[string][]byte{
		"v1.19.30": fakeWindowsArchive(t, "v1.19.30"),
		"v1.19.29": fakeWindowsArchive(t, "v1.19.29"),
	}
	server := newReleaseServer(t, archives, nil)
	defer server.Close()
	manager := newTestVersionManager(t, server)

	checked, err := manager.Check(context.Background())
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if checked.LatestVersion != "v1.19.30" || len(checked.AvailableVersions) != 2 {
		t.Fatalf("unexpected checked state: %+v", checked)
	}

	installed, err := manager.Download(context.Background(), "v1.19.30")
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}
	if len(installed.InstalledVersions) != 1 || installed.InstalledVersions[0].Version != "v1.19.30" {
		t.Fatalf("unexpected installed versions: %+v", installed.InstalledVersions)
	}
	wantExecutable := filepath.Join(
		installed.CoreDirectory,
		"v1.19.30",
		"windows-amd64",
		"mihomo.exe",
	)
	if installed.InstalledVersions[0].ExecutablePath != wantExecutable {
		t.Fatalf("ExecutablePath = %q, want %q", installed.InstalledVersions[0].ExecutablePath, wantExecutable)
	}

	selected, err := manager.Select("v1.19.30")
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	if selected.SelectedVersion != "v1.19.30" || !selected.InstalledVersions[0].Selected {
		t.Fatalf("selection was not persisted: %+v", selected)
	}
	if _, err := manager.Remove("v1.19.30"); err == nil {
		t.Fatal("Remove() removed the selected version")
	}

	if _, err := manager.Download(context.Background(), "v1.19.29"); err != nil {
		t.Fatalf("Download(second version) error = %v", err)
	}
	removed, err := manager.Remove("v1.19.29")
	if err != nil {
		t.Fatalf("Remove(unselected) error = %v", err)
	}
	if len(removed.InstalledVersions) != 1 || removed.InstalledVersions[0].Version != "v1.19.30" {
		t.Fatalf("unexpected versions after remove: %+v", removed.InstalledVersions)
	}
}

func TestVersionManagerRejectsDigestMismatchWithoutInstalling(t *testing.T) {
	archives := map[string][]byte{"v1.19.30": fakeWindowsArchive(t, "v1.19.30")}
	digests := map[string]string{"v1.19.30": strings.Repeat("0", 64)}
	server := newReleaseServer(t, archives, digests)
	defer server.Close()
	manager := newTestVersionManager(t, server)

	if _, err := manager.Download(context.Background(), "v1.19.30"); err == nil || !strings.Contains(err.Error(), "SHA-256 mismatch") {
		t.Fatalf("Download() error = %v, want SHA-256 mismatch", err)
	}
	state, err := manager.State()
	if err != nil {
		t.Fatalf("State() error = %v", err)
	}
	if len(state.InstalledVersions) != 0 {
		t.Fatalf("digest mismatch installed a core: %+v", state.InstalledVersions)
	}
	entries, err := os.ReadDir(state.CoreDirectory)
	if err != nil {
		t.Fatalf("ReadDir(core) error = %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".download-") {
			t.Fatalf("temporary download was not cleaned: %s", entry.Name())
		}
	}
}

func TestVersionManagerCanCancelActiveDownload(t *testing.T) {
	archive := fakeWindowsArchive(t, "v1.19.30")
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/releases":
			writeReleaseCatalog(t, response, request.Host, map[string][]byte{"v1.19.30": archive}, nil)
		case "/asset/v1.19.30":
			response.Header().Set("Content-Length", strconv.Itoa(len(archive)))
			response.WriteHeader(http.StatusOK)
			if flusher, ok := response.(http.Flusher); ok {
				flusher.Flush()
			}
			close(started)
			<-request.Context().Done()
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()
	manager := newTestVersionManager(t, server)

	result := make(chan error, 1)
	go func() {
		_, err := manager.Download(context.Background(), "v1.19.30")
		result <- err
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("download did not start")
	}
	manager.CancelDownload()
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("cancelled download returned no error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancelled download did not stop")
	}
}

func TestVersionManagerImportsOfficialArchiveAsManualVersion(t *testing.T) {
	server := newReleaseServer(t, map[string][]byte{}, nil)
	defer server.Close()
	manager := newTestVersionManager(t, server)
	archivePath := filepath.Join(t.TempDir(), "mihomo-download.zip")
	if err := os.WriteFile(archivePath, fakeWindowsArchive(t, "v1.19.30"), 0o600); err != nil {
		t.Fatalf("WriteFile(import) error = %v", err)
	}

	version, state, err := manager.Import(context.Background(), archivePath)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if version != "v1.19.30" {
		t.Fatalf("Import() version = %q", version)
	}
	if len(state.InstalledVersions) != 1 || state.InstalledVersions[0].Source != InstallationSourceManual {
		t.Fatalf("manual import state = %+v", state.InstalledVersions)
	}
	if _, err := manager.Select(version); err != nil {
		t.Fatalf("Select(manual import) error = %v", err)
	}
}

func TestVersionManagerUsesDigestIdentityForUnknownManualExecutable(t *testing.T) {
	server := newReleaseServer(t, map[string][]byte{}, nil)
	defer server.Close()
	manager := newTestVersionManager(t, server)
	executable := append([]byte{'M', 'Z', 0, 0}, bytes.Repeat([]byte{0x5a}, 128)...)
	executablePath := filepath.Join(t.TempDir(), "mihomo.exe")
	if err := os.WriteFile(executablePath, executable, 0o700); err != nil {
		t.Fatalf("WriteFile(import) error = %v", err)
	}
	digest := sha256.Sum256(executable)
	wantVersion := "unknown-" + hex.EncodeToString(digest[:])[:16]

	version, state, err := manager.Import(context.Background(), executablePath)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if version != wantVersion {
		t.Fatalf("Import() version = %q, want %q", version, wantVersion)
	}
	if len(state.InstalledVersions) != 1 || state.InstalledVersions[0].Version != wantVersion {
		t.Fatalf("unknown import state = %+v", state.InstalledVersions)
	}
}

func TestCatalogIgnoresPrereleasesAndAssetsWithoutDigest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/releases" {
			http.NotFound(response, request)
			return
		}
		_ = json.NewEncoder(response).Encode([]map[string]any{
			{
				"tag_name":   "Prerelease-Alpha",
				"prerelease": true,
				"assets":     []map[string]any{},
			},
			{
				"tag_name": "v1.19.30",
				"assets": []map[string]any{{
					"name":                 "mihomo-windows-amd64-compatible-v1.19.30.zip",
					"browser_download_url": "http://" + request.Host + "/asset/v1.19.30",
					"digest":               "",
					"size":                 10,
				}},
			},
		})
	}))
	defer server.Close()
	manager := newTestVersionManager(t, server)

	if _, err := manager.Check(context.Background()); err == nil || !strings.Contains(err.Error(), "no verified") {
		t.Fatalf("Check() error = %v, want no verified assets", err)
	}
}

func newTestVersionManager(t *testing.T, server *httptest.Server) *VersionManager {
	t.Helper()
	manager, err := NewVersionManager(VersionManagerOptions{
		DataDirectory: filepath.Join(t.TempDir(), "jeemi_data"),
		GOOS:          "windows",
		GOARCH:        "amd64",
		CatalogURL:    server.URL + "/releases",
		HTTPClient:    server.Client(),
		Now: func() time.Time {
			return time.Date(2026, 9, 2, 8, 0, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatalf("NewVersionManager() error = %v", err)
	}
	return manager
}

func newReleaseServer(t *testing.T, archives map[string][]byte, digests map[string]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch {
		case request.URL.Path == "/releases":
			writeReleaseCatalog(t, response, request.Host, archives, digests)
		case strings.HasPrefix(request.URL.Path, "/asset/"):
			version := strings.TrimPrefix(request.URL.Path, "/asset/")
			archive, ok := archives[version]
			if !ok {
				http.NotFound(response, request)
				return
			}
			response.Header().Set("Content-Length", strconv.Itoa(len(archive)))
			_, _ = response.Write(archive)
		default:
			http.NotFound(response, request)
		}
	}))
}

func writeReleaseCatalog(t *testing.T, response http.ResponseWriter, host string, archives map[string][]byte, digests map[string]string) {
	t.Helper()
	versions := []string{"v1.19.30", "v1.19.29"}
	releases := make([]map[string]any, 0, len(archives))
	for _, version := range versions {
		archive, ok := archives[version]
		if !ok {
			continue
		}
		digest := sha256.Sum256(archive)
		digestText := hex.EncodeToString(digest[:])
		if override, ok := digests[version]; ok {
			digestText = override
		}
		releases = append(releases, map[string]any{
			"tag_name":     version,
			"published_at": "2026-08-16T10:11:18Z",
			"draft":        false,
			"prerelease":   false,
			"assets": []map[string]any{{
				"name":                 "mihomo-windows-amd64-compatible-" + version + ".zip",
				"browser_download_url": "http://" + host + "/asset/" + version,
				"digest":               "sha256:" + digestText,
				"size":                 len(archive),
			}},
		})
	}
	response.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(response).Encode(releases); err != nil {
		t.Errorf("encode release catalog: %v", err)
	}
}

func fakeWindowsArchive(t *testing.T, version string) []byte {
	t.Helper()
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	file, err := writer.Create("mihomo-windows-amd64-" + version + ".exe")
	if err != nil {
		t.Fatalf("Create(zip file) error = %v", err)
	}
	binary := append([]byte{'M', 'Z', 0, 0}, bytes.Repeat([]byte(version), 32)...)
	if _, err := file.Write(binary); err != nil {
		t.Fatalf("Write(zip file) error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close(zip) error = %v", err)
	}
	return archive.Bytes()
}
