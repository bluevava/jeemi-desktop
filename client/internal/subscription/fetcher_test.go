package subscription

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPFetcherPreservesRawConfigurationAndDetectsFormat(t *testing.T) {
	want := "{\"mode\":\"rule\",\"proxies\":[]}\n"
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.UserAgent() == "" {
			t.Error("subscription request omitted User-Agent")
		}
		response.Header().Set("Content-Type", "application/json")
		response.Header().Set("Profile-Title", "Remote profile")
		response.Header().Set("Subscription-Userinfo", "upload=10; download=20; total=100; expire=1893456000")
		_, _ = response.Write([]byte(want))
	}))
	defer server.Close()

	fetched, err := NewHTTPFetcher().Fetch(context.Background(), server.URL+"/private?token=secret")
	if err != nil {
		t.Fatal(err)
	}
	if string(fetched.Contents) != want || fetched.Format != FormatJSON {
		t.Fatalf("unexpected fetched content: %+v", fetched)
	}
	if fetched.SuggestedName != "Remote profile" || !fetched.RemoteProfile.HasTraffic || fetched.RemoteProfile.TotalBytes != 100 {
		t.Fatalf("response metadata was not normalised: %+v", fetched)
	}
}

func TestHTTPFetcherDoesNotEchoSensitiveURLOnFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		http.Error(response, "upstream failed", http.StatusBadGateway)
	}))
	defer server.Close()
	secret := "very-secret-token"
	_, err := NewHTTPFetcher().Fetch(context.Background(), server.URL+"/subscription?token="+secret)
	if err == nil {
		t.Fatal("Fetch() unexpectedly succeeded")
	}
	if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "/subscription") {
		t.Fatalf("fetch error leaked the subscription URL: %v", err)
	}
}

func TestHTTPFetcherRejectsNonHTTPURLsAndUnsafeRedirects(t *testing.T) {
	if _, err := NewHTTPFetcher().Fetch(context.Background(), "file:///tmp/sub.yaml"); err == nil {
		t.Fatal("Fetch() accepted a non-HTTP URL")
	}
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Location", "file:///tmp/private.yaml")
		response.WriteHeader(http.StatusFound)
	}))
	defer server.Close()
	if _, err := NewHTTPFetcher().Fetch(context.Background(), server.URL); err == nil {
		t.Fatal("Fetch() followed an unsafe redirect")
	}
}

func TestHTTPFetcherDoesNotForwardSensitiveSourceAsRedirectReferer(t *testing.T) {
	secret := "redirect-secret-token"
	target := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if strings.Contains(request.Referer(), secret) {
			t.Errorf("redirect Referer leaked the subscription URL: %q", request.Referer())
		}
		_, _ = response.Write([]byte("mode: rule\n"))
	}))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		http.Redirect(response, request, target.URL+"/config.yaml", http.StatusFound)
	}))
	defer source.Close()

	if _, err := NewHTTPFetcher().Fetch(context.Background(), source.URL+"/subscription?token="+secret); err != nil {
		t.Fatal(err)
	}
}
