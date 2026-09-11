package localscript

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHTTPFetcherDownloadsJavaScriptWithoutRedirectReferer(t *testing.T) {
	const source = "function main(config) { return config; }\n"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/source", http.StatusFound)
			return
		}
		if r.Header.Get("Referer") != "" {
			t.Error("redirect leaked the source URL")
		}
		// Raw file hosting does not always supply a JavaScript MIME type or suffix.
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte(source))
	}))
	defer server.Close()
	got, err := (&HTTPFetcher{}).Fetch(context.Background(), server.URL+"/redirect?token=private")
	if err != nil || got != source {
		t.Fatalf("downloaded source differs: %v", err)
	}
}

func TestHTTPFetcherLimitsAndSafeErrors(t *testing.T) {
	for _, fixture := range []string{"status", "oversize", "streamed-oversize", "empty", "invalid-utf8", "bad-redirect", "redirect-loop"} {
		t.Run(fixture, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch fixture {
				case "status":
					w.WriteHeader(http.StatusForbidden)
				case "oversize":
					w.Header().Set("Content-Length", fmt.Sprint(maxScriptBytes+1))
				case "streamed-oversize":
					w.(http.Flusher).Flush()
					_, _ = w.Write([]byte(strings.Repeat("a", maxScriptBytes+1)))
				case "invalid-utf8":
					_, _ = w.Write([]byte{0xff})
				case "bad-redirect":
					http.Redirect(w, r, "file:///private.js", http.StatusFound)
				case "redirect-loop":
					http.Redirect(w, r, r.URL.String(), http.StatusFound)
				}
			}))
			defer server.Close()
			_, err := (&HTTPFetcher{}).Fetch(context.Background(), server.URL+"/source?token=private")
			if err == nil {
				t.Fatal("invalid download accepted")
			}
			if strings.Contains(err.Error(), "private") || strings.Contains(err.Error(), server.URL) {
				t.Fatal("error contains the source URL")
			}
		})
	}
}

func TestHTTPFetcherCancellationAndDeadline(t *testing.T) {
	for _, deadline := range []bool{false, true} {
		t.Run(fmt.Sprint(deadline), func(t *testing.T) {
			entered := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				close(entered)
				<-r.Context().Done()
			}))
			defer server.Close()
			var ctx context.Context
			var cancel context.CancelFunc
			if deadline {
				ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
			} else {
				ctx, cancel = context.WithCancel(context.Background())
			}
			defer cancel()
			done := make(chan error, 1)
			go func() { _, err := (&HTTPFetcher{}).Fetch(ctx, server.URL); done <- err }()
			select {
			case <-entered:
			case <-time.After(2 * time.Second):
				t.Fatal("download did not start")
			}
			if !deadline {
				cancel()
			}
			select {
			case err := <-done:
				want := context.Canceled
				if deadline {
					want = context.DeadlineExceeded
				}
				if !errors.Is(err, want) {
					t.Fatalf("cancellation = %v", err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("cancellation did not stop download")
			}
		})
	}
}

func TestNormalizeSourceURL(t *testing.T) {
	got, err := NormalizeSourceURL("  https://example.test/code?token=private#ignored \n")
	if err != nil || got != "https://example.test/code?token=private" {
		t.Fatal("URL normalization failed", err)
	}
	for _, value := range []string{"", "file:///private.js", "javascript:private", "http://", "https://example.test:70000/private", "https://example.test/a\nb", strings.Repeat("a", maxSourceURLBytes+1)} {
		if _, err := NormalizeSourceURL(value); err == nil || strings.Contains(err.Error(), "private") {
			t.Fatal("invalid URL accepted or leaked")
		}
	}
}
