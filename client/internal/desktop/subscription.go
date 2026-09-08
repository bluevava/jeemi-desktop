package desktop

import (
	"fmt"

	"jeemi/internal/application"
	"jeemi/internal/config/fallbackoverride"
	"jeemi/internal/mihomo/delaycache"
	"jeemi/internal/subscription"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) GetSubscriptionState() (subscription.State, error) {
	return a.service.SubscriptionState()
}

func (a *App) GetSubscription(id string) (subscription.Detail, error) {
	return a.service.Subscription(id)
}

func (a *App) DetectSubscriptionIcon(sourceURL string) (subscription.IconDetectionResult, error) {
	return a.service.DetectSubscriptionIcon(sourceURL)
}

func (a *App) ImportSubscriptionURL(input subscription.ImportURLInput) (subscription.State, error) {
	return a.service.ImportSubscriptionURL(input)
}

func (a *App) ImportSubscriptionFile(labels subscription.DialogLabels) (subscription.DialogImportResult, error) {
	ctx := a.context()
	if ctx == nil {
		return subscription.DialogImportResult{}, fmt.Errorf("application window is unavailable")
	}
	path, err := wailsRuntime.OpenFileDialog(ctx, wailsRuntime.OpenDialogOptions{
		Title: labels.Title,
		Filters: []wailsRuntime.FileFilter{{
			DisplayName: labels.FilterName,
			Pattern:     "*.yaml;*.json;*.txt;*.conf",
		}},
	})
	if err != nil {
		return subscription.DialogImportResult{}, fmt.Errorf("open subscription file dialog: %w", err)
	}
	if path == "" {
		state, stateErr := a.service.SubscriptionState()
		return subscription.DialogImportResult{Cancelled: true, State: state}, stateErr
	}
	state, err := a.service.ImportSubscriptionFile(path)
	return subscription.DialogImportResult{State: state}, err
}

func (a *App) ImportSubscriptionQRCodeImage(labels subscription.DialogLabels) (subscription.DialogImportResult, error) {
	ctx := a.context()
	if ctx == nil {
		return subscription.DialogImportResult{}, fmt.Errorf("application window is unavailable")
	}
	path, err := wailsRuntime.OpenFileDialog(ctx, wailsRuntime.OpenDialogOptions{
		Title: labels.Title,
		Filters: []wailsRuntime.FileFilter{{
			DisplayName: labels.FilterName,
			Pattern:     "*.png;*.jpg;*.jpeg;*.gif",
		}},
	})
	if err != nil {
		return subscription.DialogImportResult{}, fmt.Errorf("open QR code image dialog: %w", err)
	}
	if path == "" {
		state, stateErr := a.service.SubscriptionState()
		return subscription.DialogImportResult{Cancelled: true, State: state}, stateErr
	}
	state, err := a.service.ImportSubscriptionQRCodeImage(path)
	return subscription.DialogImportResult{State: state}, err
}

func (a *App) RefreshSubscription(id string) (subscription.State, error) {
	return a.service.RefreshSubscription(id)
}

func (a *App) UpdateSubscription(input subscription.UpdateInput) (subscription.State, error) {
	return a.service.UpdateSubscription(input)
}

func (a *App) DeleteSubscription(id string) (subscription.State, error) {
	return a.service.DeleteSubscription(id)
}

func (a *App) SetSubscriptionLocalScript(id, localScriptID string) (subscription.State, error) {
	return a.service.SetSubscriptionLocalScript(id, localScriptID)
}

func (a *App) GetSubscriptionText(id string) (subscription.TextView, error) {
	return a.service.SubscriptionText(id)
}

func (a *App) SetSubscriptionLocalConfig(id, localConfigID string) (subscription.State, error) {
	return a.service.SetSubscriptionLocalConfig(id, localConfigID)
}

func (a *App) SetSubscriptionRuleProviderEnabled(id, providerName string, enabled bool) (subscription.State, error) {
	return a.service.SetSubscriptionRuleProviderEnabled(id, providerName, enabled)
}

func (a *App) SelectSubscription(id string) (subscription.State, error) {
	return a.service.SelectSubscription(id)
}

func (a *App) GetSubscriptionPreferences() (subscription.Preferences, error) {
	return a.service.SubscriptionPreferences()
}

func (a *App) SaveSubscriptionPreferences(input subscription.PreferencesInput) (subscription.Preferences, error) {
	return a.service.SaveSubscriptionPreferences(input)
}

func (a *App) SaveSubscriptionSelectorDisplayPreferences(input subscription.SelectorDisplayPreferencesInput) (subscription.Preferences, error) {
	return a.service.SaveSubscriptionSelectorDisplayPreferences(input)
}

func (a *App) GetProxyDelayCache(scope delaycache.Scope) (delaycache.Snapshot, error) {
	return a.service.ProxyDelayCache(scope)
}

func (a *App) SaveProxyDelayCache(update delaycache.Update) (delaycache.Snapshot, error) {
	return a.service.SaveProxyDelayCache(update)
}

func (a *App) SetSubscriptionFallback(id string, input fallbackoverride.Selection, expectedRevision int) (application.SubscriptionFallbackResult, error) {
	return a.service.SetSubscriptionFallback(id, input, expectedRevision)
}

func (a *App) CheckSubscriptionRefresh(id string) (application.SubscriptionRefreshResult, error) {
	return a.service.CheckSubscriptionRefresh(id)
}

func (a *App) ResolveSubscriptionRefresh(token string, detach bool) (subscription.State, error) {
	return a.service.ResolveSubscriptionRefresh(token, detach)
}
