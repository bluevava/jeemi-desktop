//go:build windows

package daemon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
	"jeemi/internal/platform/winauth"
)

// The fixture is this Go test binary, never a real proxy core. An explicit test
// marker enables a loopback HTTP fixture; it never changes system networking.
func TestMain(m *testing.M) {
	if len(os.Args) == 5 && os.Args[1] == "-d" && os.Args[3] == "-f" && filepath.Base(os.Args[0]) == "mihomo.exe" {
		if _, err := os.Stat(filepath.Join(os.Args[2], "test-controller.enabled")); err == nil {
			server, err := startControllerFixture(os.Args[4])
			if err != nil {
				os.Exit(2)
			}
			defer server.Close()
		}
		image, err := os.Executable()
		if err != nil || os.WriteFile(filepath.Join(os.Args[2], "test-image.txt"), []byte(image), 0600) != nil {
			os.Exit(2)
		}
		for deadline := time.Now().Add(20 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
			if _, err := os.Stat(filepath.Join(os.Args[2], "test-exit")); err == nil {
				os.Exit(0)
			}
		}
		os.Exit(3)
	}
	os.Exit(m.Run())
}

func selectedCoreFixture(t *testing.T, executable bool) (Target, []byte, *serviceCores, string, string) {
	t.Helper()
	root := t.TempDir()
	data := []byte("MZ" + strings.Repeat("x", 128))
	if executable {
		self, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		data, err = os.ReadFile(self)
		if err != nil {
			t.Fatal(err)
		}
	}
	hash := sha256.Sum256(data)
	target := Target{Version: "v1.2.3", SHA256: hex.EncodeToString(hash[:]), Size: int64(len(data))}
	target.ExecutablePath = filepath.Join(root, "带 空格", "jeemi_data", "core", "mihomo", target.Version, "windows-"+runtime.GOARCH, "mihomo.exe")
	if err := os.MkdirAll(filepath.Dir(target.ExecutablePath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target.ExecutablePath, data, 0600); err != nil {
		t.Fatal(err)
	}
	// No protected ProgramData files, service installation or ACL changes.
	serviceRoot := filepath.Join(root, "service")
	workspace := filepath.Join(serviceRoot, "session-test")
	configuration := "generations/one/bootstrap.yaml"
	writeTestWorkspace(t, workspace, configuration)
	return target, data, &serviceCores{root: serviceRoot}, workspace, configuration
}

func writeTestWorkspace(t *testing.T, workspace, configuration string) {
	t.Helper()
	directory := filepath.Dir(filepath.Join(workspace, filepath.FromSlash(configuration)))
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"bootstrap.yaml", "config.yaml"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte("external-controller: 127.0.0.1:0\nsecret: test-only\ntun:\n  enable: false\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func assertTestCoreImage(t *testing.T, workspace, executable string) {
	t.Helper()
	var image []byte
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		image, _ = os.ReadFile(filepath.Join(workspace, "test-image.txt"))
		if len(image) != 0 {
			break
		}
	}
	if !strings.EqualFold(string(image), executable) {
		t.Fatalf("actual process image %q, want selected data file %q", image, executable)
	}
}

func TestServiceRunsSelectedDataPathAndReleasesFileOnStopOrExit(t *testing.T) {
	for _, exit := range []bool{false, true} {
		name := "stop"
		if exit {
			name = "exit"
		}
		t.Run(name, func(t *testing.T) {
			target, data, cores, workspace, configuration := selectedCoreFixture(t, true)
			file, err := winauth.OpenInstalledCore(context.Background(), target)
			if err != nil {
				t.Fatal(err)
			}
			if err = cores.start(file, workspace, configuration); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := cores.stop(ctx); err != nil {
					t.Error(err)
				}
			})
			assertTestCoreImage(t, workspace, target.ExecutablePath)
			if err := os.WriteFile(target.ExecutablePath, data, 0600); err == nil {
				t.Fatal("running selected file could be overwritten")
			}
			if exit {
				if err := os.WriteFile(filepath.Join(workspace, "test-exit"), nil, 0600); err != nil {
					t.Fatal(err)
				}
				select {
				case <-cores.done:
				case <-time.After(5 * time.Second):
					t.Fatal("test core did not exit")
				}
			} else {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := cores.stop(ctx); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(target.ExecutablePath, data, 0600); err != nil {
				t.Fatalf("selected file remains locked after process exit: %v", err)
			}
			if _, err := os.Stat(filepath.Join(cores.root, "core")); !os.IsNotExist(err) {
				t.Fatal("starting the core created a helper copy")
			}
		})
	}
}

