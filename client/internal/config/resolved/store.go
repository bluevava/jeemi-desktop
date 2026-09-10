// Package resolved stores the latest safe, per-subscription runtime snapshot.
// It is deliberately separate from mihomo session generations: snapshots are
// long-lived and contain no external-controller address or secret.
package resolved

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"jeemi/internal/config/runtimecontrol"
	"jeemi/internal/geodata"
	"jeemi/internal/platform/paths"
	"jeemi/internal/runtimeconfig"
)

const manifestVersion = 1

var (
	subscriptionIDPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)
	sha256Pattern         = regexp.MustCompile(`^[a-f0-9]{64}$`)
)

type Input struct {
	ChainProxyFingerprint        string
	NormalizationFingerprint     string
	SubscriptionID               string
	SubscriptionRevision         string
	LocalConfigID                string
	LocalConfigRevision          int
	LocalScriptID                string
	LocalScriptRevision          int
	RuleProviderOverrideRevision int
	FallbackOverrideRevision     int
	CoreVersion                  string
	CoreValidated                bool
	Preferences                  runtimeconfig.Preferences
	GeoDataPreferences           geodata.Preferences
	GeoDataFingerprint           string
	Configuration                []byte
}

type Manifest struct {
	ChainProxyFingerprint        string                    `json:"chainProxyFingerprint"`
	NormalizationFingerprint     string                    `json:"normalizationFingerprint"`
	Version                      int                       `json:"version"`
	Fingerprint                  string                    `json:"fingerprint"`
	GeneratedAt                  string                    `json:"generatedAt"`
	SubscriptionID               string                    `json:"subscriptionId"`
	SubscriptionRevision         string                    `json:"subscriptionRevision"`
	LocalConfigID                string                    `json:"localConfigId"`
	LocalConfigRevision          int                       `json:"localConfigRevision"`
	LocalScriptID                string                    `json:"localScriptId"`
	LocalScriptRevision          int                       `json:"localScriptRevision"`
	RuleProviderOverrideRevision int                       `json:"ruleProviderOverrideRevision"`
	FallbackOverrideRevision     int                       `json:"fallbackOverrideRevision"`
	CoreVersion                  string                    `json:"coreVersion"`
	CoreValidated                bool                      `json:"coreValidated"`
	ConfigurationSHA256          string                    `json:"configurationSha256"`
	Preferences                  runtimeconfig.Preferences `json:"preferences"`
	GeoDataPreferences           geodata.Preferences       `json:"geoDataPreferences"`
	GeoDataFingerprint           string                    `json:"geoDataFingerprint"`
}

type Snapshot struct {
	Directory     string
	Configuration []byte
	Manifest      Manifest
}

type Store struct {
	root string
	now  func() time.Time
}

func NewStore(dataDirectory string) (*Store, error) {
	root, err := paths.MihomoResolvedDirectoryFromRoot(dataDirectory)
	if err != nil {
		return nil, err
	}
	return &Store{root: root, now: time.Now}, nil
}

func (s *Store) Save(input Input) (Snapshot, error) {
	if err := validateInput(input); err != nil {
		return Snapshot{}, err
	}
	configuration, err := runtimecontrol.StripSessionFields(input.Configuration)
	if err != nil {
		return Snapshot{}, err
	}
	input.Configuration = configuration
	fingerprint, configurationDigest, err := Fingerprint(input)
	if err != nil {
		return Snapshot{}, err
	}
	directory := filepath.Join(s.root, input.SubscriptionID)
	if err := ensureSubscriptionDirectory(directory, s.root, input.SubscriptionID); err != nil {
		return Snapshot{}, err
	}
	if err := os.MkdirAll(s.root, 0o700); err != nil {
		return Snapshot{}, fmt.Errorf("create resolved runtime root: %w", err)
	}
	manifest := Manifest{
		ChainProxyFingerprint:    input.ChainProxyFingerprint,
		NormalizationFingerprint: input.NormalizationFingerprint,
		Version:                  manifestVersion, Fingerprint: fingerprint,
		GeneratedAt:    s.now().UTC().Format(time.RFC3339Nano),
		SubscriptionID: input.SubscriptionID, SubscriptionRevision: input.SubscriptionRevision,
		LocalConfigID: input.LocalConfigID, LocalConfigRevision: input.LocalConfigRevision,
		LocalScriptID: input.LocalScriptID, LocalScriptRevision: input.LocalScriptRevision,
		RuleProviderOverrideRevision: input.RuleProviderOverrideRevision,
		FallbackOverrideRevision:     input.FallbackOverrideRevision,
		CoreVersion:                  input.CoreVersion, CoreValidated: input.CoreValidated,
		ConfigurationSHA256: configurationDigest, Preferences: input.Preferences,
		GeoDataPreferences: input.GeoDataPreferences, GeoDataFingerprint: input.GeoDataFingerprint,
	}
	manifestContents, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return Snapshot{}, fmt.Errorf("encode resolved runtime manifest: %w", err)
	}
	stage, err := os.MkdirTemp(s.root, "."+input.SubscriptionID+"-candidate-")
	if err != nil {
		return Snapshot{}, fmt.Errorf("create resolved runtime candidate: %w", err)
	}
	defer os.RemoveAll(stage)
	if err := os.Chmod(stage, 0o700); err != nil {
		return Snapshot{}, fmt.Errorf("protect resolved runtime candidate: %w", err)
	}
	if err := writePrivateFile(filepath.Join(stage, "current.yaml"), configuration); err != nil {
		return Snapshot{}, fmt.Errorf("write resolved runtime configuration: %w", err)
	}
	if err := writePrivateFile(filepath.Join(stage, "manifest.json"), append(manifestContents, '\n')); err != nil {
		return Snapshot{}, fmt.Errorf("write resolved runtime manifest: %w", err)
	}
	if err := replaceSnapshotDirectory(directory, stage); err != nil {
		return Snapshot{}, fmt.Errorf("replace resolved runtime snapshot: %w", err)
	}
	return Snapshot{Directory: directory, Configuration: configuration, Manifest: manifest}, nil
}

