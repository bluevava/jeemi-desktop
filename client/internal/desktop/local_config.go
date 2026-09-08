package desktop

import (
	configresources "jeemi/internal/config/resources"
	configschema "jeemi/internal/config/schema"
	"jeemi/internal/profile"
)

func (a *App) GetConfigCatalog() configschema.Catalog {
	return a.service.ConfigCatalog()
}

func (a *App) GetLocalConfigState() (profile.LocalConfigState, error) {
	return a.service.LocalConfigState()
}

func (a *App) GetLocalConfig(id string) (profile.LocalConfig, error) {
	return a.service.LocalConfig(id)
}

func (a *App) PreviewLocalConfig(input profile.SaveLocalConfigInput) (profile.LocalConfigPreview, error) {
	return a.service.PreviewLocalConfig(input)
}

func (a *App) SaveLocalConfig(input profile.SaveLocalConfigInput) (profile.LocalConfig, error) {
	return a.service.SaveLocalConfig(input)
}

func (a *App) DeleteLocalConfig(id string) (profile.LocalConfigState, error) {
	return a.service.DeleteLocalConfig(id)
}

func (a *App) GetLocalConfigResources() (configresources.State, error) {
	return a.service.LocalConfigResources()
}

func (a *App) SaveStrategyGroup(input configresources.StrategyGroup) (configresources.State, error) {
	return a.service.SaveStrategyGroup(input)
}

func (a *App) DeleteStrategyGroup(id string) (configresources.State, error) {
	return a.service.DeleteStrategyGroup(id)
}

func (a *App) SaveRuleSet(input configresources.RuleSet) (configresources.State, error) {
	return a.service.SaveRuleSet(input)
}

func (a *App) DeleteRuleSet(id string) (configresources.State, error) {
	return a.service.DeleteRuleSet(id)
}
