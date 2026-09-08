//go:build darwin && cgo

package processmemory

/*
#cgo LDFLAGS: -lproc
#include "proc_darwin.h"
*/
import "C"

import "os"

func darwinProcessInfo(info *C.jeemi_memory_process) darwinProcess {
	return darwinProcess{pid: int(info.pid), parent: int(info.parent), uid: uint32(info.uid), started: uint64(info.started), coalition: uint64(info.coalition), name: C.GoString(&info.name[0])}
}

func readDarwinProcesses(rootPID int) (map[int]darwinProcess, bool) {
	var root C.jeemi_memory_process
	if rootPID <= 0 || C.jeemi_memory_process_info(C.int(rootPID), &root) <= 0 {
		return nil, false
	}
	processes := map[int]darwinProcess{rootPID: darwinProcessInfo(&root)}
	var parent C.jeemi_memory_process
	if C.jeemi_memory_process_info(root.parent, &parent) > 0 {
		processes[int(parent.pid)] = darwinProcessInfo(&parent)
	}
	size := C.jeemi_memory_pids(root.uid, nil, 0)
	for attempts := 0; attempts < 3 && size > 0 && size < 4<<20; attempts++ {
		pids := make([]C.int, int(size)/4+256)
		count := C.jeemi_memory_pids(root.uid, &pids[0], C.int(len(pids)*4))
		if count <= 0 {
			return processes, false
		}
		if int(count) >= len(pids)*4 {
			size = count
			continue
		}
		complete := true
		for _, pid := range pids[:int(count)/4] {
			if pid <= 0 {
				continue
			}
			var info C.jeemi_memory_process
			status := C.jeemi_memory_process_info(pid, &info)
			if status < 0 {
				complete = false
			}
			if status <= 0 {
				continue
			}
			processes[int(pid)] = darwinProcessInfo(&info)
		}
		return processes, complete
	}
	return processes, false
}

func darwinResident(p darwinProcess) *uint64 {
	var bytes C.uint64_t
	if p.pid <= 0 || C.jeemi_memory_resident(C.int(p.pid), C.uint64_t(p.started), &bytes) == 0 {
		return nil
	}
	return valuePointer(uint64(bytes))
}

func captureDesktop(mihomoPID int) Snapshot {
	processes, complete := readDarwinProcesses(os.Getpid())
	if !complete {
		return buildSnapshot(darwinResident(processes[os.Getpid()]), nil, nil)
	}
	return darwinDesktop(processes, os.Getpid(), mihomoPID, darwinResident)
}

func CaptureHelper(mihomoPID int) HelperSnapshot {
	processes, complete := readDarwinProcesses(os.Getpid())
	result := HelperSnapshot{HelperBytes: darwinResident(processes[os.Getpid()]), MihomoPID: mihomoPID}
	if mihomoPID <= 0 {
		result.MihomoBytes = valuePointer(0)
		return result
	}
	if !complete {
		return result
	}
	members := darwinMembers(processes, os.Getpid(), 0)
	if !members[mihomoPID] {
		return result
	}
	total := valuePointer(0)
	for pid := range darwinMembers(processes, mihomoPID, 0) {
		total = sum(total, darwinResident(processes[pid]))
	}
	result.MihomoBytes = total
	return result
}

func CaptureWebView(clientPID, mihomoPID int) *uint64 {
	processes, complete := readDarwinProcesses(clientPID)
	if !complete {
		return nil
	}
	return darwinDesktop(processes, clientPID, mihomoPID, darwinResident).WebViewBytes
}
