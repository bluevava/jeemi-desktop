package appupdate

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadVerifiesBytesAndNeverOverwrites(t *testing.T) {
	body := "verified release archive"
	item := asset{URL: "https://github.com/example", Size: int64(len(body)), Digest: fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(body)))}
	for _, tt := range []struct {
		name, body string
		status     int
		want       error
	}{
		{"valid", body, 200, nil}, {"truncated", body[:5], 200, ErrDownload}, {"oversized", body + "x", 200, ErrDownload},
		{"digest", "unverified release data!", 200, ErrChecksum}, {"partial", body, 206, ErrDownload},
	} {
		t.Run(tt.name, func(t *testing.T) {
			filename := filepath.Join(t.TempDir(), "archive")
			var progress int64
			err := download(context.Background(), responseClient(tt.status, tt.body), item, filename, func(n int64) { progress = n })
			if err != tt.want {
				t.Fatalf("%v want %v", err, tt.want)
			}
			if tt.want == nil {
				if progress != item.Size {
					t.Fatal(progress)
				}
				if err = download(context.Background(), responseClient(200, body), item, filename, nil); err != ErrDownload {
					t.Fatal("overwrote archive")
				}
				actual, _ := os.ReadFile(filename)
				if string(actual) != body {
					t.Fatal(string(actual))
				}
			}
		})
	}
}

func TestDownloadRedirectsStayOnGitHubHTTPS(t *testing.T) {
	client := downloadClient()
	for _, raw := range []string{"http://github.com/file", "https://example.com/file", "https://github.com.evil.test/file", "https://user:password@github.com/file"} {
		u, _ := url.Parse(raw)
		if client.CheckRedirect(&http.Request{URL: u}, nil) == nil {
			t.Fatal(raw)
		}
	}
	u, _ := url.Parse("https://release-assets.githubusercontent.com/file?signed=opaque")
	if err := client.CheckRedirect(&http.Request{URL: u}, nil); err != nil {
		t.Fatal(err)
	}
}
