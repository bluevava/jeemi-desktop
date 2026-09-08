//go:build linux

package sessionend

import (
	"github.com/godbus/dbus/v5"
	"testing"
)

func TestOnlyConfirmedLogindShutdownIsAccepted(t *testing.T) {
	event := dbus.Signal{Sender: ":1.9", Path: loginPath, Name: loginService + ".Manager.PrepareForShutdown", Body: []interface{}{true}}
	if !loginShutdown(&event, ":1.9") {
		t.Fatal("shutdown ignored")
	}
	event.Body = []interface{}{false}
	if loginShutdown(&event, ":1.9") {
		t.Fatal("cancelled shutdown accepted")
	}
	event.Body = []interface{}{true}
	if loginShutdown(&event, ":1.8") {
		t.Fatal("unrelated sender accepted")
	}
	event.Name = loginService + ".Manager.PrepareForSleep"
	if loginShutdown(&event, ":1.9") {
		t.Fatal("sleep stopped proxy")
	}
}
