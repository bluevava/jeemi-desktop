//go:build linux

package processmemory

import (
	"errors"
	"fmt"
	"io/fs"
	"math"
	"strconv"
	"strings"
	"syscall"
)

type linuxProcess struct {
	pid           int
	parentPID     int
	name          string
	startTime     uint64
	residentBytes uint64
}

func readLinuxProcesses(proc fs.FS, pageSize uint64) (map[int]linuxProcess, error) {
	return readLinuxProcessesWithout(proc, pageSize, 0)
}

func readLinuxProcessesWithout(proc fs.FS, pageSize uint64, excludedPID int) (map[int]linuxProcess, error) {
	entries, err := fs.ReadDir(proc, ".")
	if err != nil {
		return nil, err
	}
	result := make(map[int]linuxProcess)
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 || pid == excludedPID || !entry.IsDir() {
			continue
		}
		contents, err := fs.ReadFile(proc, entry.Name()+"/stat")
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ESRCH) {
			continue // The process exited while /proc was being enumerated.
		}
		if err != nil {
			return nil, err // Do not present a partial enumeration as a total.
		}
		process, err := parseLinuxProcessStat(string(contents), pageSize)
		if err != nil || process.pid != pid {
			return nil, fmt.Errorf("invalid process memory record")
		}
		result[pid] = process
	}
	return result, nil
}

func parseLinuxProcessStat(contents string, pageSize uint64) (linuxProcess, error) {
	invalid := fmt.Errorf("invalid process memory record")
	opening, closing := strings.IndexByte(contents, '('), strings.LastIndexByte(contents, ')')
	if opening < 1 || closing <= opening || pageSize == 0 {
		return linuxProcess{}, invalid
	}
	pid, err := strconv.Atoi(strings.TrimSpace(contents[:opening]))
	if err != nil || pid <= 0 {
		return linuxProcess{}, invalid
	}
	// comm may contain spaces, parentheses or newlines. Only split the fields
	// after its final ')'. RSS and identity come from the same kernel record.
	fields := strings.Fields(contents[closing+1:])
	if len(fields) < 22 || len(fields[0]) != 1 {
		return linuxProcess{}, invalid
	}
	parentPID, err := strconv.Atoi(fields[1])
	if err != nil || parentPID < 0 {
		return linuxProcess{}, invalid
	}
	startTime, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil {
		return linuxProcess{}, invalid
	}
	pages, err := strconv.ParseUint(fields[21], 10, 64)
	if err != nil || pages > math.MaxUint64/pageSize {
		return linuxProcess{}, invalid
	}
	return linuxProcess{
		pid: pid, parentPID: parentPID, name: contents[opening+1 : closing],
		startTime: startTime, residentBytes: pages * pageSize,
	}, nil
}
