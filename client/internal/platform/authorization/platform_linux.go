//go:build linux

package authorization

import (
	"context"
	"jeemi/internal/platform/coreauth"
	"jeemi/internal/platform/requirements"
)

func Inspect(ctx context.Context, target *coreauth.Target, directory string) (Status, error) {
	service, err := coreauth.ServiceStatus(ctx)
	if err != nil {
		return Status{}, err
	}
	s := Status{Platform: "linux", Kind: "service", Ready: service.Ready, Present: service.Present, Code: service.Code, Action: service.Action, Steps: []Step{{"service", service.Ready}}}
	targets, err := coreauth.CleanupTargets(directory)
	if err != nil {
		return s, err
	}
	legacy, err := coreauth.HasAuthorization(targets)
	s.Present = s.Present || legacy
	if err != nil {
		s.Present = true
	}
	if target == nil {
		return s, nil
	} // Missing core is handled by core management.
	if !service.Ready {
		return s, nil
	}
	coreErr := requirements.CheckCoreCapabilities(target.ExecutablePath, 0)
	authErr := coreauth.Check(target.ExecutablePath, 0)
	s.Steps = append(s.Steps, Step{"core_permissions", coreErr == nil}, Step{"dns_permissions", coreauth.CheckResolverAuthorization() == nil})
	if authErr != nil {
		s.Ready = false
		s.Code = requirements.Code(authErr, "inspection_failed")
		if s.Code == "linux_core_permission_required" {
			// An already approved service can grant a newly selected core on
			// the explicit start, without another installation/password UI.
			s.Ready, s.Code, s.Action = true, "ready", ""
		}
	}
	return s, nil
}
func Setup(ctx context.Context, target *coreauth.Target) error {
	state, err := coreauth.ServiceStatus(ctx)
	if err != nil {
		return err
	}
	if !state.Ready {
		if err = coreauth.InstallService(ctx); err != nil {
			return err
		}
	}
	if target == nil {
		return nil
	}
	return coreauth.Authorize(ctx, *target)
}
func Remove(ctx context.Context, directory string) error {
	targets, err := coreauth.CleanupTargets(directory)
	if err != nil {
		return err
	}
	return coreauth.Cleanup(ctx, targets)
}
func OpenSettings(context.Context) error {
	return &requirements.Error{Code: "authorization_unsupported", Message: "authorization_unsupported"}
}
