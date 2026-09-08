//go:build linux

package screen

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/png"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/prop"
)

// All portal tests own an isolated bus. They cannot activate the user's real
// desktop portal, show a permission dialog, or capture the real screen.
func privateBus(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("dbus-daemon"); err != nil {
		t.Skip("dbus-daemon is not installed")
	}
	daemon := exec.Command("dbus-daemon", "--session", "--nofork", "--print-address=1")
	stdout, err := daemon.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := daemon.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = daemon.Process.Kill(); _ = daemon.Wait() })
	scanner := bufio.NewScanner(stdout)
	if !scanner.Scan() {
		t.Fatal("private bus returned no address")
	}
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", scanner.Text())
}

type fakePortal struct {
	connection *dbus.Conn
	response   uint32
	uri        string
	pending    bool
	denied     bool
	seen       chan dbus.ObjectPath
	closed     chan struct{}
}

func (p *fakePortal) Screenshot(sender dbus.Sender, parent string, options map[string]dbus.Variant) (dbus.ObjectPath, *dbus.Error) {
	if p.denied {
		return "", dbus.NewError("org.freedesktop.DBus.Error.AccessDenied", nil)
	}
	token, ok := options["handle_token"].Value().(string)
	if !ok || token == "" || parent != "" || options["interactive"].Value() != false {
		return "", dbus.NewError("org.freedesktop.DBus.Error.InvalidArgs", nil)
	}
	path := dbus.ObjectPath("/org/freedesktop/portal/desktop/request/" + strings.ReplaceAll(strings.TrimPrefix(string(sender), ":"), ".", "_") + "/" + token)
	if !path.IsValid() {
		return "", dbus.NewError("org.freedesktop.DBus.Error.InvalidArgs", nil)
	}
	if err := p.connection.Export(&fakeRequest{p.closed}, path, requestInterface); err != nil {
		return "", dbus.MakeFailedError(err)
	}
	p.seen <- path
	if !p.pending {
		// Intentionally deliver the response before returning the method reply:
		// a client subscribing after Screenshot would lose this signal.
		_ = p.connection.Emit(path, requestInterface+".Response", p.response, map[string]dbus.Variant{"uri": dbus.MakeVariant(p.uri)})
	}
	return path, nil
}

type fakeRequest struct{ closed chan struct{} }

func (r *fakeRequest) Close() *dbus.Error { r.closed <- struct{}{}; return nil }

func startFakePortal(t *testing.T, response uint32, uri string, pending, denied bool) *fakePortal {
	t.Helper()
	privateBus(t)
	connection, err := dbus.ConnectSessionBus()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	p := &fakePortal{connection, response, uri, pending, denied, make(chan dbus.ObjectPath, 4), make(chan struct{}, 4)}
	if err := connection.Export(p, portalPath, screenshotInterface); err != nil {
		t.Fatal(err)
	}
	if _, err := prop.Export(connection, portalPath, map[string]map[string]*prop.Prop{screenshotInterface: {"version": {Value: uint32(2)}}}); err != nil {
		t.Fatal(err)
	}
	if reply, err := connection.RequestName(portalName, dbus.NameFlagDoNotQueue); err != nil || reply != dbus.RequestNameReplyPrimaryOwner {
		t.Fatalf("fake portal name: %v", err)
	}
	return p
}

func testScreenshot(t *testing.T) (string, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "屏幕 screenshot.png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, image.NewRGBA(image.Rect(0, 0, 137, 83))); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return path, (&url.URL{Scheme: "file", Path: path}).String()
}

