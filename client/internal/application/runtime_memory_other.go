//go:build !windows && !linux && !darwin

package application

import (
	"context"
	"jeemi/internal/platform/processmemory"
)

func readHelperMemory(context.Context, int, bool) processmemory.HelperSnapshot {
	return processmemory.HelperSnapshot{}
}
