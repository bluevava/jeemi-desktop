//go:build windows

package winauth

import (
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc/mgr"
)

func serviceCommandMatches(command, binary, sid string) bool {
	args, err := windows.DecomposeCommandLine(command)
	if err != nil || len(args) != 3 || args[1] != "--service" || args[2] != sid {
		return false
	}
	return strings.EqualFold(filepath.Clean(args[0]), filepath.Clean(binary))
}

func registrationMatches(config mgr.Config, binary, sid string) bool {
	account := strings.ToLower(config.ServiceStartName)
	return (account == "localsystem" || account == `nt authority\system` || account == "") &&
		config.ServiceType == windows.SERVICE_WIN32_OWN_PROCESS && serviceCommandMatches(config.BinaryPathName, binary, sid)
}
