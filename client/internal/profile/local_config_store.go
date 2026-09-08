package profile

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

	"jeemi/internal/config/compose"
	"jeemi/internal/config/document"
	configresources "jeemi/internal/config/resources"
	"jeemi/internal/config/schema"
	"jeemi/internal/platform/paths"

	"gopkg.in/yaml.v3"
)

const (
	maxLocalConfigNameLength        = 80
	maxLocalConfigDescriptionLength = 500
	maxLocalConfigFields            = 512
	maxManifestBytes                = 128 << 10
	maxMergePlanBytes               = 512 << 10
	maxOverlayBytes                 = 4 << 20
)

var localConfigIDPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

type LocalConfigStoreOptions struct {
	DataDirectory string
	Now           func() time.Time
	NewID         func() (string, error)
}

type LocalConfigStore struct {
	mu    sync.Mutex
	root  string
	now   func() time.Time
	newID func() (string, error)
}

func NewLocalConfigStore(options LocalConfigStoreOptions) (*LocalConfigStore, error) {
	root, err := paths.LocalConfigsDirectoryFromRoot(options.DataDirectory)
	if err != nil {
		return nil, err
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	newID := options.NewID
	if newID == nil {
		newID = randomLocalConfigID
	}
	return &LocalConfigStore{root: root, now: now, newID: newID}, nil
}

func (s *LocalConfigStore) State() (LocalConfigState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stateUnlocked()
}

func (s *LocalConfigStore) Get(id string) (LocalConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadUnlocked(id)
}

func (s *LocalConfigStore) Preview(input SaveLocalConfigInput) (LocalConfigPreview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	prepared, err := prepareLocalConfig(input)
	if err != nil {
		return LocalConfigPreview{}, err
	}
	redacted, err := redactOverlay(prepared.overlayYAML, prepared.fields)
	if err != nil {
		return LocalConfigPreview{}, err
	}
	return LocalConfigPreview{
		SchemaVersion:       schema.Version,
		EnabledFieldCount:   len(prepared.fields),
		ProxyOverrideCount:  len(prepared.transforms),
		OverlayYAML:         string(prepared.overlayYAML),
		RedactedOverlayYAML: string(redacted),
		MergePlan:           prepared.plan,
		Fields:              prepared.fields,
		ResourcePlan:        prepared.resourcePlan,
	}, nil
}

func redactOverlay(overlayYAML []byte, fields []LocalConfigField) ([]byte, error) {
	overlay, err := document.Parse(overlayYAML)
	if err != nil {
		return nil, err
	}
	for _, field := range fields {
		definition, known := schema.FindField(field.Path)
		if !known || !definition.Sensitive || definition.Scope != schema.ScopeDirect {
			continue
		}
		value, found, err := document.Find(document.Root(overlay), field.Path)
		if err != nil || !found {
			return nil, fmt.Errorf("resolve sensitive local field %s", field.Path)
		}
		if err := document.SetMappingPath(document.Root(overlay), field.Path, redactedNode(value.Kind)); err != nil {
			return nil, err
		}
	}
	return document.Encode(overlay)
}

func redactedNode(kind yaml.Kind) *yaml.Node {
	marker := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "***"}
	switch kind {
	case yaml.SequenceNode:
		return &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Content: []*yaml.Node{marker}}
	case yaml.MappingNode:
		return &yaml.Node{
			Kind: yaml.MappingNode,
			Tag:  "!!map",
			Content: []*yaml.Node{
				{Kind: yaml.ScalarNode, Tag: "!!str", Value: "redacted"},
				{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"},
			},
		}
	default:
		return marker
	}
}

func (s *LocalConfigStore) Save(input SaveLocalConfigInput, resourceStates ...configresources.State) (LocalConfig, error) {
	return s.saveWithCommit(input, nil, resourceStates...)
}

// SaveWithResourceCommit keeps the previous directory until the resource library
// has committed. The caller must hold the library lock before entering this store.
func (s *LocalConfigStore) SaveWithResourceCommit(input SaveLocalConfigInput, state configresources.State, commit func() error) (LocalConfig, error) {
	return s.saveWithCommit(input, commit, state)
}

