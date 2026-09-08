package subscription

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"jeemi/internal/config/fallbackoverride"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"jeemi/internal/config/document"
	configresources "jeemi/internal/config/resources"
	"jeemi/internal/platform/paths"
)

const (
	manifestVersion                  = 1
	maxSubscriptionNameLength        = 80
	maxSubscriptionDescriptionLength = 500
	maxRuleProviderNameLength        = 256
	maxSubscriptionBytes             = 4 << 20
	maxSubscriptionMetadataBytes     = 512 << 10
)

var (
	subscriptionIDPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)
	localConfigIDPattern  = regexp.MustCompile(`^[a-f0-9]{32}$`)
	localScriptIDPattern  = regexp.MustCompile(`^[a-f0-9]{32}$`)
	revisionIDPattern     = regexp.MustCompile(`^[a-f0-9]{64}$`)
)

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

type createInput struct {
	Name             string
	Description      string
	SourceKind       SourceKind
	ImportMethod     ImportMethod
	SourceURL        string
	OriginalFileName string
	IconKind         IconKind
	Icon             string
	Format           Format
	Contents         []byte
	RemoteProfile    RemoteProfile
}

func NewStore(options StoreOptions) (*Store, error) {
	root, err := paths.SubscriptionsDirectoryFromRoot(options.DataDirectory)
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

func (s *Store) Directory() string {
	return s.root
}

func (s *Store) State() (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stateUnlocked()
}

func (s *Store) Get(id string) (Detail, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	metadata, err := s.loadMetadataUnlocked(id)
	if err != nil {
		return Detail{}, err
	}
	return detailFromMetadata(metadata), nil
}

func (s *Store) Create(input createInput) (Detail, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	prepared, err := prepareCreateInput(input)
	if err != nil {
		return Detail{}, err
	}
	id, err := s.newID()
	if err != nil {
		return Detail{}, fmt.Errorf("create subscription id: %w", err)
	}
	if !subscriptionIDPattern.MatchString(id) {
		return Detail{}, fmt.Errorf("generated subscription id is invalid")
	}
	if _, err := os.Lstat(filepath.Join(s.root, id)); err == nil {
		return Detail{}, fmt.Errorf("generated subscription id already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return Detail{}, fmt.Errorf("check subscription id: %w", err)
	}

	now := s.now().UTC().Format(time.RFC3339Nano)
	revision := newRevision(prepared.Format, prepared.Contents, now)
	metadata := metadataDocument{
		Fallback:              fallbackoverride.Selection{Mode: fallbackoverride.ModeNone},
		ManifestVersion:       manifestVersion,
		ID:                    id,
		Name:                  prepared.Name,
		Description:           prepared.Description,
		SourceKind:            prepared.SourceKind,
		ImportMethod:          prepared.ImportMethod,
		SourceURL:             prepared.SourceURL,
		OriginalFileName:      prepared.OriginalFileName,
		IconKind:              prepared.IconKind,
		Icon:                  prepared.Icon,
		DisabledRuleProviders: []string{},
		CurrentRevisionID:     revision.ID,
		Revisions:             []revisionDocument{revision},
		CreatedAt:             now,
		UpdatedAt:             now,
		LastFetchedAt:         now,
		RemoteProfile:         observedRemoteProfile(prepared.RemoteProfile, now),
	}
	if err := s.writeNewUnlocked(metadata, prepared.Contents); err != nil {
		return Detail{}, err
	}
	return detailFromMetadata(metadata), nil
}

func (s *Store) UpdateMetadata(input UpdateInput) (Detail, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	metadata, err := s.loadMetadataUnlocked(input.ID)
	if err != nil {
		return Detail{}, err
	}
	name, description, err := validateMetadata(input.Name, input.Description)
	if err != nil {
		return Detail{}, err
	}
	iconKind, icon, err := validateIcon(input.IconKind, input.Icon)
	if err != nil {
		return Detail{}, err
	}
	metadata.Name = name
	metadata.Description = description
	metadata.IconKind = iconKind
	metadata.Icon = icon
	metadata.UpdatedAt = s.now().UTC().Format(time.RFC3339Nano)
	if err := s.writeMetadataUnlocked(metadata); err != nil {
		return Detail{}, err
	}
	return detailFromMetadata(metadata), nil
}

func (s *Store) UpdateFromURL(input UpdateInput, fetched FetchedContent) (Detail, error) {
	return s.updateFromURLCandidate(input, fetched, nil, false)
}

func (s *Store) updateFromURLCandidate(input UpdateInput, fetched FetchedContent, before *Detail, detach bool) (Detail, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	metadata, err := s.loadMetadataUnlocked(input.ID)
	if err != nil {
		return Detail{}, err
	}
	if before != nil && !reflect.DeepEqual(detailFromMetadata(metadata), *before) {
		return Detail{}, fmt.Errorf("subscription changed while checking the candidate; refresh again")
	}
	if metadata.SourceKind != SourceURL {
		return Detail{}, fmt.Errorf("file subscriptions do not have a remote source")
	}
	name, description, err := validateMetadata(input.Name, input.Description)
	if err != nil {
		return Detail{}, err
	}
	parsedURL, err := validateRemoteURL(input.SourceURL)
	if err != nil {
		return Detail{}, err
	}
	if err := validateContents(fetched.Format, fetched.Contents); err != nil {
		return Detail{}, err
	}
	if err := validateRemoteProfile(fetched.RemoteProfile); err != nil {
		return Detail{}, err
	}
	iconKind, icon, err := validateIcon(input.IconKind, input.Icon)
	if err != nil {
		return Detail{}, err
	}

	now := s.now().UTC().Format(time.RFC3339Nano)
	revision := newRevision(fetched.Format, fetched.Contents, now)
	createdRevision := false
	if existing, found := findRevision(metadata.Revisions, revision.ID); found {
		revision = existing
		if err := s.verifyRevisionUnlocked(metadata.ID, existing); err != nil {
			return Detail{}, err
		}
	} else {
		createdRevision, err = s.writeRevisionUnlocked(metadata.ID, revision, fetched.Contents)
		if err != nil {
			return Detail{}, err
		}
	}
	if detach {
		metadata.LocalConfigID, metadata.LocalScriptID = "", ""
	}
	metadata.Name = name
	metadata.Description = description
	metadata.SourceURL = parsedURL.String()
	metadata.IconKind = iconKind
	metadata.Icon = icon
	metadata.CurrentRevisionID = revision.ID
	metadata.UpdatedAt = now
	metadata.LastFetchedAt = now
	if fetched.RemoteProfile.hasObservedValues() {
		metadata.RemoteProfile = observedRemoteProfile(fetched.RemoteProfile, now)
	}
	if !hasRevision(metadata.Revisions, revision.ID) {
		metadata.Revisions = append(metadata.Revisions, revision)
	}
	if err := s.writeMetadataUnlocked(metadata); err != nil {
		if createdRevision {
			_ = os.RemoveAll(filepath.Join(s.root, metadata.ID, "revisions", revision.ID))
		}
		return Detail{}, err
	}
	return detailFromMetadata(metadata), nil
}

func (s *Store) SetLocalConfig(id, localConfigID string) (Detail, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	metadata, err := s.loadMetadataUnlocked(id)
	if err != nil {
		return Detail{}, err
	}
	if localConfigID != "" && !localConfigIDPattern.MatchString(localConfigID) {
		return Detail{}, fmt.Errorf("invalid local configuration id")
	}
	if localConfigID != "" && metadata.LocalScriptID != "" {
		return Detail{}, fmt.Errorf("unlink the local script before associating a local configuration")
	}
	metadata.LocalConfigID = localConfigID
	metadata.UpdatedAt = s.now().UTC().Format(time.RFC3339Nano)
	if err := s.writeMetadataUnlocked(metadata); err != nil {
		return Detail{}, err
	}
	return detailFromMetadata(metadata), nil
}

func (s *Store) SetLocalScript(id, localScriptID string) (Detail, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	metadata, err := s.loadMetadataUnlocked(id)
	if err != nil {
		return Detail{}, err
	}
	if localScriptID != "" && !localScriptIDPattern.MatchString(localScriptID) {
		return Detail{}, fmt.Errorf("invalid local script id")
	}
	if localScriptID != "" && metadata.LocalConfigID != "" {
		return Detail{}, fmt.Errorf("unlink the local configuration before associating a local script")
	}
	metadata.LocalScriptID = localScriptID
	metadata.UpdatedAt = s.now().UTC().Format(time.RFC3339Nano)
	if err := s.writeMetadataUnlocked(metadata); err != nil {
		return Detail{}, err
	}
	return detailFromMetadata(metadata), nil
}

func (s *Store) SetRuleProviderEnabled(id, providerName string, enabled bool) (Detail, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	metadata, err := s.loadMetadataUnlocked(id)
	if err != nil {
		return Detail{}, err
	}
	providerName, err = validateRuleProviderName(providerName)
	if err != nil {
		return Detail{}, err
	}
	found := false
	next := make([]string, 0, len(metadata.DisabledRuleProviders)+1)
	for _, name := range metadata.DisabledRuleProviders {
		if name == providerName {
			found = true
			if enabled {
				continue
			}
		}
		next = append(next, name)
	}
	if enabled == !found {
		return detailFromMetadata(metadata), nil
	}
	if !enabled && !found {
		next = append(next, providerName)
	}
	sort.Strings(next)
	metadata.DisabledRuleProviders = next
	metadata.RuleProviderOverrideRevision++
	metadata.UpdatedAt = s.now().UTC().Format(time.RFC3339Nano)
	if err := s.writeMetadataUnlocked(metadata); err != nil {
		return Detail{}, err
	}
	return detailFromMetadata(metadata), nil
}

func (s *Store) ReferencingLocalConfig(localConfigID string) ([]Summary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	subscriptions, err := s.listUnlocked()
	if err != nil {
		return nil, err
	}
	references := make([]Summary, 0)
	for _, item := range subscriptions {
		if item.LocalConfigID == localConfigID {
			references = append(references, item)
		}
	}
	return references, nil
}

func (s *Store) ReferencingLocalScript(localScriptID string) ([]Summary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	subscriptions, err := s.listUnlocked()
	if err != nil {
		return nil, err
	}
	references := make([]Summary, 0)
	for _, item := range subscriptions {
		if item.LocalScriptID == localScriptID {
			references = append(references, item)
		}
	}
	return references, nil
}

func (s *Store) Text(id string) (TextView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	metadata, err := s.loadMetadataUnlocked(id)
	if err != nil {
		return TextView{}, err
	}
	revision, found := findRevision(metadata.Revisions, metadata.CurrentRevisionID)
	if !found {
		return TextView{}, fmt.Errorf("current subscription revision is missing")
	}
	revisionsRoot := filepath.Join(s.root, id, "revisions")
	if err := ensureManagedDirectory(revisionsRoot); err != nil {
		return TextView{}, fmt.Errorf("inspect subscription revisions root: %w", err)
	}
	revisionDirectory := filepath.Join(revisionsRoot, revision.ID)
	if err := ensureManagedDirectory(revisionDirectory); err != nil {
		return TextView{}, fmt.Errorf("inspect subscription revision directory: %w", err)
	}
	path := filepath.Join(revisionDirectory, revision.FileName)
	contents, err := readManagedFile(path, maxSubscriptionBytes)
	if err != nil {
		return TextView{}, fmt.Errorf("read subscription revision: %w", err)
	}
	digest := sha256.Sum256(contents)
	if hex.EncodeToString(digest[:]) != revision.SHA256 || int64(len(contents)) != revision.SizeBytes {
		return TextView{}, fmt.Errorf("subscription revision integrity check failed")
	}
	return TextView{
		ID:         id,
		Name:       metadata.Name,
		Format:     revision.Format,
		RevisionID: revision.ID,
		Contents:   string(contents),
	}, nil
}

func (s *Store) Delete(id string) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.loadMetadataUnlocked(id); err != nil {
		return State{}, err
	}
	target := filepath.Join(s.root, id)
	if err := ensureManagedChild(target, s.root); err != nil {
		return State{}, err
	}
	tombstone := filepath.Join(s.root, ".deleted-"+id)
	_ = os.RemoveAll(tombstone)
	if err := os.Rename(target, tombstone); err != nil {
		return State{}, fmt.Errorf("stage subscription deletion: %w", err)
	}
	if err := os.RemoveAll(tombstone); err != nil {
		_ = os.Rename(tombstone, target)
		return State{}, fmt.Errorf("delete subscription: %w", err)
	}
	return s.stateUnlocked()
}

func (s *Store) stateUnlocked() (State, error) {
	items, err := s.listUnlocked()
	if err != nil {
		return State{}, err
	}
	return State{Directory: s.root, Subscriptions: items}, nil
}

func (s *Store) listUnlocked() ([]Summary, error) {
	entries, err := os.ReadDir(s.root)
	if errors.Is(err, os.ErrNotExist) {
		return []Summary{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read subscription directory: %w", err)
	}
	items := make([]Summary, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || !subscriptionIDPattern.MatchString(entry.Name()) {
			continue
		}
		metadata, err := s.loadMetadataUnlocked(entry.Name())
		if err != nil {
			return nil, err
		}
		items = append(items, summaryFromMetadata(metadata))
	}
	sort.Slice(items, func(left, right int) bool {
		if items[left].UpdatedAt == items[right].UpdatedAt {
			return items[left].Name < items[right].Name
		}
		return items[left].UpdatedAt > items[right].UpdatedAt
	})
	return items, nil
}

func (s *Store) loadMetadataUnlocked(id string) (metadataDocument, error) {
	if !subscriptionIDPattern.MatchString(id) {
		return metadataDocument{}, fmt.Errorf("invalid subscription id")
	}
	directory := filepath.Join(s.root, id)
	if err := ensureManagedChild(directory, s.root); err != nil {
		return metadataDocument{}, err
	}
	info, err := os.Lstat(directory)
	if errors.Is(err, os.ErrNotExist) {
		return metadataDocument{}, fmt.Errorf("subscription does not exist")
	}
	if err != nil {
		return metadataDocument{}, fmt.Errorf("inspect subscription directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return metadataDocument{}, fmt.Errorf("subscription directory is not managed")
	}
	contents, err := readManagedFile(filepath.Join(directory, "metadata.json"), maxSubscriptionMetadataBytes)
	if err != nil {
		return metadataDocument{}, fmt.Errorf("read subscription metadata: %w", err)
	}
	var metadata metadataDocument
	if err := decodeStrictJSON(contents, &metadata); err != nil {
		return metadataDocument{}, fmt.Errorf("parse subscription metadata: %w", err)
	}
	if err := validateStoredMetadata(metadata, id); err != nil {
		return metadataDocument{}, err
	}
	return metadata, nil
}

func (s *Store) writeNewUnlocked(metadata metadataDocument, contents []byte) error {
	if err := os.MkdirAll(s.root, 0o700); err != nil {
		return fmt.Errorf("create subscription root: %w", err)
	}
	stage, err := os.MkdirTemp(s.root, ".subscription-stage-")
	if err != nil {
		return fmt.Errorf("create subscription stage: %w", err)
	}
	defer os.RemoveAll(stage)
	if err := os.Chmod(stage, 0o700); err != nil {
		return fmt.Errorf("protect subscription stage: %w", err)
	}
	revision := metadata.Revisions[0]
	revisionDirectory := filepath.Join(stage, "revisions", revision.ID)
	if err := os.MkdirAll(revisionDirectory, 0o700); err != nil {
		return fmt.Errorf("create subscription revision directory: %w", err)
	}
	if err := writeExclusiveFile(filepath.Join(revisionDirectory, revision.FileName), contents); err != nil {
		return err
	}
	metadataBytes, err := marshalMetadata(metadata)
	if err != nil {
		return err
	}
	if err := writeExclusiveFile(filepath.Join(stage, "metadata.json"), metadataBytes); err != nil {
		return err
	}
	target := filepath.Join(s.root, metadata.ID)
	if err := ensureManagedChild(target, s.root); err != nil {
		return err
	}
	if err := os.Rename(stage, target); err != nil {
		return fmt.Errorf("activate subscription: %w", err)
	}
	return nil
}

func (s *Store) writeRevisionUnlocked(id string, revision revisionDocument, contents []byte) (bool, error) {
	revisionsRoot := filepath.Join(s.root, id, "revisions")
	if err := ensureManagedChild(revisionsRoot, s.root); err != nil {
		return false, err
	}
	if err := os.MkdirAll(revisionsRoot, 0o700); err != nil {
		return false, fmt.Errorf("create subscription revisions root: %w", err)
	}
	if err := ensureManagedDirectory(revisionsRoot); err != nil {
		return false, fmt.Errorf("inspect subscription revisions root: %w", err)
	}
	target := filepath.Join(revisionsRoot, revision.ID)
	if info, err := os.Lstat(target); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return false, fmt.Errorf("subscription revision path is not managed")
		}
		contents, readErr := readManagedFile(filepath.Join(target, revision.FileName), maxSubscriptionBytes)
		if readErr != nil {
			return false, fmt.Errorf("verify existing subscription revision: %w", readErr)
		}
		digest := sha256.Sum256(contents)
		if hex.EncodeToString(digest[:]) != revision.SHA256 || int64(len(contents)) != revision.SizeBytes {
			return false, fmt.Errorf("existing subscription revision integrity check failed")
		}
		return false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("inspect subscription revision: %w", err)
	}
	stage, err := os.MkdirTemp(revisionsRoot, ".revision-stage-")
	if err != nil {
		return false, fmt.Errorf("create subscription revision stage: %w", err)
	}
	defer os.RemoveAll(stage)
	if err := os.Chmod(stage, 0o700); err != nil {
		return false, fmt.Errorf("protect subscription revision stage: %w", err)
	}
	if err := writeExclusiveFile(filepath.Join(stage, revision.FileName), contents); err != nil {
		return false, err
	}
	if err := os.Rename(stage, target); err != nil {
		if _, statErr := os.Stat(target); statErr == nil {
			return false, nil
		}
		return false, fmt.Errorf("activate subscription revision: %w", err)
	}
	return true, nil
}

func (s *Store) verifyRevisionUnlocked(id string, revision revisionDocument) error {
	revisionsRoot := filepath.Join(s.root, id, "revisions")
	if err := ensureManagedDirectory(revisionsRoot); err != nil {
		return fmt.Errorf("inspect existing subscription revisions root: %w", err)
	}
	revisionDirectory := filepath.Join(revisionsRoot, revision.ID)
	if err := ensureManagedDirectory(revisionDirectory); err != nil {
		return fmt.Errorf("inspect existing subscription revision directory: %w", err)
	}
	path := filepath.Join(revisionDirectory, revision.FileName)
	contents, err := readManagedFile(path, maxSubscriptionBytes)
	if err != nil {
		return fmt.Errorf("read existing subscription revision: %w", err)
	}
	digest := sha256.Sum256(contents)
	if hex.EncodeToString(digest[:]) != revision.SHA256 || int64(len(contents)) != revision.SizeBytes {
		return fmt.Errorf("existing subscription revision integrity check failed")
	}
	return nil
}

func (s *Store) writeMetadataUnlocked(metadata metadataDocument) error {
	directory := filepath.Join(s.root, metadata.ID)
	if err := ensureManagedChild(directory, s.root); err != nil {
		return err
	}
	if err := ensureManagedDirectory(directory); err != nil {
		return fmt.Errorf("inspect subscription directory: %w", err)
	}
	contents, err := marshalMetadata(metadata)
	if err != nil {
		return err
	}
	stage, err := os.CreateTemp(directory, ".metadata-stage-")
	if err != nil {
		return fmt.Errorf("create subscription metadata stage: %w", err)
	}
	stagePath := stage.Name()
	defer os.Remove(stagePath)
	if err := stage.Chmod(0o600); err != nil {
		stage.Close()
		return fmt.Errorf("protect subscription metadata stage: %w", err)
	}
	if _, err := stage.Write(contents); err != nil {
		stage.Close()
		return fmt.Errorf("write subscription metadata stage: %w", err)
	}
	if err := stage.Sync(); err != nil {
		stage.Close()
		return fmt.Errorf("sync subscription metadata stage: %w", err)
	}
	if err := stage.Close(); err != nil {
		return fmt.Errorf("close subscription metadata stage: %w", err)
	}
	target := filepath.Join(directory, "metadata.json")
	backup := filepath.Join(directory, ".metadata.previous")
	_ = os.Remove(backup)
	if err := os.Rename(target, backup); err != nil {
		return fmt.Errorf("stage previous subscription metadata: %w", err)
	}
	if err := os.Rename(stagePath, target); err != nil {
		_ = os.Rename(backup, target)
		return fmt.Errorf("activate subscription metadata: %w", err)
	}
	_ = os.Remove(backup)
	return nil
}

func prepareCreateInput(input createInput) (createInput, error) {
	name, description, err := validateMetadata(input.Name, input.Description)
	if err != nil {
		return createInput{}, err
	}
	input.Name = name
	input.Description = description
	input.IconKind, input.Icon, err = validateIcon(input.IconKind, input.Icon)
	if err != nil {
		return createInput{}, err
	}
	if !validImportMethod(input.ImportMethod) {
		return createInput{}, fmt.Errorf("subscription import method is invalid")
	}
	switch input.SourceKind {
	case SourceURL:
		if input.ImportMethod == ImportFile {
			return createInput{}, fmt.Errorf("subscription URL import method is invalid")
		}
		parsed, err := validateRemoteURL(input.SourceURL)
		if err != nil {
			return createInput{}, err
		}
		input.SourceURL = parsed.String()
		input.OriginalFileName = ""
	case SourceFile:
		if input.ImportMethod != ImportFile || filepath.Base(input.OriginalFileName) != input.OriginalFileName || strings.TrimSpace(input.OriginalFileName) == "" {
			return createInput{}, fmt.Errorf("subscription file source metadata is invalid")
		}
		input.SourceURL = ""
		input.RemoteProfile = RemoteProfile{}
	default:
		return createInput{}, fmt.Errorf("subscription source kind is invalid")
	}
	if err := validateContents(input.Format, input.Contents); err != nil {
		return createInput{}, err
	}
	if err := validateRemoteProfile(input.RemoteProfile); err != nil {
		return createInput{}, err
	}
	return input, nil
}

func validateMetadata(name, description string) (string, string, error) {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if name == "" || len([]rune(name)) > maxSubscriptionNameLength {
		return "", "", fmt.Errorf("subscription name must contain 1 to %d characters", maxSubscriptionNameLength)
	}
	if len([]rune(description)) > maxSubscriptionDescriptionLength {
		return "", "", fmt.Errorf("subscription description exceeds %d characters", maxSubscriptionDescriptionLength)
	}
	return name, description, nil
}

func validateRuleProviderName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > maxRuleProviderNameLength {
		return "", fmt.Errorf("rule provider name must contain 1 to %d characters", maxRuleProviderNameLength)
	}
	if strings.ContainsAny(name, "\x00\r\n") {
		return "", fmt.Errorf("rule provider name contains unsupported control characters")
	}
	return name, nil
}

