//go:build linux

package screen

import (
	"context"
	"image"
	"os"
	"strings"
)

func useScreenshotPortal(session, waylandDisplay string) bool {
	switch strings.ToLower(strings.TrimSpace(session)) {
	case "wayland":
		return true
	case "x11":
		return false
	default:
		return waylandDisplay != ""
	}
}

func captureDesktop(ctx context.Context) ([]image.Image, error) {
	if useScreenshotPortal(os.Getenv("XDG_SESSION_TYPE"), os.Getenv("WAYLAND_DISPLAY")) {
		// A Wayland screenshot is one portal result in native pixels. Never
		// enumerate X11 monitors or crop it using XWayland logical coordinates.
		return capturePortal(ctx, readPortalScreenshot)
	}
	return captureNative(ctx)
}
