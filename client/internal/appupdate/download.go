package appupdate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func downloadClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Minute, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 || req.URL.Scheme != "https" || req.URL.User != nil {
			return ErrDownload
		}
		switch req.URL.Host {
		case "github.com", "release-assets.githubusercontent.com", "objects.githubusercontent.com":
			return nil
		default:
			return ErrDownload
		}
	}}
}

func download(ctx context.Context, client *http.Client, item asset, filename string, progress func(int64)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, item.URL, nil)
	if err != nil {
		return ErrDownload
	}
	req.Header.Set("User-Agent", "Jeemi-updater")
	response, err := client.Do(req)
	if err != nil {
		return ErrDownload
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || (response.ContentLength >= 0 && response.ContentLength != item.Size) {
		return ErrDownload
	}
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return ErrDownload
	}
	defer file.Close()
	hash := sha256.New()
	n, err := io.Copy(io.MultiWriter(file, hash), &progressReader{reader: io.LimitReader(response.Body, item.Size+1), progress: progress})
	if err != nil || n != item.Size {
		return ErrDownload
	}
	if hex.EncodeToString(hash.Sum(nil)) != strings.TrimPrefix(item.Digest, "sha256:") {
		return ErrChecksum
	}
	if file.Sync() != nil {
		return ErrDownload
	}
	return nil
}

type progressReader struct {
	reader   io.Reader
	total    int64
	progress func(int64)
}

func (r *progressReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.total += int64(n)
	if r.progress != nil {
		r.progress(r.total)
	}
	return n, err
}
