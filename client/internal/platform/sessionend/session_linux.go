//go:build linux

package sessionend

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"
)

const loginService = "org.freedesktop.login1"
const loginPath dbus.ObjectPath = "/org/freedesktop/login1"
const sessionService = "org.gnome.SessionManager"

func start(r *request, quit func()) (func(), error) {
	ctx, cancel := context.WithCancel(context.Background())
	var once sync.Once
	finish := func(reply func()) {
		r.finish()
		if reply != nil {
			reply()
		}
		once.Do(quit)
	}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGHUP, os.Interrupt)
	go func() {
		defer signal.Stop(signals)
		select {
		case <-ctx.Done():
		case <-signals:
			finish(nil)
		}
	}()
	go watchLogin(ctx, func() { finish(nil) })
	go watchDesktopSession(ctx, finish)
	return cancel, nil
}

func busOwner(ctx context.Context, bus *dbus.Conn, name string) string {
	var owner string
	query, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	_ = bus.BusObject().CallWithContext(query, "org.freedesktop.DBus.GetNameOwner", dbus.FlagNoAutoStart, name).Store(&owner)
	return owner
}

func watchLogin(ctx context.Context, finish func()) {
	bus, err := dbus.ConnectSystemBus()
	if err != nil {
		return
	}
	defer bus.Close()
	owner := busOwner(ctx, bus, loginService)
	if owner == "" {
		return
	}
	events := make(chan *dbus.Signal, 8)
	bus.Signal(events)
	defer bus.RemoveSignal(events)
	if err = bus.AddMatchSignal(dbus.WithMatchSender(loginService), dbus.WithMatchObjectPath(loginPath), dbus.WithMatchInterface(loginService+".Manager"), dbus.WithMatchMember("PrepareForShutdown")); err != nil {
		return
	}
	// A delay inhibitor gives cleanup a short window. No interactive Polkit
	// authorization is requested; if unavailable, notifications still work.
	var descriptor dbus.UnixFD
	query, cancel := context.WithTimeout(ctx, time.Second)
	err = bus.Object(loginService, loginPath).CallWithContext(query, loginService+".Manager.Inhibit", dbus.FlagNoAutoStart, "shutdown", "Jeemi", "Restore proxy settings", "delay").Store(&descriptor)
	cancel()
	var lock *os.File
	if err == nil {
		lock = os.NewFile(uintptr(descriptor), "shutdown-delay")
		defer lock.Close()
	}
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			if loginShutdown(event, owner) {
				finish()
				return
			}
		}
	}
}

func loginShutdown(event *dbus.Signal, owner string) bool {
	if event == nil || event.Sender != owner || event.Path != loginPath || event.Name != loginService+".Manager.PrepareForShutdown" || len(event.Body) != 1 {
		return false
	}
	confirmed, ok := event.Body[0].(bool)
	return ok && confirmed
}

func watchDesktopSession(ctx context.Context, finish func(func())) {
	bus, err := dbus.ConnectSessionBus()
	if err != nil {
		return
	}
	defer bus.Close()
	owner := busOwner(ctx, bus, sessionService)
	if owner == "" {
		return
	}
	manager := bus.Object(sessionService, "/org/gnome/SessionManager")
	// Subscribe before registering: logout can begin during RegisterClient.
	events := make(chan *dbus.Signal, 8)
	bus.Signal(events)
	defer bus.RemoveSignal(events)
	if err = bus.AddMatchSignal(dbus.WithMatchSender(sessionService), dbus.WithMatchInterface(sessionService+".ClientPrivate")); err != nil {
		return
	}
	query, cancel := context.WithTimeout(ctx, time.Second)
	var client dbus.ObjectPath
	err = manager.CallWithContext(query, sessionService+".RegisterClient", dbus.FlagNoAutoStart, "com.jeemi.Jeemi", os.Getenv("DESKTOP_AUTOSTART_ID")).Store(&client)
	cancel()
	if err != nil {
		return
	}
	defer func() {
		q, c := context.WithTimeout(context.Background(), time.Second)
		defer c()
		_ = manager.CallWithContext(q, sessionService+".UnregisterClient", dbus.FlagNoAutoStart, client).Err
	}()
	respond := func() {
		q, c := context.WithTimeout(context.Background(), time.Second)
		defer c()
		_ = bus.Object(sessionService, client).CallWithContext(q, sessionService+".ClientPrivate.EndSessionResponse", dbus.FlagNoAutoStart, true, "").Err
	}
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			if event == nil || event.Sender != owner || event.Path != client {
				continue
			}
			switch event.Name {
			case sessionService + ".ClientPrivate.QueryEndSession":
				respond()
			case sessionService + ".ClientPrivate.EndSession":
				// Reply before exiting the Wails loop, even if cleanup times out.
				finish(respond)
				return
			case sessionService + ".ClientPrivate.Stop":
				finish(nil)
				return
			}
		}
	}
}