func (s *LocalConfigStore) saveWithCommit(input SaveLocalConfigInput, commit func() error, resourceStates ...configresources.State) (LocalConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	prepared, err := prepareLocalConfig(input, resourceStates...)
	if err != nil {
		return LocalConfig{}, err
	}

	identifier := input.ID
	now := s.now().UTC()
	createdAt := now
	revision := 1
	if identifier == "" {
		identifier, err = s.newID()
		if err != nil {
			return LocalConfig{}, fmt.Errorf("create local configuration id: %w", err)
		}
		if !localConfigIDPattern.MatchString(identifier) {
			return LocalConfig{}, fmt.Errorf("generated local configuration id is invalid")
		}
		if _, statErr := os.Stat(filepath.Join(s.root, identifier)); statErr == nil {
			return LocalConfig{}, fmt.Errorf("generated local configuration id already exists")
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return LocalConfig{}, fmt.Errorf("check local configuration id: %w", statErr)
		}
	} else {
		existing, loadErr := s.loadUnlocked(identifier)
		if loadErr != nil {
			return LocalConfig{}, loadErr
		}
		createdTime, parseErr := time.Parse(time.RFC3339Nano, existing.CreatedAt)
		if parseErr != nil {
			return LocalConfig{}, fmt.Errorf("stored local configuration creation time is invalid")
		}
		createdAt = createdTime
		revision = existing.Revision + 1
	}

	manifest := manifestDocument{
		ManifestVersion:        manifestVersion,
		ID:                     identifier,
		Name:                   prepared.name,
		Description:            prepared.description,
		SchemaVersion:          schema.Version,
		Revision:               revision,
		CreatedAt:              createdAt.Format(time.RFC3339Nano),
		UpdatedAt:              now.Format(time.RFC3339Nano),
		EnabledFieldCount:      len(prepared.fields),
		RuleProviderCount:      prepared.ruleProviderCount,
		StrategyGroupCount:     len(prepared.resourcePlan.StrategyGroupIDs),
		StaticValidationStatus: "static_passed",
	}
	if err := s.writeWithCommitUnlocked(manifest, prepared.overlayYAML, prepared.plan, prepared.transforms, prepared.resourcePlan, commit); err != nil {
		return LocalConfig{}, err
	}
	return s.loadUnlocked(identifier)
}

func (s *LocalConfigStore) Delete(id string) (LocalConfigState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !localConfigIDPattern.MatchString(id) {
		return LocalConfigState{}, fmt.Errorf("invalid local configuration id")
	}
	if _, err := s.loadUnlocked(id); err != nil {
		return LocalConfigState{}, err
	}
	target := filepath.Join(s.root, id)
	if err := ensureManagedConfigPath(target, s.root); err != nil {
		return LocalConfigState{}, err
	}
	tombstone := filepath.Join(s.root, ".deleted-"+id)
	_ = os.RemoveAll(tombstone)
	if err := os.Rename(target, tombstone); err != nil {
		return LocalConfigState{}, fmt.Errorf("stage local configuration deletion: %w", err)
	}
	if err := os.RemoveAll(tombstone); err != nil {
		_ = os.Rename(tombstone, target)
		return LocalConfigState{}, fmt.Errorf("delete local configuration: %w", err)
	}
	return s.stateUnlocked()
}

func (s *LocalConfigStore) Directory() string {
	return s.root
}

type preparedLocalConfig struct {
	name              string
	description       string
	fields            []LocalConfigField
	plan              []compose.Rule
	transforms        []localFieldTransformDoc
	resourcePlan      configresources.Plan
	overlayYAML       []byte
	ruleProviderCount int
}

