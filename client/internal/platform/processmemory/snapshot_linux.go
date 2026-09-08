//go:build linux

package processmemory

import (
	"io/fs"
	"math"
	"os"
	"strings"
)

func captureDesktop(mihomoPID int) Snapshot {
	return captureLinuxMode(os.DirFS("/proc"), os.Getpid(), mihomoPID, uint64(os.Getpagesize()), true)
}

func captureLinux(proc fs.FS, clientPID, mihomoPID int, pageSize uint64) Snapshot {
	return captureLinuxMode(proc, clientPID, mihomoPID, pageSize, false)
}

func captureLinuxMode(proc fs.FS, clientPID, mihomoPID int, pageSize uint64, desktop bool) Snapshot {
	excluded := 0
	if desktop {
		excluded = mihomoPID
	}
	processes, err := readLinuxProcessesWithout(proc, pageSize, excluded)
	if err != nil {
		processes = nil
	}
	return snapshotLinux(processes, clientPID, mihomoPID, desktop)
}

func snapshotLinux(processes map[int]linuxProcess, clientPID, mihomoPID int, desktop bool) Snapshot {
	var mihomo *uint64
	if mihomoPID <= 0 {
		mihomo = valuePointer(0)
	}
	unavailable := buildSnapshot(nil, nil, mihomo)
	root, ok := processes[clientPID]
	if !ok || clientPID == mihomoPID {
		return unavailable
	}
	children := make(map[int][]linuxProcess)
	for _, process := range processes {
		children[process.parentPID] = append(children[process.parentPID], process)
	}

	const (
		clientGroup = iota
		webKitGroup
		mihomoGroup
	)
	type member struct {
		process linuxProcess
		group   int
	}
	queue := []member{{root, clientGroup}}
	seen := make(map[int]bool)
	var totals [3]uint64
	var total uint64
	foundCore := false
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		process := current.process
		if seen[process.pid] {
			continue
		}
		if desktop && (process.pid == mihomoPID || process.name == "mihomo") {
			continue // Do not read or count the core subtree as GUI helper processes.
		}
		seen[process.pid] = true
		group := current.group
		if process.pid == mihomoPID {
			group = mihomoGroup
			foundCore = true
		} else if process.pid != clientPID && group != mihomoGroup && strings.HasPrefix(process.name, "WebKit") {
			// Linux comm truncates names to 15 bytes (e.g. WebKitWebProces).
			// Only descendants of this Jeemi instance can enter this group.
			group = webKitGroup
		}
		if process.residentBytes > math.MaxUint64-total {
			return unavailable
		}
		total += process.residentBytes
		totals[group] += process.residentBytes
		for _, child := range children[process.pid] {
			// A child older than its sampled parent belongs to a previous use
			// of that PID, not this process tree.
			if child.startTime >= process.startTime {
				queue = append(queue, member{child, group})
			}
		}
	}
	if foundCore {
		mihomo = valuePointer(totals[mihomoGroup])
	}
	// Non-WebKit/non-core descendants include the short-lived core guard and
	// sandbox launch helpers. Count them with the client so the total is not
	// missing these processes. Every PID contributes to exactly one category.
	return buildSnapshot(valuePointer(totals[clientGroup]), valuePointer(totals[webKitGroup]), mihomo)
}

func CaptureHelper(mihomoPID int) HelperSnapshot {
	proc := os.DirFS("/proc")
	pageSize := uint64(os.Getpagesize())
	processes, err := readLinuxProcesses(proc, pageSize)
	if err != nil {
		processes = nil
	}
	helper := snapshotLinux(processes, os.Getpid(), mihomoPID, false)
	result := HelperSnapshot{HelperBytes: helper.ClientBytes, MihomoPID: mihomoPID}
	if mihomoPID <= 0 {
		result.MihomoBytes = valuePointer(0)
	} else {
		// On Linux the authorized core remains a GUI child, outside the service tree.
		result.MihomoBytes = snapshotLinux(processes, mihomoPID, 0, false).TotalBytes
	}
	return result
}

func CaptureWebView(clientPID, mihomoPID int) *uint64 {
	return captureLinuxMode(os.DirFS("/proc"), clientPID, mihomoPID, uint64(os.Getpagesize()), true).WebViewBytes
}
