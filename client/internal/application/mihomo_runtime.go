package application

import (
	"fmt"
	"time"

	"jeemi/internal/core"
)

func (s *Service) StartProxy() (RuntimeStatus, error) {
	return s.runAuthorizedProxy(reconcileTriggerManualStart, reconcileStart)
}

func (s *Service) StopProxy() (RuntimeStatus, error) {
	s.cancelCoreOperation()
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()
	ctx, cancel := s.operationContext(15 * time.Second)
	defer cancel()
	if _, err := s.runtimeManager.Stop(ctx); err != nil {
		return s.RuntimeStatus(), err
	}
	s.updateConfigurationStatus(func(status *RuntimeConfigurationStatus) {
		status.AppliedFingerprint = ""
		if status.State != RuntimeConfigurationFailed {
			if status.DesiredFingerprint == "" {
				status.State = RuntimeConfigurationIdle
			} else {
				status.State = RuntimeConfigurationReady
			}
		}
		status.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	})
	return s.RuntimeStatus(), nil
}

// RestartProxy always creates a fresh mihomo process session. The desired
// resolved snapshot is built and validated before the current process is
// stopped, so an invalid edit cannot tear down a healthy runtime.
func (s *Service) RestartProxy() (RuntimeStatus, error) {
	return s.runAuthorizedProxy(reconcileTriggerManualRestart, reconcileRestart)
}

func (s *Service) SelectRuntimeProxy(group, proxy string) error {
	ctx, cancel := s.operationContext(8 * time.Second)
	defer cancel()
	return s.runtimeManager.SelectProxy(ctx, group, proxy)
}

func (s *Service) RememberRuntimeProxySelection(sessionID, subscriptionID, group, proxy string) error {
	ctx, cancel := s.operationContext(8 * time.Second)
	defer cancel()
	return s.runtimeManager.RememberProxySelection(ctx, sessionID, subscriptionID, group, proxy)
}

func (s *Service) UpdateRuntimeRuleProvider(name string) error {
	ctx, cancel := s.operationContext(35 * time.Second)
	defer cancel()
	return s.runtimeManager.UpdateRuleProvider(ctx, name)
}

func (s *Service) UpdateRuntimeProxyProvider(name string) error {
	ctx, cancel := s.operationContext(35 * time.Second)
	defer cancel()
	return s.runtimeManager.UpdateProxyProvider(ctx, name)
}

func (s *Service) selectedMihomoInstallation() (core.InstalledVersion, error) {
	state, err := s.versionManager.State()
	if err != nil {
		return core.InstalledVersion{}, err
	}
	if state.SelectedVersion == "" {
		return core.InstalledVersion{}, fmt.Errorf("select an installed mihomo version before starting")
	}
	for _, installed := range state.InstalledVersions {
		if installed.Version == state.SelectedVersion {
			return installed, nil
		}
	}
	return core.InstalledVersion{}, fmt.Errorf("selected mihomo version is unavailable or failed integrity verification")
}
