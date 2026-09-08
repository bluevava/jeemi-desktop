package geodata

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"jeemi/internal/config/document"
	"jeemi/internal/configfile"
	"jeemi/internal/platform/paths"

	"gopkg.in/yaml.v3"
)

const (
	manifestVersion = 1
	maxAssetBytes   = 192 << 20
)

var sha256Pattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

type Validator interface {
	Validate(context.Context, string, Kind, string) error
}

type ManagerOptions struct {
	DataDirectory string
	HTTPClient    HTTPClient
	Validator     Validator
	Now           func() time.Time
}

type Manager struct {
	directory        string
	revisions        string
	runtimeDirectory string
	httpClient       HTTPClient
	validator        Validator
	now              func() time.Time

	operationMu sync.Mutex
	stateMu     sync.RWMutex
	downloading string
	cancel      context.CancelFunc

	healthMu       sync.Mutex
	healthCacheKey string
	healthCache    Health
}

type manifest struct {
	Version         int               `json:"version"`
	Desired         map[string]string `json:"desired"`
	Active          map[string]string `json:"active"`
	ActiveGeoIPMode string            `json:"activeGeoIpMode"`
	ActiveLoader    string            `json:"activeLoader"`
	LastCheckedAt   string            `json:"lastCheckedAt"`
	AvailableSHA256 map[string]string `json:"availableSha256"`
	AvailableURLs   map[string]string `json:"availableUrls"`
}

