package localscript

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const maxSourceURLBytes = 8192

type Fetcher interface {
	Fetch(context.Context, string) (string, error)
}

type HTTPFetcher struct{ Client *http.Client }

// NormalizeSourceURL never includes a supplied URL in validation errors.
func NormalizeSourceURL(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > maxSourceURLBytes || strings.IndexFunc(value, unicode.IsSpace) >= 0 {
		return "", fmt.Errorf("local script URL is invalid or too long")
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Opaque != "" {
		return "", fmt.Errorf("local script URL must use http or https")
	}
	if parsed.User != nil && parsed.User.Username() == "" {
		return "", fmt.Errorf("local script URL credentials are invalid")
	}
	if port := parsed.Port(); port != "" {
		number, err := strconv.Atoi(port)
		if err != nil || number < 1 || number > 65535 {
			return "", fmt.Errorf("local script URL port is invalid")
		}
	}
	parsed.Fragment = ""
	return parsed.String(), nil
}

// Fetch only downloads bytes. JavaScript validation/execution stays in the sandbox.
func (f *HTTPFetcher) Fetch(ctx context.Context, sourceURL string) (string, error) {
	address, err := NormalizeSourceURL(sourceURL)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return "", fmt.Errorf("create local script download request")
	}
	request.Header.Set("Accept", "text/javascript, application/javascript, text/plain, */*")
	request.Header.Set("User-Agent", "Jeemi local script client")
	client := http.Client{}
	if f.Client != nil {
		client = *f.Client
	}
	redirect := client.CheckRedirect
	client.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if len(via) >= 8 {
			return fmt.Errorf("local script redirect limit exceeded")
		}
		if _, err := NormalizeSourceURL(request.URL.String()); err != nil {
			return err
		}
		if redirect != nil {
			if err := redirect(request, via); err != nil {
				return err
			}
		}
		request.Header.Del("Referer")
		return nil
	}
	response, err := client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("download local script failed")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("local script server returned HTTP %d", response.StatusCode)
	}
	if response.ContentLength > maxScriptBytes {
		return "", fmt.Errorf("local script download exceeds %d bytes", maxScriptBytes)
	}
	contents, err := io.ReadAll(io.LimitReader(response.Body, maxScriptBytes+1))
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("read local script download failed")
	}
	if len(contents) > maxScriptBytes || len(contents) == 0 || !utf8.Valid(contents) {
		return "", fmt.Errorf("local script download is empty, too large, or not UTF-8 text")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return string(contents), nil
}
