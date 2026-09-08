package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"jeemi/internal/platform/paths"
	"jeemi/internal/settings"
)

const (
	InstallationSourceOfficial = "official"
	InstallationSourceManual   = "manual"
	OfficialReleasesURL        = "https://github.com/MetaCubeX/mihomo/releases"
)

type VersionManagerOptions struct {
	DataDirectory string
	GOOS          string
	GOARCH        string
	CatalogURL    string
	HTTPClient    *http.Client
	Now           func() time.Time
	Settings      *settings.Store
}

type VersionManager struct {
	dataDirectory string
	coreDirectory string
	target        PlatformTarget
	catalogURL    string
	httpClient    *http.Client
	settings      *settings.Store
	now           func() time.Time

	operationMu sync.Mutex
	stateMu     sync.RWMutex
	catalog     []releaseAsset
	lastChecked time.Time
	downloading string
	cancel      context.CancelFunc
}

func NewVersionManager(options VersionManagerOptions) (*VersionManager, error) {
	dataDirectory := filepath.Clean(options.DataDirectory)
	if options.DataDirectory == "" || !filepath.IsAbs(dataDirectory) {
		return nil, fmt.Errorf("mihomo data directory must be absolute")
	}

	goos := options.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}
	goarch := options.GOARCH
	if goarch == "" {
		goarch = runtime.GOARCH
	}
	target, err := ResolvePlatformTarget(goos, goarch)
	if err != nil {
		return nil, err
	}
	coreDirectory, err := paths.MihomoDirectoryFromRoot(dataDirectory)
	if err != nil {
		return nil, err
	}
	settingsStore := options.Settings
	if settingsStore == nil {
		settingsPath, err := paths.SettingsFileFromRoot(dataDirectory)
		if err != nil {
			return nil, err
		}
		settingsStore, err = settings.NewStore(settingsPath)
		if err != nil {
			return nil, err
		}
	}

	catalogURL := options.CatalogURL
	if catalogURL == "" {
		catalogURL = officialCatalogURL
	}
	client := options.HTTPClient
	if client == nil {
		client = &http.Client{
			Timeout: 15 * time.Minute,
			CheckRedirect: func(request *http.Request, via []*http.Request) error {
				if request.URL.Scheme != "https" {
					return fmt.Errorf("mihomo download redirect must use HTTPS")
				}
				if len(via) >= 10 {
					return fmt.Errorf("mihomo download exceeded redirect limit")
				}
				return nil
			},
		}
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}

	return &VersionManager{
		dataDirectory: dataDirectory,
		coreDirectory: coreDirectory,
		target:        target,
		catalogURL:    catalogURL,
		httpClient:    client,
		settings:      settingsStore,
		now:           now,
	}, nil
}

func (m *VersionManager) Check(ctx context.Context) (VersionManagerState, error) {
	m.operationMu.Lock()
	defer m.operationMu.Unlock()

	catalog, err := m.fetchCatalog(ctx)
	if err != nil {
		return VersionManagerState{}, err
	}
	m.stateMu.Lock()
	m.catalog = catalog
	m.lastChecked = m.now().UTC()
	m.stateMu.Unlock()
	return m.State()
}

func (m *VersionManager) Download(ctx context.Context, version string) (VersionManagerState, error) {
	if !stableVersionPattern.MatchString(version) {
		return VersionManagerState{}, fmt.Errorf("invalid mihomo version")
	}

	m.operationMu.Lock()
	defer m.operationMu.Unlock()

	downloadContext, cancel := context.WithCancel(ctx)
	m.stateMu.Lock()
	m.downloading = version
	m.cancel = cancel
	m.stateMu.Unlock()
	defer func() {
		cancel()
		m.stateMu.Lock()
		m.downloading = ""
		m.cancel = nil
		m.stateMu.Unlock()
	}()

	installed, err := m.installedVersions()
	if err != nil {
		return VersionManagerState{}, err
	}
	for _, item := range installed {
		if item.Version == version {
			return m.State()
		}
	}

	asset, err := m.findReleaseAsset(downloadContext, version)
	if err != nil {
		return VersionManagerState{}, err
	}
	if err := m.downloadAndInstall(downloadContext, asset); err != nil {
		return VersionManagerState{}, err
	}
	return m.State()
}

