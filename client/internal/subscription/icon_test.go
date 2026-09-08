package subscription

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"
)

func TestHTTPIconDetectorUsesOriginAndCandidatePriority(t *testing.T) {
	var mu sync.Mutex
	requested := make(map[string]int)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		mu.Lock()
		requested[request.URL.RequestURI()]++
		mu.Unlock()
		switch request.URL.Path {
		case "/favicon.png", "/sub.ico":
			response.Header().Set("Content-Type", "image/png")
			_, _ = response.Write([]byte("\x89PNG\r\n\x1a\nimage"))
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()

	detector := &HTTPIconDetector{Client: server.Client()}
	icon, found, err := detector.Detect(context.Background(), server.URL+"/private/sub?token=secret")
	if err != nil {
		t.Fatal(err)
	}
	if !found || icon != server.URL+"/favicon.png" {
		t.Fatalf("Detect() = %q, %t, want the highest-priority available icon", icon, found)
	}
	mu.Lock()
	defer mu.Unlock()
	for _, path := range subscriptionIconPaths[:2] {
		if requested[path] != 1 {
			t.Fatalf("candidate %q requested %d times; requests = %+v", path, requested[path], requested)
		}
	}
	for requestURI := range requested {
		if parsed, _ := url.Parse(requestURI); parsed.RawQuery != "" || parsed.Path == "/private/sub" {
			t.Fatalf("icon request leaked the subscription path or query: %q", requestURI)
		}
	}
}

func TestHTTPIconDetectorRejectsCrossOriginRedirect(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "image/png")
		_, _ = response.Write([]byte("\x89PNG\r\n\x1a\nimage"))
	}))
	defer target.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/favicon.ico" {
			http.Redirect(response, request, target.URL+"/icon.png", http.StatusFound)
			return
		}
		http.NotFound(response, request)
	}))
	defer origin.Close()

	detector := &HTTPIconDetector{Client: origin.Client()}
	if icon, found, err := detector.Detect(context.Background(), origin.URL+"/sub"); err != nil || found || icon != "" {
		t.Fatalf("Detect() followed a cross-origin redirect: icon=%q found=%t err=%v", icon, found, err)
	}
}

func TestHTTPIconDetectorRejectsDeclaredImageWithoutImageSignature(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "image/png")
		_, _ = response.Write([]byte("<html>not an image</html>"))
	}))
	defer server.Close()

	detector := &HTTPIconDetector{Client: server.Client()}
	if icon, found, err := detector.Detect(context.Background(), server.URL+"/sub"); err != nil || found || icon != "" {
		t.Fatalf("Detect() accepted a response based only on Content-Type: icon=%q found=%t err=%v", icon, found, err)
	}
}

func TestHTTPIconDetectorKeepsCompletedLowerPriorityMatchAtDeadline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/favicon.ico":
			<-request.Context().Done()
		case "/favicon.png":
			_, _ = response.Write([]byte("\x89PNG\r\n\x1a\nimage"))
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	detector := &HTTPIconDetector{Client: server.Client()}
	icon, found, err := detector.Detect(ctx, server.URL+"/sub")
	if err != nil || !found || icon != server.URL+"/favicon.png" {
		t.Fatalf("Detect() = %q, %t, %v; want completed fallback icon", icon, found, err)
	}
}
