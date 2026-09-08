//go:build windows

package daemon

import (
	"encoding/binary"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Network setup and exit cleanup target only the running core's listeners.
// Normal GUI controller requests connect directly without helper forwarding.
func ownsTCP(pid uint32, port uint16, loopbackOnly bool) bool {
	if pid == 0 || port == 0 {
		return false
	}
	procedure := windows.NewLazySystemDLL("iphlpapi.dll").NewProc("GetExtendedTcpTable")
	var size uint32
	result, _, _ := procedure.Call(0, uintptr(unsafe.Pointer(&size)), 0, windows.AF_INET, 3, 0) // TCP_TABLE_OWNER_PID_LISTENER
	if result != uintptr(windows.ERROR_INSUFFICIENT_BUFFER) || size < 4 || size > 4<<20 {
		return false
	}
	buffer := make([]byte, size)
	result, _, _ = procedure.Call(uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&size)), 0, windows.AF_INET, 3, 0) // TCP_TABLE_OWNER_PID_LISTENER
	if result != 0 {
		return false
	}
	count := int(binary.LittleEndian.Uint32(buffer))
	if count > (len(buffer)-4)/24 {
		return false
	}
	for i := 0; i < count; i++ {
		row := buffer[4+i*24 : 4+(i+1)*24]
		if binary.LittleEndian.Uint32(row[20:]) != pid || binary.BigEndian.Uint16(row[8:10]) != port {
			continue
		}
		if !loopbackOnly || (row[4] == 127 && row[5] == 0 && row[6] == 0 && row[7] == 1) {
			return true
		}
	}
	return false
}