func (m *VersionManager) Import(ctx context.Context, sourcePath string) (string, VersionManagerState, error) {
	m.operationMu.Lock()
	defer m.operationMu.Unlock()

	version, err := m.importLocal(ctx, sourcePath)
	if err != nil {
		return "", VersionManagerState{}, err
	}
	state, err := m.State()
	if err != nil {
		return "", VersionManagerState{}, err
	}
	return version, state, nil
}

func (m *VersionManager) CancelDownload() {
	m.stateMu.RLock()
	cancel := m.cancel
	m.stateMu.RUnlock()
	if cancel != nil {
		cancel()
	}
}

func (m *VersionManager) Select(version string) (VersionManagerState, error) {
	if !validInstalledVersion(version) {
		return VersionManagerState{}, fmt.Errorf("invalid mihomo version")
	}
	m.operationMu.Lock()
	defer m.operationMu.Unlock()

	installed, err := m.installedVersions()
	if err != nil {
		return VersionManagerState{}, err
	}
	found := false
	for _, item := range installed {
		if item.Version == version {
			found = true
			break
		}
	}
	if !found {
		return VersionManagerState{}, fmt.Errorf("mihomo version is not installed")
	}
	if _, err := m.settings.SetMihomoVersion(version); err != nil {
		return VersionManagerState{}, err
	}
	return m.State()
}

func (m *VersionManager) Remove(version string) (VersionManagerState, error) {
	if !validInstalledVersion(version) {
		return VersionManagerState{}, fmt.Errorf("invalid mihomo version")
	}
	m.operationMu.Lock()
	defer m.operationMu.Unlock()

	document, err := m.settings.Load()
	if err != nil {
		return VersionManagerState{}, err
	}
	if document.Mihomo.SelectedVersion == version {
		return VersionManagerState{}, fmt.Errorf("selected mihomo version cannot be removed")
	}

	versionDirectory := filepath.Join(m.coreDirectory, version)
	targetDirectory := filepath.Join(versionDirectory, m.target.ID)
	if err := ensureManagedDescendant(targetDirectory, m.coreDirectory); err != nil {
		return VersionManagerState{}, err
	}
	if err := os.RemoveAll(targetDirectory); err != nil {
		return VersionManagerState{}, fmt.Errorf("remove mihomo version: %w", err)
	}
	if entries, readErr := os.ReadDir(versionDirectory); readErr == nil && len(entries) == 0 {
		_ = os.Remove(versionDirectory)
	}
	return m.State()
}

func (m *VersionManager) State() (VersionManagerState, error) {
	document, err := m.settings.Load()
	if err != nil {
		return VersionManagerState{}, err
	}
	installed, err := m.installedVersions()
	if err != nil {
		return VersionManagerState{}, err
	}

	m.stateMu.RLock()
	catalog := append([]releaseAsset(nil), m.catalog...)
	lastChecked := m.lastChecked
	downloading := m.downloading
	m.stateMu.RUnlock()

	installedSet := make(map[string]struct{}, len(installed))
	for index := range installed {
		installed[index].Selected = installed[index].Version == document.Mihomo.SelectedVersion
		installedSet[installed[index].Version] = struct{}{}
	}
	available := make([]AvailableVersion, 0, len(catalog))
	for _, asset := range catalog {
		_, isInstalled := installedSet[asset.Version]
		available = append(available, AvailableVersion{
			Version:     asset.Version,
			PublishedAt: asset.PublishedAt,
			AssetName:   asset.Name,
			ArchiveSize: asset.Size,
			SHA256:      asset.SHA256,
			Installed:   isInstalled,
			Selected:    asset.Version == document.Mihomo.SelectedVersion,
		})
	}

	state := VersionManagerState{
		DataDirectory:      m.dataDirectory,
		CoreDirectory:      m.coreDirectory,
		Target:             m.target.ID,
		SelectedVersion:    document.Mihomo.SelectedVersion,
		DownloadingVersion: downloading,
		InstalledVersions:  installed,
		AvailableVersions:  available,
	}
	if len(available) > 0 {
		state.LatestVersion = available[0].Version
	}
	if !lastChecked.IsZero() {
		state.LastCheckedAt = lastChecked.Format(time.RFC3339)
	}
	return state, nil
}

func (m *VersionManager) CoreDirectory() string {
	return m.coreDirectory
}

