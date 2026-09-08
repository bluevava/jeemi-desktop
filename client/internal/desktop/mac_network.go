package desktop

import "jeemi/internal/platform/macnetwork"

func (a *App) GetMacNetworkAuthorization() macnetwork.Status {
	return a.service.GetMacNetworkAuthorization()
}
func (a *App) SetupMacNetworkAuthorization() (macnetwork.Status, error) {
	return a.service.SetupMacNetworkAuthorization()
}
func (a *App) OpenMacNetworkSettings() error       { return a.service.OpenMacNetworkSettings() }
func (a *App) OpenMacApplicationsDirectory() error { return a.service.OpenMacApplicationsDirectory() }
func (a *App) RemoveMacNetworkHelper() (macnetwork.Status, error) {
	return a.service.RemoveMacNetworkHelper()
}
