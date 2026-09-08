//go:build windows

package winauth

import (
	"context"
	"os"
	"testing"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc/mgr"
)

func TestServiceRegistrationAllowsEquivalentWindowsCommandLines(t *testing.T) {
	const sid = "S-1-5-21-1-2-3-1001"
	const binary = `C:\ProgramData\JeemiAuthorization\helper.exe`
	for _, command := range []string{serviceCommand(binary, sid), `"c:\programdata\JeemiAuthorization\helper.exe"  --service ` + sid} {
		config := mgr.Config{BinaryPathName: command, ServiceType: windows.SERVICE_WIN32_OWN_PROCESS, ServiceStartName: "LocalSystem"}
		if !registrationMatches(config, binary, sid) {
			t.Fatalf("equivalent registration rejected: %s", command)
		}
		config.ServiceStartName = sid
		if registrationMatches(config, binary, sid) {
			t.Fatal("non-system service accepted")
		}
	}
	for _, command := range []string{serviceCommand(binary, sid) + " --extra", serviceCommand(`C:\Other\helper.exe`, sid), serviceCommand(binary, "S-1-5-21-1-2-3-1002")} {
		if serviceCommandMatches(command, binary, sid) {
			t.Fatalf("unrelated service accepted: %s", command)
		}
	}
}

// Explicit, read-only diagnostic. Ordinary tests never connect to an installed
// service. Set this to the already installed release's helper digest to check
// the ordinary-user SCM/pipe path, without installing or changing networking.
func TestInstalledServiceReadOnly(t *testing.T) {
	expected := os.Getenv("JEEMI_TEST_INSTALLED_AUTHORIZER")
	if expected == "" {
		t.Skip("opt-in installed service status check")
	}
	if !digestPattern.MatchString(expected) {
		t.Fatal("invalid expected helper digest")
	}
	previous := helperSHA256
	helperSHA256 = expected
	t.Cleanup(func() { helperSHA256 = previous })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	state, err := Status(ctx)
	if err != nil || !state.Present || !state.Ready {
		t.Fatalf("installed service status: %+v, %v", state, err)
	}
}
