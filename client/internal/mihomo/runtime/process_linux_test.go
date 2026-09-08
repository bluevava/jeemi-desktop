//go:build linux

package runtime

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestLinuxStopEscalatesForWholeProcessGroup(t *testing.T) {
	directory := t.TempDir()
	executable := filepath.Join(directory, "fake-mihomo")
	pidPath := filepath.Join(directory, "child.pid")
	// This isolated shell helper has no networking. Both it and its child
	// ignore TERM, exercising the SIGKILL and process-reaping path.
	if err := os.WriteFile(executable, []byte("#!/bin/sh\ntrap '' TERM\nsleep 60 &\necho $! > \"$4\"\nwait\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	process, err := (commandDriver{}).Start(executable, directory, pidPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = syscall.Kill(-process.PID(), syscall.SIGKILL) })
	var child int
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		contents, _ := os.ReadFile(pidPath)
		child, _ = strconv.Atoi(strings.TrimSpace(string(contents)))
		if child > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if child == 0 {
		t.Fatal("helper did not report child")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	if err := process.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-process.Done():
	default:
		t.Fatal("Stop did not reap the process")
	}
	contents, err := os.ReadFile("/proc/" + strconv.Itoa(child) + "/stat")
	if err == nil {
		_, tail, _ := strings.Cut(string(contents), ") ")
		if !strings.HasPrefix(tail, "Z ") {
			t.Fatalf("descendant survived forced stop: %s", tail)
		}
	}
}
