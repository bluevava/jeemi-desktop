//go:build windows

package application

import (
	"context"
	"jeemi/internal/platform/processmemory"
	"jeemi/internal/platform/winauth"
)

func readHelperMemory(ctx context.Context, _ int, webView bool) processmemory.HelperSnapshot {
	return winauth.Memory(ctx, webView)
}
