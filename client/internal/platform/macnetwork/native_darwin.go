//go:build darwin

package macnetwork

/*
#cgo CFLAGS: -x objective-c -fobjc-arc -mmacosx-version-min=13.0
#cgo LDFLAGS: -mmacosx-version-min=13.0 -framework Foundation -framework AppKit -framework Security -framework ServiceManagement -framework SystemConfiguration
#include <stdlib.h>
#include "network_darwin.h"
*/
import "C"

import (
	"context"
	"encoding/json"
	"os/exec"
	"time"
	"unsafe"
)

func nativeFacts() Facts {
	data := C.jeemi_network_facts()
	if data == nil {
		return Facts{Supported: true, Present: true, Service: "unknown"}
	}
	defer C.free(unsafe.Pointer(data))
	var facts Facts
	if json.Unmarshal([]byte(C.GoString(data)), &facts) != nil {
		return Facts{Supported: true, Present: true, Service: "unknown"}
	}
	if LocalTesting {
		// A launchd job can remain registered after someone deletes its plist or
		// executable. Its registration, not its PID, still needs cleanup.
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		err := exec.CommandContext(ctx, "/bin/launchctl", "print", "system/"+ServiceName).Run()
		if err == nil {
			facts.Present = true
		}
		if ctx.Err() != nil {
			facts.Present = true
			facts.Service = "unknown"
		}
	}
	return facts
}

func nativeManage(action string) error {
	if LocalTesting {
		return localManage(action)
	}
	text := C.CString(action)
	defer C.free(unsafe.Pointer(text))
	if C.jeemi_network_manage(text) != 0 {
		return Failure("registration_failed")
	}
	return nil
}

func nativeCall(request []byte) ([]byte, error) {
	text := C.CString(string(request))
	defer C.free(unsafe.Pointer(text))
	reply := C.jeemi_network_call(text)
	if reply == nil {
		return nil, Failure("unreachable")
	}
	defer C.free(unsafe.Pointer(reply))
	return []byte(C.GoString(reply)), nil
}

var serverRequest func(uint64, uint32, int, []byte) []byte
var serverDisconnect func(uint64)
var serverConnect func(uint64)

func nativeDisconnect() { C.jeemi_network_disconnect() }

// Serve is only called by the separate root daemon; ordinary tests and GUI
// bindings never register an XPC listener.
func Serve(request func(uint64, uint32, int, []byte) []byte, connected, disconnected func(uint64)) error {
	serverRequest, serverConnect, serverDisconnect = request, connected, disconnected
	if C.jeemi_network_serve() != 0 {
		return Failure("identity_failed")
	}
	return nil
}

//export jeemiNetworkRequest
func jeemiNetworkRequest(id C.uint64_t, uid C.uint32_t, pid C.int, text *C.char) *C.char {
	if serverRequest == nil {
		return C.CString(string((Response{Code: "helper_failed"}).Marshal()))
	}
	return C.CString(string(serverRequest(uint64(id), uint32(uid), int(pid), []byte(C.GoString(text)))))
}

//export jeemiNetworkConnected
func jeemiNetworkConnected(id C.uint64_t) {
	if serverConnect != nil {
		serverConnect(uint64(id))
	}
}

//export jeemiNetworkDisconnected
func jeemiNetworkDisconnected(id C.uint64_t) {
	if serverDisconnect != nil {
		serverDisconnect(uint64(id))
	}
}

// ReadPreferences / WritePreferences are used only by the root daemon. The
// payload contains fixed proxy/DNS keys indexed by native network service ID.
func ReadPreferences() ([]byte, error) {
	data := C.jeemi_network_read_preferences()
	if data == nil {
		return nil, Failure("network_failed")
	}
	defer C.free(unsafe.Pointer(data))
	return []byte(C.GoString(data)), nil
}
func WritePreferences(data []byte) error {
	text := C.CString(string(data))
	defer C.free(unsafe.Pointer(text))
	if C.jeemi_network_write_preferences(text) != 0 {
		return Failure("network_failed")
	}
	return nil
}

type processNetworkState struct {
	Device   string `json:"device"`
	DNSReady bool   `json:"dnsReady"`
	TCPReady bool   `json:"tcpReady"`
}

func readProcessNetwork(pid int, host string, port int) (processNetworkState, error) {
	var state processNetworkState
	address := C.CString(host)
	defer C.free(unsafe.Pointer(address))
	data := C.jeemi_network_process_state(C.int(pid), address, C.int(port))
	if data == nil {
		return state, Failure("tun_unavailable")
	}
	defer C.free(unsafe.Pointer(data))
	if json.Unmarshal([]byte(C.GoString(data)), &state) != nil {
		return state, Failure("tun_unavailable")
	}
	return state, nil
}

// ProcessNetwork is used only by the daemon with its own child process ID.
func ProcessNetwork(pid int, host string, port int) (string, bool, error) {
	state, err := readProcessNetwork(pid, host, port)
	return state.Device, state.DNSReady, err
}
func HasTCPListener(pid int, host string, port int) bool {
	state, err := readProcessNetwork(pid, host, port)
	return err == nil && state.TCPReady
}
