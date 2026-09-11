package iconcache

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func testPNG(t *testing.T) []byte {
	t.Helper()
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

func testClient(contents []byte, calls *atomic.Int32) func() *http.Client {
	return func() *http.Client {
		return &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
			calls.Add(1)
			if r.Header.Get("Referer") != "" || r.Header.Get("Authorization") != "" {
				return nil, errors.New("unexpected credentials")
			}
			return &http.Response{StatusCode: 200, ContentLength: int64(len(contents)), Body: io.NopCloser(bytes.NewReader(contents)), Header: make(http.Header)}, nil
		})}
	}
}

func TestCachedIconSurvivesClientRestartAndOffline(t *testing.T) {
	root := t.TempDir()
	var calls atomic.Int32
	cache := New(root, testClient(testPNG(t), &calls))
	first, err := cache.Load(context.Background(), "https://assets.example/icon.png?key=private")
	if err != nil || !strings.HasPrefix(first, "data:image/png;base64,") {
		t.Fatalf("first load: %v", err)
	}
	restarted := New(root, testClient([]byte("offline"), &calls))
	second, err := restarted.Load(context.Background(), "https://assets.example/icon.png?key=private#fragment")
	if err != nil || first != second || calls.Load() != 1 {
		t.Fatalf("cache reuse failed: %v, calls %d", err, calls.Load())
	}
	files, _ := os.ReadDir(cache.directory)
	if len(files) != 1 || len(files[0].Name()) != 68 || strings.Contains(files[0].Name(), "private") {
		t.Fatal("unexpected cache names")
	}
}

func TestFailedRefreshPreservesExpiredIconAndCorruptCacheCanRecover(t *testing.T) {
	var calls atomic.Int32
	cache := New(t.TempDir(), testClient(testPNG(t), &calls))
	first, err := cache.Load(context.Background(), "https://assets.example/icon.png")
	if err != nil {
		t.Fatal(err)
	}
	files, _ := os.ReadDir(cache.directory)
	path := filepath.Join(cache.directory, files[0].Name())
	old := time.Now().Add(-cacheAge - time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	cache.client = testClient([]byte("<html>offline</html>"), &calls)
	second, err := cache.Load(context.Background(), "https://assets.example/icon.png")
	if err != nil || first != second {
		t.Fatalf("stale cache was lost: %v", err)
	}
	info, _ := os.Stat(path)
	if !info.ModTime().Equal(old) {
		t.Fatal("failed refresh changed cache age")
	}
	if err := os.WriteFile(path, []byte("broken cache"), 0600); err != nil {
		t.Fatal(err)
	}
	cache.client = testClient(testPNG(t), &calls)
	third, err := cache.Load(context.Background(), "https://assets.example/icon.png")
	if err != nil || third != first || calls.Load() != 3 {
		t.Fatalf("corrupt cache did not recover: %v", err)
	}
}

func TestRejectsInvalidURLsAndNonImagesWithoutCaching(t *testing.T) {
	var calls atomic.Int32
	cache := New(t.TempDir(), testClient([]byte("<html>private failure</html>"), &calls))
	for _, address := range []string{"file:///secret", "data:image/png;base64,AA==", "https://user:secret@assets.example/icon", "https://", "", "https://assets.example/icon?token=secret"} {
		_, err := cache.Load(context.Background(), address)
		if !errors.Is(err, ErrUnavailable) || strings.Contains(err.Error(), "secret") {
			t.Fatalf("unexpected error %v", err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("invalid URL requested: %d", calls.Load())
	}
	if _, err := os.Stat(cache.directory); !os.IsNotExist(err) {
		t.Fatal("failed image created cache")
	}
}

func TestDownloadSizeStatusRedirectAndCancellation(t *testing.T) {
	for _, fixture := range []struct {
		name   string
		status int
		size   int64
		body   []byte
	}{
		{"partial", 206, 4, []byte("data")},
		{"empty", 200, 0, nil},
		{"truncated", 200, 8, []byte("data")},
		{"oversized header", 200, maxBytes + 1, nil},
		{"oversized stream", 200, -1, bytes.Repeat([]byte("a"), maxBytes+1)},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			cache := New(t.TempDir(), func() *http.Client {
				return &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
					return &http.Response{StatusCode: fixture.status, ContentLength: fixture.size, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(fixture.body))}, nil
				})}
			})
			if _, err := cache.Load(context.Background(), "https://assets.example/icon"); !errors.Is(err, ErrUnavailable) {
				t.Fatal(err)
			}
		})
	}
	started := make(chan struct{})
	cache := New(t.TempDir(), func() *http.Client {
		return &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
			close(started)
			<-r.Context().Done()
			return nil, r.Context().Err()
		})}
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := cache.Load(ctx, "https://assets.example/icon"); done <- err }()
	<-started
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, ErrUnavailable) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("request was not cancelled")
	}
	var calls atomic.Int32
	cache = New(t.TempDir(), func() *http.Client {
		return &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
			calls.Add(1)
			return &http.Response{StatusCode: 302, Header: http.Header{"Location": {"https://user:password@elsewhere.example/icon"}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
		})}
	})
	if _, err := cache.Load(context.Background(), "https://assets.example/icon"); !errors.Is(err, ErrUnavailable) || calls.Load() != 1 {
		t.Fatalf("unsafe redirect: %v", err)
	}
}

func TestDownloadsHaveBoundedConcurrency(t *testing.T) {
	var current, peak atomic.Int32
	started := make(chan struct{}, 8)
	release := make(chan struct{})
	contents := testPNG(t)
	cache := New(t.TempDir(), func() *http.Client {
		return &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
			value := current.Add(1)
			defer current.Add(-1)
			for old := peak.Load(); value > old && !peak.CompareAndSwap(old, value); old = peak.Load() {
			}
			started <- struct{}{}
			select {
			case <-release:
			case <-r.Context().Done():
				return nil, r.Context().Err()
			}
			return &http.Response{StatusCode: 200, ContentLength: int64(len(contents)), Body: io.NopCloser(bytes.NewReader(contents))}, nil
		})}
	})
	var group sync.WaitGroup
	for n := 0; n < 8; n++ {
		group.Add(1)
		go func() { defer group.Done(); _, _ = cache.Load(context.Background(), "https://assets.example/icon") }()
	}
	for n := 0; n < 4; n++ {
		<-started
	}
	close(release)
	group.Wait()
	if peak.Load() > 4 {
		t.Fatalf("unbounded concurrency %d", peak.Load())
	}
}

func TestSVGValidation(t *testing.T) {
	if mime, err := imageType([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16"><path d="M0 0h16v16z"/></svg>`)); err != nil || mime != "image/svg+xml" {
		t.Fatal("valid SVG rejected")
	}
	for _, data := range []string{`<svg><script>alert(1)</script></svg>`, `<svg onload="x()"/>`, `<svg><image href="https://example.com/x"/></svg>`, `<!DOCTYPE svg><svg/>`, `<svg/><svg/>`, `<svg>`, `<html/>`, `<svg><style>@import "x.css";</style></svg>`, `<svg fill="url(https://example.com/x.svg#paint)"/>`, `<?xml-stylesheet href="x.css"?><svg/>`} {
		if _, err := imageType([]byte(data)); err == nil {
			t.Fatalf("invalid SVG accepted: %s", data)
		}
	}
}
