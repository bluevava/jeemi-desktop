package subscription

import (
	"context"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"testing"

	"jeemi/internal/platform/screen"
)

type captureFunc func(context.Context) ([]image.Image, error)

func (f captureFunc) CaptureAll(ctx context.Context) ([]image.Image, error) { return f(ctx) }

type decoderFunc func(image.Image) ([]string, error)

func (f decoderFunc) Decode(value image.Image) ([]string, error) { return f(value) }

func TestScreenImportCancellationAndFailureDoNotFetchOrPersist(t *testing.T) {
	for _, failure := range []error{screen.ErrCancelled, context.DeadlineExceeded, screen.ErrUnavailable} {
		store, _ := newTestStore(t)
		fetcher := &queuedFetcher{}
		manager, err := NewManager(ManagerOptions{Store: store, Fetcher: fetcher, Screen: captureFunc(func(context.Context) ([]image.Image, error) { return nil, failure })})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := manager.ImportQRScreen(context.Background()); !errors.Is(err, failure) {
			t.Fatalf("lost capture result: %v", err)
		}
		state, err := store.State()
		if err != nil || len(state.Subscriptions) != 0 || len(fetcher.urls) != 0 {
			t.Fatal("failed capture changed subscriptions or fetched a URL")
		}
	}
}

func TestCancelDuringQRDecodeDoesNotImport(t *testing.T) {
	store, _ := newTestStore(t)
	fetcher := &queuedFetcher{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	manager, err := NewManager(ManagerOptions{Store: store, Fetcher: fetcher, QRDecoder: decoderFunc(func(image.Image) ([]string, error) { cancel(); return []string{"https://example.invalid/sub"}, nil })})
	if err != nil {
		t.Fatal(err)
	}
	_, err = manager.ImportQRScreenImages(ctx, []image.Image{image.NewRGBA(image.Rect(0, 0, 1, 1))})
	if !errors.Is(err, context.Canceled) || len(fetcher.urls) != 0 {
		t.Fatalf("cancelled decoder imported: %v", err)
	}
}

func TestScreenQRRecognisesWholePortalImageIncludingOffsetCodes(t *testing.T) {
	// Simulate a physical-pixel desktop image at a different size from XWayland
	// logical bounds. Both codes must be considered; the first is not a URL.
	desktop := image.NewRGBA(image.Rect(0, 0, 2000, 1100))
	draw.Draw(desktop, desktop.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	for index, text := range []string{"not a subscription", "https://qr.example/sub?token=test"} {
		code := makeQRCode(t, text)
		position := image.Pt(150+index*1200, 400)
		draw.Draw(desktop, code.Bounds().Add(position), code, code.Bounds().Min, draw.Src)
	}
	store, _ := newTestStore(t)
	fetcher := &queuedFetcher{results: []FetchedContent{{Contents: []byte("mode: rule\n"), Format: FormatYAML}}}
	manager, err := NewManager(ManagerOptions{Store: store, Fetcher: fetcher})
	if err != nil {
		t.Fatal(err)
	}
	state, err := manager.ImportQRScreenImages(context.Background(), []image.Image{desktop})
	if err != nil || len(state.Subscriptions) != 1 || state.Subscriptions[0].SourceLabel != "https://qr.example" {
		t.Fatalf("desktop QR recognition failed: %v", err)
	}
}

func TestScreenQRSkipsUnusableMonitorAndReportsNoURL(t *testing.T) {
	store, _ := newTestStore(t)
	manager, err := NewManager(ManagerOptions{Store: store, Fetcher: &queuedFetcher{}})
	if err != nil {
		t.Fatal(err)
	}
	blank := image.NewRGBA(image.Rect(0, 0, 100, 100))
	value, err := manager.recognizeURL(context.Background(), []image.Image{blank, makeQRCode(t, "https://qr.example/sub")})
	if err != nil || value != "https://qr.example/sub" {
		t.Fatalf("second monitor not decoded: %v", err)
	}
	if _, err := manager.recognizeURL(context.Background(), []image.Image{makeQRCode(t, "file:///untrusted")}); !errors.Is(err, ErrNoQRCode) {
		t.Fatalf("non-URL QR accepted: %v", err)
	}
}
