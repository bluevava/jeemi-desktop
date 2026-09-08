//go:build !darwin && !linux && !windows

package authorization

import (
	"context"
	"jeemi/internal/platform/coreauth"
	"runtime"
)

// The current Windows release elevates the GUI via its manifest. There is no
// installed helper service to discover/remove. Do not invent one in the UI.
func Inspect(context.Context, *coreauth.Target, string) (Status, error) {
	return Status{Platform: runtime.GOOS, Kind: "application", Ready: true, Code: "ready", Steps: []Step{}}, nil
}
func Setup(context.Context, *coreauth.Target) error { return nil }
func Remove(context.Context, string) error          { return nil }
func OpenSettings(context.Context) error            { return coreauth.ErrCancelled }
