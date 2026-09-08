//go:build darwin

package daemon

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Runs without root, service registration, TUN, DNS or system proxy changes.
func TestMacSandboxIsolation(t *testing.T) {
	if os.Getenv("JEEMI_SANDBOX_PROBE") == "1" {
		t.Skip("parent only")
	}
	if _, err := os.Stat("/usr/bin/sandbox-exec"); err != nil {
		t.Fatal("sandbox-exec unavailable")
	}
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	data := filepath.Join(base, "data")
	home := filepath.Join(data, "runtime")
	os.MkdirAll(home, 0700)
	allowed := filepath.Join(data, "config.yaml")
	outside := filepath.Join(base, "private.txt")
	os.WriteFile(allowed, []byte("allowed"), 0600)
	os.WriteFile(outside, []byte("private"), 0600)
	if _, err := os.ReadFile(filepath.Join("/System/Volumes/Data", base, "private.txt")); err != nil {
		t.Fatalf("data volume alias fixture unavailable: %v", err)
	}
	os.Symlink(outside, filepath.Join(home, "escape"))
	executable, _ := os.Executable()
	executable, _ = filepath.EvalSymlinks(executable)
	profile, err := SandboxProfile(data, home, executable)
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("/usr/bin/sandbox-exec", "-p", profile, executable, "-test.run=^TestMacSandboxProbe$", "-test.v")
	command.Env = []string{"JEEMI_SANDBOX_PROBE=1", "JEEMI_SANDBOX_BASE=" + base, "PATH=/usr/bin:/bin", "HOME=" + data, "TMPDIR=" + home}
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("sandbox fixture failed: %v\n%s", err, output)
	}
}
func TestMacSandboxProbe(t *testing.T) {
	if os.Getenv("JEEMI_SANDBOX_PROBE") != "1" {
		t.Skip("isolated child only")
	}
	base := os.Getenv("JEEMI_SANDBOX_BASE")
	home := filepath.Join(base, "data", "runtime")
	if _, err := os.ReadFile(filepath.Join(base, "data", "config.yaml")); err != nil {
		t.Error("managed read denied")
	}
	if err := os.WriteFile(filepath.Join(home, "cache"), []byte("cache"), 0600); err != nil {
		t.Error("runtime write denied")
	}
	for _, name := range []string{filepath.Join(base, "private.txt"), filepath.Join(home, "escape"), filepath.Join("/System/Volumes/Data", base, "private.txt")} {
		if _, err := os.ReadFile(name); err == nil {
			t.Error("outside data was readable")
		}
		if os.WriteFile(name, []byte("bad"), 0600) == nil {
			t.Error("outside data was writable")
		}
	}
	if os.WriteFile(filepath.Join(base, "data", "config.yaml"), []byte("bad"), 0600) == nil {
		t.Error("read-only input was writable")
	}
	if exec.Command("/bin/true").Run() == nil {
		t.Error("arbitrary executable was allowed")
	}
}
