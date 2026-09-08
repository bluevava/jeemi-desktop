//go:build darwin

package main

import (
	"fmt"
	"os"
	"runtime"

	"jeemi/internal/platform/macnetwork"
	"jeemi/internal/platform/macnetwork/daemon"
)

func main() {
	if macnetwork.LocalTesting && len(os.Args) == 2 && os.Args[1] == "--recover-local" {
		if err := daemon.RecoverForRemoval(); err != nil {
			os.Exit(1)
		}
		return
	}
	if macnetwork.LocalTesting && len(os.Args) == 2 && os.Args[1] == "--install-local" {
		if err := macnetwork.InstallLocalFromBundle(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println("Jeemi local authorization helper installed.")
		return
	}
	if len(os.Args) == 4 && os.Args[1] == "--snapshot" && (os.Args[3] == "initial" || os.Args[3] == "update") {
		if daemon.SnapshotCommand(os.Args[2], os.Args[3] == "initial") != nil {
			os.Exit(1)
		}
		return
	}
	if len(os.Args) != 1 {
		os.Exit(2)
	}
	runtime.LockOSThread()
	if err := daemon.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "Jeemi authorization helper could not start.")
		os.Exit(1)
	}
}