func validateContents(format Format, contents []byte) error {
	if !validFormat(format) {
		return fmt.Errorf("subscription format is unsupported")
	}
	if len(contents) == 0 || len(contents) > maxSubscriptionBytes {
		return fmt.Errorf("subscription configuration must contain 1 to %d bytes", maxSubscriptionBytes)
	}
	validationContents := bytes.TrimPrefix(contents, []byte{0xef, 0xbb, 0xbf})
	if format == FormatJSON && !json.Valid(validationContents) {
		return fmt.Errorf("subscription JSON is invalid")
	}
	if _, err := document.Parse(validationContents); err != nil {
		return fmt.Errorf("subscription configuration is invalid: %w", err)
	}
	return nil
}

func validateStoredMetadata(metadata metadataDocument, id string) error {
	if metadata.ManifestVersion != manifestVersion || metadata.ID != id {
		return fmt.Errorf("subscription metadata is inconsistent")
	}
	if _, _, err := validateMetadata(metadata.Name, metadata.Description); err != nil {
		return fmt.Errorf("subscription metadata is invalid")
	}
	if metadata.LocalConfigID != "" && !localConfigIDPattern.MatchString(metadata.LocalConfigID) {
		return fmt.Errorf("subscription local configuration reference is invalid")
	}
	if metadata.LocalScriptID != "" && !localScriptIDPattern.MatchString(metadata.LocalScriptID) {
		return fmt.Errorf("subscription local script reference is invalid")
	}
	if metadata.LocalConfigID != "" && metadata.LocalScriptID != "" {
		return fmt.Errorf("subscription local handler references are mutually exclusive")
	}
	if kind, icon, err := validateIcon(metadata.IconKind, metadata.Icon); err != nil || kind != metadata.IconKind || icon != metadata.Icon {
		return fmt.Errorf("subscription icon metadata is invalid")
	}
	if metadata.FallbackOverrideRevision < 0 {
		return fmt.Errorf("subscription fallback revision is invalid")
	}
	if normalized, err := fallbackoverride.Normalize(metadata.Fallback); err != nil || normalized != metadata.Fallback {
		return fmt.Errorf("subscription fallback is invalid")
	}
	if metadata.RuleProviderOverrideRevision < 0 {
		return fmt.Errorf("subscription rule provider override revision is invalid")
	}
	seenDisabledProviders := make(map[string]struct{}, len(metadata.DisabledRuleProviders))
	for _, providerName := range metadata.DisabledRuleProviders {
		validated, err := validateRuleProviderName(providerName)
		if err != nil || validated != providerName {
			return fmt.Errorf("subscription disabled rule provider is invalid")
		}
		if _, duplicate := seenDisabledProviders[providerName]; duplicate {
			return fmt.Errorf("subscription disabled rule providers contain duplicates")
		}
		seenDisabledProviders[providerName] = struct{}{}
	}
	if err := validateRemoteProfile(metadata.RemoteProfile); err != nil {
		return fmt.Errorf("subscription response metadata is invalid")
	}
	if metadata.RemoteProfile.ObservedAt != "" {
		if _, err := time.Parse(time.RFC3339Nano, metadata.RemoteProfile.ObservedAt); err != nil {
			return fmt.Errorf("subscription response metadata time is invalid")
		}
	}
	if metadata.SourceKind == SourceURL {
		if _, err := validateRemoteURL(metadata.SourceURL); err != nil || metadata.OriginalFileName != "" || metadata.ImportMethod == ImportFile {
			return fmt.Errorf("subscription URL source metadata is invalid")
		}
	} else if metadata.SourceKind == SourceFile {
		if metadata.SourceURL != "" || metadata.ImportMethod != ImportFile || filepath.Base(metadata.OriginalFileName) != metadata.OriginalFileName || metadata.OriginalFileName == "" {
			return fmt.Errorf("subscription file source metadata is invalid")
		}
	} else {
		return fmt.Errorf("subscription source metadata is invalid")
	}
	if !validImportMethod(metadata.ImportMethod) || len(metadata.Revisions) == 0 || !revisionIDPattern.MatchString(metadata.CurrentRevisionID) {
		return fmt.Errorf("subscription revision metadata is invalid")
	}
	if _, err := time.Parse(time.RFC3339Nano, metadata.CreatedAt); err != nil {
		return fmt.Errorf("subscription creation time is invalid")
	}
	if _, err := time.Parse(time.RFC3339Nano, metadata.UpdatedAt); err != nil {
		return fmt.Errorf("subscription update time is invalid")
	}
	if _, err := time.Parse(time.RFC3339Nano, metadata.LastFetchedAt); err != nil {
		return fmt.Errorf("subscription fetch time is invalid")
	}
	seen := make(map[string]struct{}, len(metadata.Revisions))
	for _, revision := range metadata.Revisions {
		if revision.ID != revision.SHA256 || !revisionIDPattern.MatchString(revision.ID) || revision.FileName != "source."+string(revision.Format) || !validFormat(revision.Format) || revision.SizeBytes < 1 || revision.SizeBytes > maxSubscriptionBytes {
			return fmt.Errorf("subscription revision metadata is inconsistent")
		}
		if _, duplicate := seen[revision.ID]; duplicate {
			return fmt.Errorf("subscription revision metadata contains duplicates")
		}
		seen[revision.ID] = struct{}{}
		if _, err := time.Parse(time.RFC3339Nano, revision.CreatedAt); err != nil {
			return fmt.Errorf("subscription revision time is invalid")
		}
	}
	if _, ok := seen[metadata.CurrentRevisionID]; !ok {
		return fmt.Errorf("current subscription revision is missing")
	}
	return nil
}

