//go:build linux

package tray

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/prop"

	"jeemi/internal/platform/desktop"
)

func TestTrayRegistrationRequiresThisProcess(t *testing.T) {
	if containsTrayItem([]string{":1.7/StatusNotifierItem"}, ":1.8", "org.kde.StatusNotifierItem-42-1") {
		t.Fatal("another app counted as our tray")
	}
	for _, entry := range []string{":1.8/StatusNotifierItem", ":1.8@/StatusNotifierItem", "org.kde.StatusNotifierItem-42-1", "org.kde.StatusNotifierItem-42-1/StatusNotifierItem"} {
		if !containsTrayItem([]string{entry}, ":1.8", "org.kde.StatusNotifierItem-42-1") {
			t.Fatalf("KDE/Ubuntu entry was not accepted: %s", entry)
		}
	}
	for _, entry := range []string{":1.88@/StatusNotifierItem", ":1.8@/Other", "org.kde.StatusNotifierItem-42-10"} {
		if containsTrayItem([]string{entry}, ":1.8", "org.kde.StatusNotifierItem-42-1") {
			t.Fatalf("wrong item accepted: %s", entry)
		}
	}
}

type testWatcher struct {
	properties *prop.Properties
	connection *dbus.Conn
	registered chan string
	iconPixels []byte
}

type wirePixmap struct {
	Width, Height int32
	Pixels        []byte
}
type wireMenuLayout struct {
	ID         int32
	Properties map[string]dbus.Variant
	Children   []dbus.Variant
}

