package runtime

import (
	"context"

	"jeemi/internal/runtimeconfig"
)

func (m *Manager) CheckPrerequisites(ctx context.Context, request StartRequest, restarting bool) error {
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	return m.preflight(ctx, request, restarting)
}

func (m *Manager) preflight(ctx context.Context, request StartRequest, restarting bool) error {
	if checker, ok := m.driver.(interface {
		Preflight(string, runtimeconfig.Preferences, int) error
	}); ok {
		m.mu.RLock()
		pid := 0
		if !restarting && m.process != nil && m.active != nil && sameExecutable(m.goos, m.active.ExecutablePath, request.ExecutablePath) {
			pid = m.process.PID()
		}
		m.mu.RUnlock()
		if err := checker.Preflight(request.ExecutablePath, request.Preferences, pid); err != nil {
			return err
		}
	}
	if request.Preferences.ProxyMode == runtimeconfig.ProxyModeSystemProxy {
		if checker, ok := m.systemProxy.(interface{ Check(context.Context) error }); ok {
			return checker.Check(ctx)
		}
	}
	return nil
}
