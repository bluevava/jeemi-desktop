package appupdate

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func responseClient(status int, body string) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), ContentLength: -1}, nil
	})}
}

func TestLatestStableVersionComparison(t *testing.T) {
	for _, tt := range []struct {
		current, latest string
		newer           bool
	}{
		{"0.1.0", "v0.1.1", true}, {"0.1.1", "v0.1.1", false}, {"0.1.1", "v0.1.0", false},
		{"0.9.9", "v0.10.0", true}, {"v2.0.0", "v1.99.99", false}, {"1.0.0-dev", "v1.0.0", true},
		{"1.0.0+local", "v1.0.0", false},
	} {
		t.Run(tt.current+"_"+tt.latest, func(t *testing.T) {
			client := responseClient(200, fmt.Sprintf(`{"tag_name":%q,"draft":false,"prerelease":false}`, tt.latest))
			result, err := check(context.Background(), tt.current, client)
			if err != nil || result.UpdateAvailable != tt.newer || result.LatestVersion != tt.latest {
				t.Fatalf("%+v %v", result, err)
			}
		})
	}
}

func TestReleaseErrorsAreBoundedAndSanitized(t *testing.T) {
	for _, tt := range []struct {
		status int
		body   string
		want   error
	}{
		{404, "", ErrNoRelease}, {429, "", ErrRateLimited}, {403, "", ErrRateLimited}, {500, "private body", ErrCheckFailed},
		{200, `{"tag_name":"v1.0.0","draft":true}`, ErrInvalidRelease},
		{200, `{"tag_name":"v1.0.0-rc.1"}`, ErrInvalidRelease},
		{200, `{"tag_name":"v1.0.0","prerelease":true}`, ErrInvalidRelease},
		{200, `{"tag_name":"v01.0.0"}`, ErrInvalidRelease},
		{200, strings.Repeat("x", maxReleaseBytes+1), ErrInvalidRelease},
	} {
		_, err := check(context.Background(), "0.1.0", responseClient(tt.status, tt.body))
		if err != tt.want {
			t.Fatalf("status %d: %v, want %v", tt.status, err, tt.want)
		}
	}
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != latestReleaseURL || r.Header.Get("Authorization") != "" || r.Header.Get("Accept") != "application/vnd.github+json" {
			t.Fatal("unexpected request")
		}
		return nil, fmt.Errorf("sensitive remote detail")
	})}
	_, err := check(context.Background(), "0.1.0", client)
	if err != ErrCheckFailed {
		t.Fatal(err)
	}
}

func TestExactAssetMappingAndSource(t *testing.T) {
	for _, platform := range []string{"windows", "linux", "darwin"} {
		for _, arch := range []string{"amd64", "arm64"} {
			target, err := targetFor(platform, arch)
			if err != nil {
				t.Fatal(err)
			}
			name := "Jeemi-v1.2.3-" + target.directory + target.extension
			item := asset{Name: name, Size: 42, Digest: "sha256:" + strings.Repeat("a", 64), URL: RepositoryURL + "/releases/download/v1.2.3/" + name}
			published := release{TagName: "v1.2.3", Assets: []asset{item}}
			if _, err = published.archive(target); err != nil {
				t.Fatal(err)
			}
			item.URL = "https://example.com/" + name
			published.Assets = []asset{item}
			if _, err = published.archive(target); err != ErrInvalidRelease {
				t.Fatal(err)
			}
			published.Assets = nil
			if _, err = published.archive(target); err != ErrAssetUnavailable {
				t.Fatal(err)
			}
		}
	}
	if _, err := targetFor("windows", "386"); err != ErrAssetUnavailable {
		t.Fatal(err)
	}
}
