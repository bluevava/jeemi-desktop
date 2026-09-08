package runtime

import (
	"fmt"
	"os"

	"jeemi/internal/config/runtimecontrol"
)

// ConfigurationView is a controller-session-free copy of the active generation.
// Paths and controller-session details deliberately stay inside the runtime
// manager so the Wails boundary cannot turn this into arbitrary file access.
type ConfigurationView struct {
	GenerationID string
	CoreVersion  string
	Source       Source
	Contents     []byte
}

func (m *Manager) ActiveConfiguration() (ConfigurationView, error) {
	m.operationMu.Lock()
	defer m.operationMu.Unlock()

	m.mu.RLock()
	if m.status.State != StateRunning || !m.status.ControllerReady || m.active == nil {
		m.mu.RUnlock()
		return ConfigurationView{}, fmt.Errorf("mihomo has no active runtime generation")
	}
	active := *m.active
	m.mu.RUnlock()

	contents, err := os.ReadFile(active.ConfigPath)
	if err != nil {
		return ConfigurationView{}, fmt.Errorf("read active runtime configuration: %w", err)
	}
	safeContents, err := runtimecontrol.StripSessionFields(contents)
	if err != nil {
		return ConfigurationView{}, fmt.Errorf("remove active controller session fields: %w", err)
	}
	return ConfigurationView{
		GenerationID: active.ID,
		CoreVersion:  active.CoreVersion,
		Source:       active.Source,
		Contents:     safeContents,
	}, nil
}

// ActiveStartRequest returns a credential-free in-memory restart point for
// application-level asset transactions. It is intentionally not exposed by
// the Wails binding surface.
func (m *Manager) ActiveStartRequest() (StartRequest, error) {
	m.operationMu.Lock()
	defer m.operationMu.Unlock()

	m.mu.RLock()
	if m.status.State != StateRunning || !m.status.ControllerReady || m.active == nil {
		m.mu.RUnlock()
		return StartRequest{}, fmt.Errorf("mihomo has no active runtime generation")
	}
	active := *m.active
	m.mu.RUnlock()
	contents, err := os.ReadFile(active.ConfigPath)
	if err != nil {
		return StartRequest{}, fmt.Errorf("read active runtime configuration: %w", err)
	}
	safeContents, err := runtimecontrol.StripSessionFields(contents)
	if err != nil {
		return StartRequest{}, fmt.Errorf("remove active controller session fields: %w", err)
	}
	return StartRequest{
		ExecutablePath: active.ExecutablePath,
		CoreVersion:    active.CoreVersion,
		Configuration:  safeContents,
		Preferences:    active.Preferences,
		Source:         active.Source,
	}, nil
}
