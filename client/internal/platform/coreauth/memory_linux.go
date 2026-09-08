//go:build linux

package coreauth

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
	"jeemi/internal/platform/processmemory"
)

func Memory(ctx context.Context, mihomoPID int, webView bool) processmemory.HelperSnapshot {
	reply, err := serviceCall(ctx, serviceRequest{Operation: "memory", MihomoPID: mihomoPID, WebViewMemory: webView})
	if err == nil && reply.Memory != nil {
		return *reply.Memory
	}
	// An unavailable socket alone does not prove that a stopped/broken service
	// is absent. Check persistent registration before reporting zero.
	uid := uint32(os.Getuid())
	for _, path := range []string{serviceSocket(uid), serviceDirectory(uid), filepath.Join("/etc/systemd/system", serviceName(uid))} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			return processmemory.HelperSnapshot{}
		}
	}
	return processmemory.AbsentHelper()
}

func serviceMemory(clientPID int, uid uint32, mihomoPID int, webView bool) processmemory.HelperSnapshot {
	// Keep the selected Linux child tied to a kernel lifetime while sampling.
	// IPC cannot query an unrelated PID, including another user's/root process.
	selectedPID, lifetime := 0, -1
	if mihomoPID > 0 {
		fd, err := unix.PidfdOpen(mihomoPID, 0)
		if err == nil {
			lifetime = fd
			defer unix.Close(fd)
			path := "/proc/" + strconv.Itoa(mihomoPID)
			status, statusErr := os.ReadFile(path + "/status")
			data, readErr := os.ReadFile(path + "/stat")
			image, imageErr := os.Readlink(path + "/exe")
			if statusErr == nil && readErr == nil && imageErr == nil {
				// File-capability processes can expose root-owned /proc directories even
				// though the process still runs as the GUI user. Read the kernel UID row.
				if memoryProcessUID(string(status), uid) && filepath.Base(image) == "mihomo" && directMemoryChild(string(data), mihomoPID, clientPID) {
					selectedPID = mihomoPID
				}
			}
		}
	}
	result := processmemory.CaptureHelper(selectedPID)
	result.MihomoPID = mihomoPID
	if mihomoPID > 0 {
		if selectedPID == 0 {
			result.MihomoBytes = nil
		} else if ready, err := unix.Poll([]unix.PollFd{{Fd: int32(lifetime), Events: unix.POLLIN}}, 0); err != nil || ready != 0 {
			result.MihomoBytes = nil
		}
	}
	if webView {
		result.WebViewBytes = processmemory.CaptureWebView(clientPID, mihomoPID)
	}
	return result
}

func memoryProcessUID(status string, uid uint32) bool {
	for _, line := range strings.Split(status, "\n") {
		if !strings.HasPrefix(line, "Uid:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 5 {
			return false
		}
		for _, field := range fields[1:] {
			value, err := strconv.ParseUint(field, 10, 32)
			if err != nil || uint32(value) != uid {
				return false
			}
		}
		return true
	}
	return false
}

func directMemoryChild(stat string, pid, clientPID int) bool {
	opening, closing := strings.IndexByte(stat, '('), strings.LastIndexByte(stat, ')')
	if opening < 1 || closing <= opening || clientPID <= 0 || pid == clientPID {
		return false
	}
	actual, err := strconv.Atoi(strings.TrimSpace(stat[:opening]))
	fields := strings.Fields(stat[closing+1:])
	if err != nil || actual != pid || len(fields) < 22 {
		return false
	}
	parent, err := strconv.Atoi(fields[1])
	return err == nil && parent == clientPID
}
