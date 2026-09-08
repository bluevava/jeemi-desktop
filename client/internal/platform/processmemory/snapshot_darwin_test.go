//go:build darwin && cgo

package processmemory

import (
	"os"
	"testing"
)

func TestDarwinResidentReadsOnlySameProcessLifetime(t *testing.T) {
	processes, _ := readDarwinProcesses(os.Getpid())
	own, ok := processes[os.Getpid()]
	if !ok {
		t.Fatal("own process identity unavailable")
	}
	bytes := darwinResident(own)
	if bytes == nil || *bytes == 0 {
		t.Fatal("own resident memory unavailable")
	}
	own.started++
	if darwinResident(own) != nil {
		t.Fatal("reused process identity accepted")
	}
	helper := CaptureHelper(0)
	if helper.HelperBytes == nil || *helper.HelperBytes == 0 || helper.MihomoBytes == nil || *helper.MihomoBytes != 0 {
		t.Fatal("helper self/stopped measurement unavailable")
	}
}
