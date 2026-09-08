//go:build windows

package daemon

import (
	"context"
	"os"
	"runtime"

	"golang.org/x/sys/windows"
	"jeemi/internal/platform/winauth"
)

// Open and verify the selected data-directory core using the kernel-verified
// caller's token. Restore the service identity before creating the core process.
func openCallerCore(ctx context.Context, caller windows.Handle, target Target) (*winauth.LockedCore, error) {
	var token windows.Token
	if windows.OpenProcessToken(caller, windows.TOKEN_QUERY|windows.TOKEN_DUPLICATE, &token) != nil {
		return nil, failure("authorization_identity_failed")
	}
	defer token.Close()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	result, _, _ := windows.NewLazySystemDLL("advapi32.dll").NewProc("ImpersonateLoggedOnUser").Call(uintptr(token))
	if result == 0 {
		return nil, failure("authorization_identity_failed")
	}
	defer func() {
		if windows.RevertToSelf() != nil {
			os.Exit(1)
		}
	}()
	return winauth.OpenInstalledCore(ctx, target)
}
