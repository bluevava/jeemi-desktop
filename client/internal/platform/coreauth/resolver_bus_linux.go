//go:build linux

package coreauth

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
	"golang.org/x/sys/unix"
)

func systemConnection(ctx context.Context) (*dbus.Conn, error) {
	// Do not honor a caller-controlled DBUS_SYSTEM_BUS_ADDRESS in a root helper.
	return dbus.Connect("unix:path=/run/dbus/system_bus_socket", dbus.WithContext(ctx))
}

func checkResolverSupport(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	bus, err := systemConnection(ctx)
	if err != nil {
		return failure("linux_resolver_authorization_unavailable")
	}
	defer bus.Close()
	var version dbus.Variant
	err = bus.Object("org.freedesktop.systemd1", "/org/freedesktop/systemd1").CallWithContext(ctx,
		"org.freedesktop.DBus.Properties.Get", 0, "org.freedesktop.systemd1.Manager", "Version").Store(&version)
	text, ok := version.Value().(string)
	if err != nil || !ok || !supportsResolverInterfaceScope(text) {
		return failure("linux_resolver_authorization_unavailable")
	}
	return nil
}

func supportsResolverInterfaceScope(version string) bool {
	end := 0
	for end < len(version) && version[end] >= '0' && version[end] <= '9' {
		end++
	}
	major, err := strconv.Atoi(version[:end])
	// v257 supplies a trusted interface detail for all four operations. Older
	// versions must never be supported by silently dropping that constraint.
	return err == nil && major >= 257
}

type polkitSubject struct {
	Kind    string
	Details map[string]dbus.Variant
}
type polkitDecision struct {
	Authorized bool
	Challenge  bool
	Details    map[string]string
}

func verifyResolverAuthorization(ctx context.Context, uid uint32, pid int) error {
	var status unix.Stat_t
	if pid <= 1 || unix.Stat(fmt.Sprintf("/proc/%d", pid), &status) != nil || status.Uid != uid {
		return failure("linux_resolver_authorization_failed")
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return failure("linux_resolver_authorization_failed")
	}
	end := strings.LastIndexByte(string(data), ')')
	if end < 0 {
		return failure("linux_resolver_authorization_failed")
	}
	fields := strings.Fields(string(data[end+1:]))
	if len(fields) < 20 {
		return failure("linux_resolver_authorization_failed")
	}
	start, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil {
		return failure("linux_resolver_authorization_failed")
	}
	subject := polkitSubject{"unix-process", map[string]dbus.Variant{
		"pid": dbus.MakeVariant(uint32(pid)), "start-time": dbus.MakeVariant(start), "uid": dbus.MakeVariant(int32(uid)),
	}}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	bus, err := systemConnection(ctx)
	if err != nil {
		return failure("linux_resolver_authorization_failed")
	}
	defer bus.Close()
	check := func() bool {
		for _, action := range resolverActions {
			var decision polkitDecision
			err := bus.Object("org.freedesktop.PolicyKit1", "/org/freedesktop/PolicyKit1/Authority").CallWithContext(ctx,
				"org.freedesktop.PolicyKit1.Authority.CheckAuthorization", 0, subject, action,
				map[string]string{"interface": "JeemiTun"}, uint32(0), "").Store(&decision)
			if err != nil || !decision.Authorized {
				return false
			}
		}
		return true
	}
	// Polkit watches the rules directory asynchronously. Never restart its daemon
	// and never permit these checks to display authentication UI.
	for {
		if check() {
			return nil
		}
		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return failure("linux_resolver_authorization_failed")
		case <-timer.C:
		}
	}
}
