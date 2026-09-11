package externalui

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func archiveFixture(t *testing.T, names ...string) []byte {
	t.Helper()
	var data bytes.Buffer
	writer := zip.NewWriter(&data)
	for _, name := range names {
		file, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte("fixture")); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

func releaseFixture(data []byte, version string) map[string]any {
	digest := sha256.Sum256(data)
	return map[string]any{"tag_name": version, "assets": []map[string]any{{"name": "dist.zip", "browser_download_url": ArchiveURL(version), "size": len(data), "digest": "sha256:" + hex.EncodeToString(digest[:])}}}
}

func testManager(t *testing.T, data []byte) (*Manager, *atomic.Int32) {
	t.Helper()
	calls := &atomic.Int32{}
	client := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		payload := data
		if r.URL.Host == "api.github.com" {
			version := "v3.26.0"
			if strings.Contains(r.URL.Path, "/tags/") {
				version = filepath.Base(r.URL.Path)
			}
			var content any = releaseFixture(data, version)
			if r.URL.RawQuery != "" {
				content = []any{content}
			}
			payload, _ = json.Marshal(content)
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(payload))}, nil
	})}
	return NewManager(Options{DataDirectory: t.TempDir(), HTTPClient: client}), calls
}

func TestOfflineReadsAndPersistentResourceReuse(t *testing.T) {
	m, calls := testManager(t, archiveFixture(t, "dist/index.html", "dist/assets/app.js"))
	if len(m.State().Installed) != 0 || calls.Load() != 0 {
		t.Fatal("state unexpectedly fetched resources")
	}
	if _, err := os.Stat(m.root); !os.IsNotExist(err) {
		t.Fatal("read created cache directory")
	}
	version, err := m.Ensure(context.Background(), "")
	if err != nil || version != "v3.26.0" || calls.Load() != 2 {
		t.Fatal(version, err, calls.Load())
	}
	index := filepath.Join(m.root, version, "dist", "index.html")
	before, _ := os.Stat(index)
	// Recreate the manager to simulate a client restart with no in-memory catalog.
	restarted := NewManager(Options{DataDirectory: filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(m.root)))), HTTPClient: &http.Client{Transport: roundTrip(func(*http.Request) (*http.Response, error) {
		t.Error("cache reuse accessed the network")
		return nil, errors.New("offline")
	})}})
	if _, err := restarted.Ensure(context.Background(), version); err != nil {
		t.Fatal(err)
	}
	if latest, err := restarted.Ensure(context.Background(), ""); err != nil || latest != version {
		t.Fatal(latest, err)
	}
	after, _ := os.Stat(index)
	if !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("cache was rewritten")
	}
	if err := os.Remove(filepath.Join(m.root, version, "dist", "assets", "app.js")); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Ensure(context.Background(), version); err != nil || calls.Load() != 4 {
		t.Fatal("missing resource was not repaired", err)
	}
}

func TestVerifiedCatalogAndSpecifiedVersion(t *testing.T) {
	m, _ := testManager(t, archiveFixture(t, "index.html"))
	if err := m.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	if m.State().LatestVersion != "v3.26.0" || m.State().LastCheckedAt == "" {
		t.Fatal("missing latest release")
	}
	if version, err := m.Download(context.Background(), "v3.10.0"); err != nil || version != "v3.10.0" {
		t.Fatal(version, err)
	}
	for _, invalid := range []string{"latest", "../v3.1.0", "v3.1.0-beta", "v03.1.0"} {
		if _, err := m.Download(context.Background(), invalid); err == nil {
			t.Fatal("accepted invalid release", invalid)
		}
	}
}

