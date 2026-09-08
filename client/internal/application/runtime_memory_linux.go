//go:build linux

package application

import (
	"context"
	"jeemi/internal/platform/coreauth"
	"jeemi/internal/platform/processmemory"
)

func readHelperMemory(ctx context.Context, pid int, webView bool) processmemory.HelperSnapshot {
	return coreauth.Memory(ctx, pid, webView)
}
