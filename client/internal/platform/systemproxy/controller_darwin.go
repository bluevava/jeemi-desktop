//go:build darwin

package systemproxy

import (
	"context"

	"jeemi/internal/platform/macnetwork"
	"jeemi/internal/runtimeconfig"
)

// The helper owns the original system values in root-owned storage. The GUI
// cannot replace a recovery snapshot or direct a privileged arbitrary write.
type macController struct{}

func newPlatformController(string) Controller         { return macController{} }
func (macController) Check(ctx context.Context) error { return macnetwork.RequireReady(ctx) }
func (macController) Apply(ctx context.Context, preferences runtimeconfig.Preferences) error {
	if err := runtimeconfig.Validate(preferences); err != nil {
		return err
	}
	if preferences.ProxyMode != runtimeconfig.ProxyModeSystemProxy {
		return macnetwork.Failure("invalid_request")
	}
	endpoint, err := proxyServerValue(preferences.ListenerType, preferences.ListenPort)
	if err != nil {
		return err
	}
	_, err = macnetwork.Call(ctx, macnetwork.Request{Operation: "proxy", Endpoint: endpoint})
	return err
}
func (macController) Restore(ctx context.Context) error {
	state := macnetwork.Inspect(ctx)
	if !state.Ready && (state.Code == "helper_missing" || state.Code == "signing_required" || state.Code == "registration_required" || state.Code == "installation_required" || state.Code == "local_installation_required" || state.Code == "local_signature_invalid") {
		return nil
	}
	_, err := macnetwork.Call(ctx, macnetwork.Request{Operation: "restore"})
	return err
}
func (macController) Recover(context.Context) error {
	// The root daemon restores its journal before accepting a new connection.
	// A missing/unapproved helper must not block launching the ordinary GUI.
	return nil
}
