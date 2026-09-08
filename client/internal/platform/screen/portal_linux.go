//go:build linux

package screen

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"image"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	portalName          = "org.freedesktop.portal.Desktop"
	portalPath          = dbus.ObjectPath("/org/freedesktop/portal/desktop")
	screenshotInterface = "org.freedesktop.portal.Screenshot"
	requestInterface    = "org.freedesktop.portal.Request"
	portalMethodTimeout = 5 * time.Second
)

func portalError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	var remote dbus.Error
	if errors.As(err, &remote) {
		switch remote.Name {
		case "org.freedesktop.DBus.Error.AccessDenied", "org.freedesktop.portal.Error.NotAllowed":
			return ErrDenied
		case "org.freedesktop.DBus.Error.ServiceUnknown", "org.freedesktop.DBus.Error.NameHasNoOwner", "org.freedesktop.DBus.Error.UnknownMethod", "org.freedesktop.DBus.Error.UnknownInterface":
			return ErrUnavailable
		}
	}
	return ErrFailed
}

func capturePortal(ctx context.Context, readImage func(context.Context, string) (image.Image, error)) ([]image.Image, error) {
	// A private connection scopes signals and is closed on every exit. It never
	// changes the desktop's stored permission decisions or calls a root helper.
	setup, cancelSetup := context.WithTimeout(ctx, portalMethodTimeout)
	defer cancelSetup()
	connectionContext, cancelConnection := context.WithCancel(context.Background())
	defer cancelConnection()
	stopSetup := context.AfterFunc(setup, cancelConnection)
	defer stopSetup()
	connection, err := dbus.ConnectSessionBus(dbus.WithContext(connectionContext))
	if err != nil {
		return nil, ErrUnavailable
	}
	defer connection.Close()
	// This normal property read can activate an installed desktop portal.
	var version dbus.Variant
	if err := connection.Object(portalName, portalPath).CallWithContext(setup,
		"org.freedesktop.DBus.Properties.Get", 0, screenshotInterface, "version").Store(&version); err != nil {
		return nil, portalError(ctx, err)
	}
	if value, ok := version.Value().(uint32); !ok || value < 1 {
		return nil, ErrUnavailable
	}
	var owner string
	if err := connection.BusObject().CallWithContext(setup, "org.freedesktop.DBus.GetNameOwner", 0, portalName).Store(&owner); err != nil {
		return nil, ErrUnavailable
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return nil, ErrFailed
	}
	token := "jeemi_" + hex.EncodeToString(random[:])
	uniqueName := connection.Names()[0]
	prefix := "/org/freedesktop/portal/desktop/request/" + strings.ReplaceAll(strings.TrimPrefix(uniqueName, ":"), ".", "_") + "/"
	request := dbus.ObjectPath(prefix + token)
	signals := make(chan *dbus.Signal, 16)
	connection.Signal(signals)
	defer connection.RemoveSignal(signals)
	// Subscribe before Screenshot, including signals delivered before its
	// method reply. Match only this portal owner and this connection's requests.
	if err := connection.AddMatchSignalContext(setup, dbus.WithMatchSender(owner), dbus.WithMatchInterface(requestInterface),
		dbus.WithMatchMember("Response"), dbus.WithMatchPathNamespace(dbus.ObjectPath(strings.TrimSuffix(prefix, "/")))); err != nil {
		return nil, ErrUnavailable
	}
	if err := connection.AddMatchSignalContext(setup, dbus.WithMatchSender("org.freedesktop.DBus"), dbus.WithMatchInterface("org.freedesktop.DBus"),
		dbus.WithMatchMember("NameOwnerChanged"), dbus.WithMatchArg(0, portalName)); err != nil {
		return nil, ErrUnavailable
	}
	stopSetup()
	finished := false
	defer func() {
		if !finished {
			closeContext, cancel := context.WithTimeout(context.Background(), 750*time.Millisecond)
			defer cancel()
			_ = connection.Object(owner, request).CallWithContext(closeContext, requestInterface+".Close", 0).Err
			// A success signal may already have been queued when cancellation
			// won the select. Dispose of its file without decoding or importing.
			cancelled, cancelRead := context.WithCancel(context.Background())
			cancelRead()
			for len(signals) > 0 {
				signal := <-signals
				if signal != nil && signal.Sender == owner && signal.Path == request && signal.Name == requestInterface+".Response" && len(signal.Body) == 2 && signal.Body[0] == uint32(0) {
					if results, ok := signal.Body[1].(map[string]dbus.Variant); ok {
						if uri, ok := results["uri"].Value().(string); ok {
							_, _ = readImage(cancelled, uri)
						}
					}
				}
			}
		}
	}()
	callContext, cancelCall := context.WithTimeout(ctx, portalMethodTimeout)
	var returned dbus.ObjectPath
	err = connection.Object(owner, portalPath).CallWithContext(callContext, screenshotInterface+".Screenshot", 0, "", map[string]dbus.Variant{
		"handle_token": dbus.MakeVariant(token), "modal": dbus.MakeVariant(false), "interactive": dbus.MakeVariant(false),
	}).Store(&returned)
	cancelCall()
	if err != nil {
		return nil, portalError(ctx, err)
	}
	if !returned.IsValid() || !strings.HasPrefix(string(returned), prefix) {
		return nil, ErrFailed
	}
	request = returned
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-connection.Context().Done():
			return nil, ErrUnavailable
		case signal := <-signals:
			if signal == nil {
				return nil, ErrUnavailable
			}
			if signal.Sender == "org.freedesktop.DBus" && signal.Name == "org.freedesktop.DBus.NameOwnerChanged" && len(signal.Body) == 3 && signal.Body[0] == portalName && signal.Body[2] != owner {
				return nil, ErrUnavailable
			}
			if signal.Sender != owner || signal.Path != request || signal.Name != requestInterface+".Response" {
				continue
			}
			if len(signal.Body) != 2 {
				return nil, ErrFailed
			}
			response, ok := signal.Body[0].(uint32)
			if !ok {
				return nil, ErrFailed
			}
			finished = true
			switch response {
			case 1:
				return nil, ErrCancelled
			case 0:
				results, ok := signal.Body[1].(map[string]dbus.Variant)
				if !ok {
					return nil, ErrImage
				}
				uri, ok := results["uri"].Value().(string)
				if !ok {
					return nil, ErrImage
				}
				// Consume/clean the one returned screenshot even if cancellation
				// arrived with the response; the reader checks context before decode.
				captured, err := readImage(ctx, uri)
				if err != nil {
					return nil, err
				}
				return []image.Image{captured}, nil
			default:
				return nil, ErrFailed
			}
		}
	}
}
