//go:build linux

package requirements

import (
	"encoding/binary"
	"fmt"
	"os"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

const tunCapabilities = uint64(1<<unix.CAP_NET_ADMIN | 1<<unix.CAP_NET_RAW | 1<<unix.CAP_NET_BIND_SERVICE)

// CheckTUN inspects permissions only. It never opens/configures TUN, changes
// capabilities, invokes sudo, or starts an elevated process.
func CheckTUN(executable string, runningPID int) error {
	info, err := os.Stat("/dev/net/tun")
	if err != nil || info.Mode()&os.ModeCharDevice == 0 || unix.Access("/dev/net/tun", unix.R_OK|unix.W_OK) != nil {
		return &Error{Code: "linux_tun_device_unavailable", Message: "Linux TUN device /dev/net/tun is unavailable"}
	}
	return CheckCoreCapabilities(executable, runningPID)
}

// CheckCoreCapabilities applies to every Linux core process, including system
// proxy mode. Only explicit application actions may request missing privileges.
func CheckCoreCapabilities(executable string, runningPID int) error {
	statusPath := "/proc/self/status"
	if runningPID > 0 {
		statusPath = fmt.Sprintf("/proc/%d/status", runningPID)
	}
	status, err := os.ReadFile(statusPath)
	if err != nil {
		return tunPermissionError()
	}
	if runningPID > 0 {
		if statusCapability(string(status), "CapEff")&tunCapabilities != tunCapabilities {
			return tunPermissionError()
		}
		return nil
	}
	var filesystem unix.Statfs_t
	if err := unix.Statfs(executable, &filesystem); err != nil || filesystem.Flags&unix.ST_NOEXEC != 0 {
		return &Error{Code: "linux_core_not_executable", Message: "selected Linux core must be executable on a filesystem without noexec"}
	}
	if statusCapability(string(status), "CapAmb")&tunCapabilities != tunCapabilities &&
		(statusCapability(string(status), "NoNewPrivs") != 0 || statusCapability(string(status), "CapBnd")&tunCapabilities != tunCapabilities) {
		return &Error{Code: "linux_core_privileges_blocked", Message: "Linux process restrictions prevent acquiring the required network capabilities"}
	}
	if filesystem.Flags&unix.ST_NOSUID != 0 && os.Geteuid() != 0 && statusCapability(string(status), "CapAmb")&tunCapabilities != tunCapabilities {
		return &Error{Code: "linux_core_authorization_filesystem", Message: "Linux nosuid mount prevents using persistent file capabilities"}
	}
	buffer := make([]byte, 24)
	size, _ := unix.Getxattr(executable, "security.capability", buffer)
	if size < 0 || size > len(buffer) {
		size = 0
	}
	if !canStartTUN(string(status), buffer[:size], os.Geteuid() == 0, filesystem.Flags&unix.ST_NOSUID != 0) {
		return tunPermissionError()
	}
	return nil
}

func tunPermissionError() error {
	return &Error{Code: "linux_core_permission_required", Message: "selected Linux mihomo needs persistent network capabilities; use start or full restart to authorize this verified core"}
}

func statusCapability(status, name string) uint64 {
	for _, line := range strings.Split(status, "\n") {
		key, value, found := strings.Cut(line, ":")
		if found && key == name {
			parsed, _ := strconv.ParseUint(strings.TrimSpace(value), 16, 64)
			return parsed
		}
	}
	return 0
}

func canStartTUN(status string, fileCapabilities []byte, root, nosuid bool) bool {
	if statusCapability(status, "CapAmb")&tunCapabilities == tunCapabilities {
		return true
	}
	if statusCapability(status, "CapBnd")&tunCapabilities != tunCapabilities {
		return false
	}
	if root && statusCapability(status, "CapEff")&tunCapabilities == tunCapabilities {
		return true
	}
	if nosuid || statusCapability(status, "NoNewPrivs") != 0 || (len(fileCapabilities) != 20 && len(fileCapabilities) != 24) {
		return false
	}
	return HasNetworkCapabilities(fileCapabilities)
}

func HasNetworkCapabilities(fileCapabilities []byte) bool {
	if len(fileCapabilities) != 20 && len(fileCapabilities) != 24 {
		return false
	}
	magic := binary.LittleEndian.Uint32(fileCapabilities)
	if magic&1 == 0 || (magic&0xff000000 != 0x02000000 && magic&0xff000000 != 0x03000000) {
		return false
	}
	permitted := uint64(binary.LittleEndian.Uint32(fileCapabilities[4:8])) |
		uint64(binary.LittleEndian.Uint32(fileCapabilities[12:16]))<<32
	return permitted&tunCapabilities == tunCapabilities
}