func prepareLocalConfig(input SaveLocalConfigInput, resourceStates ...configresources.State) (preparedLocalConfig, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" || len([]rune(name)) > maxLocalConfigNameLength {
		return preparedLocalConfig{}, fmt.Errorf("local configuration name must contain 1 to %d characters", maxLocalConfigNameLength)
	}
	description := strings.TrimSpace(input.Description)
	if len([]rune(description)) > maxLocalConfigDescriptionLength {
		return preparedLocalConfig{}, fmt.Errorf("local configuration description exceeds %d characters", maxLocalConfigDescriptionLength)
	}
	if len(input.Fields) > maxLocalConfigFields {
		return preparedLocalConfig{}, fmt.Errorf("local configuration exceeds %d enabled fields", maxLocalConfigFields)
	}
	fields := make([]LocalConfigField, len(input.Fields))
	copy(fields, input.Fields)
	sort.Slice(fields, func(left, right int) bool { return fields[left].Path < fields[right].Path })
	overlay := document.New()
	plan := make([]compose.Rule, 0, len(fields))
	transforms := make([]localFieldTransformDoc, 0)
	resourcePlan, err := configresources.NormalizePlan(input.ResourcePlan)
	if err != nil {
		return preparedLocalConfig{}, err
	}
	seen := make(map[string]struct{}, len(fields))
	ruleProviderCount := 0
	for index := range fields {
		field := &fields[index]
		definition, found := schema.FindField(field.Path)
		if !found {
			return preparedLocalConfig{}, fmt.Errorf("configuration field %s is not in schema %s", field.Path, schema.Version)
		}
		if definition.Locked {
			return preparedLocalConfig{}, fmt.Errorf("configuration field %s is managed by Jeemi", field.Path)
		}
		if _, duplicate := seen[field.Path]; duplicate {
			return preparedLocalConfig{}, fmt.Errorf("duplicate local configuration field %s", field.Path)
		}
		seen[field.Path] = struct{}{}

		value, err := document.ParseValue(field.ValueYAML)
		if err != nil {
			return preparedLocalConfig{}, fmt.Errorf("parse local field %s: %w", field.Path, err)
		}
		if !matchesSchemaKind(value.Kind, definition.Kind) {
			return preparedLocalConfig{}, fmt.Errorf("local field %s has the wrong YAML type", field.Path)
		}
		if definition.Editor == schema.EditorEnum && !containsString(definition.Options, value.Value) {
			return preparedLocalConfig{}, fmt.Errorf("local field %s has an unsupported value", field.Path)
		}
		if field.Strategy == "" {
			field.Strategy = definition.DefaultStrategy
		}
		if !containsStrategy(definition.Strategies, field.Strategy) {
			return preparedLocalConfig{}, fmt.Errorf("local field %s has an unsupported merge strategy", field.Path)
		}
		if field.Strategy == compose.StrategyMergeByName {
			if field.ConflictPolicy == "" {
				field.ConflictPolicy = definition.DefaultConflict
			}
			if field.ConflictPolicy != compose.ConflictError && field.ConflictPolicy != compose.ConflictUseLocal {
				return preparedLocalConfig{}, fmt.Errorf("local field %s has an invalid conflict policy", field.Path)
			}
		} else {
			field.ConflictPolicy = ""
		}
		normalized, err := document.EncodeValue(value)
		if err != nil {
			return preparedLocalConfig{}, err
		}
		field.ValueYAML = normalized
		if definition.Scope == schema.ScopeAllProxies {
			transforms = append(transforms, localFieldTransformDoc{Path: field.Path, ValueYAML: normalized})
			continue
		}
		if err := document.SetMappingPath(document.Root(overlay), field.Path, value); err != nil {
			return preparedLocalConfig{}, err
		}
		plan = append(plan, compose.Rule{
			Path:           field.Path,
			Strategy:       field.Strategy,
			ConflictPolicy: field.ConflictPolicy,
		})
		if field.Path == "/rule-providers" && value.Kind == yaml.MappingNode {
			ruleProviderCount = len(value.Content) / 2
		}
	}
	if err := compose.ValidatePlan(document.Root(overlay), plan); err != nil {
		return preparedLocalConfig{}, err
	}
	overlayYAML, err := document.Encode(overlay)
	if err != nil {
		return preparedLocalConfig{}, err
	}
	if len(overlayYAML) > maxOverlayBytes {
		return preparedLocalConfig{}, fmt.Errorf("local configuration overlay exceeds the %d byte limit", maxOverlayBytes)
	}
	if _, err := document.Parse(overlayYAML); err != nil {
		return preparedLocalConfig{}, fmt.Errorf("validate generated local configuration overlay: %w", err)
	}
	if len(resourceStates) > 0 {
		count, countErr := configresources.RuleProviderCount(resourcePlan, resourceStates[0])
		if countErr != nil {
			return preparedLocalConfig{}, countErr
		}
		ruleProviderCount += count
	}
	return preparedLocalConfig{
		name:              name,
		description:       description,
		fields:            fields,
		plan:              plan,
		transforms:        transforms,
		overlayYAML:       overlayYAML,
		resourcePlan:      resourcePlan,
		ruleProviderCount: ruleProviderCount,
	}, nil
}

