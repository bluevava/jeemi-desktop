//go:build !windows && !linux && !darwin

package systemproxy

import (
	"context"
	"fmt"
)

type unsupportedBackend struct{}

func newBackend() backend { return unsupportedBackend{} }

func (unsupportedBackend) Capture(context.Context) (snapshot, error) {
	return snapshot{}, fmt.Errorf("system proxy integration is not implemented on this platform")
}

func (unsupportedBackend) Apply(context.Context, string) error {
	return fmt.Errorf("system proxy integration is not implemented on this platform")
}
func (unsupportedBackend) Restore(context.Context, snapshot) error { return nil }
func (unsupportedBackend) Notify(context.Context) error            { return nil }