func NewManager(options ManagerOptions) (*Manager, error) {
	if strings.TrimSpace(options.DataDirectory) == "" || !filepath.IsAbs(options.DataDirectory) {
		return nil, fmt.Errorf("GEO data directory must be absolute")
	}
	directory, err := paths.GeoDataDirectoryFromRoot(options.DataDirectory)
	if err != nil {
		return nil, err
	}
	runtimeDirectory, err := paths.MihomoRuntimeDirectoryFromRoot(options.DataDirectory)
	if err != nil {
		return nil, err
	}
	client := options.HTTPClient
	if client == nil {
		client = defaultHTTPClient()
	}
	validator := options.Validator
	if validator == nil {
		validator = coreValidator{stagingRoot: directory}
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	manager := &Manager{
		directory: directory, revisions: filepath.Join(directory, "revisions"), runtimeDirectory: runtimeDirectory,
		httpClient: client, validator: validator, now: now,
	}
	if err := manager.adoptExistingRuntimeAssets(); err != nil {
		return nil, err
	}
	return manager, nil
}

func (m *Manager) State(preferences Preferences) (State, error) {
	preferences = Normalize(preferences)
	if err := ValidatePreferences(preferences); err != nil {
		return State{}, err
	}
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	return m.stateLocked(preferences)
}

func (m *Manager) stateLocked(preferences Preferences) (State, error) {
	document, err := m.loadManifest()
	if err != nil {
		return State{}, err
	}
	m.stateMu.RLock()
	downloading := m.downloading
	m.stateMu.RUnlock()
	state := State{
		Directory: m.directory, RuntimeDirectory: m.runtimeDirectory, Preferences: preferences,
		ActiveGeoIPMode: activeMode(document), ActiveLoader: activeLoader(document),
		LastCheckedAt: document.LastCheckedAt, DownloadingKind: downloading,
		RestartRequired: m.pendingLocked(document, preferences), Health: m.healthForDocument(document, preferences),
		Assets: make([]AssetState, 0, len(allKinds())),
	}
	selected := make(map[Kind]struct{}, len(SelectedKinds(preferences)))
	for _, kind := range SelectedKinds(preferences) {
		selected[kind] = struct{}{}
	}
	for _, kind := range allKinds() {
		canonicalName, _ := CanonicalName(kind)
		desiredSHA := document.Desired[string(kind)]
		activeSHA := document.Active[string(kind)]
		availableSHA := ""
		if sourceURL, sourceErr := preferences.URLFor(kind); sourceErr == nil && document.AvailableURLs[string(kind)] == sourceURL {
			availableSHA = document.AvailableSHA256[string(kind)]
		}
		asset := AssetState{
			Kind: kind, CanonicalName: canonicalName, SHA256: desiredSHA, ActiveSHA256: activeSHA,
			AvailableSHA256: availableSHA, Installed: desiredSHA != "",
			Active: desiredSHA != "" && desiredSHA == activeSHA,
		}
		if _, used := selected[kind]; used {
			asset.Pending = desiredSHA != activeSHA
		}
		if desiredSHA != "" {
			revision, readErr := m.readRevision(kind, desiredSHA)
			if readErr != nil {
				asset.Installed = false
				asset.Pending = true
			} else {
				asset.Size = revision.Size
				asset.InstalledAt = revision.InstalledAt
				asset.Source = revision.Source
				asset.SourceURL = revision.SourceURL
				asset.ValidatedCoreVersion = revision.ValidatedCoreVersion
				asset.PublisherVerified = revision.PublisherVerified
			}
		}
		state.Assets = append(state.Assets, asset)
	}
	return state, nil
}

// Health reports whether the selected GeoIP, GeoSite, and ASN assets are
// installed and whether the active copies are structurally intact. It reads
// only atomically published metadata and files, so frequent runtime status
// refreshes do not wait for a long-running download operation.
func (m *Manager) Health(preferences Preferences) (Health, error) {
	preferences = Normalize(preferences)
	if err := ValidatePreferences(preferences); err != nil {
		return Health{Status: HealthInvalid}, err
	}
	document, err := m.loadManifest()
	if err != nil {
		return Health{Status: HealthInvalid}, err
	}
	return m.healthForDocument(document, preferences), nil
}

func (m *Manager) healthForDocument(document manifest, preferences Preferences) Health {
	cacheKey := m.healthSignature(document, preferences)
	m.healthMu.Lock()
	defer m.healthMu.Unlock()
	if cacheKey == m.healthCacheKey && m.healthCache.Status != "" {
		return m.healthCache
	}
	health := m.calculateHealth(document, preferences)
	m.healthCacheKey = cacheKey
	m.healthCache = health
	return health
}

func (m *Manager) calculateHealth(document manifest, preferences Preferences) Health {
	health := Health{
		Status:       HealthReady,
		MissingKinds: []Kind{},
		InvalidKinds: []Kind{},
	}
	pending := activeMode(document) != preferences.GeoIPMode || activeLoader(document) != preferences.Loader
	for _, kind := range SelectedKinds(preferences) {
		desiredSHA := document.Desired[string(kind)]
		if desiredSHA == "" {
			health.MissingKinds = append(health.MissingKinds, kind)
			continue
		}
		if _, err := m.readRevision(kind, desiredSHA); err != nil || !fileMatchesSHA256(m.revisionAssetPath(kind, desiredSHA), desiredSHA) {
			health.InvalidKinds = append(health.InvalidKinds, kind)
			continue
		}

		activeSHA := document.Active[string(kind)]
		if activeSHA == "" {
			pending = true
			continue
		}
		activeRevision, err := m.readRevision(kind, activeSHA)
		if err != nil || !fileMatchesSHA256(m.revisionAssetPath(kind, activeSHA), activeSHA) {
			health.InvalidKinds = append(health.InvalidKinds, kind)
			continue
		}
		name, _ := CanonicalName(kind)
		info, err := os.Lstat(filepath.Join(m.runtimeDirectory, name))
		if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() != activeRevision.Size {
			health.InvalidKinds = append(health.InvalidKinds, kind)
			continue
		}
		if !fileMatchesSHA256(filepath.Join(m.runtimeDirectory, name), activeSHA) {
			health.InvalidKinds = append(health.InvalidKinds, kind)
			continue
		}
		if activeSHA != desiredSHA {
			pending = true
		}
	}

	switch {
	case len(health.InvalidKinds) > 0:
		health.Status = HealthInvalid
	case len(health.MissingKinds) > 0:
		health.Status = HealthMissing
	case pending:
		health.Status = HealthPending
	}
	return health
}

func (m *Manager) healthSignature(document manifest, preferences Preferences) string {
	var signature strings.Builder
	signature.WriteString(preferences.GeoIPMode)
	signature.WriteByte('|')
	signature.WriteString(preferences.Loader)
	for _, kind := range SelectedKinds(preferences) {
		desiredSHA := document.Desired[string(kind)]
		activeSHA := document.Active[string(kind)]
		fmt.Fprintf(&signature, "|%s:%s:%s", kind, desiredSHA, activeSHA)
		if desiredSHA != "" {
			appendFileSignature(&signature, filepath.Join(m.revisions, string(kind), desiredSHA, "metadata.json"))
			appendFileSignature(&signature, m.revisionAssetPath(kind, desiredSHA))
		}
		if activeSHA != "" && activeSHA != desiredSHA {
			appendFileSignature(&signature, filepath.Join(m.revisions, string(kind), activeSHA, "metadata.json"))
			appendFileSignature(&signature, m.revisionAssetPath(kind, activeSHA))
		}
		name, _ := CanonicalName(kind)
		appendFileSignature(&signature, filepath.Join(m.runtimeDirectory, name))
	}
	return signature.String()
}

func appendFileSignature(destination *strings.Builder, path string) {
	info, err := os.Lstat(path)
	if err != nil {
		fmt.Fprintf(destination, "|missing:%T", err)
		return
	}
	fmt.Fprintf(destination, "|%d:%d:%d", info.Size(), info.ModTime().UnixNano(), info.Mode())
}

func fileMatchesSHA256(path, expected string) bool {
	actual, err := fileSHA256(path)
	return err == nil && actual == expected
}

func (m *Manager) DesiredFingerprint(preferences Preferences) (string, error) {
	preferences = Normalize(preferences)
	if err := ValidatePreferences(preferences); err != nil {
		return "", err
	}
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	document, err := m.loadManifest()
	if err != nil {
		return "", err
	}
	assets := make(map[string]string, 3)
	for _, kind := range SelectedKinds(preferences) {
		assets[string(kind)] = document.Desired[string(kind)]
	}
	identity := struct {
		GeoIPMode string            `json:"geoIpMode"`
		Loader    string            `json:"loader"`
		Assets    map[string]string `json:"assets"`
	}{GeoIPMode: preferences.GeoIPMode, Loader: preferences.Loader, Assets: assets}
	contents, err := json.Marshal(identity)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(contents)
	return hex.EncodeToString(digest[:]), nil
}

func (m *Manager) HasPending(preferences Preferences) (bool, error) {
	preferences = Normalize(preferences)
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	document, err := m.loadManifest()
	if err != nil {
		return false, err
	}
	return m.pendingLocked(document, preferences), nil
}

func (m *Manager) pendingLocked(document manifest, preferences Preferences) bool {
	if activeMode(document) != preferences.GeoIPMode || activeLoader(document) != preferences.Loader {
		return true
	}
	for _, kind := range SelectedKinds(preferences) {
		if document.Desired[string(kind)] != document.Active[string(kind)] {
			return true
		}
		if document.Active[string(kind)] != "" {
			name, _ := CanonicalName(kind)
			info, err := os.Lstat(filepath.Join(m.runtimeDirectory, name))
			if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
				return true
			}
		}
	}
	return false
}

