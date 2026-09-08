//go:build linux

package main

import (
	"os"

	"jeemi/internal/platform/coreauth"
)

func main() {
	if len(os.Args) == 3 && os.Args[1] == "--service" {
		os.Exit(coreauth.RunService(os.Args[2]))
	}
	os.Exit(coreauth.RunHelper(os.Stdin, os.Stdout))
}
