//go:build windows

package runtime

import (
	"os/exec"
	"testing"

	"golang.org/x/sys/windows"
)

func TestConfigureCommandAvoidsConsoleHost(t *testing.T) {
	command := exec.Command("mihomo.exe")
	configureCommand(command)
	if command.SysProcAttr == nil {
		t.Fatal("configureCommand() did not set SysProcAttr")
	}
	if command.SysProcAttr.CreationFlags&windows.CREATE_NO_WINDOW == 0 {
		t.Fatalf(
			"CreationFlags = %#x, want CREATE_NO_WINDOW",
			command.SysProcAttr.CreationFlags,
		)
	}
	if command.SysProcAttr.CreationFlags&windows.CREATE_NEW_PROCESS_GROUP != 0 {
		t.Fatalf(
			"CreationFlags = %#x, must not allocate a console process group",
			command.SysProcAttr.CreationFlags,
		)
	}
}