func TestRejectsUnverifiedOrForeignReleases(t *testing.T) {
	for _, mutation := range []string{"digest", "url", "asset", "draft", "prerelease", "tag"} {
		t.Run(mutation, func(t *testing.T) {
			m, _ := testManager(t, nil)
			entry := releaseFixture([]byte("fixture"), "v3.26.0")
			asset := entry["assets"].([]map[string]any)[0]
			switch mutation {
			case "digest":
				asset["digest"] = "sha256:bad"
			case "url":
				asset["browser_download_url"] = "https://example.test/dist.zip"
			case "asset":
				asset["name"] = "dist-no-fonts.zip"
			case "tag":
				entry["tag_name"] = "v3.26.0-beta"
			default:
				entry[mutation] = true
			}
			data, _ := json.Marshal(entry)
			m.client.Transport = roundTrip(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(data))}, nil
			})
			if _, err := m.fetch(context.Background(), "/latest"); err == nil {
				t.Fatal("accepted unsupported release")
			}
		})
	}
}

func TestBadArchiveOrCancelledDownloadPreservesInstalledVersion(t *testing.T) {
	for _, kind := range []string{"digest", "cancel"} {
		t.Run(kind, func(t *testing.T) {
			data := archiveFixture(t, "index.html")
			m, _ := testManager(t, data)
			if _, err := m.Download(context.Background(), "v3.25.0"); err != nil {
				t.Fatal(err)
			}
			original := m.client.Transport
			entered := make(chan struct{})
			m.client.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
				if r.URL.Host != "api.github.com" {
					if kind == "cancel" {
						close(entered)
						<-r.Context().Done()
						return nil, r.Context().Err()
					}
					return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("bad"))}, nil
				}
				return original.RoundTrip(r)
			})
			finished := make(chan error, 1)
			go func() { _, err := m.Download(context.Background(), "v3.26.0"); finished <- err }()
			if kind == "cancel" {
				select {
				case <-entered:
				case <-time.After(3 * time.Second):
					t.Fatal("download did not start")
				}
				if m.State().Phase != "downloading" {
					t.Fatal("progress missing")
				}
				if err := m.Check(context.Background()); err == nil {
					t.Fatal("concurrent operation was allowed")
				}
				m.Cancel()
			}
			select {
			case err := <-finished:
				if err == nil {
					t.Fatal("download succeeded")
				}
			case <-time.After(3 * time.Second):
				t.Fatal("download did not cancel")
			}
			if _, err := m.Installed("v3.25.0"); err != nil {
				t.Fatal("old resources lost", err)
			}
			if _, err := os.Stat(filepath.Join(m.root, "v3.26.0")); !os.IsNotExist(err) {
				t.Fatal("bad release became visible")
			}
			staged, _ := os.ReadDir(m.staging)
			if len(staged) != 0 {
				t.Fatal("temporary archive was retained")
			}
		})
	}
}

func TestZIPRejectsUnsafePathsDuplicatesAndMissingIndex(t *testing.T) {
	for _, names := range [][]string{{"../index.html"}, {"/index.html"}, {"dist/index.html", "dist/../secret"}, {"index.html", "INDEX.HTML"}, {"assets/app.js"}, {"dist/index.html", "elsewhere.txt"}, {`dist\index.html`}} {
		t.Run(fmt.Sprint(names), func(t *testing.T) {
			m, _ := testManager(t, archiveFixture(t, names...))
			if _, err := m.Download(context.Background(), "v3.26.0"); err == nil {
				t.Fatal("unsafe ZIP accepted")
			}
		})
	}
	var data bytes.Buffer
	writer := zip.NewWriter(&data)
	header := &zip.FileHeader{Name: "index.html"}
	header.SetMode(os.ModeSymlink | 0600)
	file, _ := writer.CreateHeader(header)
	file.Write([]byte("../secret"))
	writer.Close()
	m, _ := testManager(t, data.Bytes())
	if _, err := m.Download(context.Background(), "v3.26.0"); err == nil {
		t.Fatal("ZIP link accepted")
	}
}

func TestVersionOrder(t *testing.T) {
	if !Newer("v3.26.0", "v3.9.0") || Newer("v3.26.0", "v3.26.0") || Newer("v3.26.0", "v4.0.0") {
		t.Fatal("incorrect semantic version comparison")
	}
}
