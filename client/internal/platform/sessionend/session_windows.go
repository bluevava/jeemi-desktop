//go:build windows

package sessionend

import (
	"fmt"
	"runtime"
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	wmQueryEndSession = 0x11
	wmEndSession      = 0x16
	wmClose           = 0x10
	wmDestroy         = 2
)

func sessionMessage(message uint32, confirmed uintptr, finish func()) (bool, uintptr) {
	switch message {
	case wmQueryEndSession:
		return true, 1 // A query can still be cancelled elsewhere.
	case wmEndSession:
		if confirmed != 0 {
			finish()
		}
		return true, 0
	}
	return false, 0
}

type windowClass struct {
	Size, Style                        uint32
	Proc                               uintptr
	ClassExtra, WindowExtra            int32
	Instance, Icon, Cursor, Background windows.Handle
	Menu, Name                         *uint16
	SmallIcon                          windows.Handle
}
type windowMessage struct {
	Window         windows.Handle
	Message        uint32
	WParam, LParam uintptr
	Time           uint32
	X, Y           int32
	Private        uint32
}

func start(r *request, quit func()) (func(), error) {
	user := windows.NewLazySystemDLL("user32.dll")
	post := user.NewProc("PostMessageW")
	var handle atomic.Uintptr
	ready := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		module, _, _ := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetModuleHandleW").Call(0)
		name, _ := windows.UTF16PtrFromString("JeemiSessionEnd")
		proc := windows.NewCallback(func(hwnd uintptr, message uint32, wparam, lparam uintptr) uintptr {
			if handled, result := sessionMessage(message, wparam, func() { r.finish(); go quit() }); handled {
				return result
			}
			switch message {
			case wmClose:
				user.NewProc("DestroyWindow").Call(hwnd)
				return 0
			case wmDestroy:
				user.NewProc("PostQuitMessage").Call(0)
				return 0
			}
			result, _, _ := user.NewProc("DefWindowProcW").Call(hwnd, uintptr(message), wparam, lparam)
			return result
		})
		class := windowClass{Proc: proc, Instance: windows.Handle(module), Name: name}
		class.Size = uint32(unsafe.Sizeof(class))
		atom, _, err := user.NewProc("RegisterClassExW").Call(uintptr(unsafe.Pointer(&class)))
		if atom == 0 {
			ready <- fmt.Errorf("register session window: %w", err)
			return
		}
		defer user.NewProc("UnregisterClassW").Call(uintptr(unsafe.Pointer(name)), module)
		// A hidden top-level window receives broadcasts; HWND_MESSAGE would not.
		hwnd, _, err := user.NewProc("CreateWindowExW").Call(0, uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(name)), 0, 0, 0, 0, 0, 0, 0, module, 0)
		if hwnd == 0 {
			ready <- fmt.Errorf("create session window: %w", err)
			return
		}
		handle.Store(hwnd)
		ready <- nil
		defer handle.Store(0)
		var message windowMessage
		for {
			result, _, _ := user.NewProc("GetMessageW").Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
			if int32(result) <= 0 {
				return
			}
			user.NewProc("TranslateMessage").Call(uintptr(unsafe.Pointer(&message)))
			user.NewProc("DispatchMessageW").Call(uintptr(unsafe.Pointer(&message)))
		}
	}()
	if err := <-ready; err != nil {
		return func() {}, err
	}
	return func() {
		if hwnd := handle.Load(); hwnd != 0 {
			post.Call(hwnd, wmClose, 0, 0)
		}
	}, nil
}
