package runtime

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"jeemi/internal/config/dnstool"
	"jeemi/internal/config/document"
	"jeemi/internal/config/runtimecontrol"
	"jeemi/internal/platform/paths"
	"jeemi/internal/runtimeconfig"

	"gopkg.in/yaml.v3"
)

type generationStore struct {
	root        string
	generations string
	driver      Driver
	now         func() time.Time
}

type generationManifest struct {
	Version             int                       `json:"version"`
	ID                  string                    `json:"id"`
	CreatedAt           string                    `json:"createdAt"`
	CoreVersion         string                    `json:"coreVersion"`
	ConfigurationSHA256 string                    `json:"configurationSha256"`
	Source              Source                    `json:"source"`
	Preferences         runtimeconfig.Preferences `json:"preferences"`
}

type activationState struct {
	Version              int    `json:"version"`
	ActiveGenerationID   string `json:"activeGenerationId"`
	PreviousGenerationID string `json:"previousGenerationId"`
	ActivatedAt          string `json:"activatedAt"`
}

func newGenerationStore(dataDirectory string, driver Driver, now func() time.Time) (*generationStore, error) {
	root, err := paths.MihomoRuntimeDirectoryFromRoot(dataDirectory)
	if err != nil {
		return nil, err
	}
	if driver == nil {
		driver = newCommandDriver()
	}
	if now == nil {
		now = time.Now
	}
	return &generationStore{root: root, generations: filepath.Join(root, "generations"), driver: driver, now: now}, nil
}

func (s *generationStore) Prepare(ctx context.Context, request StartRequest, existingSession *ControllerSession) (Generation, error) {
	if err := runtimeconfig.Validate(request.Preferences); err != nil {
		return Generation{}, err
	}
	if len(request.Configuration) == 0 || strings.TrimSpace(request.CoreVersion) == "" {
		return Generation{}, fmt.Errorf("runtime configuration and core version are required")
	}
	session, err := s.session(existingSession)
	if err != nil {
		return Generation{}, err
	}
	address := strings.TrimPrefix(session.BaseURL, "http://")
	controlled, err := runtimecontrol.Apply(request.Configuration, runtimecontrol.Options{
		Address:        address,
		Secret:         session.Secret,
		AllowedOrigins: runtimecontrol.DefaultAllowedOrigins(),
	})
	if err != nil {
		return Generation{}, err
	}
	controlled, dnsSession, err := dnstool.Prepare(controlled, session.Secret)
	if err != nil {
		return Generation{}, err
	}
	bootstrap, err := bootstrapConfiguration(controlled)
	if err != nil {
		return Generation{}, err
	}
	digest := sha256.Sum256(controlled)
	id := s.now().UTC().Format("20060102T150405.000000000Z") + "-" + hex.EncodeToString(digest[:6])

	if err := os.MkdirAll(s.generations, 0o700); err != nil {
		return Generation{}, fmt.Errorf("create mihomo generations directory: %w", err)
	}
	stage, err := os.MkdirTemp(s.generations, ".candidate-")
	if err != nil {
		return Generation{}, fmt.Errorf("create mihomo generation candidate: %w", err)
	}
	defer os.RemoveAll(stage)
	if err := os.Chmod(stage, 0o700); err != nil {
		return Generation{}, err
	}
	configPath := filepath.Join(stage, "config.yaml")
	bootstrapPath := filepath.Join(stage, "bootstrap.yaml")
	if err := writePrivateFile(configPath, controlled); err != nil {
		return Generation{}, err
	}
	if err := writePrivateFile(bootstrapPath, bootstrap); err != nil {
		return Generation{}, err
	}
	manifest := generationManifest{
		Version: 1, ID: id, CreatedAt: s.now().UTC().Format(time.RFC3339Nano),
		CoreVersion: request.CoreVersion, ConfigurationSHA256: hex.EncodeToString(digest[:]),
		Source: request.Source, Preferences: request.Preferences,
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return Generation{}, err
	}
	if err := writePrivateFile(filepath.Join(stage, "generation.json"), append(manifestBytes, '\n')); err != nil {
		return Generation{}, err
	}
	if err := s.driver.Validate(ctx, request.ExecutablePath, s.root, configPath); err != nil {
		return Generation{}, err
	}
	if string(bootstrap) != string(controlled) {
		if err := s.driver.Validate(ctx, request.ExecutablePath, s.root, bootstrapPath); err != nil {
			return Generation{}, fmt.Errorf("validate mihomo bootstrap configuration: %w", err)
		}
	}
	target := filepath.Join(s.generations, id)
	if err := ensureDescendant(target, s.generations); err != nil {
		return Generation{}, err
	}
	if err := os.Rename(stage, target); err != nil {
		return Generation{}, fmt.Errorf("promote mihomo generation: %w", err)
	}
	return Generation{
		ID: id, Directory: target,
		ConfigPath: filepath.Join(target, "config.yaml"), BootstrapPath: filepath.Join(target, "bootstrap.yaml"),
		CoreVersion: request.CoreVersion, ExecutablePath: request.ExecutablePath,
		Preferences: request.Preferences, Source: request.Source, Session: session, DNS: dnsSession,
	}, nil
}

