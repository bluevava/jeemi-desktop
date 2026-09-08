//go:build windows

package systemproxy

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Preserve an authenticated user's token solely for notifying WinINet after
// GUI termination. No commands, paths or user-supplied tokens are accepted.
type serviceNotifier struct {
	mu    sync.Mutex
	sid   string
	token windows.Token
}

func (n *serviceNotifier) set(token windows.Token) error {
	user, err := token.GetTokenUser()
	if err != nil || user.User.Sid.String() != n.sid {
		return os.ErrPermission
	}
	var duplicate windows.Token
	if err = windows.DuplicateTokenEx(token, windows.TOKEN_QUERY|windows.TOKEN_IMPERSONATE, nil, windows.SecurityImpersonation, windows.TokenImpersonation, &duplicate); err != nil {
		return err
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.token != 0 {
		n.token.Close()
	}
	n.token = duplicate
	return nil
}
func (n *serviceNotifier) bind(pid uint32) error {
	process, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(process)
	var token windows.Token
	if err = windows.OpenProcessToken(process, windows.TOKEN_QUERY|windows.TOKEN_DUPLICATE, &token); err != nil {
		return err
	}
	defer token.Close()
	return n.set(token)
}
func (n *serviceNotifier) activeUser() {
	// An elevated installer may run as the same account without SYSTEM's
	// WTS privilege. Its own token is sufficient for this fixed notification.
	var current windows.Token
	if windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY|windows.TOKEN_DUPLICATE, &current) == nil {
		err := n.set(current)
		current.Close()
		if err == nil {
			return
		}
	}
	// SYSTEM also supports the target user's RDP session, not only the active
	// physical console. Other users' tokens are discarded immediately.
	var sessions *windows.WTS_SESSION_INFO
	var count uint32
	if windows.WTSEnumerateSessions(0, 0, 1, &sessions, &count) != nil {
		return
	}
	defer windows.WTSFreeMemory(uintptr(unsafe.Pointer(sessions)))
	if count > 1024 || (sessions == nil && count != 0) {
		return
	}
	for _, session := range unsafe.Slice(sessions, count) {
		var token windows.Token
		if windows.WTSQueryUserToken(session.SessionID, &token) != nil {
			continue
		}
		err := n.set(token)
		token.Close()
		if err == nil {
			return
		}
	}
}
func (n *serviceNotifier) notify(ctx context.Context) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.token == 0 {
		// Keep the recovery journal until a real account context is available.
		return os.ErrPermission
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	result, _, err := windows.NewLazySystemDLL("advapi32.dll").NewProc("ImpersonateLoggedOnUser").Call(uintptr(n.token))
	if result == 0 {
		return err
	}
	defer func() {
		if windows.RevertToSelf() != nil {
			os.Exit(1)
		}
	}()
	return (windowsBackend{}).Notify(ctx)
}
func (n *serviceNotifier) close() {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.token != 0 {
		n.token.Close()
		n.token = 0
	}
}

type nativeServiceController struct {
	*manager
	notifier *serviceNotifier
}

func (c *nativeServiceController) BindClient(pid uint32) error { return c.notifier.bind(pid) }
func (c *nativeServiceController) Close()                      { c.notifier.close() }

// Only the standalone helper entry uses this protected journal and account.
func NewWindowsServiceController(root, sid string) (Controller, error) {
	notifier := &serviceNotifier{sid: sid}
	notifier.activeUser()
	return &nativeServiceController{manager: newManager(filepath.Join(root, "system-proxy-recovery.json"), windowsBackend{SID: sid, notifier: notifier}), notifier: notifier}, nil
}