func validateRemoteProfile(profile RemoteProfile) error {
	if profile.UploadBytes < 0 || profile.UploadBytes > maxJavaScriptSafeInteger ||
		profile.DownloadBytes < 0 || profile.DownloadBytes > maxJavaScriptSafeInteger ||
		profile.TotalBytes < 0 || profile.TotalBytes > maxJavaScriptSafeInteger ||
		profile.ExpiresAt < 0 || profile.ExpiresAt > maxJavaScriptSafeInteger ||
		profile.UpdateIntervalHours < 0 || profile.UpdateIntervalHours > 24*365 {
		return fmt.Errorf("subscription response metadata is out of range")
	}
	if !profile.HasTraffic && (profile.UploadBytes != 0 || profile.DownloadBytes != 0 || profile.TotalBytes != 0) {
		return fmt.Errorf("subscription traffic metadata is inconsistent")
	}
	return nil
}

func newRevision(format Format, contents []byte, createdAt string) revisionDocument {
	digest := sha256.Sum256(contents)
	id := hex.EncodeToString(digest[:])
	return revisionDocument{
		ID:        id,
		FileName:  "source." + string(format),
		Format:    format,
		SHA256:    id,
		SizeBytes: int64(len(contents)),
		CreatedAt: createdAt,
	}
}

