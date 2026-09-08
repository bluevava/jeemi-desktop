package subscription

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxSubscriptionIconBytes = 512 << 10

var subscriptionIconPaths = []string{
	"/favicon.ico",
	"/favicon.png",
	"/sub.ico",
	"/sub.png",
}

type IconDetector interface {
	Detect(context.Context, string) (string, bool, error)
}

type HTTPIconDetector struct {
	Client *http.Client
}

func NewHTTPIconDetector() *HTTPIconDetector {
	return &HTTPIconDetector{Client: &http.Client{Timeout: 5 * time.Second}}
}

func (d *HTTPIconDetector) Detect(ctx context.Context, sourceURL string) (string, bool, error) {
	source, err := validateRemoteURL(sourceURL)
	if err != nil {
		return "", false, err
	}
	origin := &url.URL{Scheme: source.Scheme, Host: source.Host}
	client := d.Client
	if client == nil {
		client = NewHTTPIconDetector().Client
	}
	copyClient := *client
	copyClient.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if len(via) >= maxRedirects {
			return fmt.Errorf("subscription icon redirect limit exceeded")
		}
		if !sameOrigin(origin, request.URL) {
			return fmt.Errorf("subscription icon redirect changed origin")
		}
		request.Header.Del("Referer")
		return nil
	}

	type detection struct {
		index int
		url   string
		ok    bool
	}
	detectionContext, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	results := make(chan detection, len(subscriptionIconPaths))
	for index, path := range subscriptionIconPaths {
		candidate := *origin
		candidate.Path = path
		go func(index int, candidateURL string) {
			results <- detection{index: index, url: candidateURL, ok: detectImage(detectionContext, &copyClient, candidateURL)}
		}(index, candidate.String())
	}

	found := make([]detection, len(subscriptionIconPaths))
	finished := make([]bool, len(subscriptionIconPaths))
	completed := 0
	for completed < len(subscriptionIconPaths) {
		select {
		case result := <-results:
			found[result.index] = result
			finished[result.index] = true
			completed++
			for index := range subscriptionIconPaths {
				if !finished[index] {
					break
				}
				if found[index].ok {
					return found[index].url, true, nil
				}
			}
		case <-detectionContext.Done():
		drainResults:
			for completed < len(subscriptionIconPaths) {
				select {
				case result := <-results:
					found[result.index] = result
					finished[result.index] = true
					completed++
				default:
					break drainResults
				}
			}
			for index := range subscriptionIconPaths {
				if finished[index] && found[index].ok {
					return found[index].url, true, nil
				}
			}
			return "", false, nil
		}
	}
	return "", false, nil
}

func detectImage(ctx context.Context, client *http.Client, candidateURL string) bool {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, candidateURL, nil)
	if err != nil {
		return false
	}
	request.Header.Set("Accept", "image/png,image/x-icon,image/vnd.microsoft.icon,image/*;q=0.8")
	request.Header.Set("User-Agent", "Jeemi subscription icon detector")
	response, err := client.Do(request)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices || response.ContentLength > maxSubscriptionIconBytes {
		return false
	}
	contents, err := io.ReadAll(io.LimitReader(response.Body, maxSubscriptionIconBytes+1))
	if err != nil || len(contents) == 0 || len(contents) > maxSubscriptionIconBytes {
		return false
	}
	return looksLikeImage(response.Header.Get("Content-Type"), contents)
}

func looksLikeImage(_ string, contents []byte) bool {
	if len(contents) >= 8 && bytes.Equal(contents[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}) {
		return true
	}
	return len(contents) >= 4 &&
		(bytes.Equal(contents[:4], []byte{0x00, 0x00, 0x01, 0x00}) ||
			bytes.Equal(contents[:4], []byte{0x00, 0x00, 0x02, 0x00}))
}

func sameOrigin(expected, actual *url.URL) bool {
	return strings.EqualFold(expected.Scheme, actual.Scheme) && strings.EqualFold(expected.Host, actual.Host)
}

type disabledIconDetector struct{}

func (disabledIconDetector) Detect(context.Context, string) (string, bool, error) {
	return "", false, nil
}
