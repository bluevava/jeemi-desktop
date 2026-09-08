//go:build windows

package processmemory

import (
	"errors"
	"os"
	"syscall"
	"testing"
	"unsafe"
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

func TestPrivateMemoryUsesExtendedWorkingSetOrPrivateCommitFallback(t *testing.T) {
	ex2Size := uint32(unsafe.Sizeof(processMemoryCountersEx2{}))
	exSize := uint32(unsafe.Sizeof(processMemoryCountersEx{}))
	for _, tc := range []struct {
		name        string
		extended    processMemoryCountersEx2
		extendedErr error
		fallbackErr error
		want        uint64
		wantCalls   int
	}{
		{name: "extended working set", extended: processMemoryCountersEx2{PrivateWorkingSetSize: 4096}, want: 4096, wantCalls: 1},
		{name: "successful older API without extended fields", want: 8192, wantCalls: 2},
		{name: "unsupported extended structure", extendedErr: syscall.EINVAL, want: 8192, wantCalls: 2},
		{name: "fallback failure stays unavailable", fallbackErr: syscall.EACCES, wantCalls: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			bytes, err := readPrivateMemory(func(size uint32) (processMemoryCountersEx2, error) {
				calls++
				if calls == 1 {
					if size != ex2Size {
						t.Fatalf("initial counter size = %d, want %d", size, ex2Size)
					}
					return tc.extended, tc.extendedErr
				}
				if calls != 2 || size != exSize {
					t.Fatalf("fallback call %d with counter size %d, want %d", calls, size, exSize)
				}
				return processMemoryCountersEx2{processMemoryCountersEx: processMemoryCountersEx{
					PrivateUsage: 8192, WorkingSetSize: 65536,
				}}, tc.fallbackErr
			})
			if bytes != tc.want || !errors.Is(err, tc.fallbackErr) || calls != tc.wantCalls {
				t.Fatalf("readPrivateMemory() = %d, %v (%d calls); want %d, %v (%d calls)", bytes, err, calls, tc.want, tc.fallbackErr, tc.wantCalls)
			}
		})
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