func (s *generationStore) ValidateResolved(ctx context.Context, executablePath string, configuration []byte) error {
	if len(configuration) == 0 {
		return fmt.Errorf("resolved runtime configuration is empty")
	}
	if err := os.MkdirAll(s.root, 0o700); err != nil {
		return fmt.Errorf("create mihomo runtime directory: %w", err)
	}
	temporary, err := os.CreateTemp(s.root, ".resolved-validation-*.yaml")
	if err != nil {
		return fmt.Errorf("create resolved runtime validation file: %w", err)
	}
	path := temporary.Name()
	defer os.Remove(path)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(configuration); err != nil {
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
	if err := s.driver.Validate(ctx, executablePath, s.root, path); err != nil {
		return fmt.Errorf("validate resolved runtime configuration: %w", err)
	}
	return nil
}

func (s *generationStore) Activate(generation Generation) error {
	statePath := filepath.Join(s.root, "active-generation.json")
	previous := ""
	if contents, err := os.ReadFile(statePath); err == nil {
		var current activationState
		if json.Unmarshal(contents, &current) == nil && current.Version == 1 {
			previous = current.ActiveGenerationID
		}
	}
	state := activationState{
		Version: 1, ActiveGenerationID: generation.ID,
		PreviousGenerationID: previous, ActivatedAt: s.now().UTC().Format(time.RFC3339Nano),
	}
	contents, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return replacePrivateFile(statePath, append(contents, '\n'))
}

// EraseSessionFiles removes only generated files that contain the supplied
// runtime secret. Generation manifests intentionally exclude the secret and
// remain available as a source/audit record.
func (s *generationStore) EraseSessionFiles(session ControllerSession) error {
	if len(session.Secret) < 32 {
		return fmt.Errorf("refuse to erase files for an invalid controller session")
	}
	entries, err := os.ReadDir(s.generations)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read mihomo generations for session cleanup: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		directory := filepath.Join(s.generations, entry.Name())
		if err := ensureDescendant(directory, s.generations); err != nil {
			return err
		}
		containsSecret := false
		for _, name := range []string{"config.yaml", "bootstrap.yaml"} {
			contents, readErr := os.ReadFile(filepath.Join(directory, name))
			if readErr == nil && bytes.Contains(contents, []byte(session.Secret)) {
				containsSecret = true
				break
			}
			if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
				return fmt.Errorf("read generated mihomo session file: %w", readErr)
			}
		}
		if containsSecret {
			if err := eraseGeneratedSessionFiles(directory); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *generationStore) EraseGenerationSessionFiles(generation Generation) error {
	if err := ensureDescendant(generation.Directory, s.generations); err != nil {
		return err
	}
	return eraseGeneratedSessionFiles(generation.Directory)
}

// EraseAllSessionFiles is used only before a new runtime manager accepts
// operations. At that point every prior control session is stale.
func (s *generationStore) EraseAllSessionFiles() error {
	entries, err := os.ReadDir(s.generations)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read stale mihomo generations: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		directory := filepath.Join(s.generations, entry.Name())
		if err := ensureDescendant(directory, s.generations); err != nil {
			return err
		}
		if err := eraseGeneratedSessionFiles(directory); err != nil {
			return err
		}
	}
	return nil
}

func eraseGeneratedSessionFiles(directory string) error {
	for _, name := range []string{"config.yaml", "bootstrap.yaml"} {
		if err := os.Remove(filepath.Join(directory, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("erase generated mihomo session file: %w", err)
		}
	}
	return nil
}

func (s *generationStore) session(existing *ControllerSession) (ControllerSession, error) {
	if existing != nil {
		parsed, err := url.Parse(existing.BaseURL)
		if err != nil || parsed.Scheme != "http" || parsed.Path != "" {
			return ControllerSession{}, fmt.Errorf("existing mihomo controller session is invalid")
		}
		host, _, err := net.SplitHostPort(parsed.Host)
		if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() || len(existing.Secret) < 32 {
			return ControllerSession{}, fmt.Errorf("existing mihomo controller session is unsafe")
		}
		return *existing, nil
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return ControllerSession{}, fmt.Errorf("reserve mihomo controller address: %w", err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		return ControllerSession{}, err
	}
	secret, err := randomHex(32)
	if err != nil {
		return ControllerSession{}, err
	}
	id, err := randomHex(16)
	if err != nil {
		return ControllerSession{}, err
	}
	return ControllerSession{ID: id, BaseURL: "http://" + address, Secret: secret}, nil
}

func bootstrapConfiguration(final []byte) ([]byte, error) {
	configuration, err := document.Parse(final)
	if err != nil {
		return nil, err
	}
	tun, found, err := document.Find(document.Root(configuration), "/tun/enable")
	if err != nil || !found || tun.Value != "true" {
		return append([]byte{}, final...), err
	}
	if err := document.SetMappingPath(document.Root(configuration), "/tun/enable", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "false"}); err != nil {
		return nil, err
	}
	return document.Encode(configuration)
}

func randomHex(bytesCount int) (string, error) {
	contents := make([]byte, bytesCount)
	if _, err := rand.Read(contents); err != nil {
		return "", err
	}
	return hex.EncodeToString(contents), nil
}

func writePrivateFile(path string, contents []byte) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create private runtime file: %w", err)
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

func replacePrivateFile(path string, contents []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".runtime-state-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
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
	backup := path + ".previous"
	_ = os.Remove(backup)
	if _, err := os.Stat(path); err == nil {
		if err := os.Rename(path, backup); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		_ = os.Rename(backup, path)
		return err
	}
	_ = os.Remove(backup)
	return nil
}

func ensureDescendant(path, root string) error {
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("runtime path escapes the managed generation root")
	}
	return nil
}
