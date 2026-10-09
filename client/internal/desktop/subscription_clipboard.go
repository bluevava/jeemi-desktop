package desktop

import (
	"fmt"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// CopySubscriptionSource accepts only a subscription ID. Neither the full URL
// nor the managed source path crosses the Wails bridge.
func (a *App) CopySubscriptionSource(id string) error {
	ctx := a.context()
	if ctx == nil {
		return fmt.Errorf("application window is unavailable")
	}
	return a.copySubscriptionSource(id, func(text string) error {
		return wailsRuntime.ClipboardSetText(ctx, text)
	})
}

func (a *App) copySubscriptionSource(id string, setText func(string) error) error {
	address, err := a.service.SubscriptionSourceAddress(id)
	if err != nil {
		return fmt.Errorf("subscription source is unavailable")
	}
	if err := setText(address); err != nil {
		return fmt.Errorf("copy subscription source failed")
	}
	return nil
}