func TestServiceFollowsSelectedVersionAcrossRestarts(t *testing.T) {
	target, data, cores, workspace, configuration := selectedCoreFixture(t, true)
	coreRoot := filepath.Dir(filepath.Dir(filepath.Dir(target.ExecutablePath)))
	s := &server{root: cores.root, shutdown: make(chan struct{})}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := cores.stop(ctx); err != nil {
			t.Error(err)
		}
	})
	for _, version := range []string{target.Version, "unknown-0123456789abcdef"} {
		selected := target
		selected.Version = version
		selected.ExecutablePath = filepath.Join(coreRoot, version, "windows-"+runtime.GOARCH, "mihomo.exe")
		if err := os.MkdirAll(filepath.Dir(selected.ExecutablePath), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(selected.ExecutablePath, data, 0600); err != nil {
			t.Fatal(err)
		}
		writeTestWorkspace(t, workspace, configuration)
		if err := s.prepareCore(windows.CurrentProcess(), selected); err != nil {
			t.Fatal(err)
		}
		file, err := openCallerCore(context.Background(), windows.CurrentProcess(), selected)
		if err != nil {
			t.Fatal(err)
		}
		if err := cores.start(file, workspace, configuration); err != nil {
			t.Fatal(err)
		}
		assertTestCoreImage(t, workspace, selected.ExecutablePath)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err = cores.stop(ctx)
		cancel()
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(filepath.Join(cores.root, "core")); !os.IsNotExist(err) {
		t.Fatal("changing versions created a helper copy")
	}
}

func TestStartFailureReleasesSelectedFile(t *testing.T) {
	target, data, cores, workspace, configuration := selectedCoreFixture(t, false)
	file, err := winauth.OpenInstalledCore(context.Background(), target)
	if err != nil {
		t.Fatal(err)
	}
	if err := cores.start(file, workspace, configuration); err == nil {
		t.Fatal("invalid test executable started")
	}
	if err := os.WriteFile(target.ExecutablePath, data, 0600); err != nil {
		t.Fatalf("failed start left source locked: %v", err)
	}
}

func TestPreparationAlwaysChecksSelectedCoreWithoutCreatingOrUsingCopies(t *testing.T) {
	target, data, cores, _, _ := selectedCoreFixture(t, false)
	s := &server{root: cores.root, shutdown: make(chan struct{})}
	if err := s.prepareCore(windows.CurrentProcess(), target); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(cores.root, "core")); !os.IsNotExist(err) {
		t.Fatal("preparation created a helper copy")
	}
	// Simulate a legacy helper copy. A missing or changed selected core must
	// fail even when a matching old copy is available in the service directory.
	legacy := filepath.Join(cores.root, "core", "mihomo", target.Version, "windows-"+runtime.GOARCH, "mihomo.exe")
	if err := os.MkdirAll(filepath.Dir(legacy), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, data, 0600); err != nil {
		t.Fatal(err)
	}
	data[len(data)-1]++
	if err := os.WriteFile(target.ExecutablePath, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.prepareCore(windows.CurrentProcess(), target); err == nil || err.Error() != "authorization_core_changed" {
		t.Fatalf("preparation accepted a changed selected core: %v", err)
	}
	if err := os.Remove(target.ExecutablePath); err != nil {
		t.Fatal(err)
	}
	if err := s.prepareCore(windows.CurrentProcess(), target); err == nil {
		t.Fatal("preparation used an old helper copy after selected core removal")
	}
}
