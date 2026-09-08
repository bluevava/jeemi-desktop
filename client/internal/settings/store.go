package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"jeemi/internal/geodata"
	"jeemi/internal/runtimeconfig"
)

const (
	SelectorDensityLarge        = "large"
	SelectorDensityMedium       = "medium"
	SelectorDensitySmall        = "small"
	DefaultSelectorDensity      = SelectorDensityMedium
	DefaultDelayTestConcurrency = 8
	MaxDelayTestConcurrency     = 50
	ConnectionResetOff          = "off"
	ConnectionResetSelector     = "selector"
	ConnectionResetAll          = "all"
	DefaultConnectionResetMode  = ConnectionResetSelector
	SelectorSortDefault         = "default"
	SelectorSortDelay           = "delay"
	SelectorSortName            = "name"
	DefaultSelectorSortMode     = SelectorSortDefault
	SelectorViewPanel           = "panel"
	SelectorViewTabs            = "tabs"
	DefaultSelectorViewMode     = SelectorViewPanel
)

var subscriptionIDPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

type MihomoSettings struct {
	SelectedVersion string `json:"selectedVersion"`
}

type SubscriptionSettings struct {
	SelectedID           string `json:"selectedId"`
	SelectorDensity      string `json:"selectorDensity"`
	DelayTestConcurrency int    `json:"delayTestConcurrency"`
	ConnectionResetMode  string `json:"connectionResetMode"`
	SelectorSortMode     string `json:"selectorSortMode"`
	SelectorViewMode     string `json:"selectorViewMode"`
}

type Document struct {
	Mihomo        MihomoSettings            `json:"mihomo"`
	Subscriptions SubscriptionSettings      `json:"subscriptions"`
	Runtime       runtimeconfig.Preferences `json:"runtime"`
	GeoData       geodata.Preferences       `json:"geoData"`
}

// Store owns Jeemi's Go-side settings document. Callers edit a full document
// under the mutex so later settings fields do not overwrite each other.
type Store struct {
	mu   sync.Mutex
	path string
}

func NewStore(path string) (*Store, error) {
	if path == "" || !filepath.IsAbs(path) {
		return nil, fmt.Errorf("settings path must be absolute")
	}
	return &Store{path: filepath.Clean(path)}, nil
}

func (s *Store) Load() (Document, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	document, err := s.loadUnlocked()
	if err != nil {
		return Document{}, err
	}
	normalizeDocument(&document)
	return document, nil
}

func (s *Store) SetMihomoVersion(version string) (Document, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	document, err := s.loadUnlocked()
	if err != nil {
		return Document{}, err
	}
	document.Mihomo.SelectedVersion = version
	if err := s.saveUnlocked(document); err != nil {
		return Document{}, err
	}
	return document, nil
}

func (s *Store) SetSelectedSubscription(id string) (Document, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id = strings.TrimSpace(id)
	if id != "" && !subscriptionIDPattern.MatchString(id) {
		return Document{}, fmt.Errorf("selected subscription id is invalid")
	}
	document, err := s.loadUnlocked()
	if err != nil {
		return Document{}, err
	}
	normalizeDocument(&document)
	document.Subscriptions.SelectedID = id
	if err := s.saveUnlocked(document); err != nil {
		return Document{}, err
	}
	return document, nil
}

func (s *Store) SetSubscriptionPreferences(density string, delayTestConcurrency int, connectionResetMode string) (Document, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !validSelectorDensity(density) {
		return Document{}, fmt.Errorf("selector density is invalid")
	}
	if delayTestConcurrency < 1 || delayTestConcurrency > MaxDelayTestConcurrency {
		return Document{}, fmt.Errorf("delay test concurrency must be between 1 and %d", MaxDelayTestConcurrency)
	}
	if !validConnectionResetMode(connectionResetMode) {
		return Document{}, fmt.Errorf("connection reset mode is invalid")
	}
	document, err := s.loadUnlocked()
	if err != nil {
		return Document{}, err
	}
	normalizeDocument(&document)
	document.Subscriptions.SelectorDensity = density
	document.Subscriptions.DelayTestConcurrency = delayTestConcurrency
	document.Subscriptions.ConnectionResetMode = connectionResetMode
	if err := s.saveUnlocked(document); err != nil {
		return Document{}, err
	}
	return document, nil
}

