//go:build windows

package winauth

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func serviceCommand(binary, sid string) string {
	return syscall.EscapeArg(binary) + " --service " + syscall.EscapeArg(sid)
}

type shellExecuteInfo struct {
	Size       uint32
	Mask       uint32
	Window     windows.Handle
	Verb       *uint16
	File       *uint16
	Parameters *uint16
	Directory  *uint16
	Show       int32
	Instance   windows.Handle
	IDList     uintptr
	Class      *uint16
	ClassKey   windows.Handle
	HotKey     uint32
	Icon       windows.Handle
	Process    windows.Handle
}

func elevate(ctx context.Context, operation string) error {
	sid, err := ownerSID()
	if err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	helper := filepath.Join(filepath.Dir(executable), HelperName)
	locked, err := lockFile(helper)
	if err != nil {
		return failure("authorization_bundle_required")
	}
	defer locked.Close()
	digest, err := fileDigest(helper)
	if err != nil || !digestPattern.MatchString(helperSHA256) || digest != helperSHA256 {
		return failure("authorization_bundle_required")
	}
	verb, _ := windows.UTF16PtrFromString("runas")
	file, _ := windows.UTF16PtrFromString(helper)
	arguments, _ := windows.UTF16PtrFromString(operation + " " + syscall.EscapeArg(sid))
	info := shellExecuteInfo{Mask: 0x00000040 | 0x00000100, Verb: verb, File: file, Parameters: arguments, Show: windows.SW_HIDE}
	info.Size = uint32(unsafe.Sizeof(info))
	result, _, callErr := windows.NewLazySystemDLL("shell32.dll").NewProc("ShellExecuteExW").Call(uintptr(unsafe.Pointer(&info)))
	if result == 0 {
		if callErr == windows.ERROR_CANCELLED {
			return context.Canceled
		}
		return failure("authorization_service_failed")
	}
	if info.Process == 0 {
		return failure("authorization_service_failed")
	}
	defer windows.CloseHandle(info.Process)
	for {
		wait, err := windows.WaitForSingleObject(info.Process, 100)
		if err != nil {
			return redacted(err)
		}
		if wait == windows.WAIT_OBJECT_0 {
			var code uint32
			if windows.GetExitCodeProcess(info.Process, &code) != nil || code != 0 {
				return failure("authorization_service_failed")
			}
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		} // Never kill an installer halfway through its transaction.
	}
}

func Install(ctx context.Context) error {
	if err := elevate(ctx, "--install"); err != nil {
		return err
	}
	state, err := Status(ctx)
	if err != nil {
		return err
	}
	if !state.Ready {
		return failure(state.Code)
	}
	return nil
}
func Remove(ctx context.Context) error {
	state, err := Status(ctx)
	if err != nil {
		return err
	}
	if !state.Present {
		return nil
	}
	if state.Ready {
		if _, err = Call(ctx, Request{Operation: "remove"}); err != nil {
			return err
		}
	} else if err = elevate(ctx, "--remove"); err != nil {
		return err
	}
	// SCM completes deletion only when the running instance and handles close.
	for i := 0; i < 50; i++ {
		state, err = Status(ctx)
		if err == nil && !state.Present {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return failure("authorization_cleanup_incomplete")
}