func (m *VersionManager) findReleaseAsset(ctx context.Context, version string) (releaseAsset, error) {
	m.stateMu.RLock()
	catalog := append([]releaseAsset(nil), m.catalog...)
	m.stateMu.RUnlock()
	for _, asset := range catalog {
		if asset.Version == version {
			return asset, nil
		}
	}

	fetched, err := m.fetchCatalog(ctx)
	if err != nil {
		return releaseAsset{}, err
	}
	m.stateMu.Lock()
	m.catalog = fetched
	m.lastChecked = m.now().UTC()
	m.stateMu.Unlock()
	for _, asset := range fetched {
		if asset.Version == version {
			return asset, nil
		}
	}
	return releaseAsset{}, fmt.Errorf("mihomo version is not available for target %s", m.target.ID)
}

func (m *VersionManager) installedVersions() ([]InstalledVersion, error) {
	entries, err := os.ReadDir(m.coreDirectory)
	if errors.Is(err, os.ErrNotExist) {
		return []InstalledVersion{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read mihomo core directory: %w", err)
	}

	installed := make([]InstalledVersion, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || !validInstalledVersion(entry.Name()) {
			continue
		}
		targetDirectory := filepath.Join(m.coreDirectory, entry.Name(), m.target.ID)
		metadataContents, err := os.ReadFile(filepath.Join(targetDirectory, "metadata.json"))
		if err != nil {
			continue
		}
		var metadata installationMetadata
		if err := json.Unmarshal(metadataContents, &metadata); err != nil ||
			metadata.Version != entry.Name() ||
			metadata.Target != m.target.ID ||
			!validInstallationAsset(m.target, metadata) ||
			!isSHA256(metadata.ArchiveSHA256) ||
			!isSHA256(metadata.BinarySHA256) {
			continue
		}
		executablePath := filepath.Join(targetDirectory, m.target.ExecutableName)
		info, err := os.Stat(executablePath)
		if err != nil || !info.Mode().IsRegular() || info.Size() != metadata.BinarySize {
			continue
		}
		digest, err := sha256File(executablePath)
		if err != nil || digest != metadata.BinarySHA256 {
			continue
		}
		installed = append(installed, InstalledVersion{
			Version:        metadata.Version,
			Target:         metadata.Target,
			ExecutablePath: executablePath,
			BinarySize:     metadata.BinarySize,
			BinarySHA256:   metadata.BinarySHA256,
			InstalledAt:    metadata.InstalledAt,
			Source:         normalizedInstallationSource(metadata.Source),
		})
	}
	sort.Slice(installed, func(i, j int) bool {
		leftStable := stableVersionPattern.MatchString(installed[i].Version)
		rightStable := stableVersionPattern.MatchString(installed[j].Version)
		if leftStable != rightStable {
			return leftStable
		}
		if leftStable {
			return compareStableVersions(installed[i].Version, installed[j].Version) > 0
		}
		return installed[i].InstalledAt > installed[j].InstalledAt
	})
	return installed, nil
}

func validInstalledVersion(version string) bool {
	return stableVersionPattern.MatchString(version) || unknownVersionPattern.MatchString(version)
}

func normalizedInstallationSource(source string) string {
	if source == InstallationSourceManual {
		return source
	}
	return InstallationSourceOfficial
}

func validInstallationAsset(target PlatformTarget, metadata installationMetadata) bool {
	if filepath.Base(metadata.AssetName) != metadata.AssetName || strings.TrimSpace(metadata.AssetName) == "" {
		return false
	}
	if normalizedInstallationSource(metadata.Source) == InstallationSourceManual {
		return true
	}
	expectedAssetName, err := target.AssetName(metadata.Version)
	return err == nil && metadata.AssetName == expectedAssetName
}

func compareStableVersions(left, right string) int {
	leftParts := strings.Split(strings.TrimPrefix(left, "v"), ".")
	rightParts := strings.Split(strings.TrimPrefix(right, "v"), ".")
	for index := 0; index < 3; index++ {
		leftValue, _ := strconv.Atoi(leftParts[index])
		rightValue, _ := strconv.Atoi(rightParts[index])
		if leftValue < rightValue {
			return -1
		}
		if leftValue > rightValue {
			return 1
		}
	}
	return 0
}

func ensureManagedDescendant(path, root string) error {
	cleanPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	cleanRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(cleanRoot, cleanPath)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("managed path escapes mihomo directory")
	}
	return nil
}
