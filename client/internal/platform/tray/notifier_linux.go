//go:build linux

package tray

import (
	"fmt"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/godbus/dbus/v5/prop"

	"jeemi/internal/platform/desktop"
)

const (
	notifierInterface = "org.kde.StatusNotifierItem"
	notifierPath      = dbus.ObjectPath("/StatusNotifierItem")
	menuInterface     = "com.canonical.dbusmenu"
	menuPath          = dbus.ObjectPath("/StatusNotifierMenu")
)

type linuxTooltip struct {
	IconName    string
	Pixmaps     []linuxPixmap
	Title       string
	Description string
}

// The theme name matches desktop.Prepare; the embedded pixmap also supports
// hosts without that theme/cache and portable runs where registration failed.
func (b *linuxTrayBackend) tooltipValue() linuxTooltip {
	return linuxTooltip{IconName: desktop.AppID, Pixmaps: b.pixmaps, Title: b.tooltip}
}

func (b *linuxTrayBackend) export(connection *dbus.Conn) error {
	b.nativeMu.Lock()
	defer b.nativeMu.Unlock()
	if len(b.pixmaps) == 0 || len(b.items) == 0 {
		return fmt.Errorf("tray icon and menu are not prepared")
	}
	notifier := &linuxNotifier{backend: b}
	properties, err := exportLinuxObject(connection, notifierPath, notifierInterface, notifier, map[string]interface{}{
		"Category": "ApplicationStatus", "Id": desktop.AppID, "Title": desktop.Name,
		"Status": "Active", "WindowId": uint32(0), "IconName": desktop.AppID,
		"IconPixmap": b.pixmaps, "IconThemePath": "", "Menu": menuPath, "ItemIsMenu": false,
		"OverlayIconName": "", "OverlayIconPixmap": []linuxPixmap{},
		"AttentionIconName": "", "AttentionIconPixmap": []linuxPixmap{},
		"AttentionMovieName": "", "ToolTip": b.tooltipValue(),
	}, []introspect.Signal{{Name: "NewIcon"}, {Name: "NewToolTip"}})
	if err != nil {
		return err
	}
	_, err = exportLinuxObject(connection, menuPath, menuInterface, &linuxDBusMenu{backend: b}, map[string]interface{}{
		"Version": uint32(3), "TextDirection": "ltr", "Status": "normal", "IconThemePath": []string{},
	}, []introspect.Signal{{Name: "LayoutUpdated", Args: []introspect.Arg{{Name: "revision", Type: "u"}, {Name: "parent", Type: "i"}}}})
	if err != nil {
		return err
	}
	reply, err := connection.RequestName(linuxTrayName(), dbus.NameFlagDoNotQueue)
	if err != nil {
		return err
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		return fmt.Errorf("tray bus name is already owned")
	}
	b.connection, b.properties = connection, properties
	return nil
}

// Only expose the protocol methods, with read-only properties. Updates from
// Jeemi use the internal property handle and emit signals from this same owner.
func exportLinuxObject(connection *dbus.Conn, path dbus.ObjectPath, iface string, methods interface{}, values map[string]interface{}, signals []introspect.Signal) (*prop.Properties, error) {
	if err := connection.Export(methods, path, iface); err != nil {
		return nil, err
	}
	spec := make(map[string]*prop.Prop, len(values))
	for name, value := range values {
		spec[name] = &prop.Prop{Value: value, Emit: prop.EmitTrue}
	}
	properties, err := prop.Export(connection, path, map[string]map[string]*prop.Prop{iface: spec})
	if err != nil {
		return nil, err
	}
	err = connection.Export(introspect.NewIntrospectable(&introspect.Node{
		Name: string(path),
		Interfaces: []introspect.Interface{
			introspect.IntrospectData, prop.IntrospectData,
			{Name: iface, Methods: introspect.Methods(methods), Properties: properties.Introspection(iface), Signals: signals},
		},
	}), path, "org.freedesktop.DBus.Introspectable")
	return properties, err
}

type linuxNotifier struct{ backend *linuxTrayBackend }

func (n *linuxNotifier) Activate(_, _ int32) *dbus.Error {
	n.backend.nativeMu.Lock()
	callback := n.backend.activate
	n.backend.nativeMu.Unlock()
	if callback != nil {
		go callback()
	}
	return nil
}

func (n *linuxNotifier) SecondaryActivate(x, y int32) *dbus.Error { return n.Activate(x, y) }
func (*linuxNotifier) ContextMenu(_, _ int32) *dbus.Error         { return nil }
func (*linuxNotifier) Scroll(_ int32, _ string) *dbus.Error       { return nil }
