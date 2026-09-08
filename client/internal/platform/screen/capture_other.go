//go:build !linux

package screen

import (
	"context"
	"image"
)

func captureDesktop(ctx context.Context) ([]image.Image, error) { return captureNative(ctx) }
