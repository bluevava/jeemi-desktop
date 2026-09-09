package application

import mihomoruntime "jeemi/internal/mihomo/runtime"

type ProxyControlStatus struct {
	Running bool
	Busy    bool
}

// ProxyControlsStatus is safe to sample while the window is hidden. It does
// not load settings, inspect permissions, capture memory or call mihomo's API.
func (s *Service) ProxyControlsStatus() ProxyControlStatus {
	s.mu.RLock()
	busy := s.shuttingDown || s.coreOperationCancel != nil || s.runtime.CoreAuthorizationPending
	configuration := s.runtime.Configuration.State
	s.mu.RUnlock()
	return proxyControlStatus(s.runtimeManager.Status().State, configuration, busy)
}

func proxyControlStatus(state mihomoruntime.State, configuration RuntimeConfigurationState, busy bool) ProxyControlStatus {
	switch configuration {
	case RuntimeConfigurationBuilding, RuntimeConfigurationValidating, RuntimeConfigurationApplying:
		busy = true
	}
	switch state {
	case mihomoruntime.StateRunning:
		return ProxyControlStatus{Running: true, Busy: busy}
	case mihomoruntime.StateStopped, mihomoruntime.StateFailed:
		return ProxyControlStatus{Busy: busy}
	default:
		return ProxyControlStatus{Busy: true}
	}
}
