package runtime

import (
	"context"
	"sync"
	"time"

	"jeemi/internal/platform/requirements"
)

// HoldHelperRecovery keeps an existing core running during the native prompt,
// but its planned exit must not become an automatic restart. The release may
// restore normal recovery only for the same, verifiably live process. A caller
// cancelled during an installer transaction cannot assume the installer ended.
func (m *Manager) HoldHelperRecovery() func(bool) {
	m.operationMu.Lock()
	m.mu.Lock()
	process := m.process
	m.recoveryBlocked = process
	if process == nil && m.status.State == StateRecovering {
		m.status.DesiredRunning = false
		m.status.State = StateStopped
	}
	m.mu.Unlock()
	m.operationMu.Unlock()
	var once sync.Once
	return func(resumeHealthy bool) {
		once.Do(func() {
			if !resumeHealthy || process == nil {
				return
			}
			m.operationMu.Lock()
			defer m.operationMu.Unlock()
			select {
			case <-process.Done():
				m.processExitedLocked(process)
				return
			default:
			}
			// Service process watchers poll. Confirm directly so a delayed Done
			// signal cannot revive the old generation after a helper replacement.
			if probe, ok := process.(interface {
				IsRunning(context.Context) (bool, error)
			}); ok {
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				alive, err := probe.IsRunning(ctx)
				cancel()
				if err != nil {
					return
				}
				if !alive {
					m.processExitedLocked(process)
					return
				}
			}
			m.mu.Lock()
			defer m.mu.Unlock()
			if m.process == process && m.recoveryBlocked == process {
				m.recoveryBlocked = nil
			}
		})
	}
}

// RetryStartupRecovery is used by the primary GUI after explicit authorization
// or helper repair/removal. An unavailable helper must not permanently poison
// all later starts. Never erase the session of a currently managed process.
func (m *Manager) RetryStartupRecovery(ctx context.Context) error {
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	m.mu.RLock()
	needed := m.startupFailure != nil && m.process == nil
	m.mu.RUnlock()
	if !needed {
		return nil
	}
	return m.recoverSystemProxyLocked(ctx)
}

func startupAuthorizationRequired(err error) bool {
	switch requirements.Code(err, "") {
	case "authorization_install_required", "authorization_update_required",
		"authorization_repair_required", "authorization_bundle_required":
		return true
	default:
		return false
	}
}
