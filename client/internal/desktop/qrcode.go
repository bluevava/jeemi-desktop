package desktop

import (
	"context"
	"errors"
	"image"
	"sync"
	"time"

	"jeemi/internal/platform/screen"
	"jeemi/internal/subscription"
)

// One operation spans capture, recognition and import so repeated clicks or
// concurrent bindings cannot hide/restore the same window out of order.
type screenImportOperation struct {
	mu     sync.Mutex
	cancel context.CancelFunc
}

func (o *screenImportOperation) begin(parent context.Context) (context.Context, func(), bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.cancel != nil {
		return nil, nil, false
	}
	ctx, cancel := context.WithCancel(parent)
	o.cancel = cancel
	return ctx, func() {
		o.mu.Lock()
		defer o.mu.Unlock()
		cancel()
		o.cancel = nil
	}, true
}

func (o *screenImportOperation) cancelActive() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.cancel != nil {
		o.cancel()
	}
}

func (a *App) ImportSubscriptionQRCodeScreen() (subscription.ScreenImportResult, error) {
	windowContext := a.context()
	if windowContext == nil || a.quitRequested.Load() {
		return subscription.ScreenImportResult{Cancelled: true}, nil
	}
	ctx, finish, ok := a.screenImport.begin(windowContext)
	if !ok {
		return screenImportFailure(screen.ErrBusy), nil
	}
	defer finish()
	if a.quitRequested.Load() {
		return subscription.ScreenImportResult{Cancelled: true}, nil
	}
	images, err := captureForQR(ctx, 300*time.Millisecond, a.service.CaptureSubscriptionQRCodeScreen,
		func() { a.hideMainWindowToTray(windowContext) },
		func() {
			if !a.quitRequested.Load() && windowContext.Err() == nil {
				a.showMainWindow(windowContext)
			}
		})
	if err != nil {
		return screenImportFailure(err), nil
	}
	// The original window is visible before CPU decoding or network fetching.
	state, err := a.service.ImportSubscriptionQRCodeCapture(ctx, images)
	if errors.Is(err, subscription.ErrNoQRCode) {
		return subscription.ScreenImportResult{ErrorCode: "screen_qr_not_found"}, nil
	}
	if errors.Is(err, context.Canceled) {
		return subscription.ScreenImportResult{Cancelled: true}, nil
	}
	if err != nil {
		return subscription.ScreenImportResult{}, err
	}
	return subscription.ScreenImportResult{State: &state}, nil
}

func screenImportFailure(err error) subscription.ScreenImportResult {
	code := screen.FailureCode(err)
	if code == string(screen.ErrCancelled) {
		return subscription.ScreenImportResult{Cancelled: true}
	}
	return subscription.ScreenImportResult{ErrorCode: code}
}

func captureForQR(ctx context.Context, settle time.Duration, capture func(context.Context) ([]image.Image, error), hide, restore func()) ([]image.Image, error) {
	ctx, cancel := context.WithTimeout(ctx, screen.CaptureTimeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	hide()
	defer restore()
	// Give the compositor time to remove Jeemi before taking the screenshot.
	timer := time.NewTimer(settle)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return capture(ctx)
}