func summaryFromMetadata(metadata metadataDocument) Summary {
	revision, _ := findRevision(metadata.Revisions, metadata.CurrentRevisionID)
	return Summary{
		Fallback:                     metadata.Fallback,
		FallbackResetTarget:          metadata.FallbackResetTarget,
		ID:                           metadata.ID,
		Name:                         metadata.Name,
		Description:                  metadata.Description,
		SourceKind:                   metadata.SourceKind,
		ImportMethod:                 metadata.ImportMethod,
		SourceLabel:                  sourceLabel(metadata),
		Format:                       revision.Format,
		IconKind:                     metadata.IconKind,
		Icon:                         metadata.Icon,
		LocalConfigID:                metadata.LocalConfigID,
		LocalScriptID:                metadata.LocalScriptID,
		RuleProviderOverrideRevision: metadata.RuleProviderOverrideRevision,
		FallbackOverrideRevision:     metadata.FallbackOverrideRevision,
		DisabledRuleProviders:        append([]string{}, metadata.DisabledRuleProviders...),
		CurrentRevisionID:            metadata.CurrentRevisionID,
		RevisionCount:                len(metadata.Revisions),
		SizeBytes:                    revision.SizeBytes,
		CreatedAt:                    metadata.CreatedAt,
		UpdatedAt:                    metadata.UpdatedAt,
		LastFetchedAt:                metadata.LastFetchedAt,
		RemoteProfile:                metadata.RemoteProfile,
	}
}

