//go:build windows

package processmemory

import (
	"errors"
	"os"
	"testing"
)

func TestProcessPrivateMemoryReadsCurrentProcess(t *testing.T) {
	bytes, err := processPrivateMemory(uint32(os.Getpid()))
	if err != nil {
		t.Fatalf("processPrivateMemory() error = %v", err)
	}
	if bytes == 0 {
		t.Fatal("processPrivateMemory() = 0, want a non-zero measurement")
	}
}

func TestWindowsWebViewRejectsForeignTreesAndReusedParentPID(t *testing.T) {
	entries := []processEntry{{pid: 11, parentPID: 10, name: "msedgewebview2.exe"}, {pid: 12, parentPID: 11, name: "msedgewebview2.exe"}, {pid: 21, parentPID: 20, name: "msedgewebview2.exe"}, {pid: 13, parentPID: 10, name: "msedgewebview2.exe"}, {pid: 14, parentPID: 13, name: "msedgewebview2.exe"}}
	starts := map[uint32]uint64{10: 10, 11: 11, 12: 12, 13: 5, 14: 6}
	seen := map[uint32]bool{}
	value := descendantMemory(10, entries, func(p processEntry) bool { return p.name == "msedgewebview2.exe" }, func(pid uint32) (uint64, error) { return starts[pid], nil }, func(pid uint32, started uint64) (uint64, error) {
		seen[pid] = true
		if started != starts[pid] {
			t.Fatal("memory read lost process identity")
		}
		return uint64(pid), nil
	})
	if value == nil || *value != 23 || len(seen) != 2 {
		t.Fatal("foreign or reused process entered web memory")
	}
	value = descendantMemory(10, entries, func(processEntry) bool { return true }, func(pid uint32) (uint64, error) { return starts[pid], nil }, func(uint32, uint64) (uint64, error) { return 0, errors.New("denied") })
	if value != nil {
		t.Fatal("partial WebView tree reported as complete")
	}
}