func activeMode(document manifest) string {
	if document.ActiveGeoIPMode == "" {
		return GeoIPModeMMDB
	}
	return document.ActiveGeoIPMode
}

func activeLoader(document manifest) string {
	if document.ActiveLoader == "" {
		return LoaderMemoryConservative
	}
	return document.ActiveLoader
}

func (m *Manager) ValidateDesiredRequirements(configuration []byte, preferences Preferences) error {
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	document, err := m.loadManifest()
	if err != nil {
		return err
	}
	return m.validateRequirements(configuration, preferences, document.Desired, false)
}

func (m *Manager) ValidateActiveRequirements(configuration []byte, preferences Preferences) error {
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	document, err := m.loadManifest()
	if err != nil {
		return err
	}
	return m.validateRequirements(configuration, preferences, document.Active, true)
}

func (m *Manager) validateRequirements(configuration []byte, preferences Preferences, revisions map[string]string, active bool) error {
	requirements, err := requiredKinds(configuration, preferences)
	if err != nil {
		return err
	}
	for _, kind := range requirements {
		sha := revisions[string(kind)]
		if !sha256Pattern.MatchString(sha) {
			return fmt.Errorf("required GEO data asset %s is not installed", kind)
		}
		if active {
			canonicalName, _ := CanonicalName(kind)
			activePath := filepath.Join(m.runtimeDirectory, canonicalName)
			info, statErr := os.Lstat(activePath)
			if statErr != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("required active GEO data asset %s is unavailable", kind)
			}
			actualSHA, digestErr := fileSHA256(activePath)
			if digestErr != nil || actualSHA != sha {
				return fmt.Errorf("required active GEO data asset %s failed integrity verification", kind)
			}
			continue
		}
		if _, err := m.readRevision(kind, sha); err != nil {
			return fmt.Errorf("required GEO data asset %s is unavailable: %w", kind, err)
		}
	}
	return nil
}

