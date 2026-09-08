//go:build windows

package sessionend

import "testing"

func TestWindowsQueryAndCancelledShutdownDoNotStopProxy(t *testing.T) {
	calls := 0
	finish := func() { calls++ }
	if handled, result := sessionMessage(wmQueryEndSession, 0, finish); !handled || result != 1 {
		t.Fatal("query vetoed shutdown")
	}
	sessionMessage(wmEndSession, 0, finish)
	if calls != 0 {
		t.Fatal("cancelled logout stopped proxy")
	}
	sessionMessage(wmEndSession, 1, finish)
	if calls != 1 {
		t.Fatal("confirmed session end did not clean up")
	}
}
