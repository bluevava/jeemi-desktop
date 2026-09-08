//go:build linux

package processmemory

import (
	"fmt"
	"io/fs"
	"math"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
)

func memoryStat(pid, parent int, name string, started, pages uint64) string {
	fields := strings.Fields(strings.Repeat("0 ", 22))
	fields[0], fields[1] = "S", strconv.Itoa(parent)
	fields[19], fields[21] = strconv.FormatUint(started, 10), strconv.FormatUint(pages, 10)
	return fmt.Sprintf("%d (%s) %s\n", pid, name, strings.Join(fields, " "))
}

func addMemoryProcess(proc fstest.MapFS, pid, parent int, name string, started, pages uint64) {
	proc[strconv.Itoa(pid)+"/stat"] = &fstest.MapFile{Data: []byte(memoryStat(pid, parent, name, started, pages))}
}

func requireMemoryValue(t *testing.T, got *uint64, want uint64) {
	t.Helper()
	if got == nil || *got != want {
		t.Fatalf("memory = %v, want %d", got, want)
	}
}

func TestLinuxMemoryIncludesOnlyThisApplicationTree(t *testing.T) {
	proc := fstest.MapFS{}
	addMemoryProcess(proc, 100, 1, "Jeemi", 10, 10)
	addMemoryProcess(proc, 110, 100, "WebKitNetworkPr", 11, 20)
	addMemoryProcess(proc, 120, 100, "bwrap", 12, 1)
	addMemoryProcess(proc, 121, 120, "bwrap", 13, 2)
	addMemoryProcess(proc, 122, 121, "WebKitWebProces", 14, 30)
	addMemoryProcess(proc, 123, 122, "renderer-worker", 15, 3)
	addMemoryProcess(proc, 130, 100, "mihomo", 16, 40)
	addMemoryProcess(proc, 131, 130, "core-worker", 17, 4)
	addMemoryProcess(proc, 132, 130, "WebKitIgnored", 18, 5)
	addMemoryProcess(proc, 140, 100, "Jeemi", 19, 6) // Unprivileged lifetime guard.
	addMemoryProcess(proc, 200, 1, "OtherApp", 20, 200)
	addMemoryProcess(proc, 201, 200, "WebKitWebProces", 21, 300)
	addMemoryProcess(proc, 202, 200, "mihomo", 22, 400)
	// No smaps file is needed, including for the capability-bearing core.
	snapshot := captureLinux(proc, 100, 130, 4096)
	requireMemoryValue(t, snapshot.ClientBytes, (10+1+2+6)*4096)
	requireMemoryValue(t, snapshot.WebViewBytes, (20+30+3)*4096)
	requireMemoryValue(t, snapshot.MihomoBytes, (40+4+5)*4096)
	requireMemoryValue(t, snapshot.TotalBytes, (10+1+2+6+20+30+3+40+4+5)*4096)
}

func TestLinuxMemoryStoppedCoreAndNoWebProcessesAreMeasuredZero(t *testing.T) {
	proc := fstest.MapFS{}
	addMemoryProcess(proc, 100, 1, "Jeemi", 1, 10)
	snapshot := captureLinux(proc, 100, 0, 65536)
	requireMemoryValue(t, snapshot.ClientBytes, 10*65536)
	requireMemoryValue(t, snapshot.WebViewBytes, 0)
	requireMemoryValue(t, snapshot.MihomoBytes, 0)
	requireMemoryValue(t, snapshot.TotalBytes, 10*65536)
}

func TestLinuxMemoryMissingOrUnrelatedActiveCoreLeavesTotalUnavailable(t *testing.T) {
	proc := fstest.MapFS{}
	addMemoryProcess(proc, 100, 1, "Jeemi", 10, 10)
	addMemoryProcess(proc, 200, 1, "mihomo", 11, 999)
	for _, pid := range []int{200, 300} {
		snapshot := captureLinux(proc, 100, pid, 4096)
		requireMemoryValue(t, snapshot.ClientBytes, 10*4096)
		if snapshot.MihomoBytes != nil || snapshot.TotalBytes != nil {
			t.Fatal("unavailable running core was counted as zero or as a foreign process")
		}
	}
}

type unreadableMemoryFS struct {
	fs.FS
	path string
}

func (proc unreadableMemoryFS) Open(name string) (fs.File, error) {
	if name == proc.path {
		return nil, fs.ErrPermission
	}
	return proc.FS.Open(name)
}

