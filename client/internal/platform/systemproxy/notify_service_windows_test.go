//go:build windows

package systemproxy

import (
	"context"
	"errors"
	"os"
	"testing"

	"golang.org/x/sys/windows"
)

func TestServiceRecoveryRequiresAuthenticatedNotificationContext(t *testing.T) {
	n := &serviceNotifier{}
	if !errors.Is(n.notify(context.Background()), os.ErrPermission) {
		t.Fatal("missing user identity must not report notification success")
	}
}

func TestServiceNotifierOnlyRetainsMatchingAccountToken(t *testing.T) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	n := &serviceNotifier{sid: user.User.Sid.String()}
	defer n.close()
	if err = n.bind(uint32(os.Getpid())); err != nil {
		t.Fatal(err)
	}
	if n.token == 0 {
		t.Fatal("matching account was not retained")
	}
	other := &serviceNotifier{sid: "S-1-0-0"}
	defer other.close()
	if err = other.bind(uint32(os.Getpid())); err == nil || other.token != 0 {
		t.Fatal("unrelated identity accepted")
	}
	// No real WinINet notification or registry modification is performed.
}
