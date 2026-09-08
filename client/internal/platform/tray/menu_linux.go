//go:build linux

package tray

import "github.com/godbus/dbus/v5"

// Jeemi's menu is a fixed, flat list. Export only that list and its callbacks;
// neither the desktop panel nor React can add items or supply commands.
type linuxMenuItem struct {
	backend        *linuxTrayBackend
	id             int32
	title, tooltip string
	separator      bool
	callback       func()
}

func (b *linuxTrayBackend) CreateMenu() {
	b.nativeMu.Lock()
	defer b.nativeMu.Unlock()
	b.items = nil
	b.menuChanged()
}

func (b *linuxTrayBackend) AddMenuItem(title, tooltip string) menuItem {
	return b.addMenuItem(title, tooltip, false)
}

func (b *linuxTrayBackend) AddSeparator() { b.addMenuItem("", "", true) }

func (b *linuxTrayBackend) addMenuItem(title, tooltip string, separator bool) *linuxMenuItem {
	b.nativeMu.Lock()
	defer b.nativeMu.Unlock()
	item := &linuxMenuItem{backend: b, id: int32(len(b.items) + 1), title: title, tooltip: tooltip, separator: separator}
	b.items = append(b.items, item)
	b.menuChanged()
	return item
}

func (i *linuxMenuItem) Click(callback func()) {
	i.backend.nativeMu.Lock()
	defer i.backend.nativeMu.Unlock()
	i.callback = callback
}

func (i *linuxMenuItem) SetTitle(title string) {
	i.backend.nativeMu.Lock()
	defer i.backend.nativeMu.Unlock()
	if i.title != title {
		i.title = title
		i.backend.menuChanged()
	}
}

func (i *linuxMenuItem) SetTooltip(tooltip string) {
	i.backend.nativeMu.Lock()
	defer i.backend.nativeMu.Unlock()
	if i.tooltip != tooltip {
		i.tooltip = tooltip
		i.backend.menuChanged()
	}
}

// Caller holds nativeMu. A layout revision also refreshes localized labels.
func (b *linuxTrayBackend) menuChanged() {
	b.menuRevision++
	if b.connection != nil {
		_ = b.connection.Emit(menuPath, menuInterface+".LayoutUpdated", b.menuRevision, int32(0))
	}
}

type linuxMenuLayout struct {
	ID         int32
	Properties map[string]dbus.Variant
	Children   []dbus.Variant
}

type linuxMenuProperties struct {
	ID         int32
	Properties map[string]dbus.Variant
}

type linuxMenuEvent struct {
	ID        int32
	Event     string
	Data      dbus.Variant
	Timestamp uint32
}

type linuxDBusMenu struct{ backend *linuxTrayBackend }

func invalidMenuItem() *dbus.Error {
	return dbus.NewError("com.canonical.dbusmenu.Error.InvalidMenuItem", []interface{}{"unknown menu item"})
}

// Caller holds nativeMu; return fresh maps so D-Bus serialization never races
// with a language switch. An empty property list means all properties.
func (m *linuxDBusMenu) properties(id int32, names []string) (map[string]dbus.Variant, *dbus.Error) {
	if id < 0 || id > int32(len(m.backend.items)) {
		return nil, invalidMenuItem()
	}
	values := map[string]interface{}{"enabled": true, "visible": true}
	if id == 0 {
		values["children-display"] = "submenu"
	} else {
		item := m.backend.items[id-1]
		if item.separator {
			values["type"] = "separator"
		} else {
			values["label"], values["accessible-desc"] = item.title, item.tooltip
		}
	}
	result := make(map[string]dbus.Variant)
	for name, value := range values {
		if len(names) == 0 {
			result[name] = dbus.MakeVariant(value)
			continue
		}
		for _, requested := range names {
			if requested == name {
				result[name] = dbus.MakeVariant(value)
				break
			}
		}
	}
	return result, nil
}

func (m *linuxDBusMenu) GetLayout(parentID, depth int32, names []string) (uint32, linuxMenuLayout, *dbus.Error) {
	m.backend.nativeMu.Lock()
	defer m.backend.nativeMu.Unlock()
	properties, err := m.properties(parentID, names)
	layout := linuxMenuLayout{ID: parentID, Properties: properties, Children: []dbus.Variant{}}
	if err != nil {
		return 0, layout, err
	}
	if parentID == 0 && depth != 0 {
		for _, item := range m.backend.items {
			values, _ := m.properties(item.id, names)
			layout.Children = append(layout.Children, dbus.MakeVariant(linuxMenuLayout{item.id, values, []dbus.Variant{}}))
		}
	}
	return m.backend.menuRevision, layout, nil
}

func (m *linuxDBusMenu) GetGroupProperties(ids []int32, names []string) ([]linuxMenuProperties, *dbus.Error) {
	m.backend.nativeMu.Lock()
	defer m.backend.nativeMu.Unlock()
	if len(ids) == 0 {
		for id := int32(0); id <= int32(len(m.backend.items)); id++ {
			ids = append(ids, id)
		}
	}
	result := []linuxMenuProperties{}
	for _, id := range ids {
		if values, err := m.properties(id, names); err == nil {
			result = append(result, linuxMenuProperties{id, values})
		}
	}
	return result, nil
}

func (m *linuxDBusMenu) GetProperty(id int32, name string) (dbus.Variant, *dbus.Error) {
	m.backend.nativeMu.Lock()
	defer m.backend.nativeMu.Unlock()
	values, err := m.properties(id, []string{name})
	if err != nil {
		return dbus.Variant{}, err
	}
	if value, ok := values[name]; ok {
		return value, nil
	}
	return dbus.Variant{}, dbus.NewError("com.canonical.dbusmenu.Error.InvalidProperty", []interface{}{"unknown menu property"})
}

func (m *linuxDBusMenu) Event(id int32, event string, _ dbus.Variant, _ uint32) *dbus.Error {
	m.backend.nativeMu.Lock()
	var callback func()
	_, err := m.properties(id, nil)
	if err == nil && id != 0 && event == "clicked" {
		callback = m.backend.items[id-1].callback
	}
	m.backend.nativeMu.Unlock()
	if callback != nil {
		// A Quit callback may wait for the bus to close; never run it on the
		// request goroutine or while holding the menu/registration mutex.
		go callback()
	}
	return err
}

func (m *linuxDBusMenu) EventGroup(events []linuxMenuEvent) ([]int32, *dbus.Error) {
	invalid := []int32{}
	for _, event := range events {
		if err := m.Event(event.ID, event.Event, event.Data, event.Timestamp); err != nil {
			invalid = append(invalid, event.ID)
		}
	}
	return invalid, nil
}

func (m *linuxDBusMenu) AboutToShow(id int32) (bool, *dbus.Error) {
	m.backend.nativeMu.Lock()
	defer m.backend.nativeMu.Unlock()
	_, err := m.properties(id, nil)
	return false, err
}

func (m *linuxDBusMenu) AboutToShowGroup(ids []int32) ([]int32, []int32, *dbus.Error) {
	invalid := []int32{}
	for _, id := range ids {
		if _, err := m.AboutToShow(id); err != nil {
			invalid = append(invalid, id)
		}
	}
	return []int32{}, invalid, nil
}
