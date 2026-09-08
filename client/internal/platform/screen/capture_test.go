package screen

import (
	"context"
	"errors"
	"image"
	"testing"
)

func TestCaptureDisplaysUsesNativeMonitorBounds(t *testing.T) {
	rects := []image.Rectangle{image.Rect(-1920, 0, 0, 1080), {}, image.Rect(0, 0, 2560, 1440)}
	var captured []image.Rectangle
	images, err := captureDisplays(context.Background(), func() int { return len(rects) }, func(i int) image.Rectangle { return rects[i] }, func(rect image.Rectangle) (*image.RGBA, error) {
		captured = append(captured, rect)
		return image.NewRGBA(image.Rect(0, 0, rect.Dx(), rect.Dy())), nil
	})
	if err != nil || len(images) != 2 || len(captured) != 2 || captured[0] != rects[0] || captured[1] != rects[2] {
		t.Fatalf("native monitor capture changed: %v", err)
	}
}

func TestCaptureDisplaysLimitsAndCancellation(t *testing.T) {
	for _, test := range []struct {
		name      string
		count     int
		rect      image.Rectangle
		cancelled bool
		want      error
	}{
		{"no display", 0, image.Rectangle{}, false, ErrUnavailable},
		{"oversized", 1, image.Rect(0, 0, 20000, 20000), false, ErrTooLarge},
		{"aggregate", 2, image.Rect(0, 0, 6000, 6000), false, ErrTooLarge},
		{"cancelled", 1, image.Rect(0, 0, 10, 10), true, context.Canceled},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if test.cancelled {
				cancel()
			}
			_, err := captureDisplays(ctx, func() int { return test.count }, func(int) image.Rectangle { return test.rect }, func(image.Rectangle) (*image.RGBA, error) { return image.NewRGBA(image.Rect(0, 0, 1, 1)), nil })
			if !errors.Is(err, test.want) {
				t.Fatalf("got %v, want %v", err, test.want)
			}
		})
	}
}

func TestFailureCodesDoNotExposeBackendDetails(t *testing.T) {
	if FailureCode(errors.New("private screenshot path")) != string(ErrFailed) || FailureCode(context.Canceled) != string(ErrCancelled) || FailureCode(context.DeadlineExceeded) != string(ErrTimeout) {
		t.Fatal("unexpected capture error classification")
	}
}