func TestPortalEarlyResponseUsesWholeImageAndRemovesFile(t *testing.T) {
	path, uri := testScreenshot(t)
	startFakePortal(t, 0, uri, false, false)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	images, err := capturePortal(ctx, readPortalScreenshot)
	if err != nil || len(images) != 1 || images[0].Bounds() != image.Rect(0, 0, 137, 83) {
		t.Fatalf("capture: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("portal screenshot was retained")
	}
}

func TestPortalResponsesAndMissingService(t *testing.T) {
	for _, test := range []struct {
		name            string
		response        uint32
		denied, missing bool
		want            error
	}{
		{"cancel", 1, false, false, ErrCancelled},
		{"failure", 2, false, false, ErrFailed},
		{"no uri", 0, false, false, ErrImage},
		{"denied", 0, true, false, ErrDenied},
		{"unavailable", 0, false, true, ErrUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.missing {
				privateBus(t)
			} else {
				startFakePortal(t, test.response, "", false, test.denied)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_, err := capturePortal(ctx, readPortalScreenshot)
			if !errors.Is(err, test.want) {
				t.Fatalf("got %v, want %v", err, test.want)
			}
		})
	}
}

func TestPortalCancellationClosesRequest(t *testing.T) {
	for _, timeout := range []bool{false, true} {
		t.Run(map[bool]string{false: "cancel", true: "timeout"}[timeout], func(t *testing.T) {
			portal := startFakePortal(t, 0, "", true, false)
			ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
			defer cancel()
			done := make(chan error, 1)
			go func() { _, err := capturePortal(ctx, readPortalScreenshot); done <- err }()
			select {
			case <-portal.seen:
			case <-time.After(2 * time.Second):
				t.Fatal("request not started")
			}
			want := error(context.DeadlineExceeded)
			if !timeout {
				cancel()
				want = context.Canceled
			}
			select {
			case err := <-done:
				if !errors.Is(err, want) {
					t.Fatalf("capture: %v", err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("capture hung")
			}
			select {
			case <-portal.closed:
			default:
				t.Fatal("pending portal request was not closed")
			}
		})
	}
}

func TestPortalIgnoresOtherSendersAndRequestPaths(t *testing.T) {
	path, uri := testScreenshot(t)
	portal := startFakePortal(t, 0, "", true, false)
	rogue, err := dbus.ConnectSessionBus()
	if err != nil {
		t.Fatal(err)
	}
	defer rogue.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := capturePortal(ctx, readPortalScreenshot); done <- err }()
	var request dbus.ObjectPath
	select {
	case request = <-portal.seen:
	case <-ctx.Done():
		t.Fatal("no screenshot request")
	}
	if err := rogue.Emit(request, requestInterface+".Response", uint32(1), map[string]dbus.Variant{}); err != nil {
		t.Fatal(err)
	}
	if err := portal.connection.Emit(request+"_other", requestInterface+".Response", uint32(1), map[string]dbus.Variant{}); err != nil {
		t.Fatal(err)
	}
	if err := portal.connection.Emit(request, requestInterface+".Response", uint32(0), map[string]dbus.Variant{"uri": dbus.MakeVariant(uri)}); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatalf("accepted an unrelated signal: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("screenshot not consumed")
	}
}

func TestPortalHostExitEndsCapture(t *testing.T) {
	portal := startFakePortal(t, 0, "", true, false)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := capturePortal(ctx, readPortalScreenshot); done <- err }()
	select {
	case <-portal.seen:
	case <-ctx.Done():
		t.Fatal("no screenshot request")
	}
	_, _ = portal.connection.ReleaseName(portalName)
	if err := <-done; !errors.Is(err, ErrUnavailable) {
		t.Fatalf("host loss: %v", err)
	}
}

func TestPortalImageValidationAndCleanup(t *testing.T) {
	for _, test := range []string{"cancel", "bad png", "oversized", "pixel limit"} {
		t.Run(test, func(t *testing.T) {
			path, uri := testScreenshot(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var want error = ErrImage
			switch test {
			case "cancel":
				cancel()
				want = context.Canceled
			case "bad png":
				if err := os.WriteFile(path, []byte("bad"), 0600); err != nil {
					t.Fatal(err)
				}
			case "oversized":
				if err := os.Truncate(path, maxCaptureBytes+1); err != nil {
					t.Fatal(err)
				}
				want = ErrTooLarge
			case "pixel limit":
				// A valid IHDR must be rejected before allocating the claimed pixels.
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				binary.BigEndian.PutUint32(data[16:20], 10000)
				binary.BigEndian.PutUint32(data[20:24], 10000)
				binary.BigEndian.PutUint32(data[29:33], crc32.ChecksumIEEE(data[12:29]))
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
				want = ErrTooLarge
			}
			_, err := readPortalScreenshot(ctx, uri)
			if !errors.Is(err, want) {
				t.Fatalf("image validation: %v", err)
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatal("failed/cancelled screenshot was retained")
			}
		})
	}
}

func TestPortalRejectsUnsafeURIWithoutDeletingTarget(t *testing.T) {
	path, uri := testScreenshot(t)
	link := filepath.Join(filepath.Dir(path), "link.png")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"https://example.invalid/screen.png", "file://remote" + path, "file:relative.png", uri + "?query=1", uri + "#fragment", (&url.URL{Scheme: "file", Path: link}).String()} {
		if _, err := readPortalScreenshot(context.Background(), value); !errors.Is(err, ErrImage) {
			t.Fatalf("unsafe URI accepted: %v", err)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatal("unrelated file deleted")
		}
	}
	hardlink := filepath.Join(filepath.Dir(path), "hardlink.png")
	if err := os.Link(path, hardlink); err != nil {
		t.Fatal(err)
	}
	if _, err := readPortalScreenshot(context.Background(), uri); !errors.Is(err, ErrImage) {
		t.Fatal("hardlinked file accepted")
	}
}

func TestWaylandSelectionDoesNotDependOnX11Displays(t *testing.T) {
	for _, test := range []struct {
		session, display string
		want             bool
	}{{"wayland", "", true}, {"wayland", "wayland-0", true}, {"", "wayland-0", true}, {"x11", "wayland-0", false}, {"x11", "", false}, {"", "", false}} {
		if got := useScreenshotPortal(test.session, test.display); got != test.want {
			t.Fatalf("session %q chose portal=%t", test.session, got)
		}
	}
}
