package desktop

import (
	"context"
	"errors"
	"image"
	"reflect"
	"testing"
	"time"

	"jeemi/internal/platform/screen"
)

func TestScreenCaptureRestoresWindowBeforeRecognition(t *testing.T) {
	for _, failure := range []error{nil, screen.ErrCancelled, screen.ErrUnavailable, context.DeadlineExceeded} {
		var events []string
		images, err := captureForQR(context.Background(), 0, func(context.Context) ([]image.Image, error) {
			events = append(events, "capture")
			if failure != nil {
				return nil, failure
			}
			return []image.Image{image.NewRGBA(image.Rect(0, 0, 1, 1))}, nil
		}, func() { events = append(events, "hide") }, func() { events = append(events, "restore") })
		if !errors.Is(err, failure) {
			t.Fatalf("capture error changed: %v", err)
		}
		if failure == nil && len(images) == 1 {
			events = append(events, "recognize")
		}
		want := []string{"hide", "capture", "restore"}
		if failure == nil {
			want = append(want, "recognize")
		}
		if !reflect.DeepEqual(events, want) {
			t.Fatalf("window flow: %v", events)
		}
	}
}

func TestCancelledWindowDelayDoesNotCapture(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	restored := false
	_, err := captureForQR(ctx, time.Second, func(context.Context) ([]image.Image, error) {
		t.Fatal("cancelled request captured the screen")
		return nil, nil
	}, cancel, func() { restored = true })
	if !errors.Is(err, context.Canceled) || !restored {
		t.Fatalf("cancel failed: %v", err)
	}
	result := screenImportFailure(err)
	if !result.Cancelled || result.ErrorCode != "" || result.State != nil {
		t.Fatal("cancellation was reported as failed import")
	}
}

func TestScreenImportGuardAndQuitCancellation(t *testing.T) {
	var operation screenImportOperation
	ctx, finish, ok := operation.begin(context.Background())
	if !ok {
		t.Fatal("first operation rejected")
	}
	if _, _, ok := operation.begin(context.Background()); ok {
		t.Fatal("concurrent screen import allowed")
	}
	operation.cancelActive()
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatal("shutdown did not cancel capture")
	}
	finish()
	_, nextFinish, ok := operation.begin(context.Background())
	if !ok {
		t.Fatal("completed capture permanently blocked retry")
	}
	nextFinish()
}