func (s *Store) Current(subscriptionID string) (Snapshot, error) {
	if !subscriptionIDPattern.MatchString(strings.TrimSpace(subscriptionID)) {
		return Snapshot{}, fmt.Errorf("subscription id is invalid")
	}
	directory := filepath.Join(s.root, subscriptionID)
	if err := ensureSubscriptionDirectory(directory, s.root, subscriptionID); err != nil {
		return Snapshot{}, err
	}
	readDirectory := directory
	if _, err := os.Stat(readDirectory); err != nil {
		if !os.IsNotExist(err) {
			return Snapshot{}, fmt.Errorf("inspect resolved runtime directory: %w", err)
		}
		// A process interruption between the two same-volume directory renames
		// can leave only the previous complete pair. It remains a valid fallback
		// until the next successful Save replaces it.
		readDirectory = directory + ".previous"
	}
	configuration, err := os.ReadFile(filepath.Join(readDirectory, "current.yaml"))
	if err != nil {
		return Snapshot{}, fmt.Errorf("read resolved runtime configuration: %w", err)
	}
	manifestContents, err := os.ReadFile(filepath.Join(readDirectory, "manifest.json"))
	if err != nil {
		return Snapshot{}, fmt.Errorf("read resolved runtime manifest: %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(manifestContents, &manifest); err != nil {
		return Snapshot{}, fmt.Errorf("parse resolved runtime manifest: %w", err)
	}
	if manifest.Version != manifestVersion || manifest.SubscriptionID != subscriptionID {
		return Snapshot{}, fmt.Errorf("resolved runtime manifest is incompatible")
	}
	digest := sha256.Sum256(configuration)
	if hex.EncodeToString(digest[:]) != manifest.ConfigurationSHA256 {
		return Snapshot{}, fmt.Errorf("resolved runtime configuration digest mismatch")
	}
	return Snapshot{Directory: directory, Configuration: configuration, Manifest: manifest}, nil
}

// Fingerprint identifies every input that can change a resolved runtime
// snapshot. It includes source revisions and the selected core validation
// target in addition to the normalized YAML digest.
func Fingerprint(input Input) (string, string, error) {
	if err := validateInput(input); err != nil {
		return "", "", err
	}
	digest := sha256.Sum256(input.Configuration)
	configurationDigest := hex.EncodeToString(digest[:])
	identity := struct {
		ChainProxyFingerprint        string                    `json:"chainProxyFingerprint"`
		NormalizationFingerprint     string                    `json:"normalizationFingerprint"`
		SubscriptionID               string                    `json:"subscriptionId"`
		SubscriptionRevision         string                    `json:"subscriptionRevision"`
		LocalConfigID                string                    `json:"localConfigId"`
		LocalConfigRevision          int                       `json:"localConfigRevision"`
		LocalScriptID                string                    `json:"localScriptId"`
		LocalScriptRevision          int                       `json:"localScriptRevision"`
		RuleProviderOverrideRevision int                       `json:"ruleProviderOverrideRevision"`
		FallbackOverrideRevision     int                       `json:"fallbackOverrideRevision"`
		CoreVersion                  string                    `json:"coreVersion"`
		Preferences                  runtimeconfig.Preferences `json:"preferences"`
		GeoDataFingerprint           string                    `json:"geoDataFingerprint"`
		ConfigurationSHA256          string                    `json:"configurationSha256"`
	}{
		ChainProxyFingerprint:    input.ChainProxyFingerprint,
		NormalizationFingerprint: input.NormalizationFingerprint,
		SubscriptionID:           input.SubscriptionID, SubscriptionRevision: input.SubscriptionRevision,
		LocalConfigID: input.LocalConfigID, LocalConfigRevision: input.LocalConfigRevision,
		LocalScriptID: input.LocalScriptID, LocalScriptRevision: input.LocalScriptRevision,
		RuleProviderOverrideRevision: input.RuleProviderOverrideRevision,
		FallbackOverrideRevision:     input.FallbackOverrideRevision,
		CoreVersion:                  input.CoreVersion, Preferences: input.Preferences,
		GeoDataFingerprint:  input.GeoDataFingerprint,
		ConfigurationSHA256: configurationDigest,
	}
	contents, err := json.Marshal(identity)
	if err != nil {
		return "", "", err
	}
	fingerprintDigest := sha256.Sum256(contents)
	return hex.EncodeToString(fingerprintDigest[:]), configurationDigest, nil
}

func validateInput(input Input) error {
	if input.ChainProxyFingerprint != "" && !sha256Pattern.MatchString(input.ChainProxyFingerprint) {
		return fmt.Errorf("chain proxy fingerprint is invalid")
	}
	if input.NormalizationFingerprint != "" && !sha256Pattern.MatchString(input.NormalizationFingerprint) {
		return fmt.Errorf("subscription normalization fingerprint is invalid")
	}
	if input.FallbackOverrideRevision < 0 {
		return fmt.Errorf("fallback override revision is invalid")
	}
	if !subscriptionIDPattern.MatchString(strings.TrimSpace(input.SubscriptionID)) {
		return fmt.Errorf("subscription id is invalid")
	}
	if strings.TrimSpace(input.SubscriptionRevision) == "" {
		return fmt.Errorf("subscription revision is required")
	}
	if input.RuleProviderOverrideRevision < 0 {
		return fmt.Errorf("rule provider override revision is invalid")
	}
	if input.LocalConfigID != "" && !subscriptionIDPattern.MatchString(strings.TrimSpace(input.LocalConfigID)) {
		return fmt.Errorf("local configuration id is invalid")
	}
	if input.LocalScriptID != "" && !subscriptionIDPattern.MatchString(strings.TrimSpace(input.LocalScriptID)) {
		return fmt.Errorf("local script id is invalid")
	}
	if input.LocalConfigRevision < 0 || input.LocalScriptRevision < 0 {
		return fmt.Errorf("local handler revision is invalid")
	}
	if input.LocalConfigID != "" && input.LocalScriptID != "" {
		return fmt.Errorf("local configuration and local script are mutually exclusive")
	}
	if len(input.Configuration) == 0 {
		return fmt.Errorf("resolved runtime configuration is empty")
	}
	if err := runtimeconfig.Validate(input.Preferences); err != nil {
		return err
	}
	if err := geodata.ValidatePreferences(geodata.Normalize(input.GeoDataPreferences)); err != nil {
		return err
	}
	if input.GeoDataFingerprint == "" || !sha256Pattern.MatchString(input.GeoDataFingerprint) {
		return fmt.Errorf("GEO data fingerprint is invalid")
	}
	return nil
}

func ensureSubscriptionDirectory(directory, root, subscriptionID string) error {
	if !subscriptionIDPattern.MatchString(subscriptionID) {
		return fmt.Errorf("subscription id is invalid")
	}
	relative, err := filepath.Rel(root, directory)
	if err != nil || relative != subscriptionID {
		return fmt.Errorf("resolved runtime path escapes its managed root")
	}
	return nil
}

func writePrivateFile(path string, contents []byte) error {
	temporary, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := temporary.Write(contents); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return nil
}

func replaceSnapshotDirectory(target, stage string) error {
	backup := target + ".previous"
	targetExists := false
	if _, err := os.Stat(target); err == nil {
		targetExists = true
	} else if !os.IsNotExist(err) {
		return err
	}
	if targetExists {
		if err := os.RemoveAll(backup); err != nil {
			return fmt.Errorf("remove stale resolved runtime backup: %w", err)
		}
		if err := os.Rename(target, backup); err != nil {
			return fmt.Errorf("preserve previous resolved runtime snapshot: %w", err)
		}
	}
	if err := os.Rename(stage, target); err != nil {
		if targetExists {
			if restoreErr := os.Rename(backup, target); restoreErr != nil {
				return fmt.Errorf("promote candidate: %v; restore previous snapshot: %w", err, restoreErr)
			}
		}
		return fmt.Errorf("promote resolved runtime candidate: %w", err)
	}
	// The new target is complete and valid. A stale backup is now only a
	// cleanup artifact; failure to remove it must not turn a successful atomic
	// switch into an activation failure.
	_ = os.RemoveAll(backup)
	return nil
}
