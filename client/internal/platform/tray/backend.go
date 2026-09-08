//go:build !darwin && !linux

package tray

import "github.com/energye/systray"

type systemTrayBackend struct{}

func (systemTrayBackend) SetIcon(icon []byte) {
	systray.SetIcon(icon)
}

func (systemTrayBackend) SetTooltip(tooltip string) {
	systray.SetTooltip(tooltip)
}

func (systemTrayBackend) SetOnDoubleClick(callback func()) {
	systray.SetOnDClick(func(_ systray.IMenu) {
		callback()
	})
}

func (systemTrayBackend) SetContextMenuOnRightClick() {
	systray.SetOnRClick(func(menu systray.IMenu) {
		if menu != nil {
			_ = menu.ShowMenu()
		}
	})
}

func (systemTrayBackend) CreateMenu() {
	systray.CreateMenu()
}

func (systemTrayBackend) DetachMenu() {
	systray.SetMenuNil()
}

func (systemTrayBackend) AddMenuItem(title, tooltip string) menuItem {
	return systray.AddMenuItem(title, tooltip)
}

func (systemTrayBackend) AddSeparator() {
	systray.AddSeparator()
}
