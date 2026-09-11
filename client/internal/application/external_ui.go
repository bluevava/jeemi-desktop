package application

import (
	"context"
	"fmt"
	"time"

	"jeemi/internal/externalui"
)

type ZashboardState struct {
	externalui.State
	SelectedVersion string `json:"selectedVersion"`
	Enabled         bool   `json:"enabled"`
}

func (s *Service) ZashboardState() (ZashboardState, error) {
	preferences, err := s.RuntimePreferences()
	if err != nil {
		return ZashboardState{}, err
	}
	return ZashboardState{State: s.zashboard.State(), SelectedVersion: preferences.ExternalUIVersion, Enabled: preferences.ExternalUIEnabled}, nil
}

func (s *Service) CheckZashboardUpdates() (ZashboardState, error) {
	ctx, cancel := s.operationContext(time.Minute)
	defer cancel()
	if err := s.zashboard.Check(ctx); err != nil {
		return ZashboardState{}, err
	}
	return s.ZashboardState()
}

func (s *Service) DownloadZashboardVersion(version string) (ZashboardState, error) {
	ctx, cancel := s.operationContext(3 * time.Minute)
	defer cancel()
	installed, err := s.zashboard.Download(ctx, version)
	if err != nil {
		return ZashboardState{}, err
	}
	// Repairing the already desired resources retries the pending activation.
	// Downloading another version only stages it for the Settings confirmation.
	s.reconcileMu.Lock()
	preferences, loadErr := s.RuntimePreferences()
	if loadErr == nil && preferences.ExternalUIEnabled && (preferences.ExternalUIVersion == "" || preferences.ExternalUIVersion == installed) {
		_ = s.reconcileSelectedWithTimeoutLocked("external_ui", nil, reconcileAutomatic)
	}
	s.reconcileMu.Unlock()
	return s.ZashboardState()
}

func (s *Service) CancelZashboardDownload() { s.zashboard.Cancel() }

// Selecting a downloaded release preserves the independently saved Home switch.
// Only the Settings page's confirmation calls this method.
func (s *Service) SelectZashboardVersion(version string) (ZashboardState, error) {
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()
	if _, err := s.zashboard.Installed(version); err != nil {
		return ZashboardState{}, err
	}
	preferences, err := s.RuntimePreferences()
	if err != nil {
		return ZashboardState{}, err
	}
	preferences.ExternalUIVersion = version
	if _, err := s.settingsStore.SetRuntimePreferences(preferences); err != nil {
		return ZashboardState{}, err
	}
	_ = s.reconcileSelectedWithTimeoutLocked("external_ui", nil, reconcileAutomatic)
	return s.ZashboardState()
}

// Run under reconcileMu before composition. Ordinary State/projection reads
// never download resources. Once a release is selected, starts remain offline.
func (s *Service) prepareExternalUI(ctx context.Context) error {
	preferences, err := s.RuntimePreferences()
	if err != nil || !preferences.ExternalUIEnabled {
		return err
	}
	if s.zashboard == nil {
		return fmt.Errorf("zashboard manager is unavailable")
	}
	version, err := s.zashboard.Ensure(ctx, preferences.ExternalUIVersion)
	if err != nil {
		return err
	}
	if version != preferences.ExternalUIVersion {
		preferences.ExternalUIVersion = version
		_, err = s.settingsStore.SetRuntimePreferences(preferences)
	}
	return err
}

func (s *Service) OpenZashboard(open func(string)) error {
	return s.runtimeManager.OpenExternalUI(open)
}
