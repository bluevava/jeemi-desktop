//go:build windows

package daemon

import (
	"net"
	"os"
	"testing"
)

func TestCoreListenerVerifiesProcess(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	port := uint16(listener.Addr().(*net.TCPAddr).Port)
	if !ownsTCP(uint32(os.Getpid()), port, true) {
		t.Fatal("actual loopback listener not found")
	}
	if ownsTCP(0, port, true) || ownsTCP(uint32(os.Getpid()+99999), port, true) {
		t.Fatal("unrelated process accepted")
	}
	listener.Close()
	if ownsTCP(uint32(os.Getpid()), port, true) {
		t.Fatal("closed listener still accepted")
	}
}
