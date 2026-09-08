package localscript

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	manifestVersion      = 1
	maxNameLength        = 80
	maxDescriptionLength = 500
	maxScriptBytes       = 256 << 10
	maxManifestBytes     = 64 << 10
)

var scriptIDPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

type StoreOptions struct {
	DataDirectory string
	Now           func() time.Time
	NewID         func() (string, error)
}

type Store struct {
	mu    sync.Mutex
	root  string
	now   func() time.Time
	newID func() (string, error)
}

func NewStore(options StoreOptions) (*Store, error) {
	root, err := paths.LocalScriptsDirectoryFromRoot(options.DataDirectory)
	if err != nil {
		return nil, err
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	newID := options.NewID
	if newID == nil {
		newID = randomID
	}
	return &Store{root: root, now: now, newID: newID}, nil
}

func (s *Store) Directory() string { return s.root }

func (s *Store) State() (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stateUnlocked()
}

func (s *Store) Get(id string) (Script, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadUnlocked(id)
}

// Preview validates and normalizes a draft without changing persistent data.
func (s *Store) Preview(input SaveInput) (Script, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.previewUnlocked(input)
}

func (s *Store) Save(input SaveInput) (Script, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	preview, err := s.previewUnlocked(input)
	if err != nil {
		return Script{}, err
	}
	identifier := input.ID
	if identifier == "" {
		identifier, err = s.newID()
		if err != nil {
			return Script{}, fmt.Errorf("create local script id: %w", err)
		}
		if !scriptIDPattern.MatchString(identifier) {
			return Script{}, fmt.Errorf("generated local script id is invalid")
		}
		if _, err := os.Lstat(filepath.Join(s.root, identifier)); err == nil {
			return Script{}, fmt.Errorf("generated local script id already exists")
		} else if !errors.Is(err, os.ErrNotExist) {
			return Script{}, fmt.Errorf("check local script id: %w", err)
		}
	}
	preview.ID = identifier
	manifest := manifestFromScript(preview)
	if err := s.writeUnlocked(manifest, []byte(preview.Contents)); err != nil {
		return Script{}, err
	}
	return preview, nil
}

func (s *Store) Delete(id string) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.loadUnlocked(id); err != nil {
		return State{}, err
	}
	target := filepath.Join(s.root, id)
	if err := ensureManagedPath(target, s.root); err != nil {
		return State{}, err
	}
	tombstone := filepath.Join(s.root, ".deleted-"+id)
	_ = os.RemoveAll(tombstone)
	if err := os.Rename(target, tombstone); err != nil {
		return State{}, fmt.Errorf("stage local script deletion: %w", err)
	}
	if err := os.RemoveAll(tombstone); err != nil {
		_ = os.Rename(tombstone, target)
		return State{}, fmt.Errorf("delete local script: %w", err)
	}
	return s.stateUnlocked()
}

func (s *Store) previewUnlocked(input SaveInput) (Script, error) {
	name := strings.TrimSpace(input.Name)
	description := strings.TrimSpace(input.Description)
	contents := normalizeSource(input.Contents)
	if name == "" || len([]rune(name)) > maxNameLength {
		return Script{}, fmt.Errorf("local script name must contain 1 to %d characters", maxNameLength)
	}
	if len([]rune(description)) > maxDescriptionLength {
		return Script{}, fmt.Errorf("local script description exceeds %d characters", maxDescriptionLength)
	}
	if len(contents) == 0 || len(contents) > maxScriptBytes {
		return Script{}, fmt.Errorf("local script must contain 1 to %d bytes", maxScriptBytes)
	}
	if err := ValidateSource(contents); err != nil {
		return Script{}, err
	}
	now := s.now().UTC()
	createdAt := now
	revision := 1
	if input.ID != "" {
		if !scriptIDPattern.MatchString(input.ID) {
			return Script{}, fmt.Errorf("invalid local script id")
		}
		existing, err := s.loadUnlocked(input.ID)
		if err != nil {
			return Script{}, err
		}
		createdAt, err = time.Parse(time.RFC3339Nano, existing.CreatedAt)
		if err != nil {
			return Script{}, fmt.Errorf("stored local script creation time is invalid")
		}
		revision = existing.Revision + 1
	}
	return Script{Summary: Summary{
		ID: input.ID, Name: name, Description: description, Revision: revision,
		LineCount: sourceLineCount(contents), SizeBytes: int64(len([]byte(contents))),
		CreatedAt: createdAt.UTC().Format(time.RFC3339Nano), UpdatedAt: now.Format(time.RFC3339Nano),
	}, Contents: contents}, nil
}

func (s *Store) stateUnlocked() (State, error) {
	entries, err := os.ReadDir(s.root)
	if errors.Is(err, os.ErrNotExist) {
		return State{Directory: s.root, Scripts: []Summary{}}, nil
	}
	if err != nil {
		return State{}, fmt.Errorf("read local script directory: %w", err)
	}
	items := make([]Summary, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || !scriptIDPattern.MatchString(entry.Name()) {
			continue
		}
		script, err := s.loadUnlocked(entry.Name())
		if err != nil {
			return State{}, err
		}
		items = append(items, script.Summary)
	}
	sort.Slice(items, func(left, right int) bool {
		if items[left].UpdatedAt == items[right].UpdatedAt {
			return items[left].Name < items[right].Name
		}
		return items[left].UpdatedAt > items[right].UpdatedAt
	})
	return State{Directory: s.root, Scripts: items}, nil
}

