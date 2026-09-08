package application

import (
	"context"
	"jeemi/internal/platform/processmemory"
)

func captureRuntimeMemory(ctx context.Context, pid int) processmemory.Snapshot {
	desktop := processmemory.CaptureDesktop(pid)
	helper := readHelperMemory(ctx, pid, desktop.WebViewBytes == nil)
	return processmemory.Combine(desktop, helper, pid)
}