func (s *Store) SetSubscriptionSelectorDisplayPreferences(sortMode string, viewMode string) (Document, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !validSelectorSortMode(sortMode) {
		return Document{}, fmt.Errorf("selector sort mode is invalid")
	}
	if !validSelectorViewMode(viewMode) {
		return Document{}, fmt.Errorf("selector view mode is invalid")
	}
	document, err := s.loadUnlocked()
	if err != nil {
		return Document{}, err
	}
	normalizeDocument(&document)
	document.Subscriptions.SelectorSortMode = sortMode
	document.Subscriptions.SelectorViewMode = viewMode
	if err := s.saveUnlocked(document); err != nil {
		return Document{}, err
	}
	return document, nil
}

func (s *Store) SetRuntimePreferences(preferences runtimeconfig.Preferences) (Document, error) {
	if err := runtimeconfig.Validate(preferences); err != nil {
		return Document{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	document, err := s.loadUnlocked()
	if err != nil {
		return Document{}, err
	}
	normalizeDocument(&document)
	document.Runtime = preferences
	if err := s.saveUnlocked(document); err != nil {
		return Document{}, err
	}
	return document, nil
}

func (s *Store) SetGeoDataPreferences(preferences geodata.Preferences) (Document, error) {
	preferences = geodata.Normalize(preferences)
	if err := geodata.ValidatePreferences(preferences); err != nil {
		return Document{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	document, err := s.loadUnlocked()
	if err != nil {
		return Document{}, err
	}
	normalizeDocument(&document)
	document.GeoData = preferences
	if err := s.saveUnlocked(document); err != nil {
		return Document{}, err
	}
	return document, nil
}

func (s *Store) loadUnlocked() (Document, error) {
	contents, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return Document{}, nil
	}
	if err != nil {
		return Document{}, fmt.Errorf("read settings: %w", err)
	}

	var document Document
	if err := json.Unmarshal(contents, &document); err != nil {
		return Document{}, fmt.Errorf("parse settings: %w", err)
	}
	normalizeDocument(&document)
	return document, nil
}

func normalizeDocument(document *Document) {
	if !validSelectorDensity(document.Subscriptions.SelectorDensity) {
		document.Subscriptions.SelectorDensity = DefaultSelectorDensity
	}
	if document.Subscriptions.DelayTestConcurrency < 1 ||
		document.Subscriptions.DelayTestConcurrency > MaxDelayTestConcurrency {
		document.Subscriptions.DelayTestConcurrency = DefaultDelayTestConcurrency
	}
	if !validConnectionResetMode(document.Subscriptions.ConnectionResetMode) {
		document.Subscriptions.ConnectionResetMode = DefaultConnectionResetMode
	}
	if !validSelectorSortMode(document.Subscriptions.SelectorSortMode) {
		document.Subscriptions.SelectorSortMode = DefaultSelectorSortMode
	}
	if !validSelectorViewMode(document.Subscriptions.SelectorViewMode) {
		document.Subscriptions.SelectorViewMode = DefaultSelectorViewMode
	}
	document.Runtime = runtimeconfig.Normalize(document.Runtime)
	document.GeoData = geodata.Normalize(document.GeoData)
}

func validSelectorDensity(density string) bool {
	return density == SelectorDensityLarge || density == SelectorDensityMedium || density == SelectorDensitySmall
}

func validConnectionResetMode(mode string) bool {
	return mode == ConnectionResetOff || mode == ConnectionResetSelector || mode == ConnectionResetAll
}

func validSelectorSortMode(mode string) bool {
	return mode == SelectorSortDefault || mode == SelectorSortDelay || mode == SelectorSortName
}

func validSelectorViewMode(mode string) bool {
	return mode == SelectorViewPanel || mode == SelectorViewTabs
}

func (s *Store) saveUnlocked(document Document) error {
	directory := filepath.Dir(s.path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create settings directory: %w", err)
	}

	contents, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return fmt.Errorf("encode settings: %w", err)
	}
	contents = append(contents, '\n')

	temporary, err := os.CreateTemp(directory, ".settings-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary settings: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return fmt.Errorf("protect temporary settings: %w", err)
	}
	if _, err := temporary.Write(contents); err != nil {
		temporary.Close()
		return fmt.Errorf("write temporary settings: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("sync temporary settings: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary settings: %w", err)
	}

	if err := promoteFile(temporaryPath, s.path); err != nil {
		return fmt.Errorf("replace settings: %w", err)
	}
	return nil
}

func promoteFile(stagePath, targetPath string) error {
	backupPath := targetPath + ".previous"
	_ = os.Remove(backupPath)

	if _, err := os.Stat(targetPath); err == nil {
		if err := os.Rename(targetPath, backupPath); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	if err := os.Rename(stagePath, targetPath); err != nil {
		if _, backupErr := os.Stat(backupPath); backupErr == nil {
			_ = os.Rename(backupPath, targetPath)
		}
		return err
	}
	_ = os.Remove(backupPath)
	return nil
}
