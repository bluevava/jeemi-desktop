//go:build !darwin && !linux

package tray

import (
	"runtime"

	"github.com/energye/systray"
)

func (systemTrayBackend) Start(onReady, onExit func()) (func(), func()) {
	return func() {
		go func() {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			systray.Run(onReady, onExit)
		}()
	}, systray.Quit
}
