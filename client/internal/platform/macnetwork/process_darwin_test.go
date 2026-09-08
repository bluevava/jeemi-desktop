//go:build darwin

package macnetwork

import (
	"encoding/json"
	"net"
	"os"
	"testing"
)

func TestDNSOwnershipRequiresBothCoreSockets(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	udp, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port})
	if err != nil {
		t.Fatal(err)
	}
	defer udp.Close()
	_, ready, err := ProcessNetwork(os.Getpid(), "127.0.0.1", port)
	if err != nil || !ready {
		t.Fatalf("owned sockets not found: %v %v", ready, err)
	}
	udp.Close()
	if !HasTCPListener(os.Getpid(), "127.0.0.1", port) {
		t.Fatal("owned TCP proxy listener not found")
	}
	_, ready, err = ProcessNetwork(os.Getpid(), "127.0.0.1", port)
	if err != nil || ready {
		t.Fatalf("missing UDP accepted: %v %v", ready, err)
	}
}

func TestNativePreferencesReadHasTypedServiceState(t *testing.T) {
	data, err := ReadPreferences()
	if err != nil {
		t.Fatal(err)
	}
	var services map[string]struct {
		Active bool           `json:"active"`
		Proxy  map[string]any `json:"proxy"`
		DNS    map[string]any `json:"dns"`
	}
	if json.Unmarshal(data, &services) != nil {
		t.Fatal("native preference state does not match the protocol")
	}
}
