package appupdate

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"jeemi/internal/platform/paths"
)

// Subprocesses are copies of the test executable, never the installed client.
// The simulated GUI only acknowledges startup; no Wails or network is used.
func TestMain(m *testing.M) {
	if os.Getenv("JEEMI_UPDATER_TEST_PROCESS") == "1" {
		// A failed simulated restart may launch the restored executable without
		// arguments. Never let that child recursively run the test suite.
		if len(os.Args) == 1 {
			os.Exit(0)
		}
		if len(os.Args) != 3 {
			os.Exit(2)
		}
		switch os.Args[1] {
		case workerArgument:
			if !updateProcessPresentationOK(true) {
				os.Exit(2)
			}
			_, err := RunIfRequested()
			if err != nil {
				os.Exit(2)
			}
			os.Exit(0)
		case "--jeemi-update-restarted":
			if !updateProcessPresentationOK(false) {
				os.Exit(2)
			}
			dataRoot, err := paths.DataDirectory()
			if err != nil {
				os.Exit(2)
			}
			if !NewManager(dataRoot, "1.2.3").AcknowledgeRestart() {
				os.Exit(2)
			}
			time.Sleep(time.Second)
			os.Exit(0)
		}
		os.Exit(2)
	}
	os.Exit(m.Run())
}

func TestWorkerSubprocessStagesWaitsReplacesAndAcknowledges(t *testing.T) {
	testWorkerSubprocess(t, t.TempDir(), t.TempDir())
}

func TestWorkerSubprocessThroughDirectoryAliases(t *testing.T) {
	testWorkerSubprocess(t, aliasedTempDir(t), aliasedTempDir(t))
}

func testWorkerSubprocess(t *testing.T, configRoot, root string) {
	t.Helper()
	if runtime.GOOS == "darwin" {
		t.Skip("macOS requires a signed application bundle; covered by bundle unit tests")
	}
	if runtime.GOOS == "windows" {
		t.Setenv("APPDATA", configRoot)
	} else {
		t.Setenv("XDG_CONFIG_HOME", configRoot)
	}
	t.Setenv("JEEMI_UPDATER_TEST_PROCESS", "1")
	dataRoot, err := paths.DataDirectory()
	if err != nil {
		t.Fatal(err)
	}
	target, err := targetFor(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	binary, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(root, target.executable)
	if err = os.WriteFile(executable, binary, 0755); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, target.helper), []byte("old helper"), 0755); err != nil {
		t.Fatal(err)
	}
	// Match the canonical executable recorded by Manager.Prepare, while keeping
	// the data root aliased to exercise worker startup and restart acknowledgement.
	executable = canonicalTestPath(t, executable)
	id := strings.Repeat("c", 32)
	jobRoot := filepath.Join(dataRoot, "updates", "jeemi", id)
	packageFixture(t, filepath.Join(jobRoot, "extracted"), target, "1.2.3", binary)
	digest, _, err := hashFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	job := installJob{ID: id, Executable: executable, CurrentSHA256: digest, Version: "v1.2.3"}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	worker, err := launchWorker(ctx, jobRoot, job)
	if err != nil {
		t.Fatal(err)
	}
	committed := false
	t.Cleanup(func() {
		if !committed {
			worker.abort()
		}
	})
	if err = worker.commit(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(40 * time.Millisecond)
	if body, _ := os.ReadFile(filepath.Join(root, target.helper)); string(body) != "old helper" {
		t.Fatal("worker replaced before parent EOF")
	}
	// This controlled pipe close simulates the test GUI's process exit.
	if err = worker.pipe.Close(); err != nil {
		t.Fatal(err)
	}
	committed = true
	done := make(chan error, 1)
	go func() { done <- worker.command.Wait() }()
	select {
	case err = <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		_ = worker.command.Process.Kill()
		t.Fatal("worker timeout")
	}
	data, err := os.ReadFile(filepath.Join(dataRoot, "updates", "jeemi", "result.json"))
	if err != nil {
		t.Fatal(err)
	}
	var state State
	if json.Unmarshal(data, &state) != nil || state.Phase != "succeeded" {
		t.Fatalf("%s", data)
	}
	if body, _ := os.ReadFile(filepath.Join(root, target.helper)); string(body) != "new helper" {
		t.Fatal("helper was not paired")
	}
	if _, err = os.Stat(filepath.Join(root, ".jeemi-update-"+id)); !os.IsNotExist(err) {
		t.Fatal("transaction staging remains", err)
	}
	// Allow the fake GUI to exit before Windows removes its temporary image.
	time.Sleep(time.Second)
}
