//go:build darwin

package application

import (
	"context"
	"jeemi/internal/platform/macnetwork"
	"jeemi/internal/platform/processmemory"
)

func readHelperMemory(ctx context.Context, _ int, webView bool) processmemory.HelperSnapshot {
	return macnetwork.Memory(ctx, webView)
}
