package runtime

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"jeemi/internal/config/document"
	"jeemi/internal/runtimeconfig"

	"gopkg.in/yaml.v3"
)

type recordingDriver struct {
	validated   []string
	process     Process
	validateErr error
}

func (d *recordingDriver) Validate(_ context.Context, _, _, configurationPath string) error {
	d.validated = append(d.validated, configurationPath)
	return d.validateErr
}
func (d *recordingDriver) Start(_, _, _ string) (Process, error) { return d.process, nil }

func TestGenerationInjectsPrivateControllerAndUsesTUNBootstrap(t *testing.T) {
	driver := &recordingDriver{}
	dataDirectory := filepath.Join(t.TempDir(), "jeemi_data")
	store, err := newGenerationStore(dataDirectory, driver, func() time.Time {
		return time.Date(2026, 9, 2, 12, 0, 0, 123, time.UTC)
	})
	if err != nil {
		t.Fatal(err)
	}
	preferences := runtimeconfig.DefaultPreferences()
	preferences.ProxyMode = runtimeconfig.ProxyModeTUN
	generation, err := store.Prepare(context.Background(), StartRequest{
		ExecutablePath: filepath.Join(t.TempDir(), "mihomo.exe"),
		CoreVersion:    "v1.19.30", Configuration: []byte("tun:\n  enable: true\n  device: JeemiTun\nproxies: []\n"),
		Preferences: preferences, Source: Source{SubscriptionID: strings.Repeat("a", 32)},
	}, nil)
	if err == nil {
		// The recording driver does not inspect the executable, so generation
		// preparation should still be independent of a real core file.
	} else {
		t.Fatal(err)
	}
	if len(driver.validated) != 2 {
		t.Fatalf("validated = %#v", driver.validated)
	}
	finalBytes, err := os.ReadFile(generation.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	bootstrapBytes, err := os.ReadFile(generation.BootstrapPath)
	if err != nil {
		t.Fatal(err)
	}
	finalConfig, _ := document.Parse(finalBytes)
	bootstrapConfig, _ := document.Parse(bootstrapBytes)
	assertRuntimeValue(t, finalConfig, "/tun/enable", "true")
	assertRuntimeValue(t, bootstrapConfig, "/tun/enable", "false")
	assertRuntimeValue(t, finalConfig, "/external-controller", strings.TrimPrefix(generation.Session.BaseURL, "http://"))
	assertRuntimeValue(t, finalConfig, "/secret", generation.Session.Secret)
	manifest, err := os.ReadFile(filepath.Join(generation.Directory, "generation.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(manifest), generation.Session.Secret) {
		t.Fatal("generation manifest leaked the controller secret")
	}
	if err := store.Activate(generation); err != nil {
		t.Fatal(err)
	}
	if err := store.EraseSessionFiles(generation.Session); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{generation.ConfigPath, generation.BootstrapPath} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("session-only file remains at %s: %v", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(generation.Directory, "generation.json")); err != nil {
		t.Fatalf("generation metadata was removed: %v", err)
	}
}

func assertRuntimeValue(t *testing.T, configuration *yaml.Node, path, want string) {
	t.Helper()
	value, found, err := document.Find(document.Root(configuration), path)
	if err != nil || !found || value.Value != want {
		t.Fatalf("%s = %+v found=%v err=%v", path, value, found, err)
	}
}
