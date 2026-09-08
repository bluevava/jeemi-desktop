//go:build darwin

package coreauth

import (
	"context"
	"errors"
	"time"

	"jeemi/internal/platform/macnetwork"
)

func Authorize(ctx context.Context, target Target) error {
	selected, err := macnetwork.TargetForExecutable(target.ExecutablePath)
	if err != nil {
		return err
	}
	if selected.SHA256 != target.SHA256 || selected.Size != target.Size {
		return macnetwork.Failure("integrity_failed")
	}
	err = macnetwork.Prepare(ctx, selected)
	if errors.Is(err, context.Canceled) || ctx.Err() == context.Canceled {
		return ErrCancelled
	}
	return err
}
func Check(executable string, runningPID int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := macnetwork.RequireReady(ctx); err != nil {
		return err
	}
	target, err := macnetwork.TargetForExecutable(executable)
	if err != nil {
		return err
	}
	return macnetwork.CheckTarget(ctx, target)
}