func validateIcon(kind IconKind, value string) (IconKind, string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return IconNone, "", nil
	}
	switch kind {
	case IconEmoji:
		if !configresources.IsSupportedSelectorEmoji(value) {
			return IconNone, "", fmt.Errorf("subscription emoji icon must use a supported option")
		}
		return IconEmoji, value, nil
	case IconURL:
		if len(value) > maxRemoteURLLength {
			return IconNone, "", fmt.Errorf("subscription icon URL is too long")
		}
		parsed, err := url.Parse(value)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil {
			return IconNone, "", fmt.Errorf("subscription icon URL must use http or https without credentials")
		}
		parsed.Fragment = ""
		return IconURL, parsed.String(), nil
	default:
		return IconNone, "", fmt.Errorf("subscription icon type is invalid")
	}
}

func observedRemoteProfile(profile RemoteProfile, observedAt string) RemoteProfile {
	if profile.hasObservedValues() {
		profile.ObservedAt = observedAt
	} else {
		profile.ObservedAt = ""
	}
	return profile
}

func detailFromMetadata(metadata metadataDocument) Detail {
	return Detail{Summary: summaryFromMetadata(metadata), SourceURL: metadata.SourceURL}
}

func sourceLabel(metadata metadataDocument) string {
	if metadata.SourceKind == SourceFile {
		return metadata.OriginalFileName
	}
	parsed, err := url.Parse(metadata.SourceURL)
	if err != nil {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}

func findRevision(revisions []revisionDocument, id string) (revisionDocument, bool) {
	for _, revision := range revisions {
		if revision.ID == id {
			return revision, true
		}
	}
	return revisionDocument{}, false
}

func hasRevision(revisions []revisionDocument, id string) bool {
	_, found := findRevision(revisions, id)
	return found
}

func validFormat(format Format) bool {
	return format == FormatYAML || format == FormatJSON || format == FormatText
}

func validImportMethod(method ImportMethod) bool {
	return method == ImportURL || method == ImportFile || method == ImportQRImage || method == ImportQRScreen
}

func marshalMetadata(metadata metadataDocument) ([]byte, error) {
	contents, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode subscription metadata: %w", err)
	}
	return append(contents, '\n'), nil
}

func writeExclusiveFile(path string, contents []byte) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create managed subscription file: %w", err)
	}
	if _, err := file.Write(contents); err != nil {
		file.Close()
		return fmt.Errorf("write managed subscription file: %w", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return fmt.Errorf("sync managed subscription file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close managed subscription file: %w", err)
	}
	return nil
}

func readManagedFile(path string, limit int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() > limit {
		return nil, fmt.Errorf("managed subscription file is invalid or too large")
	}
	return os.ReadFile(path)
}

func ensureManagedDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("managed directory is invalid")
	}
	return nil
}

func decodeStrictJSON(contents []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("unexpected trailing JSON")
		}
		return err
	}
	return nil
}

func ensureManagedChild(path, root string) error {
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
		return fmt.Errorf("managed path escapes subscription directory")
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
