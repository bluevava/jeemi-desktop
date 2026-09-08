package screen

import (
	"context"
	"errors"
	"image"
	"time"
)

type Capturer interface {
	CaptureAll(context.Context) ([]image.Image, error)
}

type DesktopCapturer struct{}

const (
	CaptureTimeout   = 90 * time.Second
	maxCapturePixels = 50_000_000
	maxCaptureBytes  = 64 << 20
)

// Codes cross the Wails boundary; backend messages, screenshot paths and image
// contents must never become user-facing errors or logs.
type Failure string

const (
	ErrCancelled   Failure = "screen_capture_cancelled"
	ErrTimeout     Failure = "screen_capture_timeout"
	ErrUnavailable Failure = "screen_capture_unavailable"
	ErrDenied      Failure = "screen_capture_denied"
	ErrFailed      Failure = "screen_capture_failed"
	ErrImage       Failure = "screen_capture_image"
	ErrTooLarge    Failure = "screen_capture_too_large"
	ErrBusy        Failure = "screen_capture_busy"
)

func (e Failure) Error() string { return string(e) }

func FailureCode(err error) string {
	if errors.Is(err, context.Canceled) {
		return string(ErrCancelled)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return string(ErrTimeout)
	}
	var failure Failure
	if errors.As(err, &failure) {
		return string(failure)
	}
	return string(ErrFailed)
}

func (DesktopCapturer) CaptureAll(ctx context.Context) ([]image.Image, error) {
	ctx, cancel := context.WithTimeout(ctx, CaptureTimeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return captureDesktop(ctx)
}
