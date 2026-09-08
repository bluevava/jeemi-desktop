//go:build linux

package coreauth

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServiceRequestsCannotSelectCommandsOrUsers(t *testing.T) {
	for _, data := range []string{
		`{"protocol":2,"operation":"exec","command":"whoami"}`,
		`{"protocol":2,"operation":"status","uid":0}`,
		`{"protocol":2,"operation":"status","target":{}}`,
		`{"protocol":2,"operation":"remove"}`,
		`{"protocol":1,"operation":"status"}`,
		`{"protocol":2,"operation":"authorize","target":{},"targets":[]}`,
	} {
		if _, err := decodeServiceRequest([]byte(data)); err == nil {
			t.Fatalf("accepted %s", data)
		}
	}
	for _, data := range []string{`{"protocol":2,"operation":"status"}`, `{"protocol":2,"operation":"remove","targets":[]}`, `{"protocol":2,"operation":"authorize","target":{}}`} {
		if _, err := decodeServiceRequest([]byte(data)); err != nil {
			t.Fatal(err)
		}
	}
}
func TestServiceIdentityAndUnitAreFixed(t *testing.T) {
	for _, value := range []string{"0", "-1", "1000 --exec sh", "01000", "4294967296"} {
		if _, err := serviceUID(value); err == nil {
			t.Fatal(value)
		}
	}
	unit := string(serviceUnit(1000))
	for _, want := range []string{"--service 1000", "Restart=on-failure", "StartLimitBurst=3", "KillMode=control-group", "WantedBy=multi-user.target"} {
		if !strings.Contains(unit, want) {
			t.Fatal(want)
		}
	}
	if strings.Contains(unit, "mihomo") || strings.Contains(unit, "/bin/sh") {
		t.Fatal("service starts a core or shell")
	}
}

func TestRenamedHelperAcceptsOnlyItsOwnLegacyUnit(t *testing.T) {
	const uid = uint32(1000)
	for _, data := range [][]byte{serviceUnit(uid), legacyServiceUnit(uid)} {
		if !ownedServiceUnit(data, uid) {
			t.Fatal("known installation rejected")
		}
		if ownedServiceUnit(data, uid+1) {
			t.Fatal("another account's unit accepted")
		}
		if ownedServiceUnit(append(data, []byte("ExecStart=/bin/sh\n")...), uid) {
			t.Fatal("modified service accepted")
		}
	}
	if HelperName != "jeemi-authorizer" || !strings.HasSuffix(serviceBinary(uid), "/jeemi-authorizer") {
		t.Fatal("helper filename differs from other platforms")
	}
	if !serviceOwnedName(legacyHelperName) {
		t.Fatal("legacy executable cannot be cleaned")
	}
}

func TestServiceCleanupLimitsInstallerResidue(t *testing.T) {
	for _, name := range []string{HelperName, ".install-10293847", ".previous-987654321"} {
		if !serviceOwnedName(name) {
			t.Fatal("installer file rejected", name)
		}
	}
	for _, name := range []string{"..", ".install-", ".install-+1", ".previous-config", "other", "subdir/file"} {
		if serviceOwnedName(name) {
			t.Fatal("unmanaged file accepted", name)
		}
	}
}
func TestUnixPeerComesFromKernel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "peer.sock")
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	finished := make(chan error, 1)
	go func() {
		conn, err := listener.AcceptUnix()
		if err == nil {
			defer conn.Close()
			peer, e := unixPeer(conn)
			err = e
			if e == nil && (peer.Uid != uint32(os.Getuid()) || peer.Pid != int32(os.Getpid())) {
				err = os.ErrPermission
			}
		}
		finished <- err
	}()
	conn, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err = <-finished; err != nil {
		t.Fatal(err)
	}
}