func TestLinuxMemoryReadFailureDoesNotReportPartialOrZeroMeasurements(t *testing.T) {
	proc := fstest.MapFS{}
	addMemoryProcess(proc, 100, 1, "Jeemi", 10, 10)
	addMemoryProcess(proc, 110, 100, "WebKitWebProces", 11, 20)
	for _, path := range []string{".", "110/stat"} {
		snapshot := captureLinux(unreadableMemoryFS{proc, path}, 100, 0, 4096)
		if snapshot.ClientBytes != nil || snapshot.WebViewBytes != nil || snapshot.TotalBytes != nil {
			t.Fatal("incomplete enumeration was presented as a complete measurement")
		}
		requireMemoryValue(t, snapshot.MihomoBytes, 0)
	}
}

func TestLinuxMemoryHandlesExitedProcessesAndPIDReuse(t *testing.T) {
	proc := fstest.MapFS{}
	addMemoryProcess(proc, 100, 1, "Jeemi", 10, 10)
	addMemoryProcess(proc, 110, 100, "WebKitNetworkPr", 20, 20)
	addMemoryProcess(proc, 111, 110, "WebKitWebProces", 15, 100) // Previous use of PID 110.
	addMemoryProcess(proc, 112, 111, "worker", 16, 100)
	proc["120"] = &fstest.MapFile{Mode: fs.ModeDir} // Exited before reading stat.
	snapshot := captureLinux(proc, 100, 0, 4096)
	requireMemoryValue(t, snapshot.WebViewBytes, 20*4096)
	requireMemoryValue(t, snapshot.TotalBytes, 30*4096)
}

func TestLinuxMemoryParsesNamesWithoutShiftingMemoryFields(t *testing.T) {
	for _, name := range []string{"WebKitWebProces", "a (name) with )", "name\nwith newline"} {
		process, err := parseLinuxProcessStat(memoryStat(123, 12, name, 900, 37), 4096)
		if err != nil || process.name != name || process.pid != 123 || process.parentPID != 12 || process.startTime != 900 || process.residentBytes != 37*4096 {
			t.Fatalf("parsed record = %+v, error = %v", process, err)
		}
	}
}

func TestLinuxMemoryRejectsMalformedOrOverflowingRecords(t *testing.T) {
	valid := memoryStat(123, 12, "Jeemi", 900, 37)
	for _, record := range []string{
		"", "123 Jeemi S 12", "123 (Jeemi) S 12", strings.Replace(valid, "123 (", "bad (", 1),
		strings.Replace(valid, "S 12", "S -1", 1), strings.Replace(valid, " 37\n", " -1\n", 1),
		memoryStat(123, 12, "Jeemi", 900, math.MaxUint64),
	} {
		if _, err := parseLinuxProcessStat(record, 4096); err == nil {
			t.Fatalf("accepted invalid record: %q", record)
		}
	}
	if _, err := parseLinuxProcessStat(valid, 0); err == nil {
		t.Fatal("accepted zero page size")
	}
	proc := fstest.MapFS{"124/stat": {Data: []byte(valid)}}
	if _, err := readLinuxProcesses(proc, 4096); err == nil {
		t.Fatal("accepted a mismatched PID")
	}
}

func TestLinuxMemoryRejectsOverflowingTotal(t *testing.T) {
	proc := fstest.MapFS{}
	addMemoryProcess(proc, 100, 1, "Jeemi", 1, math.MaxUint64)
	addMemoryProcess(proc, 101, 100, "WebKitWebProces", 2, 1)
	if snapshot := captureLinux(proc, 100, 0, 1); snapshot.TotalBytes != nil {
		t.Fatal("overflowed total must be unavailable")
	}
}

func TestLinuxDesktopDoesNotQueryAuthorizedCore(t *testing.T) {
	proc := fstest.MapFS{}
	addMemoryProcess(proc, 100, 1, "Jeemi", 10, 10)
	addMemoryProcess(proc, 110, 100, "WebKitWebProces", 11, 20)
	addMemoryProcess(proc, 120, 100, "mihomo", 12, 30)
	snapshot := captureLinuxMode(unreadableMemoryFS{proc, "120/stat"}, 100, 120, 4096, true)
	requireMemoryValue(t, snapshot.ClientBytes, 10*4096)
	requireMemoryValue(t, snapshot.WebViewBytes, 20*4096)
	if snapshot.MihomoBytes != nil {
		t.Fatal("GUI queried core memory")
	}
}
