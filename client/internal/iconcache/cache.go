// Package iconcache keeps remote selector images separate from runtime generations.
package iconcache

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	maxBytes      = 1 << 20
	maxEntries    = 256
	maxCacheBytes = 64 << 20
	cacheAge      = 30 * 24 * time.Hour
)

var ErrUnavailable = errors.New("selector_icon_unavailable")

type Cache struct {
	directory string
	client    func() *http.Client
	mu        sync.Mutex
	slots     chan struct{}
}

func New(dataDirectory string, client func() *http.Client) *Cache {
	return &Cache{directory: filepath.Join(dataDirectory, "cache", "selector-icons"), client: client, slots: make(chan struct{}, 4)}
}

func (c *Cache) Load(ctx context.Context, rawURL string) (string, error) {
	address, err := iconURL(rawURL)
	if err != nil {
		return "", ErrUnavailable
	}
	digest := sha256.Sum256([]byte(address.String()))
	name := hex.EncodeToString(digest[:]) + ".img"
	c.mu.Lock()
	previous, modified := c.read(name)
	c.mu.Unlock()
	if previous != "" && time.Since(modified) < cacheAge {
		return previous, nil
	}
	select {
	case c.slots <- struct{}{}:
		defer func() { <-c.slots }()
	case <-ctx.Done():
		return "", ErrUnavailable
	}
	contents, err := c.download(ctx, address)
	if err != nil {
		if previous != "" && ctx.Err() == nil {
			return previous, nil
		}
		return "", ErrUnavailable
	}
	mime, err := imageType(contents)
	if err != nil {
		if previous != "" {
			return previous, nil
		}
		return "", ErrUnavailable
	}
	if ctx.Err() != nil {
		return "", ErrUnavailable
	}
	c.mu.Lock()
	c.write(name, contents)
	c.mu.Unlock()
	return imageURL(mime, contents), nil
}

func iconURL(raw string) (*url.URL, error) {
	if len(raw) == 0 || len(raw) > 4096 {
		return nil, ErrUnavailable
	}
	address, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (address.Scheme != "https" && address.Scheme != "http") || address.Hostname() == "" || address.User != nil {
		return nil, ErrUnavailable
	}
	address.Fragment = ""
	return address, nil
}

func (c *Cache) download(parent context.Context, address *url.URL) ([]byte, error) {
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	client := &http.Client{}
	if c.client != nil {
		if configured := c.client(); configured != nil {
			client = configured
		}
	}
	copyClient := *client
	copyClient.Jar = nil
	copyClient.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return ErrUnavailable
		}
		if _, err := iconURL(request.URL.String()); err != nil {
			return err
		}
		request.Header.Del("Referer")
		request.Header.Del("Authorization")
		return nil
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address.String(), nil)
	if err != nil {
		return nil, ErrUnavailable
	}
	request.Header.Set("Accept", "image/*")
	request.Header.Set("User-Agent", "Jeemi selector icons")
	response, err := copyClient.Do(request)
	if err != nil {
		return nil, ErrUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || response.ContentLength > maxBytes {
		return nil, ErrUnavailable
	}
	contents, err := io.ReadAll(io.LimitReader(response.Body, maxBytes+1))
	if err != nil || len(contents) == 0 || len(contents) > maxBytes || (response.ContentLength >= 0 && int64(len(contents)) != response.ContentLength) {
		return nil, ErrUnavailable
	}
	return contents, nil
}

func imageURL(mime string, contents []byte) string {
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(contents)
}

func (c *Cache) read(name string) (string, time.Time) {
	root, err := os.OpenRoot(c.directory)
	if err != nil {
		return "", time.Time{}
	}
	defer root.Close()
	info, err := root.Lstat(name)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxBytes {
		return "", time.Time{}
	}
	file, err := root.Open(name)
	if err != nil {
		return "", time.Time{}
	}
	defer file.Close()
	contents, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil || len(contents) > maxBytes {
		return "", time.Time{}
	}
	mime, err := imageType(contents)
	if err != nil {
		return "", time.Time{}
	}
	return imageURL(mime, contents), info.ModTime()
}

// A cache write failure must not hide an image that was successfully downloaded.
func (c *Cache) write(name string, contents []byte) {
	if os.MkdirAll(c.directory, 0700) != nil {
		return
	}
	stage, err := os.CreateTemp(c.directory, ".icon-*")
	if err != nil {
		return
	}
	defer os.Remove(stage.Name())
	_, writeErr := stage.Write(contents)
	closeErr := stage.Close()
	if writeErr != nil || closeErr != nil {
		return
	}
	if os.Rename(stage.Name(), filepath.Join(c.directory, name)) != nil {
		return
	}
	c.trim()
}

func (c *Cache) trim() {
	entries, err := os.ReadDir(c.directory)
	if err != nil {
		return
	}
	var images []os.FileInfo
	var total int64
	for _, entry := range entries {
		name := entry.Name()
		if len(name) != 68 || !strings.HasSuffix(name, ".img") {
			continue
		}
		if _, err := hex.DecodeString(name[:64]); err != nil {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		images = append(images, info)
		total += info.Size()
	}
	sort.Slice(images, func(i, j int) bool { return images[i].ModTime().Before(images[j].ModTime()) })
	for len(images) > maxEntries || total > maxCacheBytes {
		oldest := images[0]
		images = images[1:]
		if os.Remove(filepath.Join(c.directory, oldest.Name())) == nil {
			total -= oldest.Size()
		}
		if len(images) == 0 {
			break
		}
	}
}
