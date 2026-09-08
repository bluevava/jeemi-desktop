package application

import (
	"jeemi/internal/platform/macnetwork"
	"time"
)

func (s *Service) GetMacNetworkAuthorization() macnetwork.Status {
	ctx, cancel := s.operationContext(5 * time.Second)
	defer cancel()
	return macnetwork.Inspect(ctx)
}
func (s *Service) SetupMacNetworkAuthorization() (macnetwork.Status, error) {
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()
	ctx, cancel := s.operationContext(180 * time.Second)
	defer cancel()
	return macnetwork.Setup(ctx)
}
func (s *Service) OpenMacNetworkSettings() error       { return macnetwork.OpenSettings() }
func (s *Service) OpenMacApplicationsDirectory() error { return macnetwork.OpenApplications() }
func (s *Service) RemoveMacNetworkHelper() (macnetwork.Status, error) {
	s.cancelCoreOperation()
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()
	ctx, cancel := s.operationContext(180 * time.Second)
	defer cancel()
	if _, err := s.runtimeManager.Stop(ctx); err != nil {
		return macnetwork.Inspect(ctx), err
	}
	return macnetwork.Unregister(ctx)
}
