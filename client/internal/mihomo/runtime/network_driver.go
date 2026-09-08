package runtime

import (
	"context"
	"jeemi/internal/runtimeconfig"
)

// macOS delegates privileged network lifetime to its authenticated helper.
// Other platform drivers keep their existing network integration.
type managedNetwork interface {
	ActivateTUN(context.Context, runtimeconfig.Preferences) error
	DeactivateNetwork(context.Context) error
	TUNDevice() string
}

func (m *Manager) tunDevice() string {
	if network, ok := m.driver.(managedNetwork); ok {
		return network.TUNDevice()
	}
	return runtimeconfig.TUNDeviceNameForOS(m.goos)
}

func (m *Manager) reloadConfiguration(ctx context.Context, client *controllerClient, configuration string) error {
	if staging, ok := m.driver.(interface {
		StageConfiguration(context.Context, string) (string, error)
	}); ok {
		path, err := staging.StageConfiguration(ctx, configuration)
		if err != nil {
			return err
		}
		configuration = path
	}
	return client.Reload(ctx, configuration)
}
