//go:build windows

package main

import (
	"jeemi/internal/platform/systemproxy"
	"jeemi/internal/platform/winauth"
	"jeemi/internal/platform/winauth/daemon"
	"os"
)

func main() {
	if len(os.Args) != 3 {
		os.Exit(1)
	}
	factory := func(root, sid string) (winauth.Network, error) {
		return systemproxy.NewWindowsServiceController(root, sid)
	}
	var err error
	switch os.Args[1] {
	case "--service":
		err = daemon.Run(os.Args[2], factory)
	case "--install":
		err = daemon.Manage(os.Args[2], false, factory)
	case "--remove":
		err = daemon.Manage(os.Args[2], true, factory)
	default:
		os.Exit(1)
	}
	if err != nil {
		os.Exit(1)
	}
}
