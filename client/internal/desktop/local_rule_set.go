package desktop

import (
	"fmt"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
	configresources "jeemi/internal/config/resources"
)

func (a *App) PreviewRuleSetEntry(input configresources.RuleSetEntryInput) (configresources.RuleSetEntryPreview, error) {
	return a.service.PreviewRuleSetEntry(input)
}

func (a *App) SaveRuleSetEntry(input configresources.RuleSetEntryInput) (configresources.State, error) {
	return a.service.SaveRuleSetEntry(input)
}

func (a *App) OpenRuleDocumentation() error {
	ctx := a.context()
	if ctx == nil {
		return fmt.Errorf("application window is unavailable")
	}
	wailsRuntime.BrowserOpenURL(ctx, "https://wiki.metacubex.one/config/rules/")
	return nil
}