func (s *Store) loadUnlocked(id string) (Script, error) {
	if !scriptIDPattern.MatchString(strings.TrimSpace(id)) {
		return Script{}, fmt.Errorf("invalid local script id")
	}
	directory := filepath.Join(s.root, id)
	if err := ensureManagedDirectory(directory, s.root); err != nil {
		return Script{}, err
	}
	manifestBytes, err := readLimitedFile(filepath.Join(directory, "manifest.json"), maxManifestBytes)
	if err != nil {
		return Script{}, fmt.Errorf("read local script manifest: %w", err)
	}
	var manifest manifestDocument
	if err := decodeStrictJSON(manifestBytes, &manifest); err != nil {
		return Script{}, fmt.Errorf("parse local script manifest: %w", err)
	}
	contents, err := readLimitedFile(filepath.Join(directory, "script.js"), maxScriptBytes)
	if err != nil {
		return Script{}, fmt.Errorf("read local script source: %w", err)
	}
	if err := validateStored(manifest, id, contents); err != nil {
		return Script{}, err
	}
	return Script{Summary: summaryFromManifest(manifest), Contents: string(contents)}, nil
}

func (s *Store) writeUnlocked(manifest manifestDocument, contents []byte) error {
	if err := os.MkdirAll(s.root, 0o700); err != nil {
		return fmt.Errorf("create local script root: %w", err)
	}
	stage, err := os.MkdirTemp(s.root, ".local-script-stage-")
	if err != nil {
		return fmt.Errorf("create local script stage: %w", err)
	}
	defer os.RemoveAll(stage)
	if err := os.Chmod(stage, 0o700); err != nil {
		return fmt.Errorf("protect local script stage: %w", err)
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode local script manifest: %w", err)
	}
	if err := writeSyncedFile(filepath.Join(stage, "manifest.json"), append(manifestBytes, '\n')); err != nil {
		return err
	}
	if err := writeSyncedFile(filepath.Join(stage, "script.js"), contents); err != nil {
		return err
	}
	target := filepath.Join(s.root, manifest.ID)
	if err := ensureManagedPath(target, s.root); err != nil {
		return err
	}
	backup := target + ".previous"
	_ = os.RemoveAll(backup)
	if _, err := os.Stat(target); err == nil {
		if err := os.Rename(target, backup); err != nil {
			return fmt.Errorf("stage previous local script: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Rename(stage, target); err != nil {
		if _, backupErr := os.Stat(backup); backupErr == nil {
			_ = os.Rename(backup, target)
		}
		return fmt.Errorf("activate local script: %w", err)
	}
	_ = os.RemoveAll(backup)
	return nil
}

func validateStored(manifest manifestDocument, id string, contents []byte) error {
	if manifest.ManifestVersion != manifestVersion || manifest.ID != id || manifest.Revision < 1 {
		return fmt.Errorf("local script manifest is inconsistent")
	}
	if strings.TrimSpace(manifest.Name) == "" || len([]rune(manifest.Name)) > maxNameLength || len([]rune(manifest.Description)) > maxDescriptionLength {
		return fmt.Errorf("local script manifest metadata is invalid")
	}
	if _, err := time.Parse(time.RFC3339Nano, manifest.CreatedAt); err != nil {
		return fmt.Errorf("local script creation time is invalid")
	}
	if _, err := time.Parse(time.RFC3339Nano, manifest.UpdatedAt); err != nil {
		return fmt.Errorf("local script update time is invalid")
	}
	if manifest.SizeBytes != int64(len(contents)) || manifest.LineCount != sourceLineCount(string(contents)) {
		return fmt.Errorf("local script manifest statistics are inconsistent")
	}
	if err := ValidateSource(string(contents)); err != nil {
		return fmt.Errorf("stored local script is invalid: %w", err)
	}
	return nil
}

func normalizeSource(source string) string {
	source = strings.TrimPrefix(source, "\ufeff")
	source = strings.ReplaceAll(source, "\r\n", "\n")
	source = strings.ReplaceAll(source, "\r", "\n")
	source = strings.TrimRight(source, " \t\n")
	if source == "" {
		return ""
	}
	return source + "\n"
}

func sourceLineCount(source string) int {
	source = strings.TrimSuffix(source, "\n")
	if source == "" {
		return 0
	}
	return strings.Count(source, "\n") + 1
}

func manifestFromScript(script Script) manifestDocument {
	return manifestDocument{
		ManifestVersion: manifestVersion, ID: script.ID, Name: script.Name,
		Description: script.Description, Revision: script.Revision,
		LineCount: script.LineCount, SizeBytes: script.SizeBytes,
		CreatedAt: script.CreatedAt, UpdatedAt: script.UpdatedAt,
	}
}

func summaryFromManifest(manifest manifestDocument) Summary {
	return Summary{
		ID: manifest.ID, Name: manifest.Name, Description: manifest.Description,
		Revision: manifest.Revision, LineCount: manifest.LineCount, SizeBytes: manifest.SizeBytes,
		CreatedAt: manifest.CreatedAt, UpdatedAt: manifest.UpdatedAt,
	}
}

func ensureManagedPath(path, root string) error {
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
		return fmt.Errorf("managed path escapes local script directory")
	}
	return nil
}

func ensureManagedDirectory(path, root string) error {
	if err := ensureManagedPath(path, root); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("local script does not exist")
	}
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("local script directory is not managed")
	}
	return nil
}

func readLimitedFile(path string, limit int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > limit {
		return nil, fmt.Errorf("managed file is invalid or too large")
	}
	return os.ReadFile(path)
}

func writeSyncedFile(path string, contents []byte) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
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
	return file.Close()
}

func decodeStrictJSON(contents []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("unexpected trailing JSON")
	}
	return nil
}

func randomID() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}
