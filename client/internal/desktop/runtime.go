package desktop

import (
	"jeemi/internal/application"
	"jeemi/internal/runtimeconfig"
)

func (a *App) GetBootstrapState() application.BootstrapState {
	return a.service.BootstrapState()
}

func (a *App) RefreshRuntimeStatus() application.RuntimeStatus {
	return a.service.RuntimeStatus()
}

func (a *App) StartProxy() (application.RuntimeStatus, error) {
	return a.service.StartProxy()
}

func (a *App) CancelCoreAuthorization() {
	a.service.CancelCoreAuthorization()
}

func (a *App) StopProxy() (application.RuntimeStatus, error) {
	return a.service.StopProxy()
}

func (a *App) RestartProxy() (application.RuntimeStatus, error) {
	return a.service.RestartProxy()
}

func (a *App) GetRuntimeConfigurationText() (application.RuntimeConfigurationText, error) {
	return a.service.RuntimeConfigurationText()
}

func (a *App) SelectRuntimeProxy(group, proxy string) error {
	return a.service.SelectRuntimeProxy(group, proxy)
}

func (a *App) UpdateRuntimeRuleProvider(name string) error {
	return a.service.UpdateRuntimeRuleProvider(name)
}

func (a *App) UpdateRuntimeProxyProvider(name string) error {
	return a.service.UpdateRuntimeProxyProvider(name)
}

func (a *App) GetManagedStorageDirectories() application.ManagedStorageDirectories {
	return a.service.ManagedStorageDirectories()
}

func (a *App) GetRuntimePreferences() (runtimeconfig.Preferences, error) {
	return a.service.RuntimePreferences()
}

func (a *App) GetDefaultLANBypassRules() []string {
	return a.service.DefaultLANBypassRules()
}

func (a *App) ValidateRuntimeYAMLFragment(input runtimeconfig.YAMLFragmentInput) runtimeconfig.YAMLFragmentValidation {
	return a.service.ValidateRuntimeYAMLFragment(input)
}

func (a *App) SaveRuntimePreferences(input runtimeconfig.Preferences) (runtimeconfig.Preferences, error) {
	return a.service.SaveRuntimePreferences(input)
}
