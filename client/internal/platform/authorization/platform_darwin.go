//go:build darwin

package authorization

import (
	"context"
	"jeemi/internal/platform/coreauth"
	"jeemi/internal/platform/macnetwork"
)

func Inspect(ctx context.Context, _ *coreauth.Target, _ string) (Status, error) {
	s := macnetwork.Inspect(ctx)
	result := Status{Platform: "darwin", Kind: "service", Ready: s.Ready, Present: s.Present, LocalTest: s.LocalTest, Code: s.Code, Action: s.Action, Steps: []Step{}}
	if s.Action == "register" {
		result.Action = "install"
	}
	if s.Code == "version_mismatch" || s.Code == "local_update_required" {
		result.Action = "update"
	}
	for _, step := range s.Steps {
		result.Steps = append(result.Steps, Step{step.ID, step.Complete})
	}
	return result, nil
}
func Setup(ctx context.Context, _ *coreauth.Target) error {
	_, err := macnetwork.Setup(ctx)
	return err
}
func Remove(ctx context.Context, _ string) error { _, err := macnetwork.Unregister(ctx); return err }
func OpenSettings(ctx context.Context) error {
	s := macnetwork.Inspect(ctx)
	if s.Action == "applications" {
		return macnetwork.OpenApplications()
	}
	if s.Action == "settings" {
		return macnetwork.OpenSettings()
	}
	return macnetwork.Failure("invalid_request")
}
