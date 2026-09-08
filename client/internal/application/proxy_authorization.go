package application

import (
	"context"
	"jeemi/internal/platform/authorization"
	"jeemi/internal/platform/coreauth"
	"jeemi/internal/platform/requirements"
	"time"
)

func (s *Service) authorizationTarget() *coreauth.Target {
	installation, err := s.selectedMihomoInstallation()
	if err != nil {
		return nil
	}
	return &coreauth.Target{ExecutablePath: installation.ExecutablePath, SHA256: installation.BinarySHA256, Size: installation.BinarySize}
}

func (s *Service) GetProxyAuthorization() (authorization.Status, error) {
	ctx, cancel := s.operationContext(8 * time.Second)
	defer cancel()
	return authorization.Inspect(ctx, s.authorizationTarget(), s.versionManager.CoreDirectory())
}

// Helper health is independent of the selected core and whether the proxy is
// running. Inspect without a target to avoid reporting core permissions here.
func (s *Service) GetAuthorizationHelperStatus() (authorization.Status, error) {
	ctx, cancel := s.operationContext(8 * time.Second)
	defer cancel()
	return authorization.Inspect(ctx, nil, s.versionManager.CoreDirectory())
}

func (s *Service) SetupProxyAuthorization() (authorization.Status, error) {
	ctx, cancel := s.operationContext(3 * time.Minute)
	defer cancel()
	s.mu.Lock()
	if s.shuttingDown || s.coreOperationCancel != nil {
		s.mu.Unlock()
		return authorization.Status{}, &requirements.Error{Code: "authorization_busy", Message: "authorization_busy"}
	}
	s.coreOperationCancel = cancel
	s.runtime.CoreAuthorizationPending = true
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
		return authorization.Status{}, context.Canceled
	}
	releaseRecovery := s.runtimeManager.HoldHelperRecovery()
	defer func() { releaseRecovery(ctx.Err() == nil) }()
	if err := authorization.Setup(ctx, s.authorizationTarget()); err != nil {
		return authorization.Status{}, err
	}
	if ctx.Err() != nil {
		return authorization.Status{}, context.Canceled
	}
	state, err := authorization.Inspect(ctx, s.authorizationTarget(), s.versionManager.CoreDirectory())
	if err == nil && state.Ready {
		err = s.runtimeManager.RetryStartupRecovery(ctx)
	}
	return state, err
}

// Removal is idempotent and independent of helper liveness. Stop restores the
// user's networking before any authorization is revoked. Failed cleanup must
// not be presented as success.
func (s *Service) RemoveAuthorizationHelper() (authorization.Status, error) {
	s.cancelCoreOperation()
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()
	ctx, cancel := s.operationContext(3 * time.Minute)
	defer cancel()
	directory := s.versionManager.CoreDirectory()
	state, err := authorization.Inspect(ctx, nil, directory)
	if err != nil {
		return state, err
	}
	if !state.Present {
		return state, nil
	}
	if _, err := s.runtimeManager.Stop(ctx); err != nil {
		// These installers independently restore their protected network
		// journal before deleting registration. An unreachable old helper must
		// not prevent reaching that recovery path. Linux cannot substitute its
		// authorization service for the ordinary user's GNOME restoration.
		if state.Platform != "windows" && !(state.Platform == "darwin" && state.LocalTest) {
			return state, err
		}
	}
	if err := authorization.Remove(ctx, directory); err != nil {
		return state, err
	}
	state, err = authorization.Inspect(ctx, nil, directory)
	if err == nil && state.Present {
		err = &requirements.Error{Code: "authorization_cleanup_incomplete", Message: "authorization_cleanup_incomplete"}
	}
	if err == nil {
		_, err = s.runtimeManager.Stop(ctx)
		if err == nil {
			err = s.runtimeManager.RetryStartupRecovery(ctx)
		}
	}
	return state, err
}

func (s *Service) OpenProxyAuthorizationSettings() error {
	ctx, cancel := s.operationContext(8 * time.Second)
	defer cancel()
	return authorization.OpenSettings(ctx)
}
