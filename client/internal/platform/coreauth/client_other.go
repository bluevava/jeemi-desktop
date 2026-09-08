//go:build !linux && !darwin && !windows

package coreauth

import (
	"context"
	"jeemi/internal/platform/requirements"
)

func Authorize(context.Context, Target) error { return nil }

func Check(executable string, runningPID int) error {
	return requirements.CheckCoreCapabilities(executable, runningPID)
}
