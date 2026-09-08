package desktop

import (
	"context"
	"testing"
)

func TestSystemExitBypassesCloseToTray(t *testing.T) {
	a := &App{}
	a.cleanupForExit(context.Background())
	if !a.quitRequested.Load() {
		t.Fatal("system exit did not mark an explicit quit")
	}
	if a.beforeClose(context.Background()) {
		t.Fatal("system exit was intercepted")
	}
	a.cleanupForExit(context.Background())
}
