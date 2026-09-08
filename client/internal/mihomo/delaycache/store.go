// Package delaycache stores the latest proxy delay observations for a
// resolved subscription projection. The cache is derived runtime data: it is
// never written into subscription revisions or final mihomo configurations.
package delaycache

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"jeemi/internal/platform/paths"
)

const (
	documentVersion = 1
	maxEntries      = 10000
	maxNameBytes    = 1024

	StatusSuccess = "success"
	StatusError   = "error"

	SourceManual = "manual"
	SourceMihomo = "mihomo"
)

var (
	subscriptionIDPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)
	revisionIDPattern     = regexp.MustCompile(`^[a-f0-9]{64}$`)
)

type Scope struct {
	NormalizationFingerprint string `json:"normalizationFingerprint"`
	SubscriptionID           string `json:"subscriptionId"`
	SubscriptionRevision     string `json:"subscriptionRevision"`
	LocalConfigID            string `json:"localConfigId"`
	LocalConfigRevision      int    `json:"localConfigRevision"`
	LocalScriptID            string `json:"localScriptId"`
	LocalScriptRevision      int    `json:"localScriptRevision"`
}

type Entry struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	Delay    int    `json:"delay"`
	Source   string `json:"source"`
	TestedAt string `json:"testedAt"`
}

type UpdateEntry struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Delay  int    `json:"delay"`
	Source string `json:"source"`
}

type Update struct {
	Scope   Scope         `json:"scope"`
	Results []UpdateEntry `json:"results"`
}

type Snapshot struct {
	Scope     Scope   `json:"scope"`
	Results   []Entry `json:"results"`
	UpdatedAt string  `json:"updatedAt"`
}

type document struct {
	Version   int     `json:"version"`
	Scope     Scope   `json:"scope"`
	Results   []Entry `json:"results"`
	UpdatedAt string  `json:"updatedAt"`
}

type Store struct {
	mu   sync.Mutex
	root string
	now  func() time.Time
}

func NewStore(dataDirectory string) (*Store, error) {
	root, err := paths.MihomoProxyDelayCacheDirectoryFromRoot(dataDirectory)
	if err != nil {
		return nil, err
	}
	return &Store{root: root, now: time.Now}, nil
}

func (s *Store) Load(scope Scope) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := validateScope(scope); err != nil {
		return Snapshot{}, err
	}
	document, found, err := s.loadUnlocked(scope.SubscriptionID)
	if err != nil {
		return Snapshot{}, err
	}
	if !found || document.Scope != scope {
		return Snapshot{Scope: scope, Results: []Entry{}}, nil
	}
	return snapshotFromDocument(document), nil
}

func (s *Store) Merge(update Update) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := validateScope(update.Scope); err != nil {
		return Snapshot{}, err
	}
	if len(update.Results) == 0 {
		return Snapshot{}, fmt.Errorf("delay cache update is empty")
	}
	if len(update.Results) > maxEntries {
		return Snapshot{}, fmt.Errorf("delay cache update has too many entries")
	}

	current, found, err := s.loadUnlocked(update.Scope.SubscriptionID)
	if err != nil {
		// A cache is disposable derived state. A valid update is allowed to
		// replace a corrupt document instead of blocking future measurements.
		current = document{}
		found = false
	}
	entries := make(map[string]Entry)
	if found && current.Scope == update.Scope {
		for _, entry := range current.Results {
			entries[entry.Name] = entry
		}
	}
	now := s.now().UTC().Format(time.RFC3339Nano)
	for _, candidate := range update.Results {
		entry, err := normalizeUpdate(candidate, now)
		if err != nil {
			return Snapshot{}, err
		}
		entries[entry.Name] = entry
	}
	if len(entries) > maxEntries {
		return Snapshot{}, fmt.Errorf("delay cache has too many entries")
	}
	results := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		results = append(results, entry)
	}
	sort.Slice(results, func(left, right int) bool {
		return results[left].Name < results[right].Name
	})
	next := document{
		Version: documentVersion, Scope: update.Scope,
		Results: results, UpdatedAt: now,
	}
	if err := s.saveUnlocked(next); err != nil {
		return Snapshot{}, err
	}
	return snapshotFromDocument(next), nil
}

func (s *Store) Delete(subscriptionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !subscriptionIDPattern.MatchString(strings.TrimSpace(subscriptionID)) {
		return fmt.Errorf("subscription id is invalid")
	}
	path := s.filePath(subscriptionID)
	if err := ensureManagedFile(path, s.root, subscriptionID); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove proxy delay cache: %w", err)
	}
	_ = os.Remove(path + ".previous")
	return nil
}

