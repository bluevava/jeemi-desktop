package application

import (
	"context"
	"errors"
	"time"

	"jeemi/internal/platform/coreauth"
	"jeemi/internal/platform/requirements"
)

// Both proxy modes use this explicit-action entry point. Passive composition,
// automatic recovery and high-frequency API actions never show password UI.
func (s *Service) runAuthorizedProxy(trigger string, mode reconciliationMode) (RuntimeStatus, error) {
	ctx, cancel := s.operationContext(3 * time.Minute)
	defer cancel()
	s.mu.Lock()
	if s.shuttingDown {
		s.mu.Unlock()
		return s.RuntimeStatus(), context.Canceled
	}
	if s.coreOperationCancel != nil {
		s.mu.Unlock()
		return s.RuntimeStatus(), nil
	}
	s.coreOperationCancel = cancel
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.coreOperationCancel = nil
		s.runtime.CoreAuthorizationPending = false
		s.mu.Unlock()
	}()
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()
	if ctx.Err() != nil {
		return s.RuntimeStatus(), nil
	}
	if installation, err := s.selectedMihomoInstallation(); err == nil && s.authorizeCore != nil {
		s.mu.Lock()
		s.runtime.CoreAuthorizationPending = s.runtime.Platform.OS == "linux" || s.runtime.Platform.OS == "darwin" || s.runtime.Platform.OS == "windows"
		s.mu.Unlock()
		err = s.authorizeCore(ctx, coreauth.Target{
			ExecutablePath: installation.ExecutablePath,
			SHA256:         installation.BinarySHA256, Size: installation.BinarySize,
		})
		s.mu.Lock()
		s.runtime.CoreAuthorizationPending = false
		s.mu.Unlock()
		if errors.Is(err, coreauth.ErrCancelled) || errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
			return s.RuntimeStatus(), nil
		}
		if err != nil {
			_ = s.failRuntimeConfiguration(trigger, "authorize", requirements.Code(err, "core_authorization_failed"), err, nil)
			return s.RuntimeStatus(), err
		}
	}
	if ctx.Err() != nil {
		return s.RuntimeStatus(), nil
	}
	// The helper may have been updated outside this GUI. Complete any deferred
	// startup recovery after authorization, before creating a new generation.
	if err := s.runtimeManager.RetryStartupRecovery(ctx); err != nil {
		return s.RuntimeStatus(), err
	}
	if err := s.reconcileSelectedLocked(ctx, trigger, nil, mode); err != nil {
		return s.RuntimeStatus(), err
	}
	return s.RuntimeStatus(), nil
}

// CancelCoreAuthorization cancels only the pending authentication action. It
// does not stop an existing healthy core or modify the selected preferences.
func (s *Service) CancelCoreAuthorization() {
	s.mu.RLock()
	cancel := s.coreOperationCancel
	pending := s.runtime.CoreAuthorizationPending
	s.mu.RUnlock()
	if pending && cancel != nil {
		cancel()
	}
}

func (s *Service) cancelCoreOperation() {
	s.mu.RLock()
	cancel := s.coreOperationCancel
	s.mu.RUnlock()
	if cancel != nil {
		cancel()
	}
}
