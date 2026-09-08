package desktop

import (
	"fmt"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) OpenJeemiRepository() error {
	ctx := a.context()
	if ctx == nil {
		return fmt.Errorf("application window is unavailable")
	}
	wailsRuntime.BrowserOpenURL(ctx, "https://github.com/bluevava/jeemi-desktop")
	return nil
}
