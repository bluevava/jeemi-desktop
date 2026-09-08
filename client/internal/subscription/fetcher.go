package subscription

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"jeemi/internal/subscriptionformat"
)

const (
	maxRemoteURLLength = 8192
	maxRedirects       = 8
)

type FetchedContent struct {
	Contents      []byte
	Format        Format
	SuggestedName string
	RemoteProfile RemoteProfile
}

type Fetcher interface {
	Fetch(ctx context.Context, sourceURL string) (FetchedContent, error)
}

type HTTPFetcher struct {
	Client *http.Client
}

func NewHTTPFetcher() *HTTPFetcher {
	return &HTTPFetcher{Client: &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("subscription redirect limit exceeded")
			}
			if _, err := validateRemoteURL(request.URL.String()); err != nil {
				return fmt.Errorf("subscription redirect target is invalid")
			}
			request.Header.Del("Referer")
			return nil
		},
	}}
}

func (f *HTTPFetcher) Fetch(ctx context.Context, sourceURL string) (FetchedContent, error) {
	parsed, err := validateRemoteURL(sourceURL)
	if err != nil {
		return FetchedContent{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return FetchedContent{}, fmt.Errorf("create subscription request")
	}
	request.Header.Set("Accept", "application/yaml, application/json, text/yaml, text/plain, */*")
	request.Header.Set("User-Agent", "Jeemi subscription client")
	client := f.Client
	if client == nil {
		client = NewHTTPFetcher().Client
	}
	response, err := client.Do(request)
	if err != nil {
		return FetchedContent{}, fmt.Errorf("fetch subscription configuration")
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return FetchedContent{}, fmt.Errorf("subscription server returned HTTP %d", response.StatusCode)
	}
	if response.ContentLength > maxSubscriptionBytes {
		return FetchedContent{}, fmt.Errorf("subscription response exceeds the %d byte limit", maxSubscriptionBytes)
	}
	contents, err := io.ReadAll(io.LimitReader(response.Body, maxSubscriptionBytes+1))
	if err != nil {
		return FetchedContent{}, fmt.Errorf("read subscription response")
	}
	if len(contents) > maxSubscriptionBytes {
		return FetchedContent{}, fmt.Errorf("subscription response exceeds the %d byte limit", maxSubscriptionBytes)
	}
	format := detectRemoteFormat(response.Header.Get("Content-Type"), response.Request.URL, contents)
	if err := validateContents(format, contents); err != nil {
		return FetchedContent{}, err
	}
	metadata := parseResponseMetadata(response.Header)
	return FetchedContent{
		Contents:      contents,
		Format:        format,
		SuggestedName: metadata.SuggestedName,
		RemoteProfile: metadata.Profile,
	}, nil
}

func validateRemoteURL(value string) (*url.URL, error) {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > maxRemoteURLLength {
		return nil, fmt.Errorf("subscription URL is empty or too long")
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("subscription URL must use http or https")
	}
	if parsed.User != nil && parsed.User.Username() == "" {
		return nil, fmt.Errorf("subscription URL credentials are invalid")
	}
	parsed.Fragment = ""
	return parsed, nil
}

func detectRemoteFormat(contentType string, sourceURL *url.URL, contents []byte) Format {
	// Storage suffixes describe the actual body. A server's MIME type or URL
	// extension is not evidence of the subscription's syntax.
	if result, err := subscriptionformat.Normalize(contents); err == nil && result.Report.Format != "mihomo" {
		return FormatText
	}
	trimmed := bytes.TrimSpace(bytes.TrimPrefix(contents, []byte{0xef, 0xbb, 0xbf}))
	if bytes.HasPrefix(trimmed, []byte("{")) && json.Valid(trimmed) {
		return FormatJSON
	}
	extension := strings.ToLower(filepath.Ext(sourceURL.Path))
	switch extension {
	case ".txt", ".conf":
		return FormatText
	}
	return FormatYAML
}
