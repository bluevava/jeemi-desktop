//go:build linux

package tray

import (
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

// Reuse the private bus from the lifecycle test; do not create a real tray.
func checkLinuxTrayMenu(t *testing.T, connection *dbus.Conn, shown, reloaded, quit <-chan struct{}) {
	t.Helper()
	menu := connection.Object(linuxTrayName(), menuPath)
	var label dbus.Variant
	if err := menu.Call(menuInterface+".GetProperty", 0, int32(1), "label").Store(&label); err != nil || label.Value() != "Show Jeemi" {
		t.Fatalf("localized menu label missing: %v, %v", label, err)
	}
	var group []linuxMenuProperties
	if err := menu.Call(menuInterface+".GetGroupProperties", 0, []int32{1, 3, 99}, []string{"label", "type"}).Store(&group); err != nil || len(group) != 2 || group[1].Properties["type"].Value() != "separator" {
		t.Fatalf("invalid menu group: %v, %v", group, err)
	}
	var updates, invalid []int32
	if err := menu.Call(menuInterface+".AboutToShowGroup", 0, []int32{0, 99}).Store(&updates, &invalid); err != nil || len(updates) != 0 || len(invalid) != 1 || invalid[0] != 99 {
		t.Fatalf("invalid menu open result: %v, %v, %v", updates, invalid, err)
	}
	for _, action := range []struct {
		id   int32
		done <-chan struct{}
	}{{1, shown}, {2, reloaded}, {4, quit}} {
		if err := menu.Call(menuInterface+".Event", 0, action.id, "clicked", dbus.MakeVariant(int32(0)), uint32(0)).Err; err != nil {
			t.Fatal(err)
		}
		select {
		case <-action.done:
		case <-time.After(time.Second):
			t.Fatalf("menu item %d did not execute its callback", action.id)
		}
	}
	events := []linuxMenuEvent{{2, "clicked", dbus.MakeVariant(int32(0)), 0}, {99, "clicked", dbus.MakeVariant(int32(0)), 0}}
	if err := menu.Call(menuInterface+".EventGroup", 0, events).Store(&invalid); err != nil || len(invalid) != 1 || invalid[0] != 99 {
		t.Fatalf("invalid menu event group: %v, %v", invalid, err)
	}
	select {
	case <-reloaded:
	case <-time.After(time.Second):
		t.Fatal("grouped menu event did not execute")
	}
	item := connection.Object(linuxTrayName(), notifierPath)
	if err := item.Call("org.freedesktop.DBus.Properties.Set", 0, notifierInterface, "IconName", dbus.MakeVariant("wrong-icon")).Err; err == nil {
		t.Fatal("desktop host could replace Jeemi's icon association")
	}
}