func requiredKinds(configuration []byte, preferences Preferences) ([]Kind, error) {
	parsed, err := document.Parse(configuration)
	if err != nil {
		return nil, fmt.Errorf("inspect GEO data requirements: %w", err)
	}
	needed := map[Kind]bool{}
	var visit func(*yaml.Node)
	visit = func(node *yaml.Node) {
		if node == nil {
			return
		}
		if node.Kind == yaml.ScalarNode {
			value := strings.ToUpper(node.Value)
			if strings.Contains(value, "GEOIP,") || strings.Contains(value, "GEOIP:") {
				if preferences.GeoIPMode == GeoIPModeDAT {
					needed[KindGeoIPDAT] = true
				} else {
					needed[KindGeoIPMMDB] = true
				}
			}
			if strings.Contains(value, "GEOSITE,") || strings.Contains(value, "GEOSITE:") {
				needed[KindGeoSite] = true
			}
			if strings.Contains(value, "IP-ASN,") {
				needed[KindASN] = true
			}
		}
		for _, child := range node.Content {
			visit(child)
		}
	}
	visit(parsed)
	result := make([]Kind, 0, len(needed))
	for _, kind := range SelectedKinds(preferences) {
		if needed[kind] {
			result = append(result, kind)
		}
	}
	return result, nil
}

type Activation struct {
	manager  *Manager
	previous manifest
	backup   string
	changed  []Kind
	done     bool
}

func (m *Manager) PrepareActivation(preferences Preferences) (*Activation, error) {
	preferences = Normalize(preferences)
	if err := ValidatePreferences(preferences); err != nil {
		return nil, err
	}
	m.operationMu.Lock()
	document, err := m.loadManifest()
	if err != nil {
		m.operationMu.Unlock()
		return nil, err
	}
	activation := &Activation{manager: m, previous: cloneManifest(document)}
	if !m.pendingLocked(document, preferences) {
		return activation, nil
	}
	if err := os.MkdirAll(m.directory, 0o700); err != nil {
		m.operationMu.Unlock()
		return nil, fmt.Errorf("create GEO data directory: %w", err)
	}
	if err := os.MkdirAll(m.runtimeDirectory, 0o700); err != nil {
		m.operationMu.Unlock()
		return nil, fmt.Errorf("create mihomo runtime directory: %w", err)
	}
	backup, err := os.MkdirTemp(m.directory, ".activation-backup-")
	if err != nil {
		m.operationMu.Unlock()
		return nil, fmt.Errorf("create GEO data activation backup: %w", err)
	}
	activation.backup = backup
	for _, kind := range SelectedKinds(preferences) {
		desiredSHA := document.Desired[string(kind)]
		name, _ := CanonicalName(kind)
		activeInfo, activeErr := os.Lstat(filepath.Join(m.runtimeDirectory, name))
		activeAvailable := activeErr == nil && activeInfo.Mode().IsRegular() && activeInfo.Mode()&os.ModeSymlink == 0
		if desiredSHA == "" || (desiredSHA == document.Active[string(kind)] && activeAvailable) {
			continue
		}
		if _, err := m.readRevision(kind, desiredSHA); err != nil {
			_ = activation.restoreFiles()
			os.RemoveAll(backup)
			m.operationMu.Unlock()
			return nil, err
		}
		if err := m.activateRevision(kind, desiredSHA, backup); err != nil {
			_ = activation.restoreFiles()
			os.RemoveAll(backup)
			m.operationMu.Unlock()
			return nil, err
		}
		activation.changed = append(activation.changed, kind)
		document.Active[string(kind)] = desiredSHA
	}
	document.ActiveGeoIPMode = preferences.GeoIPMode
	document.ActiveLoader = preferences.Loader
	if err := m.saveManifest(document); err != nil {
		_ = activation.restoreFiles()
		os.RemoveAll(backup)
		m.operationMu.Unlock()
		return nil, err
	}
	return activation, nil
}

