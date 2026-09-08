//go:build linux

package systemproxy

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"jeemi/internal/runtimeconfig"
)

type fakeGSettings struct {
	values map[string]string
	writes []string
	locked string
	fail   string
	drop   string
}

func newFakeGSettings() *fakeGSettings {
	values, _ := gnomeProxyValues("http=127.0.0.1:8123;https=127.0.0.1:8123")
	values["mode"] = "'auto'"
	values["http/host"] = "'previous.example'"
	values["ftp/host"] = "'legacy.example'"
	values["ignore-hosts"] = "['intranet.example']"
	values["http/use-authentication"] = "true"
	return &fakeGSettings{values: values}
}

func (f *fakeGSettings) run(ctx context.Context, args ...string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	key := args[2]
	if args[1] != gnomeSchema {
		key = strings.TrimPrefix(args[1], gnomeSchema+".") + "/" + key
	}
	if _, ok := f.values[key]; !ok {
		return "", errors.New("unexpected key")
	}
	switch args[0] {
	case "writable":
		if f.locked == key {
			return "false", nil
		}
		return "true", nil
	case "get":
		return f.values[key], nil
	case "set":
		f.writes = append(f.writes, key+"="+args[3])
		if key == f.fail {
			f.fail = ""
			return "", errors.New("write failed")
		}
		if key != f.drop {
			f.values[key] = args[3]
		}
		return "", nil
	}
	return "", errors.New("unexpected command")
}

func TestGnomeModesAndCrashRecovery(t *testing.T) {
	for _, listener := range []string{runtimeconfig.ListenerTypeHTTP, runtimeconfig.ListenerTypeSOCKS, runtimeconfig.ListenerTypeMixed} {
		t.Run(listener, func(t *testing.T) {
			settings := newFakeGSettings()
			original := make(map[string]string)
			for key, value := range settings.values {
				original[key] = value
			}
			b := gnomeBackend{run: settings.run, session: func() bool { return true }}
			path := filepath.Join(t.TempDir(), "recovery.json")
			manager := newManager(path, b)
			preferences := runtimeconfig.DefaultPreferences()
			preferences.ListenerType = listener
			if err := manager.Apply(context.Background(), preferences); err != nil {
				t.Fatal(err)
			}
			if settings.values["mode"] != "'manual'" || settings.values["ftp/host"] != "''" || settings.values["http/use-authentication"] != "false" {
				t.Fatalf("stale proxy state leaked: %v", settings.values)
			}
			if listener == runtimeconfig.ListenerTypeSOCKS && settings.values["http/host"] != "''" {
				t.Fatal("SOCKS-only mode kept old HTTP proxy")
			}
			if listener == runtimeconfig.ListenerTypeHTTP && settings.values["socks/host"] != "''" {
				t.Fatal("HTTP-only mode kept old SOCKS proxy")
			}
			if settings.writes[0] != "mode='none'" || settings.writes[len(settings.writes)-1] != "mode='manual'" {
				t.Fatal("proxy mode switched before endpoints")
			}
			preferences.ListenPort++
			if err := manager.Apply(context.Background(), preferences); err != nil {
				t.Fatal(err)
			}
			// A new manager must restore the initial snapshot, not the last Jeemi port.
			if err := newManager(path, b).Recover(context.Background()); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(original, settings.values) {
				t.Fatalf("restore changed original values: %v", settings.values)
			}
		})
	}
}

func TestGnomeFailsBeforeMutationWithoutSupportedSessionOrWritableKeys(t *testing.T) {
	settings := newFakeGSettings()
	b := gnomeBackend{run: settings.run, session: func() bool { return false }}
	if _, err := b.Capture(context.Background()); err == nil {
		t.Fatal("accepted unsupported desktop")
	}
	b.session = func() bool { return true }
	settings.locked = "https/port"
	if _, err := b.Capture(context.Background()); err == nil {
		t.Fatal("accepted locked proxy settings")
	}
	if len(settings.writes) != 0 {
		t.Fatal("preflight modified settings")
	}
	if err := b.Restore(context.Background(), snapshot{Backend: "windows"}); err == nil {
		t.Fatal("accepted another platform's recovery state")
	}
}

func TestGnomeFailedWriteRollsBackAndSilentPersistenceFailureIsDetected(t *testing.T) {
	settings := newFakeGSettings()
	b := gnomeBackend{run: settings.run, session: func() bool { return true }}
	original, _ := b.Capture(context.Background())
	settings.fail = "https/port"
	manager := newManager(filepath.Join(t.TempDir(), "recovery.json"), b)
	if err := manager.Apply(context.Background(), runtimeconfig.DefaultPreferences()); err == nil {
		t.Fatal("write failure was ignored")
	}
	if !reflect.DeepEqual(original.DesktopValues, settings.values) {
		t.Fatal("partial write was not restored")
	}
	settings.drop = "mode"
	if err := b.Apply(context.Background(), "socks=127.0.0.1:7890"); err == nil {
		t.Fatal("silent gsettings failure was ignored")
	}
}

func TestGSettingsAgainstIsolatedKeyfileBackend(t *testing.T) {
	if _, err := exec.LookPath("gsettings"); err != nil {
		t.Skip("gsettings is not installed")
	}
	// No desktop/session settings are touched: every subprocess uses a private
	// keyfile under this test's temp directory, independent of the real dconf DB.
	t.Setenv("GSETTINGS_BACKEND", "keyfile")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	b := gnomeBackend{run: runGSettings, session: func() bool { return true }}
	previous, err := b.Capture(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Apply(context.Background(), "http=127.0.0.1:7897;https=127.0.0.1:7897;socks=127.0.0.1:7897"); err != nil {
		t.Fatal(err)
	}
	if err := b.Restore(context.Background(), previous); err != nil {
		t.Fatal(err)
	}
	restored, err := b.Capture(context.Background())
	if err != nil || !reflect.DeepEqual(previous, restored) {
		t.Fatalf("restore = %v, %v", restored, err)
	}
}
