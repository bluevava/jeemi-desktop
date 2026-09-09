//go:build darwin

package tray

/*
#cgo darwin CFLAGS: -x objective-c -fobjc-arc
#cgo darwin LDFLAGS: -framework Cocoa
#include <stdlib.h>
#include "tray_darwin.h"
*/
import "C"

import (
	"runtime/cgo"
	"sync"
	"unsafe"
)

type darwinTrayBackend struct {
	mu      sync.RWMutex
	session *darwinTraySession
}

type darwinTraySession struct {
	mu                           sync.RWMutex
	handle                       cgo.Handle
	onReady, onExit, doubleClick func()
	items                        map[uint32]*darwinMenuItem
	nextID                       uint32
	closed                       bool
	exitOnce                     sync.Once
}

type darwinMenuItem struct {
	mu             sync.RWMutex
	updateMu       sync.Mutex
	session        *darwinTraySession
	id             uint32
	title, tooltip string
	callback       func()
	enabled        bool
}

func newSystemBackend() backend { return &darwinTrayBackend{} }

func (b *darwinTrayBackend) Start(onReady, onExit func()) (func(), func()) {
	session := &darwinTraySession{onReady: onReady, onExit: onExit, items: make(map[uint32]*darwinMenuItem)}
	session.handle = cgo.NewHandle(session)
	b.mu.Lock()
	b.session = session
	b.mu.Unlock()
	return queuedLifecycle(
		func() { C.jeemi_tray_start(C.uintptr_t(session.handle)) },
		func() { C.jeemi_tray_stop(C.uintptr_t(session.handle)) },
		session.complete,
	)
}

func (b *darwinTrayBackend) current() *darwinTraySession {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.session
}

func (session *darwinTraySession) complete() {
	session.exitOnce.Do(func() {
		session.mu.Lock()
		session.closed = true
		session.mu.Unlock()
		// Native removal finishes before the controller is allowed to quit Wails.
		if session.onExit != nil {
			session.onExit()
		}
		session.handle.Delete()
	})
}

//export jeemiTrayEvent
func jeemiTrayEvent(handle C.uintptr_t, event C.int, itemID C.uint32_t) {
	session := cgo.Handle(handle).Value().(*darwinTraySession)
	if event == C.JEEMI_TRAY_EXIT {
		session.complete()
		return
	}
	session.mu.RLock()
	var callback func()
	if !session.closed {
		switch event {
		case C.JEEMI_TRAY_READY:
			callback = session.onReady
		case C.JEEMI_TRAY_DOUBLE_CLICK:
			callback = session.doubleClick
		case C.JEEMI_TRAY_MENU:
			if item := session.items[uint32(itemID)]; item != nil {
				item.mu.RLock()
				if item.enabled {
					callback = item.callback
				}
				item.mu.RUnlock()
			}
		}
	}
	session.mu.RUnlock()
	// Menu callbacks and controller initialization must not block Cocoa's loop.
	if callback != nil {
		go callback()
	}
}

func (b *darwinTrayBackend) SetIcon(icon []byte) {
	if session := b.current(); session != nil && len(icon) > 0 {
		data := C.CBytes(icon)
		defer C.free(data)
		C.jeemi_tray_set_icon(C.uintptr_t(session.handle), data, C.size_t(len(icon)))
	}
}

func (b *darwinTrayBackend) SetTooltip(tooltip string) {
	if session := b.current(); session != nil {
		text := C.CString(tooltip)
		defer C.free(unsafe.Pointer(text))
		C.jeemi_tray_set_tooltip(C.uintptr_t(session.handle), text)
	}
}

func (b *darwinTrayBackend) SetOnDoubleClick(callback func()) {
	if session := b.current(); session != nil {
		session.mu.Lock()
		session.doubleClick = callback
		session.mu.Unlock()
	}
}

func (b *darwinTrayBackend) SetContextMenuOnRightClick() {
	if session := b.current(); session != nil {
		C.jeemi_tray_enable_menu(C.uintptr_t(session.handle))
	}
}

// The menu is created with the status item and stays detached except while open.
func (b *darwinTrayBackend) CreateMenu() {}
func (b *darwinTrayBackend) DetachMenu() {}

func (b *darwinTrayBackend) AddMenuItem(title, tooltip string) menuItem {
	session := b.current()
	session.mu.Lock()
	session.nextID++
	item := &darwinMenuItem{session: session, id: session.nextID, title: title, tooltip: tooltip, enabled: true}
	session.items[item.id] = item
	session.mu.Unlock()
	item.update()
	return item
}

func (b *darwinTrayBackend) AddSeparator() {
	if session := b.current(); session != nil {
		C.jeemi_tray_add_separator(C.uintptr_t(session.handle))
	}
}

func (item *darwinMenuItem) Click(callback func()) {
	item.mu.Lock()
	item.callback = callback
	item.mu.Unlock()
}

func (item *darwinMenuItem) SetTitle(title string) {
	item.mu.Lock()
	item.title = title
	item.mu.Unlock()
	item.update()
}

func (item *darwinMenuItem) SetTooltip(tooltip string) {
	item.mu.Lock()
	item.tooltip = tooltip
	item.mu.Unlock()
	item.update()
}

func (item *darwinMenuItem) update() {
	item.updateMu.Lock()
	defer item.updateMu.Unlock()
	item.mu.RLock()
	title, tooltip := C.CString(item.title), C.CString(item.tooltip)
	enabled := C.int(0)
	if item.enabled {
		enabled = 1
	}
	item.mu.RUnlock()
	defer C.free(unsafe.Pointer(title))
	defer C.free(unsafe.Pointer(tooltip))
	C.jeemi_tray_update_item(C.uintptr_t(item.session.handle), C.uint32_t(item.id), title, tooltip, enabled)
}

func (item *darwinMenuItem) Enable()  { item.setEnabled(true) }
func (item *darwinMenuItem) Disable() { item.setEnabled(false) }

func (item *darwinMenuItem) setEnabled(enabled bool) {
	item.mu.Lock()
	item.enabled = enabled
	item.mu.Unlock()
	item.update()
}
