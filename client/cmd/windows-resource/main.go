// Builds the helper's PE resource before its digest is bound into the GUI.
package main

import (
	"github.com/tc-hib/winres"
	"os"
)

func main() {
	if len(os.Args) != 3 {
		os.Exit(1)
	}
	architecture := winres.Arch(os.Args[2])
	if architecture != winres.ArchAMD64 && architecture != winres.ArchARM64 {
		os.Exit(1)
	}
	rs := winres.ResourceSet{}
	rs.SetManifest(winres.AppManifest{Identity: winres.AssemblyIdentity{Name: "Jeemi.AuthorizationHelper"}, Description: "Jeemi Authorization Helper", ExecutionLevel: winres.RequireAdministrator})
	file, err := os.Create(os.Args[1])
	if err != nil {
		panic(err)
	}
	if err = rs.WriteObject(file, architecture); err != nil {
		file.Close()
		panic(err)
	}
	if err = file.Close(); err != nil {
		panic(err)
	}
}
