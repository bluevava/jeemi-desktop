//go:build windows

package coreauth

import (
	"context"
	"jeemi/internal/platform/winauth"
	"time"
)

func Authorize(ctx context.Context, target Target) error {
	state, err := winauth.Status(ctx)
	if err != nil {
		return err
	}
	if !state.Ready {
		return failure(state.Code)
	}
	return winauth.PrepareCore(ctx, target.ExecutablePath)
}
func Check(executable string, _ int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	state, err := winauth.Status(ctx)
	if err != nil {
		return err
	}
	if !state.Ready {
		return failure(state.Code)
	}
	return winauth.CheckCore(ctx, executable)
}
