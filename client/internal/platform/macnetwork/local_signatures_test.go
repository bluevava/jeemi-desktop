//go:build jeemi_local_test

package macnetwork

import (
	"bytes"
	"debug/macho"
	"encoding/binary"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func thinMachO(cpu macho.Cpu) []byte {
	var data bytes.Buffer
	_ = binary.Write(&data, binary.LittleEndian, []uint32{macho.Magic64, uint32(cpu), 0, uint32(macho.TypeExec), 0, 0, 0, 0})
	return data.Bytes()
}

func fatMachO(cpus ...macho.Cpu) []byte {
	var data bytes.Buffer
	_ = binary.Write(&data, binary.BigEndian, []uint32{macho.MagicFat, uint32(len(cpus))})
	offset := uint32(8 + 20*len(cpus))
	for _, cpu := range cpus {
		size := uint32(len(thinMachO(cpu)))
		_ = binary.Write(&data, binary.BigEndian, []uint32{uint32(cpu), 0, offset, size, 0})
		offset += size
	}
	for _, cpu := range cpus {
		data.Write(thinMachO(cpu))
	}
	return data.Bytes()
}

func TestLocalSignatureArchitecturesFollowBundleSlices(t *testing.T) {
	for _, tt := range []struct {
		name string
		data []byte
		want []string
	}{
		{"arm64", thinMachO(macho.CpuArm64), []string{"arm64"}},
		{"amd64", thinMachO(macho.CpuAmd64), []string{"x86_64"}},
		{"universal", fatMachO(macho.CpuAmd64, macho.CpuArm64), []string{"arm64", "x86_64"}},
		{"universal-reversed", fatMachO(macho.CpuArm64, macho.CpuAmd64), []string{"arm64", "x86_64"}},
		{"fat-single", fatMachO(macho.CpuArm64), []string{"arm64"}},
		{"unsupported-cpu", thinMachO(macho.Cpu386), nil},
		{"unsupported-slice", fatMachO(macho.CpuArm64, macho.Cpu386), nil},
		{"duplicate", fatMachO(macho.CpuArm64, macho.CpuArm64), nil},
		{"empty-fat", fatMachO(), nil},
		{"truncated", thinMachO(macho.CpuArm64)[:20], nil},
		{"not-macho", []byte("not an executable"), nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "program")
			if err := os.WriteFile(file, tt.data, 0o600); err != nil {
				t.Fatal(err)
			}
			got, err := codeArchitectures(file)
			if tt.want == nil {
				if err == nil {
					t.Fatalf("invalid executable accepted: %v", got)
				}
			} else if err != nil || !slices.Equal(got, tt.want) {
				t.Fatalf("architectures = %v, %v; want %v", got, err, tt.want)
			}
		})
	}
}