func (w *testWatcher) RegisterStatusNotifierItem(service string, sender dbus.Sender) *dbus.Error {
	owner, path, registered := string(sender), dbus.ObjectPath("/StatusNotifierItem"), service
	if strings.HasPrefix(service, "/") {
		path = dbus.ObjectPath(service)
		// Real Ubuntu AppIndicator returns '@', not the KDE owner/path format.
		registered = owner + "@" + service
	} else if err := w.connection.BusObject().Call("org.freedesktop.DBus.GetNameOwner", 0, service).Store(&owner); err != nil {
		return dbus.MakeFailedError(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	item := w.connection.Object(owner, path)
	call := item.CallWithContext(ctx, "org.freedesktop.DBus.Properties.GetAll", 0, "org.kde.StatusNotifierItem")
	if call.Err != nil {
		return dbus.MakeFailedError(call.Err)
	}
	// Inspect the original wire variants: dbus.Store recursively unwraps and
	// re-infers nested struct signatures, which would hide protocol type errors.
	properties := call.Body[0].(map[string]dbus.Variant)
	var pixmaps []wirePixmap
	if err := dbus.Store([]interface{}{properties["IconPixmap"].Value()}, &pixmaps); err != nil {
		return dbus.MakeFailedError(err)
	}
	if properties["Title"].Value() != desktop.Name || properties["Id"].Value() != desktop.AppID || properties["IconName"].Value() != desktop.AppID ||
		properties["IconPixmap"].Signature().String() != "a(iiay)" || len(pixmaps) != 1 || pixmaps[0].Width != 64 || pixmaps[0].Height != 64 || !bytes.Equal(pixmaps[0].Pixels, w.iconPixels) {
		details := fmt.Sprintf("title=%v id=%v icon=%v signature=%s pixmaps=%d", properties["Title"], properties["Id"], properties["IconName"], properties["IconPixmap"].Signature(), len(pixmaps))
		if len(pixmaps) == 1 {
			details += fmt.Sprintf(" size=%dx%d pixels=%x want=%x", pixmaps[0].Width, pixmaps[0].Height, sha256.Sum256(pixmaps[0].Pixels), sha256.Sum256(w.iconPixels))
		}
		return dbus.NewError("org.jeemi.Test.InvalidInitialIcon", []interface{}{details})
	}
	menuPath, ok := properties["Menu"].Value().(dbus.ObjectPath)
	if !ok {
		return dbus.NewError("org.jeemi.Test.InvalidMenu", nil)
	}
	var revision uint32
	var layout wireMenuLayout
	if err := w.connection.Object(owner, menuPath).CallWithContext(ctx, "com.canonical.dbusmenu.GetLayout", 0, int32(0), int32(1), []string{}).Store(&revision, &layout); err != nil {
		return dbus.MakeFailedError(err)
	}
	if len(layout.Children) != 8 {
		return dbus.NewError("org.jeemi.Test.IncompleteMenu", []interface{}{"full menu required before registration; no duplicates on reconnect"})
	}
	w.properties.SetMust(watcherName, "RegisteredStatusNotifierItems", []string{registered})
	w.registered <- registered
	return nil
}

func TestLinuxTrayLifecycleOnPrivateBus(t *testing.T) {
	if _, err := exec.LookPath("dbus-daemon"); err != nil {
		t.Skip("dbus-daemon is not installed")
	}
	// This test owns an isolated bus and a fake panel. It never connects to the
	// user's session bus, creates a desktop icon, or displays a real window.
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
	connection, err := dbus.ConnectSessionBus()
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if linuxTrayRegistered(context.Background(), connection, false) {
		t.Fatal("missing tray host reported ready")
	}
	icon, err := os.ReadFile("../../../build/appicon.png")
	if err != nil {
		t.Fatal(err)
	}
	preparedIcon, err := prepareTrayIcon(icon)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := png.Decode(bytes.NewReader(preparedIcon))
	if err != nil {
		t.Fatal(err)
	}
	// Check the real PNG, including its predominantly translucent artwork.
	// An opaque synthetic icon hid the old RGBA low-byte conversion bug.
	expectedPixels := make([]byte, 0, 64*64*4)
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			pixel := color.NRGBAModel.Convert(decoded.At(x, y)).(color.NRGBA)
			expectedPixels = append(expectedPixels, pixel.A, pixel.R, pixel.G, pixel.B)
		}
	}
	labels := Labels{Tooltip: "Jeemi", Show: "show", ShowTooltip: "show", Reload: "reload", ReloadTooltip: "reload", Quit: "quit", QuitTooltip: "quit"}
	waiting := newController(preparedIcon, map[string]Labels{LanguageChinese: labels, LanguageEnglish: labels}, newSystemBackend())
	waiting.Start(nil, nil, nil, ProxyControls{})
	stopped := make(chan struct{})
	go func() { waiting.Stop(); close(stopped) }()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("waiting for an absent host prevented cancellation")
	}
	englishLabels := labels
	englishLabels.Show = "Show Jeemi"
	controller := newController(preparedIcon, map[string]Labels{LanguageChinese: labels, LanguageEnglish: englishLabels}, newSystemBackend())
	shown := make(chan struct{}, 8)
	reloaded, quit := make(chan struct{}, 4), make(chan struct{}, 4)
	controller.Start(func() { shown <- struct{}{} }, func() { reloaded <- struct{}{} }, func() { quit <- struct{}{} }, ProxyControls{})
	defer controller.Stop()
	// Start while the host is absent. An extension enabled later must work
	// without restarting Jeemi, and close must remain a normal exit until ready.
	time.Sleep(100 * time.Millisecond)
	if controller.Ready() {
		t.Fatal("tray ready before host exists")
	}
	properties, err := prop.Export(connection, "/StatusNotifierWatcher", map[string]map[string]*prop.Prop{
		watcherName: {
			"IsStatusNotifierHostRegistered": {Value: true, Emit: prop.EmitTrue},
			"RegisteredStatusNotifierItems":  {Value: []string{}, Emit: prop.EmitTrue},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	registrations := make(chan string, 8)
	if err := connection.Export(&testWatcher{properties, connection, registrations, expectedPixels}, "/StatusNotifierWatcher", watcherName); err != nil {
		t.Fatal(err)
	}
	if _, err := connection.RequestName(watcherName, dbus.NameFlagDoNotQueue); err != nil {
		t.Fatal(err)
	}
	waitTrayReady(t, controller)
	select {
	case registered := <-registrations:
		if !strings.Contains(registered, "@/StatusNotifierItem") {
			t.Fatalf("Ubuntu format not exercised: %s", registered)
		}
	default:
		t.Fatal("native registration did not expose complete icon/menu")
	}
	item := connection.Object(linuxTrayName(), "/StatusNotifierItem")
	if call := item.Call("org.kde.StatusNotifierItem.Activate", 0, int32(0), int32(0)); call.Err != nil {
		t.Fatal(call.Err)
	}
	select {
	case <-shown:
	case <-time.After(time.Second):
		t.Fatal("tray activation did not restore the window")
	}
	controller.SetLanguage(LanguageEnglish)
	checkLinuxTrayMenu(t, connection, shown, reloaded, quit)
	if _, err := connection.ReleaseName(watcherName); err != nil {
		t.Fatal(err)
	}
	select {
	case <-shown:
	case <-time.After(4 * time.Second):
		t.Fatal("lost watcher did not restore existing window")
	}
	if controller.Ready() {
		t.Fatal("lost watcher remained ready")
	}
	properties.SetMust(watcherName, "RegisteredStatusNotifierItems", []string{})
	if _, err := connection.RequestName(watcherName, dbus.NameFlagDoNotQueue); err != nil {
		t.Fatal(err)
	}
	waitTrayReady(t, controller)
	select {
	case <-registrations:
	default:
		t.Fatal("watcher restart did not re-register the native item")
	}
	// A host can disappear while the watcher itself stays alive (KDE).
	properties.SetMust(watcherName, "IsStatusNotifierHostRegistered", false)
	select {
	case <-shown:
	case <-time.After(4 * time.Second):
		t.Fatal("lost tray host did not restore existing window")
	}
	if controller.Ready() {
		t.Fatal("lost tray host remained ready")
	}
	properties.SetMust(watcherName, "IsStatusNotifierHostRegistered", true)
	waitTrayReady(t, controller)
	controller.Stop()
	controller.Stop()
	if controller.Ready() {
		t.Fatal("stopped tray stayed ready")
	}
	if err := connection.BusObject().Call("org.freedesktop.DBus.GetNameOwner", 0, linuxTrayName()).Err; err == nil {
		t.Fatal("native item still registered after Stop")
	}
}

func waitTrayReady(t *testing.T, controller *Controller) {
	t.Helper()
	deadline := time.After(9 * time.Second)
	for !controller.Ready() {
		select {
		case <-deadline:
			t.Fatal("tray never registered with private watcher")
		case <-time.After(10 * time.Millisecond):
		}
	}
}