func matchesSchemaKind(kind yaml.Kind, expected schema.ValueKind) bool {
	switch expected {
	case schema.KindScalar:
		return kind == yaml.ScalarNode
	case schema.KindMapping, schema.KindNamedMapping:
		return kind == yaml.MappingNode
	case schema.KindSequence, schema.KindNamedSequence, schema.KindRules:
		return kind == yaml.SequenceNode
	default:
		return false
	}
}

func containsStrategy(strategies []compose.Strategy, candidate compose.Strategy) bool {
	for _, strategy := range strategies {
		if strategy == candidate {
			return true
		}
	}
	return false
}

func containsString(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

func (s *LocalConfigStore) stateUnlocked() (LocalConfigState, error) {
	configs, err := s.listUnlocked()
	if err != nil {
		return LocalConfigState{}, err
	}
	return LocalConfigState{Directory: s.root, Configs: configs}, nil
}

func (s *LocalConfigStore) listUnlocked() ([]LocalConfigSummary, error) {
	entries, err := os.ReadDir(s.root)
	if errors.Is(err, os.ErrNotExist) {
		return []LocalConfigSummary{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read local configuration directory: %w", err)
	}
	configs := make([]LocalConfigSummary, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || !localConfigIDPattern.MatchString(entry.Name()) {
			continue
		}
		config, err := s.loadUnlocked(entry.Name())
		if err != nil {
			return nil, err
		}
		configs = append(configs, config.LocalConfigSummary)
	}
	sort.Slice(configs, func(left, right int) bool {
		if configs[left].UpdatedAt == configs[right].UpdatedAt {
			return configs[left].Name < configs[right].Name
		}
		return configs[left].UpdatedAt > configs[right].UpdatedAt
	})
	return configs, nil
}

func (s *LocalConfigStore) loadUnlocked(id string) (LocalConfig, error) {
	if !localConfigIDPattern.MatchString(id) {
		return LocalConfig{}, fmt.Errorf("invalid local configuration id")
	}
	directory := filepath.Join(s.root, id)
	if err := ensureManagedConfigPath(directory, s.root); err != nil {
		return LocalConfig{}, err
	}
	directoryInfo, err := os.Lstat(directory)
	if errors.Is(err, os.ErrNotExist) {
		return LocalConfig{}, fmt.Errorf("local configuration does not exist")
	}
	if err != nil {
		return LocalConfig{}, fmt.Errorf("inspect local configuration directory: %w", err)
	}
	if directoryInfo.Mode()&os.ModeSymlink != 0 || !directoryInfo.IsDir() {
		return LocalConfig{}, fmt.Errorf("local configuration directory is not a managed directory")
	}
	manifestBytes, err := readLimitedFile(filepath.Join(directory, "manifest.json"), maxManifestBytes)
	if err != nil {
		return LocalConfig{}, fmt.Errorf("read local configuration manifest: %w", err)
	}
	var manifest manifestDocument
	if err := decodeStrictJSON(manifestBytes, &manifest); err != nil {
		return LocalConfig{}, fmt.Errorf("parse local configuration manifest: %w", err)
	}
	if manifest.ManifestVersion != manifestVersion || manifest.ID != id ||
		manifest.SchemaVersion != schema.Version || manifest.Revision < 1 {
		return LocalConfig{}, fmt.Errorf("local configuration manifest is inconsistent")
	}
	if strings.TrimSpace(manifest.Name) == "" || len([]rune(manifest.Name)) > maxLocalConfigNameLength || len([]rune(manifest.Description)) > maxLocalConfigDescriptionLength {
		return LocalConfig{}, fmt.Errorf("local configuration manifest metadata is invalid")
	}
	if _, err := time.Parse(time.RFC3339Nano, manifest.CreatedAt); err != nil {
		return LocalConfig{}, fmt.Errorf("local configuration creation time is invalid")
	}
	if _, err := time.Parse(time.RFC3339Nano, manifest.UpdatedAt); err != nil {
		return LocalConfig{}, fmt.Errorf("local configuration update time is invalid")
	}

	overlayBytes, err := readLimitedFile(filepath.Join(directory, "overlay.yaml"), maxOverlayBytes)
	if err != nil {
		return LocalConfig{}, fmt.Errorf("read local configuration overlay: %w", err)
	}
	overlay, err := document.Parse(overlayBytes)
	if err != nil {
		return LocalConfig{}, fmt.Errorf("parse local configuration overlay: %w", err)
	}
	planBytes, err := readLimitedFile(filepath.Join(directory, "merge-plan.json"), maxMergePlanBytes)
	if err != nil {
		return LocalConfig{}, fmt.Errorf("read local configuration merge plan: %w", err)
	}
	var plan mergePlanDocument
	if err := decodeStrictJSON(planBytes, &plan); err != nil {
		return LocalConfig{}, fmt.Errorf("parse local configuration merge plan: %w", err)
	}
	if plan.Version != mergePlanVersion || plan.Resources.Version != configresources.CurrentPlanVersion {
		return LocalConfig{}, fmt.Errorf("local configuration merge plan version is unsupported")
	}
	plan.Resources, err = configresources.NormalizePlan(plan.Resources)
	if err != nil {
		return LocalConfig{}, fmt.Errorf("validate local configuration resource plan: %w", err)
	}
	if err := compose.ValidatePlan(document.Root(overlay), plan.Rules); err != nil {
		return LocalConfig{}, fmt.Errorf("validate local configuration merge plan: %w", err)
	}
	fields := make([]LocalConfigField, 0, len(plan.Rules)+len(plan.Transforms))
	seenPaths := make(map[string]struct{}, len(plan.Rules)+len(plan.Transforms))
	for _, rule := range plan.Rules {
		definition, known := schema.FindField(rule.Path)
		if !known || definition.Locked || definition.Scope != schema.ScopeDirect {
			return LocalConfig{}, fmt.Errorf("saved local field %s is unavailable or managed by Jeemi", rule.Path)
		}
		if _, duplicate := seenPaths[rule.Path]; duplicate {
			return LocalConfig{}, fmt.Errorf("duplicate saved local field %s", rule.Path)
		}
		seenPaths[rule.Path] = struct{}{}
		value, found, err := document.Find(document.Root(overlay), rule.Path)
		if err != nil || !found {
			return LocalConfig{}, fmt.Errorf("resolve saved local field %s", rule.Path)
		}
		valueYAML, err := document.EncodeValue(value)
		if err != nil {
			return LocalConfig{}, err
		}
		if !matchesSchemaKind(value.Kind, definition.Kind) || !containsStrategy(definition.Strategies, rule.Strategy) {
			return LocalConfig{}, fmt.Errorf("saved local field %s does not match schema %s", rule.Path, schema.Version)
		}
		fields = append(fields, LocalConfigField{
			Path:           rule.Path,
			ValueYAML:      valueYAML,
			Strategy:       rule.Strategy,
			ConflictPolicy: rule.ConflictPolicy,
		})
	}
	for _, transform := range plan.Transforms {
		definition, known := schema.FindField(transform.Path)
		if !known || definition.Locked || definition.Scope != schema.ScopeAllProxies {
			return LocalConfig{}, fmt.Errorf("saved local transform %s is unavailable or managed by Jeemi", transform.Path)
		}
		if _, duplicate := seenPaths[transform.Path]; duplicate {
			return LocalConfig{}, fmt.Errorf("duplicate saved local field %s", transform.Path)
		}
		seenPaths[transform.Path] = struct{}{}
		value, err := document.ParseValue(transform.ValueYAML)
		if err != nil || !matchesSchemaKind(value.Kind, definition.Kind) {
			return LocalConfig{}, fmt.Errorf("saved local transform %s does not match schema %s", transform.Path, schema.Version)
		}
		if definition.Editor == schema.EditorEnum && !containsString(definition.Options, value.Value) {
			return LocalConfig{}, fmt.Errorf("saved local transform %s has an unsupported value", transform.Path)
		}
		normalized, err := document.EncodeValue(value)
		if err != nil {
			return LocalConfig{}, err
		}
		fields = append(fields, LocalConfigField{
			Path: transform.Path, ValueYAML: normalized, Strategy: definition.DefaultStrategy,
		})
	}
	sort.Slice(fields, func(left, right int) bool { return fields[left].Path < fields[right].Path })
	if manifest.EnabledFieldCount != len(fields) {
		return LocalConfig{}, fmt.Errorf("local configuration field count is inconsistent")
	}
	if manifest.StrategyGroupCount != len(plan.Resources.StrategyGroupIDs) {
		return LocalConfig{}, fmt.Errorf("local configuration strategy group count is inconsistent")
	}
	config := LocalConfig{
		LocalConfigSummary: LocalConfigSummary{
			ID:                     manifest.ID,
			Name:                   manifest.Name,
			Description:            manifest.Description,
			SchemaVersion:          manifest.SchemaVersion,
			Revision:               manifest.Revision,
			CreatedAt:              manifest.CreatedAt,
			UpdatedAt:              manifest.UpdatedAt,
			EnabledFieldCount:      manifest.EnabledFieldCount,
			RuleProviderCount:      manifest.RuleProviderCount,
			StrategyGroupCount:     len(plan.Resources.StrategyGroupIDs),
			StaticValidationStatus: manifest.StaticValidationStatus,
		},
		Fields:       fields,
		OverlayYAML:  string(overlayBytes),
		ResourcePlan: plan.Resources,
	}
	return config, nil
}

func (s *LocalConfigStore) writeUnlocked(manifest manifestDocument, overlayYAML []byte, plan []compose.Rule, transforms []localFieldTransformDoc, resources configresources.Plan) error {
	return s.writeWithCommitUnlocked(manifest, overlayYAML, plan, transforms, resources, nil)
}

func (s *LocalConfigStore) writeWithCommitUnlocked(manifest manifestDocument, overlayYAML []byte, plan []compose.Rule, transforms []localFieldTransformDoc, resources configresources.Plan, commit func() error) error {
	if err := os.MkdirAll(s.root, 0o700); err != nil {
		return fmt.Errorf("create local configuration root: %w", err)
	}
	stage, err := os.MkdirTemp(s.root, ".local-config-stage-")
	if err != nil {
		return fmt.Errorf("create local configuration stage: %w", err)
	}
	defer os.RemoveAll(stage)
	if err := os.Chmod(stage, 0o700); err != nil {
		return fmt.Errorf("protect local configuration stage: %w", err)
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	planBytes, err := json.MarshalIndent(mergePlanDocument{Version: mergePlanVersion, Rules: plan, Transforms: transforms, Resources: resources}, "", "  ")
	if err != nil {
		return err
	}
	for name, contents := range map[string][]byte{
		"manifest.json":   append(manifestBytes, '\n'),
		"overlay.yaml":    overlayYAML,
		"merge-plan.json": append(planBytes, '\n'),
	} {
		if err := writeSyncedFile(filepath.Join(stage, name), contents); err != nil {
			return err
		}
	}
	target := filepath.Join(s.root, manifest.ID)
	if err := ensureManagedConfigPath(target, s.root); err != nil {
		return err
	}
	return promoteDirectoryWithCommit(stage, target, commit)
}

func promoteDirectory(stage, target string) error {
	return promoteDirectoryWithCommit(stage, target, nil)
}

func promoteDirectoryWithCommit(stage, target string, commit func() error) error {
	backup := target + ".previous"
	_ = os.RemoveAll(backup)
	if _, err := os.Stat(target); err == nil {
		if err := os.Rename(target, backup); err != nil {
			return fmt.Errorf("stage previous local configuration: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Rename(stage, target); err != nil {
		if _, backupErr := os.Stat(backup); backupErr == nil {
			_ = os.Rename(backup, target)
		}
		return fmt.Errorf("activate local configuration: %w", err)
	}
	if commit != nil {
		if err := commit(); err != nil {
			// Move the candidate back to its staging path before restoring the
			// old directory; the caller owns cleanup of the staged candidate.
			if rollbackErr := os.Rename(target, stage); rollbackErr != nil {
				return errors.Join(err, fmt.Errorf("restore previous local configuration: %w", rollbackErr))
			}
			if _, backupErr := os.Stat(backup); backupErr == nil {
				if rollbackErr := os.Rename(backup, target); rollbackErr != nil {
					return errors.Join(err, fmt.Errorf("restore previous local configuration: %w", rollbackErr))
				}
			}
			return err
		}
	}
	_ = os.RemoveAll(backup)
	return nil
}

func writeSyncedFile(path string, contents []byte) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create local configuration file: %w", err)
	}
	if _, err := file.Write(contents); err != nil {
		file.Close()
		return fmt.Errorf("write local configuration file: %w", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return fmt.Errorf("sync local configuration file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close local configuration file: %w", err)
	}
	return nil
}

func readLimitedFile(path string, limit int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() > limit {
		return nil, fmt.Errorf("managed file is invalid or too large")
	}
	return os.ReadFile(path)
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

func ensureManagedConfigPath(path, root string) error {
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
		return fmt.Errorf("managed path escapes local configuration directory")
	}
	return nil
}

func randomLocalConfigID() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}
