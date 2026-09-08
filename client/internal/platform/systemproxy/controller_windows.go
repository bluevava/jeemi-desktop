//go:build windows

package systemproxy

import (
	"context"
	"jeemi/internal/platform/winauth"
	"jeemi/internal/runtimeconfig"
)

type serviceController struct{ legacy *manager }

func newPlatformController(path string) Controller {
	return &serviceController{legacy: newManager(path, newBackend())}
}
func (c *serviceController) Apply(ctx context.Context, p runtimeconfig.Preferences) error {
	if err := c.legacy.Recover(ctx); err != nil {
		return err
	}
	_, err := winauth.Call(ctx, winauth.Request{Operation: "proxy", Preferences: &p})
	if err == nil {
		err = (windowsBackend{}).Notify(ctx)
	}
	return err
}
func (c *serviceController) Restore(ctx context.Context) error {
	if err := c.legacy.Restore(ctx); err != nil {
		return err
	}
	state, err := winauth.Status(ctx)
	if err != nil {
		return err
	}
	if !state.Present {
		return nil
	}
	_, err = winauth.Call(ctx, winauth.Request{Operation: "restore"})
	if err == nil {
		err = (windowsBackend{}).Notify(ctx)
	}
	return err
}
func (c *serviceController) Recover(ctx context.Context) error { return c.Restore(ctx) }
