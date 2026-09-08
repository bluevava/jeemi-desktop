package subscription

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
)

type queuedFetcher struct {
	results []FetchedContent
	errors  []error
	urls    []string
}

func (f *queuedFetcher) Fetch(_ context.Context, sourceURL string) (FetchedContent, error) {
	f.urls = append(f.urls, sourceURL)
	index := len(f.urls) - 1
	if index < len(f.errors) && f.errors[index] != nil {
		return FetchedContent{}, f.errors[index]
	}
	if index >= len(f.results) {
		return FetchedContent{}, fmt.Errorf("missing fake fetch result")
	}
	return f.results[index], nil
}

type fixedScreen struct {
	images []image.Image
}

func (s fixedScreen) CaptureAll(context.Context) ([]image.Image, error) {
	return s.images, nil
}

type fixedIconDetector struct {
	icon  string
	found bool
	err   error
	urls  []string
}

func (d *fixedIconDetector) Detect(_ context.Context, sourceURL string) (string, bool, error) {
	d.urls = append(d.urls, sourceURL)
	return d.icon, d.found, d.err
}

func TestManagerFailedRefreshKeepsCurrentRevision(t *testing.T) {
	store, _ := newTestStore(t)
	fetcher := &queuedFetcher{
		results: []FetchedContent{{
			Contents: []byte("mode: rule\n"), Format: FormatYAML,
			RemoteProfile: RemoteProfile{HasTraffic: true, DownloadBytes: 10, TotalBytes: 100},
		}},
		errors: []error{nil, fmt.Errorf("network unavailable")},
	}
	manager, err := NewManager(ManagerOptions{Store: store, Fetcher: fetcher})
	if err != nil {
		t.Fatal(err)
	}
	state, err := manager.ImportURL(context.Background(), ImportURLInput{
		Name: "Remote", SourceURL: "https://example.com/sub?token=secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	before := state.Subscriptions[0]
	if _, err := manager.Refresh(context.Background(), before.ID); err == nil {
		t.Fatal("Refresh() unexpectedly succeeded")
	}
	afterState, err := manager.State()
	if err != nil {
		t.Fatal(err)
	}
	after := afterState.Subscriptions[0]
	if after.CurrentRevisionID != before.CurrentRevisionID || after.RevisionCount != before.RevisionCount || after.LastFetchedAt != before.LastFetchedAt {
		t.Fatalf("failed refresh changed the current subscription: before=%+v after=%+v", before, after)
	}
	if after.RemoteProfile != before.RemoteProfile {
		t.Fatalf("failed refresh changed response metadata: before=%+v after=%+v", before.RemoteProfile, after.RemoteProfile)
	}
}

func TestManagerUsesResponseNameOnlyWhenImportNameIsBlank(t *testing.T) {
	store, _ := newTestStore(t)
	fetcher := &queuedFetcher{results: []FetchedContent{
		{Contents: []byte("mode: rule\n"), Format: FormatYAML, SuggestedName: "Header name"},
	}}
	manager, err := NewManager(ManagerOptions{Store: store, Fetcher: fetcher})
	if err != nil {
		t.Fatal(err)
	}
	state, err := manager.ImportURL(context.Background(), ImportURLInput{SourceURL: "https://example.com/sub"})
	if err != nil {
		t.Fatal(err)
	}
	if state.Subscriptions[0].Name != "Header name" {
		t.Fatalf("blank import did not use response name: %+v", state.Subscriptions[0])
	}
}

func TestManagerKeepsExplicitImportNameOverResponseName(t *testing.T) {
	store, _ := newTestStore(t)
	fetcher := &queuedFetcher{results: []FetchedContent{
		{Contents: []byte("mode: rule\n"), Format: FormatYAML, SuggestedName: "Header name"},
	}}
	manager, err := NewManager(ManagerOptions{Store: store, Fetcher: fetcher})
	if err != nil {
		t.Fatal(err)
	}
	state, err := manager.ImportURL(context.Background(), ImportURLInput{Name: "My name", SourceURL: "https://example.com/sub"})
	if err != nil {
		t.Fatal(err)
	}
	if state.Subscriptions[0].Name != "My name" {
		t.Fatalf("response name replaced explicit input: %+v", state.Subscriptions[0])
	}
}

func TestManagerDetectsAndPersistsMissingRemoteIcon(t *testing.T) {
	store, _ := newTestStore(t)
	fetcher := &queuedFetcher{results: []FetchedContent{
		{Contents: []byte("mode: rule\n"), Format: FormatYAML},
	}}
	detector := &fixedIconDetector{icon: "https://example.com/favicon.ico", found: true}
	manager, err := NewManager(ManagerOptions{Store: store, Fetcher: fetcher, IconDetector: detector})
	if err != nil {
		t.Fatal(err)
	}
	state, err := manager.ImportURL(context.Background(), ImportURLInput{SourceURL: "https://example.com/sub?token=secret"})
	if err != nil {
		t.Fatal(err)
	}
	item := state.Subscriptions[0]
	if item.IconKind != IconURL || item.Icon != detector.icon || len(detector.urls) != 1 {
		t.Fatalf("missing icon was not detected and persisted: item=%+v detector=%+v", item, detector)
	}
	reloaded, err := manager.State()
	if err != nil || reloaded.Subscriptions[0].Icon != detector.icon {
		t.Fatalf("detected icon did not survive reload: state=%+v err=%v", reloaded, err)
	}
}

func TestManagerDoesNotReplaceExplicitEmojiWithDetectedIcon(t *testing.T) {
	store, _ := newTestStore(t)
	fetcher := &queuedFetcher{results: []FetchedContent{
		{Contents: []byte("mode: rule\n"), Format: FormatYAML},
	}}
	detector := &fixedIconDetector{icon: "https://example.com/favicon.ico", found: true}
	manager, err := NewManager(ManagerOptions{Store: store, Fetcher: fetcher, IconDetector: detector})
	if err != nil {
		t.Fatal(err)
	}
	state, err := manager.ImportURL(context.Background(), ImportURLInput{
		SourceURL: "https://example.com/sub", IconKind: IconEmoji, Icon: "🐱",
	})
	if err != nil {
		t.Fatal(err)
	}
	item := state.Subscriptions[0]
	if item.IconKind != IconEmoji || item.Icon != "🐱" || len(detector.urls) != 0 {
		t.Fatalf("explicit icon was replaced or triggered detection: item=%+v detector=%+v", item, detector)
	}
}

func TestManagerFailedURLChangeKeepsPreviousMetadataAndRevision(t *testing.T) {
	store, _ := newTestStore(t)
	fetcher := &queuedFetcher{
		results: []FetchedContent{{Contents: []byte("mode: rule\n"), Format: FormatYAML}},
		errors:  []error{nil, fmt.Errorf("new source unavailable")},
	}
	manager, err := NewManager(ManagerOptions{Store: store, Fetcher: fetcher})
	if err != nil {
		t.Fatal(err)
	}
	state, err := manager.ImportURL(context.Background(), ImportURLInput{
		Name: "Before", SourceURL: "https://old.example/sub?token=old",
	})
	if err != nil {
		t.Fatal(err)
	}
	before, err := manager.Get(state.Subscriptions[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = manager.Update(context.Background(), UpdateInput{
		ID: before.ID, Name: "After", Description: "not saved", SourceURL: "https://new.example/sub?token=new",
	})
	if err == nil {
		t.Fatal("Update() unexpectedly accepted a failed new source")
	}
	after, err := manager.Get(before.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Name != before.Name || after.Description != before.Description || after.SourceURL != before.SourceURL || after.CurrentRevisionID != before.CurrentRevisionID {
		t.Fatalf("failed URL edit changed saved subscription: before=%+v after=%+v", before, after)
	}
}

func TestManagerImportsOnlySupportedConfigurationFiles(t *testing.T) {
	for _, testCase := range []struct {
		extension string
		format    Format
		contents  string
	}{
		{extension: ".yaml", format: FormatYAML, contents: "mode: rule\n"},
		{extension: ".json", format: FormatJSON, contents: "{\"mode\":\"rule\"}\n"},
		{extension: ".txt", format: FormatText, contents: "mode: rule\n"},
	} {
		t.Run(testCase.extension, func(t *testing.T) {
			store, _ := newTestStore(t)
			manager, err := NewManager(ManagerOptions{Store: store, Fetcher: &queuedFetcher{}})
			if err != nil {
				t.Fatal(err)
			}
			directory := t.TempDir()
			path := filepath.Join(directory, "sample"+testCase.extension)
			if err := os.WriteFile(path, []byte(testCase.contents), 0o600); err != nil {
				t.Fatal(err)
			}
			state, err := manager.ImportFile(path)
			if err != nil {
				t.Fatal(err)
			}
			item := state.Subscriptions[0]
			if len(state.Subscriptions) != 1 || item.SourceLabel != filepath.Base(path) || item.Format != testCase.format {
				t.Fatalf("unexpected imported file state: %+v", state)
			}
			metadata, err := os.ReadFile(filepath.Join(store.Directory(), item.ID, "metadata.json"))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(metadata), directory) {
				t.Fatalf("subscription metadata persisted the selected absolute path:\n%s", metadata)
			}
		})
	}

	store, _ := newTestStore(t)
	manager, err := NewManager(ManagerOptions{Store: store, Fetcher: &queuedFetcher{}})
	if err != nil {
		t.Fatal(err)
	}
	unsupported := filepath.Join(t.TempDir(), "sample.yml")
	if err := os.WriteFile(unsupported, []byte("mode: rule\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.ImportFile(unsupported); err == nil {
		t.Fatal("ImportFile() accepted .yml despite the explicit .yaml/.json/.txt contract")
	}
}

func TestManagerImportsQRCodeImageWithoutPersistingTheImage(t *testing.T) {
	store, _ := newTestStore(t)
	fetcher := &queuedFetcher{results: []FetchedContent{{Contents: []byte("mode: rule\n"), Format: FormatYAML}}}
	manager, err := NewManager(ManagerOptions{Store: store, Fetcher: fetcher, QRDecoder: ZXingQRDecoder{}})
	if err != nil {
		t.Fatal(err)
	}
	imagePath := filepath.Join(t.TempDir(), "subscription.png")
	file, err := os.OpenFile(imagePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, makeQRCode(t, "https://image.example/sub")); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	state, err := manager.ImportQRImage(context.Background(), imagePath)
	if err != nil {
		t.Fatal(err)
	}
	item := state.Subscriptions[0]
	if item.ImportMethod != ImportQRImage || item.SourceLabel != "https://image.example" {
		t.Fatalf("unexpected QR image import: %+v", item)
	}
	entries, err := os.ReadDir(filepath.Join(store.Directory(), item.ID))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(strings.ToLower(entry.Name()), ".png") {
			t.Fatalf("QR image was persisted in the subscription directory: %s", entry.Name())
		}
	}
}

func TestManagerRecognisesScreenQRCodeAsURLImport(t *testing.T) {
	store, _ := newTestStore(t)
	fetcher := &queuedFetcher{results: []FetchedContent{{Contents: []byte("mode: rule\n"), Format: FormatYAML}}}
	qrImage := makeQRCode(t, "https://qr.example/subscription?token=sensitive")
	manager, err := NewManager(ManagerOptions{
		Store: store, Fetcher: fetcher, QRDecoder: ZXingQRDecoder{}, Screen: fixedScreen{images: []image.Image{qrImage}},
	})
	if err != nil {
		t.Fatal(err)
	}
	state, err := manager.ImportQRScreen(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	item := state.Subscriptions[0]
	if item.ImportMethod != ImportQRScreen || item.SourceLabel != "https://qr.example" || item.Name != "qr.example" {
		t.Fatalf("unexpected QR import state: %+v", item)
	}
	if strings.Contains(item.SourceLabel, "sensitive") {
		t.Fatalf("QR subscription label leaked URL credentials: %q", item.SourceLabel)
	}
}

func TestZXingQRDecoderRejectsNonQRCodeImage(t *testing.T) {
	imageValue := image.NewRGBA(image.Rect(0, 0, 120, 120))
	if _, err := (ZXingQRDecoder{}).Decode(imageValue); err == nil {
		t.Fatal("Decode() accepted an image without a QR code")
	}
}

func makeQRCode(t *testing.T, value string) image.Image {
	t.Helper()
	encoded, err := qrcode.NewQRCodeWriter().EncodeWithoutHint(value, gozxing.BarcodeFormat_QR_CODE, 360, 360)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}
