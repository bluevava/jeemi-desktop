//go:build linux

package tray

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/prop"
)

const watcherName = "org.kde.StatusNotifierWatcher"

type linuxTrayBackend struct {
	available     atomic.Bool
	nativeMu      sync.Mutex
	onUnavailable func()
	connection    *dbus.Conn
	properties    *prop.Properties
	pixmaps       []linuxPixmap
	tooltip       string
	activate      func()
	items         []*linuxMenuItem
	menuRevision  uint32
}

func newSystemBackend() backend { return &linuxTrayBackend{} }

func (b *linuxTrayBackend) Available() bool { return b.available.Load() }

func (b *linuxTrayBackend) SetOnUnavailable(callback func()) { b.onUnavailable = callback }

func (b *linuxTrayBackend) SetIcon(icon []byte) {
	b.nativeMu.Lock()
	defer b.nativeMu.Unlock()
	b.pixmaps = linuxIconPixmaps(icon)
	if b.properties != nil {
		b.properties.SetMust(notifierInterface, "IconPixmap", b.pixmaps)
		_ = b.connection.Emit(notifierPath, notifierInterface+".NewIcon")
	}
}

func (b *linuxTrayBackend) SetTooltip(tooltip string) {
	b.nativeMu.Lock()
	defer b.nativeMu.Unlock()
	b.tooltip = tooltip
	if b.properties != nil {
		b.properties.SetMust(notifierInterface, "ToolTip", b.tooltipValue())
		_ = b.connection.Emit(notifierPath, notifierInterface+".NewToolTip")
	}
}

// StatusNotifier hosts display the exported D-Bus menu on right click.
func (*linuxTrayBackend) SetContextMenuOnRightClick() {}
func (*linuxTrayBackend) DetachMenu()                 {}

func (b *linuxTrayBackend) SetOnDoubleClick(callback func()) {
	b.nativeMu.Lock()
	defer b.nativeMu.Unlock()
	b.activate = callback
}

func (b *linuxTrayBackend) Start(onReady, onExit func()) (func(), func()) {
	ctx, cancel := context.WithCancel(context.Background())
	return func() { go b.run(ctx, onReady, onExit) }, cancel
}

func (b *linuxTrayBackend) run(ctx context.Context, onReady, onExit func()) {
	defer onExit()
	defer b.available.Store(false)
	// Own the item's connection so icon bytes and identity are exported before
	// the host can read them. systray v1.0.3 truncates RGBA's 16-bit channels.
	// A missing bus/host is normal (for example GNOME without AppIndicator).
	connection, err := dbus.ConnectSessionBus()
	if err != nil {
		return
	}
	defer func() {
		b.nativeMu.Lock()
		defer b.nativeMu.Unlock()
		b.connection, b.properties = nil, nil
		_ = connection.Close()
	}()
	// A desktop extension may start after Jeemi. Wait without making window
	// close hide the app, then initialise the native item exactly once.
	if !waitForLinuxTrayHost(ctx, connection) {
		return
	}
	// Populate icon, title, callbacks and the complete menu BEFORE the watcher
	// first reads our properties. Available stays false until registration.
	onReady()
	if ctx.Err() != nil {
		return
	}
	if err := b.export(connection); err != nil {
		log.Printf("Jeemi tray export failed: %v", err)
		return
	}
	registrationContext, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
	err = connection.Object(watcherName, "/StatusNotifierWatcher").CallWithContext(registrationContext,
		watcherName+".RegisterStatusNotifierItem", 0, string(notifierPath)).Err
	cancel()
	if err != nil && ctx.Err() == nil {
		log.Printf("Jeemi initial tray registration failed: %v", err)
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var retryAfter time.Time
	for {
		registered := linuxTrayRegistered(ctx, connection, true)
		if b.available.Swap(registered) && !registered && ctx.Err() == nil && b.onUnavailable != nil {
			b.onUnavailable()
		}
		if !registered && time.Now().After(retryAfter) && linuxTrayRegistered(ctx, connection, false) {
			// Keep the same item, icon and menu across host restarts.
			registerLinuxTray(ctx, connection)
			retryAfter = time.Now().Add(5 * time.Second)
		}
		select {
		case <-ctx.Done():
			return
		case <-connection.Context().Done():
			return
		case <-ticker.C:
		}
	}
}

func linuxTrayRegistered(ctx context.Context, connection *dbus.Conn, requireItem bool) bool {
	ctx, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
	defer cancel()
	watcher := connection.Object(watcherName, "/StatusNotifierWatcher")
	var host dbus.Variant
	if err := watcher.CallWithContext(ctx, "org.freedesktop.DBus.Properties.Get", 0, watcherName, "IsStatusNotifierHostRegistered").Store(&host); err != nil || host.Value() != true {
		return false
	}
	if !requireItem {
		return true
	}
	// The native item requests this exact well-known name, then registers
	// its unique bus owner + /StatusNotifierItem with the watcher.
	name := linuxTrayName()
	var owner string
	if err := connection.BusObject().CallWithContext(ctx, "org.freedesktop.DBus.GetNameOwner", 0, name).Store(&owner); err != nil {
		return false
	}
	var pid uint32
	if err := connection.BusObject().CallWithContext(ctx, "org.freedesktop.DBus.GetConnectionUnixProcessID", 0, owner).Store(&pid); err != nil || pid != uint32(os.Getpid()) {
		return false
	}
	var registered dbus.Variant
	if err := watcher.CallWithContext(ctx, "org.freedesktop.DBus.Properties.Get", 0, watcherName, "RegisteredStatusNotifierItems").Store(&registered); err != nil {
		return false
	}
	items, ok := registered.Value().([]string)
	return ok && containsTrayItem(items, owner, name)
}

func containsTrayItem(items []string, owner, name string) bool {
	for _, item := range items {
		// KDE uses service/path; Ubuntu AppIndicator uses owner@/path when an
		// item registers by object path. Match exactly, never another app's prefix.
		if item == owner+"/StatusNotifierItem" || item == owner+"@/StatusNotifierItem" ||
			item == name+"/StatusNotifierItem" || item == name+"@/StatusNotifierItem" || item == name {
			return true
		}
	}
	return false
}

func linuxTrayName() string { return fmt.Sprintf("org.kde.StatusNotifierItem-%d-1", os.Getpid()) }

func registerLinuxTray(ctx context.Context, connection *dbus.Conn) {
	ctx, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
	defer cancel()
	_ = connection.Object(watcherName, "/StatusNotifierWatcher").CallWithContext(ctx,
		watcherName+".RegisterStatusNotifierItem", 0, linuxTrayName()).Err
}

func waitForLinuxTrayHost(ctx context.Context, connection *dbus.Conn) bool {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if linuxTrayRegistered(ctx, connection, false) {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-connection.Context().Done():
			return false
		case <-ticker.C:
		}
	}
}
