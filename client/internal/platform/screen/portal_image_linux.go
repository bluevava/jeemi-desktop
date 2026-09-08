//go:build linux

package screen

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// Only the authenticated desktop portal supplies this URI. Unlike a selected
// QR image, it denotes a one-shot screenshot owned by this request. Do not
// retain a second copy or pass its path to the renderer.
func readPortalScreenshot(ctx context.Context, uri string) (captured image.Image, resultErr error) {
	location, err := url.Parse(uri)
	if err != nil || location.Scheme != "file" || (location.Host != "" && location.Host != "localhost") ||
		location.User != nil || location.Opaque != "" || location.RawQuery != "" || location.ForceQuery || location.Fragment != "" ||
		!filepath.IsAbs(location.Path) || strings.ContainsRune(location.Path, 0) || strings.ToLower(filepath.Ext(location.Path)) != ".png" {
		return nil, ErrImage
	}
	// Anchor reads and cleanup to the same opened directory, even if its name
	// changes. Never follow a final symlink or remove a different replacement.
	root, err := os.OpenRoot(filepath.Dir(location.Path))
	if err != nil {
		return nil, ErrImage
	}
	defer root.Close()
	name := filepath.Base(location.Path)
	file, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, ErrImage
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, ErrImage
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Getuid()) || stat.Nlink != 1 {
		return nil, ErrImage
	}
	removed := false
	remove := func() error {
		current, err := root.Lstat(name)
		if err != nil || !os.SameFile(info, current) {
			return ErrImage
		}
		if err := root.Remove(name); err != nil {
			return ErrImage
		}
		removed = true
		return nil
	}
	defer func() {
		if !removed {
			if err := remove(); err != nil && resultErr == nil {
				captured, resultErr = nil, err
			}
		}
	}()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if info.Size() > maxCaptureBytes {
		return nil, ErrTooLarge
	}
	if info.Size() < 1 {
		return nil, ErrImage
	}
	contents, err := io.ReadAll(io.LimitReader(file, maxCaptureBytes+1))
	if err != nil {
		return nil, ErrImage
	}
	if len(contents) > maxCaptureBytes {
		return nil, ErrTooLarge
	}
	if err := remove(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	configuration, err := png.DecodeConfig(bytes.NewReader(contents))
	if err != nil || configuration.Width < 1 || configuration.Height < 1 {
		return nil, ErrImage
	}
	if uint64(configuration.Width)*uint64(configuration.Height) > maxCapturePixels {
		return nil, ErrTooLarge
	}
	captured, err = png.Decode(bytes.NewReader(contents))
	if err != nil {
		return nil, ErrImage
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return captured, nil
}