func (s *Store) loadUnlocked(subscriptionID string) (document, bool, error) {
	path := s.filePath(subscriptionID)
	if err := ensureManagedFile(path, s.root, subscriptionID); err != nil {
		return document{}, false, err
	}
	contents, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		contents, err = os.ReadFile(path + ".previous")
	}
	if errors.Is(err, os.ErrNotExist) {
		return document{}, false, nil
	}
	if err != nil {
		return document{}, false, fmt.Errorf("read proxy delay cache: %w", err)
	}
	var value document
	if err := json.Unmarshal(contents, &value); err != nil {
		return document{}, false, fmt.Errorf("parse proxy delay cache: %w", err)
	}
	if value.Version != documentVersion {
		return document{}, false, fmt.Errorf("proxy delay cache version is unsupported")
	}
	if err := validateScope(value.Scope); err != nil {
		return document{}, false, err
	}
	if len(value.Results) > maxEntries {
		return document{}, false, fmt.Errorf("proxy delay cache has too many entries")
	}
	for _, entry := range value.Results {
		if err := validateEntry(entry); err != nil {
			return document{}, false, err
		}
	}
	return value, true, nil
}

func (s *Store) saveUnlocked(value document) error {
	if err := os.MkdirAll(s.root, 0o700); err != nil {
		return fmt.Errorf("create proxy delay cache directory: %w", err)
	}
	contents, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode proxy delay cache: %w", err)
	}
	contents = append(contents, '\n')
	temporary, err := os.CreateTemp(s.root, ".proxy-delays-*.tmp")
	if err != nil {
		return fmt.Errorf("create proxy delay cache candidate: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return fmt.Errorf("protect proxy delay cache candidate: %w", err)
	}
	if _, err := temporary.Write(contents); err != nil {
		temporary.Close()
		return fmt.Errorf("write proxy delay cache candidate: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("sync proxy delay cache candidate: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close proxy delay cache candidate: %w", err)
	}
	if err := promoteFile(temporaryPath, s.filePath(value.Scope.SubscriptionID)); err != nil {
		return fmt.Errorf("replace proxy delay cache: %w", err)
	}
	return nil
}

func (s *Store) filePath(subscriptionID string) string {
	return filepath.Join(s.root, subscriptionID+".json")
}

func validateScope(scope Scope) error {
	if scope.NormalizationFingerprint != "" && !revisionIDPattern.MatchString(scope.NormalizationFingerprint) {
		return fmt.Errorf("subscription normalization fingerprint is invalid")
	}
	if !subscriptionIDPattern.MatchString(strings.TrimSpace(scope.SubscriptionID)) {
		return fmt.Errorf("subscription id is invalid")
	}
	if !revisionIDPattern.MatchString(strings.TrimSpace(scope.SubscriptionRevision)) {
		return fmt.Errorf("subscription revision is invalid")
	}
	if scope.LocalConfigID != "" && !subscriptionIDPattern.MatchString(strings.TrimSpace(scope.LocalConfigID)) {
		return fmt.Errorf("local configuration id is invalid")
	}
	if scope.LocalConfigRevision < 0 {
		return fmt.Errorf("local configuration revision is invalid")
	}
	if scope.LocalScriptID != "" && !subscriptionIDPattern.MatchString(strings.TrimSpace(scope.LocalScriptID)) {
		return fmt.Errorf("local script id is invalid")
	}
	if scope.LocalScriptRevision < 0 {
		return fmt.Errorf("local script revision is invalid")
	}
	if scope.LocalConfigID != "" && scope.LocalScriptID != "" {
		return fmt.Errorf("local configuration and local script scopes are mutually exclusive")
	}
	return nil
}

func normalizeUpdate(candidate UpdateEntry, testedAt string) (Entry, error) {
	entry := Entry{
		Name: strings.TrimSpace(candidate.Name), Status: candidate.Status,
		Delay: candidate.Delay, Source: candidate.Source, TestedAt: testedAt,
	}
	if err := validateEntry(entry); err != nil {
		return Entry{}, err
	}
	return entry, nil
}

func validateEntry(entry Entry) error {
	if entry.Name == "" || len(entry.Name) > maxNameBytes || strings.ContainsAny(entry.Name, "\r\n\x00") {
		return fmt.Errorf("proxy delay cache node name is invalid")
	}
	if entry.Status != StatusSuccess && entry.Status != StatusError {
		return fmt.Errorf("proxy delay cache status is invalid")
	}
	if entry.Status == StatusSuccess && (entry.Delay < 1 || entry.Delay > 65535) {
		return fmt.Errorf("proxy delay cache value is invalid")
	}
	if entry.Status == StatusError && entry.Delay != 0 {
		return fmt.Errorf("failed proxy delay cache entry has a value")
	}
	if entry.Source != SourceManual && entry.Source != SourceMihomo {
		return fmt.Errorf("proxy delay cache source is invalid")
	}
	if _, err := time.Parse(time.RFC3339Nano, entry.TestedAt); err != nil {
		return fmt.Errorf("proxy delay cache timestamp is invalid")
	}
	return nil
}

func snapshotFromDocument(value document) Snapshot {
	return Snapshot{
		Scope: value.Scope, Results: append([]Entry{}, value.Results...),
		UpdatedAt: value.UpdatedAt,
	}
}

func ensureManagedFile(path, root, subscriptionID string) error {
	if !subscriptionIDPattern.MatchString(subscriptionID) {
		return fmt.Errorf("subscription id is invalid")
	}
	relative, err := filepath.Rel(root, path)
	if err != nil || relative != subscriptionID+".json" {
		return fmt.Errorf("proxy delay cache path escapes its managed root")
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
