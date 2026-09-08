//go:build windows

package processmemory

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var getProcessMemoryInfo = windows.NewLazySystemDLL("psapi.dll").NewProc("GetProcessMemoryInfo")

type processMemoryCountersEx struct {
	Size                       uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
	PrivateUsage               uintptr
}

type processMemoryCountersEx2 struct {
	processMemoryCountersEx
	PrivateWorkingSetSize uintptr
	SharedCommitUsage     uint64
}

type processEntry struct {
	pid       uint32
	parentPID uint32
	name      string
}

func captureDesktop(mihomoPID int) Snapshot {
	client := memoryPointerForPID(uint32(os.Getpid()))
	entries, entriesErr := listProcesses()
	var webView *uint64
	if entriesErr == nil {
		webView = descendantMemoryPointer(
			uint32(os.Getpid()),
			entries,
			func(entry processEntry) bool {
				return strings.EqualFold(entry.name, "msedgewebview2.exe")
			},
		)
	}
	return buildSnapshot(client, webView, nil)
}

// CaptureHelper runs inside the authorization service. The caller supplies
// only the core held by its lifecycle controller, never a GUI-provided PID.
func CaptureHelper(mihomoPID int) HelperSnapshot {
	result := HelperSnapshot{HelperBytes: memoryPointerForPID(uint32(os.Getpid())), MihomoPID: mihomoPID}
	if mihomoPID <= 0 {
		result.MihomoBytes = valuePointer(0)
	} else if entries, err := listProcesses(); err == nil {
		result.MihomoBytes = processTreeMemoryPointer(uint32(mihomoPID), entries)
	}
	return result
}

// CaptureWebView is a fallback for an authenticated pipe client's own tree.
func CaptureWebView(clientPID, mihomoPID int) *uint64 {
	entries, err := listProcesses()
	if err != nil || clientPID <= 0 {
		return nil
	}
	return descendantMemoryPointer(uint32(clientPID), entries, func(entry processEntry) bool {
		return strings.EqualFold(entry.name, "msedgewebview2.exe")
	})
}

func memoryPointerForPID(pid uint32) *uint64 {
	bytes, err := processPrivateMemory(pid)
	if err != nil {
		return nil
	}
	return valuePointer(bytes)
}

func processPrivateMemory(pid uint32) (uint64, error) {
	return processPrivateMemoryAt(pid, 0)
}

func processPrivateMemoryAt(pid uint32, started uint64) (uint64, error) {
	handle, err := windows.OpenProcess(
		windows.PROCESS_QUERY_INFORMATION|windows.PROCESS_VM_READ,
		false,
		pid,
	)
	if err != nil {
		return 0, err
	}
	defer windows.CloseHandle(handle)
	if started != 0 {
		actual, err := processHandleStarted(handle)
		if err != nil || actual != started {
			return 0, fmt.Errorf("process identity changed")
		}
	}

	// PROCESS_MEMORY_COUNTERS_EX2 exposes the private working set used by the
	// Windows Task Manager Processes view. It avoids multiplying shared
	// WebView2 pages across the browser, renderer, GPU and utility processes.
	counters := processMemoryCountersEx2{}
	counters.Size = uint32(unsafe.Sizeof(counters))
	result, _, callErr := getProcessMemoryInfo.Call(
		uintptr(handle),
		uintptr(unsafe.Pointer(&counters)),
		uintptr(counters.Size),
	)
	if result != 0 {
		return uint64(counters.PrivateWorkingSetSize), nil
	}

	// Older Windows builds do not understand EX2. PrivateUsage is committed
	// private memory rather than resident private memory, but remains a safer
	// fallback than the shared-page WorkingSetSize previously reported.
	fallback := processMemoryCountersEx{}
	fallback.Size = uint32(unsafe.Sizeof(fallback))
	result, _, fallbackErr := getProcessMemoryInfo.Call(
		uintptr(handle),
		uintptr(unsafe.Pointer(&fallback)),
		uintptr(fallback.Size),
	)
	if result != 0 {
		return uint64(fallback.PrivateUsage), nil
	}
	if fallbackErr != nil && !errors.Is(fallbackErr, syscall.Errno(0)) {
		return 0, fallbackErr
	}
	if callErr != nil && !errors.Is(callErr, syscall.Errno(0)) {
		return 0, callErr
	}
	return 0, fmt.Errorf("GetProcessMemoryInfo failed")
}

func processHandleStarted(handle windows.Handle) (uint64, error) {
	var created, exited, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(handle, &created, &exited, &kernel, &user); err != nil {
		return 0, err
	}
	return uint64(created.HighDateTime)<<32 | uint64(created.LowDateTime), nil
}

func processStarted(pid uint32) (uint64, error) {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return 0, err
	}
	defer windows.CloseHandle(handle)
	return processHandleStarted(handle)
}

func descendantMemoryPointer(
	rootPID uint32,
	entries []processEntry,
	include func(processEntry) bool,
) *uint64 {
	return descendantMemory(rootPID, entries, include, processStarted, processPrivateMemoryAt)
}

func descendantMemory(rootPID uint32, entries []processEntry, include func(processEntry) bool, started func(uint32) (uint64, error), read func(uint32, uint64) (uint64, error)) *uint64 {
	children := make(map[uint32][]processEntry)
	for _, entry := range entries {
		children[entry.parentPID] = append(children[entry.parentPID], entry)
	}

	queue := []uint32{rootPID}
	rootStarted, err := started(rootPID)
	if err != nil {
		return nil
	}
	starts := map[uint32]uint64{rootPID: rootStarted}
	seen := map[uint32]struct{}{rootPID: {}}
	var total uint64
	for len(queue) > 0 {
		parent := queue[0]
		queue = queue[1:]
		for _, child := range children[parent] {
			if _, ok := seen[child.pid]; ok {
				continue
			}
			seen[child.pid] = struct{}{}
			childStarted, err := started(child.pid)
			if err != nil {
				return nil
			}
			if childStarted < starts[parent] {
				continue
			}
			starts[child.pid] = childStarted
			queue = append(queue, child.pid)
			if !include(child) {
				continue
			}
			privateMemory, queryErr := read(child.pid, childStarted)
			if queryErr != nil {
				return nil
			}
			next := sum(&total, &privateMemory)
			if next == nil {
				return nil
			}
			total = *next
		}
	}
	return valuePointer(total)
}

func processTreeMemoryPointer(rootPID uint32, entries []processEntry) *uint64 {
	started, err := processStarted(rootPID)
	if err != nil {
		return nil
	}
	rootMemory, err := processPrivateMemoryAt(rootPID, started)
	if err != nil {
		return nil
	}
	descendants := descendantMemoryPointer(
		rootPID,
		entries,
		func(processEntry) bool { return true },
	)
	if descendants == nil {
		return nil
	}
	return sum(&rootMemory, descendants)
}

func listProcesses() ([]processEntry, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snapshot)

	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	if err := windows.Process32First(snapshot, &entry); err != nil {
		return nil, err
	}
	result := make([]processEntry, 0, 128)
	for {
		result = append(result, processEntry{
			pid:       entry.ProcessID,
			parentPID: entry.ParentProcessID,
			name:      windows.UTF16ToString(entry.ExeFile[:]),
		})
		entry.Size = uint32(unsafe.Sizeof(windows.ProcessEntry32{}))
		if err := windows.Process32Next(snapshot, &entry); err != nil {
			if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
				break
			}
			return nil, err
		}
	}
	return result, nil
}
