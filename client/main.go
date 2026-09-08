package main

import (
	"embed"

	"jeemi/internal/desktop"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var applicationIconPNG []byte

//go:embed build/windows/icon.ico
var applicationIconICO []byte

//go:embed frontend/src/i18n/tray-labels.json
var trayLabelsJSON []byte

func main() {
	if err := desktop.Run(desktop.Resources{
		Assets:     assets,
		IconPNG:    applicationIconPNG,
		IconICO:    applicationIconICO,
		TrayLabels: trayLabelsJSON,
	}); err != nil {
		println("Jeemi startup failed:", err.Error())
	}
}
