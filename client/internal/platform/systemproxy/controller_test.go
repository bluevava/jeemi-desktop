package systemproxy

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"jeemi/internal/runtimeconfig"
)

type fakeBackend struct {
	captured    snapshot
	applied     []string
	restored    []snapshot
	failApply   bool
	failRestore bool
	failNotify  bool
}

func (f *fakeBackend) Capture(context.Context) (snapshot, error) { return f.captured, nil }
func (f *fakeBackend) Apply(_ context.Context, value string) error {
	if f.failApply {
		return errors.New("apply failed")
	}
	f.applied = append(f.applied, value)
	return nil
}
func (f *fakeBackend) Restore(_ context.Context, value snapshot) error {
	f.restored = append(f.restored, value)
	if f.failRestore {
		return errors.New("restore failed")
	}
	return nil
}
func (f *fakeBackend) Notify(context.Context) error {
	if f.failNotify {
		return errors.New("notify failed")
	}
	return nil
}

func TestManagerRetainsRecoveryUntilRestoreAndNotifySucceed(t *testing.T) {
	for _, notify := range []bool{false, true} {
		platform := &fakeBackend{failApply: !notify, failRestore: !notify, failNotify: notify}
		path := filepath.Join(t.TempDir(), "recovery.json")
		manager := newManager(path, platform)
		if err := manager.Apply(context.Background(), runtimeconfig.DefaultPreferences()); err == nil {
			t.Fatal("expected apply failure")
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("recovery record discarded: %v", err)
		}
		platform.failRestore, platform.failNotify = false, false
		if err := manager.Recover(context.Background()); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("recovery record retained after success")
		}
	}
}

func TestManagerPersistsOriginalStateOnceAndRestoresIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime", "recovery.json")
	platform := &fakeBackend{captured: snapshot{
		ProxyEnableExists: true, ProxyEnable: 0, ProxyServerExists: true, ProxyServer: "old:8080",
		ProxyOverrideCaptured: true, ProxyOverrideExists: true, ProxyOverride: "intranet.example;<local>",
	}}
	manager := newManager(path, platform)
	preferences := runtimeconfig.DefaultPreferences()
	preferences.ListenPort = 7897
	if err := manager.Apply(context.Background(), preferences); err != nil {
		t.Fatal(err)
	}
	preferences.ListenPort = 7898
	if err := manager.Apply(context.Background(), preferences); err != nil {
		t.Fatal(err)
	}
	if len(platform.applied) != 2 || platform.applied[1] != "http=127.0.0.1:7898;https=127.0.0.1:7898;socks=127.0.0.1:7898" {
		t.Fatalf("unexpected applied values: %#v", platform.applied)
	}
	if err := manager.Restore(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(platform.restored) != 1 || platform.restored[0].ProxyServer != "old:8080" || platform.restored[0].ProxyOverride != "intranet.example;<local>" {
		t.Fatalf("restore = %#v", platform.restored)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("recovery state remains: %v", err)
	}
}

func TestManagerRollsBackWhenApplyFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recovery.json")
	platform := &fakeBackend{captured: snapshot{ProxyEnableExists: true}, failApply: true}
	manager := newManager(path, platform)
	if err := manager.Apply(context.Background(), runtimeconfig.DefaultPreferences()); err == nil {
		t.Fatal("Apply() succeeded")
	}
	if len(platform.restored) != 1 {
		t.Fatalf("original proxy was not restored: %#v", platform.restored)
	}
}
