//go:build windows

package winauth

import (
	"context"
)

// CheckCore verifies the selected data-directory core; it never writes files.
func CheckCore(ctx context.Context, executable string) error {
	target, err := TargetForExecutable(executable)
	if err != nil {
		return err
	}
	_, err = Call(ctx, Request{Operation: "check", Target: &target})
	return err
}

// PrepareCore runs only from the explicit proxy-start path. The service checks
// the selected local file without copying it, downloading it or requesting UAC.
func PrepareCore(ctx context.Context, executable string) error {
	target, err := TargetForExecutable(executable)
	if err != nil {
		return err
	}
	_, err = Call(ctx, Request{Operation: "prepare", Target: &target})
	return err
}
