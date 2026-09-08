package desktop

import "jeemi/internal/platform/authorization"

func (a *App) GetProxyAuthorization() (authorization.Status, error) {
	return a.service.GetProxyAuthorization()
}
func (a *App) SetupProxyAuthorization() (authorization.Status, error) {
	return a.service.SetupProxyAuthorization()
}
func (a *App) RemoveAuthorizationHelper() (authorization.Status, error) {
	return a.service.RemoveAuthorizationHelper()
}
func (a *App) OpenProxyAuthorizationSettings() error {
	return a.service.OpenProxyAuthorizationSettings()
}
