package screen

import (
	"context"
	"image"
	"sync"

	"github.com/kbinani/screenshot"
)

// Some native capture APIs cannot be interrupted. Bound the caller's wait and
// allow at most one outstanding native capture, even after cancellation.
var nativeCaptureMu sync.Mutex

func captureNative(ctx context.Context) ([]image.Image, error) {
	if !nativeCaptureMu.TryLock() {
		return nil, ErrBusy
	}
	type result struct {
		images []image.Image
		err    error
	}
	done := make(chan result, 1)
	go func() {
		defer nativeCaptureMu.Unlock()
		images, err := captureDisplays(ctx, screenshot.NumActiveDisplays, screenshot.GetDisplayBounds, screenshot.CaptureRect)
		done <- result{images, err}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-done:
		return result.images, result.err
	}
}

func captureDisplays(ctx context.Context, count func() int, bounds func(int) image.Rectangle, capture func(image.Rectangle) (*image.RGBA, error)) ([]image.Image, error) {
	displayCount := count()
	if displayCount < 1 || displayCount > 16 {
		return nil, ErrUnavailable
	}
	images := make([]image.Image, 0, displayCount)
	var pixels uint64
	for index := 0; index < displayCount; index++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		rect := bounds(index)
		if rect.Empty() {
			continue
		}
		if rect.Dx() > maxCapturePixels || rect.Dy() > maxCapturePixels {
			return nil, ErrTooLarge
		}
		pixels += uint64(rect.Dx()) * uint64(rect.Dy())
		if pixels > maxCapturePixels {
			return nil, ErrTooLarge
		}
		captured, err := capture(rect)
		if err != nil || captured == nil {
			return nil, ErrFailed
		}
		images = append(images, captured)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(images) == 0 {
		return nil, ErrUnavailable
	}
	return images, nil
}