func (activation *Activation) Commit() error {
	if activation == nil || activation.done {
		return nil
	}
	activation.done = true
	if activation.backup != "" {
		_ = os.RemoveAll(activation.backup)
	}
	activation.manager.operationMu.Unlock()
	return nil
}

func (activation *Activation) Rollback() error {
	if activation == nil || activation.done {
		return nil
	}
	activation.done = true
	fileErr := activation.restoreFiles()
	manifestErr := activation.manager.saveManifest(activation.previous)
	if activation.backup != "" {
		_ = os.RemoveAll(activation.backup)
	}
	activation.manager.operationMu.Unlock()
	if fileErr != nil && manifestErr != nil {
		return fmt.Errorf("restore GEO data files: %v; restore manifest: %w", fileErr, manifestErr)
	}
	if fileErr != nil {
		return fileErr
	}
	return manifestErr
}

func (activation *Activation) restoreFiles() error {
	if activation.backup == "" {
		return nil
	}
	var firstErr error
	for _, kind := range activation.changed {
		name, _ := CanonicalName(kind)
		target := filepath.Join(activation.manager.runtimeDirectory, name)
		backup := filepath.Join(activation.backup, name)
		_ = os.Remove(target)
		if _, err := os.Stat(backup); err == nil {
			if err := os.Rename(backup, target); err != nil && firstErr == nil {
				firstErr = err
			}
		} else if !errors.Is(err, os.ErrNotExist) && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (m *Manager) activateRevision(kind Kind, sha, backupDirectory string) error {
	name, _ := CanonicalName(kind)
	source := m.revisionAssetPath(kind, sha)
	actualSHA, err := fileSHA256(source)
	if err != nil || actualSHA != sha {
		return fmt.Errorf("GEO data revision %s failed integrity verification", kind)
	}
	target := filepath.Join(m.runtimeDirectory, name)
	backup := filepath.Join(backupDirectory, name)
	if info, err := os.Lstat(target); err == nil {
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("active GEO data target %s is not a regular file", name)
		}
		if err := os.Rename(target, backup); err != nil {
			return fmt.Errorf("preserve active GEO data %s: %w", name, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	stage, err := os.CreateTemp(m.runtimeDirectory, ".geo-activate-*")
	if err != nil {
		_ = os.Rename(backup, target)
		return err
	}
	stagePath := stage.Name()
	defer os.Remove(stagePath)
	if err := stage.Chmod(0o600); err != nil {
		stage.Close()
		_ = os.Rename(backup, target)
		return err
	}
	sourceFile, err := os.Open(source)
	if err != nil {
		stage.Close()
		_ = os.Rename(backup, target)
		return err
	}
	_, copyErr := io.Copy(stage, sourceFile)
	closeSourceErr := sourceFile.Close()
	syncErr := stage.Sync()
	closeStageErr := stage.Close()
	if copyErr != nil || closeSourceErr != nil || syncErr != nil || closeStageErr != nil {
		_ = os.Rename(backup, target)
		return fmt.Errorf("stage GEO data %s", name)
	}
	if err := os.Rename(stagePath, target); err != nil {
		_ = os.Rename(backup, target)
		return fmt.Errorf("activate GEO data %s: %w", name, err)
	}
	return nil
}

func (m *Manager) CleanOldRevisions(preferences Preferences) (State, error) {
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	document, err := m.loadManifest()
	if err != nil {
		return State{}, err
	}
	keep := map[string]struct{}{}
	for kind, sha := range document.Desired {
		keep[kind+":"+sha] = struct{}{}
	}
	for kind, sha := range document.Active {
		keep[kind+":"+sha] = struct{}{}
	}
	for _, kind := range allKinds() {
		kindDirectory := filepath.Join(m.revisions, string(kind))
		entries, readErr := os.ReadDir(kindDirectory)
		if errors.Is(readErr, os.ErrNotExist) {
			continue
		}
		if readErr != nil {
			return State{}, readErr
		}
		for _, entry := range entries {
			if !entry.IsDir() || !sha256Pattern.MatchString(entry.Name()) {
				continue
			}
			if _, ok := keep[string(kind)+":"+entry.Name()]; ok {
				continue
			}
			target := filepath.Join(kindDirectory, entry.Name())
			if err := ensureDescendant(target, m.revisions); err != nil {
				return State{}, err
			}
			if err := os.RemoveAll(target); err != nil {
				return State{}, fmt.Errorf("remove old GEO data revision: %w", err)
			}
		}
	}
	return m.stateLocked(preferences)
}

func (m *Manager) Directory() string { return m.directory }

func (m *Manager) loadManifest() (loaded manifest, resultErr error) {
	result := manifest{
		Version: manifestVersion, Desired: map[string]string{}, Active: map[string]string{}, AvailableSHA256: map[string]string{}, AvailableURLs: map[string]string{},
	}
	path := filepath.Join(m.directory, "manifest.json")
	contents, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return manifest{}, configfile.Unreadable(path, fmt.Errorf("read GEO data manifest: %w", err))
	}
	defer func() {
		if resultErr != nil {
			resultErr = configfile.Invalid(path, contents, resultErr)
		}
	}()
	if err := json.Unmarshal(contents, &result); err != nil {
		return manifest{}, fmt.Errorf("parse GEO data manifest: %w", err)
	}
	if result.Version != manifestVersion {
		return manifest{}, fmt.Errorf("GEO data manifest is incompatible")
	}
	if result.Desired == nil {
		result.Desired = map[string]string{}
	}
	if result.Active == nil {
		result.Active = map[string]string{}
	}
	if result.AvailableSHA256 == nil {
		result.AvailableSHA256 = map[string]string{}
	}
	if result.AvailableURLs == nil {
		result.AvailableURLs = map[string]string{}
	}
	for _, values := range []map[string]string{result.Desired, result.Active, result.AvailableSHA256} {
		for kind, sha := range values {
			if !validKind(Kind(kind)) || (sha != "" && !sha256Pattern.MatchString(sha)) {
				return manifest{}, fmt.Errorf("GEO data manifest contains an invalid revision")
			}
		}
	}
	for kind := range result.AvailableURLs {
		if !validKind(Kind(kind)) {
			return manifest{}, fmt.Errorf("GEO data manifest contains an invalid update source")
		}
	}
	if result.ActiveGeoIPMode != "" && result.ActiveGeoIPMode != GeoIPModeMMDB && result.ActiveGeoIPMode != GeoIPModeDAT {
		return manifest{}, fmt.Errorf("GEO data manifest contains an invalid active mode")
	}
	if result.ActiveLoader != "" && result.ActiveLoader != LoaderMemoryConservative && result.ActiveLoader != LoaderStandard {
		return manifest{}, fmt.Errorf("GEO data manifest contains an invalid active loader")
	}
	return result, nil
}

func (m *Manager) saveManifest(document manifest) error {
	if err := os.MkdirAll(m.directory, 0o700); err != nil {
		return fmt.Errorf("create GEO data directory: %w", err)
	}
	contents, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return err
	}
	return replaceFile(filepath.Join(m.directory, "manifest.json"), append(contents, '\n'))
}

func (m *Manager) readRevision(kind Kind, sha string) (Revision, error) {
	if !validKind(kind) || !sha256Pattern.MatchString(sha) {
		return Revision{}, fmt.Errorf("GEO data revision identity is invalid")
	}
	directory := filepath.Join(m.revisions, string(kind), sha)
	if err := ensureDescendant(directory, m.revisions); err != nil {
		return Revision{}, err
	}
	contents, err := os.ReadFile(filepath.Join(directory, "metadata.json"))
	if err != nil {
		return Revision{}, fmt.Errorf("read GEO data revision metadata: %w", err)
	}
	var revision Revision
	if err := json.Unmarshal(contents, &revision); err != nil {
		return Revision{}, fmt.Errorf("parse GEO data revision metadata: %w", err)
	}
	if revision.Kind != kind || revision.SHA256 != sha || revision.Size <= 0 {
		return Revision{}, fmt.Errorf("GEO data revision metadata is invalid")
	}
	asset := m.revisionAssetPath(kind, sha)
	info, err := os.Lstat(asset)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() != revision.Size {
		return Revision{}, fmt.Errorf("GEO data revision file is unavailable")
	}
	return revision, nil
}

func (m *Manager) revisionAssetPath(kind Kind, sha string) string {
	name, _ := CanonicalName(kind)
	return filepath.Join(m.revisions, string(kind), sha, name)
}

func (m *Manager) adoptExistingRuntimeAssets() error {
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	document, err := m.loadManifest()
	if err != nil {
		return err
	}
	changed := false
	for _, kind := range allKinds() {
		if document.Active[string(kind)] != "" {
			continue
		}
		name, _ := CanonicalName(kind)
		path := filepath.Join(m.runtimeDirectory, name)
		info, statErr := os.Lstat(path)
		if errors.Is(statErr, os.ErrNotExist) {
			continue
		}
		if statErr != nil {
			return statErr
		}
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() <= 0 || info.Size() > maxAssetBytes {
			continue
		}
		sha, err := fileSHA256(path)
		if err != nil {
			return err
		}
		revision := Revision{
			Kind: kind, SHA256: sha, Size: info.Size(), InstalledAt: m.now().UTC().Format(time.RFC3339Nano),
			Source: "existing", SourceName: name,
		}
		if err := m.installRevision(path, revision); err != nil {
			return err
		}
		document.Desired[string(kind)] = sha
		document.Active[string(kind)] = sha
		changed = true
	}
	if changed {
		return m.saveManifest(document)
	}
	return nil
}

func (m *Manager) installRevision(sourcePath string, revision Revision) error {
	targetParent := filepath.Join(m.revisions, string(revision.Kind))
	if err := os.MkdirAll(targetParent, 0o700); err != nil {
		return err
	}
	target := filepath.Join(targetParent, revision.SHA256)
	if err := ensureDescendant(target, m.revisions); err != nil {
		return err
	}
	if existing, err := m.readRevision(revision.Kind, revision.SHA256); err == nil && existing.SHA256 == revision.SHA256 {
		return nil
	}
	stage, err := os.MkdirTemp(targetParent, ".revision-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	if err := os.Chmod(stage, 0o700); err != nil {
		return err
	}
	name, _ := CanonicalName(revision.Kind)
	stagedAsset := filepath.Join(stage, name)
	if err := copyFile(sourcePath, stagedAsset, maxAssetBytes); err != nil {
		return err
	}
	stagedSHA, err := fileSHA256(stagedAsset)
	if err != nil || stagedSHA != revision.SHA256 {
		return fmt.Errorf("GEO data revision digest changed during installation")
	}
	metadata, err := json.MarshalIndent(revision, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(stage, "metadata.json"), append(metadata, '\n'), 0o600); err != nil {
		return err
	}
	if _, err := os.Stat(target); err == nil {
		if err := os.RemoveAll(target); err != nil {
			return fmt.Errorf("replace invalid GEO data revision: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Rename(stage, target); err != nil {
		return fmt.Errorf("install GEO data revision: %w", err)
	}
	return nil
}

func cloneManifest(value manifest) manifest {
	result := value
	result.Desired = cloneStringMap(value.Desired)
	result.Active = cloneStringMap(value.Active)
	result.AvailableSHA256 = cloneStringMap(value.AvailableSHA256)
	result.AvailableURLs = cloneStringMap(value.AvailableURLs)
	return result
}

func cloneStringMap(value map[string]string) map[string]string {
	result := make(map[string]string, len(value))
	for key, item := range value {
		result[key] = item
	}
	return result
}

func replaceFile(path string, contents []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".manifest-*.tmp")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(contents); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	backup := path + ".previous"
	_ = os.Remove(backup)
	if _, err := os.Stat(path); err == nil {
		if err := os.Rename(path, backup); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Rename(backup, path)
		return err
	}
	_ = os.Remove(backup)
	return nil
}

func copyFile(sourcePath, destinationPath string, limit int64) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()
	destination, err := os.OpenFile(destinationPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	written, copyErr := io.Copy(destination, io.LimitReader(source, limit+1))
	syncErr := destination.Sync()
	closeErr := destination.Close()
	if copyErr != nil {
		return copyErr
	}
	if written <= 0 || written > limit {
		return fmt.Errorf("GEO data file size is invalid")
	}
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func ensureDescendant(path, root string) error {
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("GEO data path escapes its managed root")
	}
	return nil
}
