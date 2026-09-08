// Package systemproxy applies and restores the operating-system proxy through
// a narrow, crash-recoverable adapter. It never controls mihomo itself.
package systemproxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"jeemi/internal/platform/paths"
	"jeemi/internal/runtimeconfig"
)

type Controller interface {
	Apply(context.Context, runtimeconfig.Preferences) error
	Restore(context.Context) error
	Recover(context.Context) error
}

type snapshot struct {
	Backend               string            `json:"backend,omitempty"`
	DesktopValues         map[string]string `json:"desktopValues,omitempty"`
	ProxyEnableExists     bool              `json:"proxyEnableExists"`
	ProxyEnable           uint64            `json:"proxyEnable"`
	ProxyServerExists     bool              `json:"proxyServerExists"`
	ProxyServer           string            `json:"proxyServer"`
	ProxyOverrideCaptured bool              `json:"proxyOverrideCaptured,omitempty"`
	ProxyOverrideExists   bool              `json:"proxyOverrideExists,omitempty"`
	ProxyOverride         string            `json:"proxyOverride,omitempty"`
}

type recoveryState struct {
	Version  int      `json:"version"`
	Snapshot snapshot `json:"snapshot"`
}

type backend interface {
	Capture(context.Context) (snapshot, error)
	Apply(context.Context, string) error
	Restore(context.Context, snapshot) error
	Notify(context.Context) error
}

type manager struct {
	mu      sync.Mutex
	path    string
	backend backend
}

func New(dataDirectory string) (Controller, error) {
	runtimeDirectory, err := paths.MihomoRuntimeDirectoryFromRoot(dataDirectory)
	if err != nil {
		return nil, err
	}
	return newPlatformController(filepath.Join(runtimeDirectory, "system-proxy-recovery.json")), nil
}

func newManager(path string, platformBackend backend) *manager {
	return &manager{path: path, backend: platformBackend}
}

func (m *manager) Check(ctx context.Context) error {
	if checker, ok := m.backend.(interface{ Check(context.Context) error }); ok {
		return checker.Check(ctx)
	}
	return nil
}

func (m *manager) Apply(ctx context.Context, preferences runtimeconfig.Preferences) error {
	if err := runtimeconfig.Validate(preferences); err != nil {
		return err
	}
	if preferences.ProxyMode != runtimeconfig.ProxyModeSystemProxy {
		return fmt.Errorf("system proxy can only be applied in system proxy mode")
	}
	proxyServer, err := proxyServerValue(preferences.ListenerType, preferences.ListenPort)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	state, exists, err := m.loadState()
	if err != nil {
		return err
	}
	if !exists {
		captured, err := m.backend.Capture(ctx)
		if err != nil {
			return fmt.Errorf("capture system proxy state: %w", err)
		}
		state = recoveryState{Version: 2, Snapshot: captured}
		if err := m.saveState(state); err != nil {
			return err
		}
	}
	if err := m.backend.Apply(ctx, proxyServer); err != nil {
		return errors.Join(fmt.Errorf("apply system proxy: %w", err), m.rollback(state))
	}
	if err := m.backend.Notify(ctx); err != nil {
		return errors.Join(fmt.Errorf("notify system proxy change: %w", err), m.rollback(state))
	}
	return nil
}

func (m *manager) Recover(ctx context.Context) error {
	return m.Restore(ctx)
}

func (m *manager) Restore(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	state, exists, err := m.loadState()
	if err != nil || !exists {
		return err
	}
	return m.restoreState(ctx, state)
}

// A cancelled apply still needs a bounded, independent rollback. Never discard
// the crash recovery record until both restoration and notification succeed.
func (m *manager) rollback(state recoveryState) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return m.restoreState(ctx, state)
}

func (m *manager) restoreState(ctx context.Context, state recoveryState) error {
	if err := m.backend.Restore(ctx, state.Snapshot); err != nil {
		return fmt.Errorf("restore system proxy: %w", err)
	}
	if err := m.backend.Notify(ctx); err != nil {
		return fmt.Errorf("notify restored system proxy: %w", err)
	}
	if err := os.Remove(m.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove system proxy recovery state: %w", err)
	}
	return nil
}

func (m *manager) loadState() (recoveryState, bool, error) {
	contents, err := os.ReadFile(m.path)
	if errors.Is(err, os.ErrNotExist) {
		return recoveryState{}, false, nil
	}
	if err != nil {
		return recoveryState{}, false, fmt.Errorf("read system proxy recovery state: %w", err)
	}
	var state recoveryState
	if err := json.Unmarshal(contents, &state); err != nil || (state.Version != 1 && state.Version != 2) {
		return recoveryState{}, false, fmt.Errorf("system proxy recovery state is invalid")
	}
	return state, true, nil
}

func (m *manager) saveState(state recoveryState) error {
	if err := os.MkdirAll(filepath.Dir(m.path), 0o700); err != nil {
		return fmt.Errorf("create system proxy recovery directory: %w", err)
	}
	contents, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	contents = append(contents, '\n')
	temporary, err := os.CreateTemp(filepath.Dir(m.path), ".system-proxy-*.tmp")
	if err != nil {
		return fmt.Errorf("create system proxy recovery state: %w", err)
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
	if err := os.Rename(temporaryPath, m.path); err != nil {
		return fmt.Errorf("activate system proxy recovery state: %w", err)
	}
	return nil
}

func proxyServerValue(listenerType string, port int) (string, error) {
	endpoint := fmt.Sprintf("127.0.0.1:%d", port)
	switch listenerType {
	case runtimeconfig.ListenerTypeHTTP:
		return "http=" + endpoint + ";https=" + endpoint, nil
	case runtimeconfig.ListenerTypeSOCKS:
		return "socks=" + endpoint, nil
	case runtimeconfig.ListenerTypeMixed:
		return "http=" + endpoint + ";https=" + endpoint + ";socks=" + endpoint, nil
	default:
		return "", fmt.Errorf("system proxy listener type is invalid")
	}
}
