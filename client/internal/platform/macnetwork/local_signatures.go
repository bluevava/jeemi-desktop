//go:build jeemi_local_test

package macnetwork

import (
	"debug/macho"
	"errors"
	"os"
	"slices"
)

// Read the slices actually present in this build. Single-architecture bundles
// must not require the other CPU's signature, while older universal bundles
// must still verify every slice. This does not require Xcode or lipo at runtime.
func codeArchitectures(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, Failure("local_signature_invalid")
	}
	defer file.Close()
	fat, err := macho.NewFatFile(file)
	var binaries []*macho.File
	if errors.Is(err, macho.ErrNotFat) {
		thin, err := macho.NewFile(file)
		if err != nil {
			return nil, Failure("local_signature_invalid")
		}
		binaries = append(binaries, thin)
	} else if err != nil {
		return nil, Failure("local_signature_invalid")
	} else {
		for _, arch := range fat.Arches {
			binaries = append(binaries, arch.File)
		}
	}
	if len(binaries) < 1 || len(binaries) > 2 {
		return nil, Failure("local_signature_invalid")
	}
	architectures := make([]string, 0, len(binaries))
	for _, binary := range binaries {
		var arch string
		switch binary.Cpu {
		case macho.CpuArm64:
			arch = "arm64"
		case macho.CpuAmd64:
			arch = "x86_64"
		default:
			return nil, Failure("local_signature_invalid")
		}
		if binary.Type != macho.TypeExec || slices.Contains(architectures, arch) {
			return nil, Failure("local_signature_invalid")
		}
		architectures = append(architectures, arch)
	}
	slices.Sort(architectures)
	return architectures, nil
}
