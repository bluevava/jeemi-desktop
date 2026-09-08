package desktop

import (
	"io/fs"
	"runtime"

	platformdesktop "jeemi/internal/platform/desktop"
	"jeemi/internal/platform/processguard"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

// Resources are embedded by the executable entry point and passed to the desktop host.
type Resources struct {
	Assets     fs.FS
	IconPNG    []byte
	IconICO    []byte
	TrayLabels []byte
}

func (r Resources) trayIcon() []byte {
	if runtime.GOOS == "windows" {
		return r.IconICO
	}
	return r.IconPNG
}

// Run assembles the desktop host and starts Wails. Bindings builds only expose method metadata.
func Run(resources Resources) error {
	if runLocalNetworkStatus() {
		return nil
	}
	if handled, err := processguard.RunIfRequested(); handled {
		if err != nil {
			return err
		}
		return nil
	}
	app, webViewDirectory, err := assembleApplication(resources)
	if err != nil {
		return err
	}

	return wails.Run(&options.App{
		Title:            platformdesktop.Name,
		Width:            1180,
		Height:           760,
		MinWidth:         980,
		MinHeight:        640,
		Frameless:        runtime.GOOS != "darwin",
		BackgroundColour: &options.RGBA{R: 244, G: 246, B: 249, A: 1},
		AssetServer:      &assetserver.Options{Assets: resources.Assets},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "com.jeemi.desktop",
			OnSecondInstanceLaunch: func(_ options.SecondInstanceData) {
				if ctx := app.context(); ctx != nil {
					app.showMainWindow(ctx)
				}
			},
		},
		Windows: &windows.Options{
			WebviewUserDataPath: webViewDirectory,
		},
		// Keep AppKit's window buttons above the shared 46px web title bar.
		// HiddenInset adds a native toolbar with a separate, system-sized height.
		Mac:           &mac.Options{TitleBar: mac.TitleBarHidden()},
		Linux:         &linux.Options{Icon: resources.IconPNG, ProgramName: platformdesktop.AppID},
		OnStartup:     app.startup,
		OnShutdown:    app.shutdown,
		OnBeforeClose: app.beforeClose,
		Bind:          []interface{}{app},
	})
}
