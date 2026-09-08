package tray

import (
	"testing"
	"time"
)

type fakeMenuItem struct {
	title   string
	tooltip string
	click   func()
}

func (item *fakeMenuItem) Click(callback func())     { item.click = callback }
func (item *fakeMenuItem) SetTitle(title string)     { item.title = title }
func (item *fakeMenuItem) SetTooltip(tooltip string) { item.tooltip = tooltip }

type fakeBackend struct {
	icon              []byte
	tooltip           string
	doubleClick       func()
	rightClickEnabled bool
	menuCreated       bool
	menuDetached      bool
	separatorCount    int
	items             []*fakeMenuItem
	startCount        int
	endCount          int
	onExit            func()
}

type delayedExitBackend struct {
	*fakeBackend
	exitRequested chan struct{}
	releaseExit   chan struct{}
}

func (backend *delayedExitBackend) Start(onReady, onExit func()) (func(), func()) {
	backend.onExit = onExit
	return func() {
			backend.startCount++
			onReady()
		}, func() {
			backend.endCount++
			close(backend.exitRequested)
			go func() {
				<-backend.releaseExit
				onExit()
			}()
		}
}

func (backend *fakeBackend) Start(onReady, onExit func()) (func(), func()) {
	backend.onExit = onExit
	return func() {
			backend.startCount++
			onReady()
		}, func() {
			backend.endCount++
			if backend.onExit != nil {
				backend.onExit()
			}
		}
}

func (backend *fakeBackend) SetIcon(icon []byte) {
	backend.icon = append([]byte(nil), icon...)
}

func (backend *fakeBackend) SetTooltip(tooltip string) {
	backend.tooltip = tooltip
}

func (backend *fakeBackend) SetOnDoubleClick(callback func()) {
	backend.doubleClick = callback
}

func (backend *fakeBackend) SetContextMenuOnRightClick() {
	backend.rightClickEnabled = true
}

func (backend *fakeBackend) CreateMenu() {
	backend.menuCreated = true
}

func (backend *fakeBackend) DetachMenu() {
	backend.menuDetached = true
}

func (backend *fakeBackend) AddMenuItem(title, tooltip string) menuItem {
	item := &fakeMenuItem{title: title, tooltip: tooltip}
	backend.items = append(backend.items, item)
	return item
}

func (backend *fakeBackend) AddSeparator() {
	backend.separatorCount++
}

func TestControllerProvidesLocalizedTrayActionsAndLifecycle(t *testing.T) {
	labels := map[string]Labels{
		LanguageChinese: {
			Tooltip:     "Jeemi 托盘",
			Show:        "显示 Jeemi",
			ShowTooltip: "显示界面",
			Quit:        "退出 Jeemi",
			QuitTooltip: "退出应用",
			Reload:      "重新加载界面", ReloadTooltip: "恢复界面", ReloadMessage: "丢弃草稿？", ReloadConfirm: "重新加载", ReloadCancel: "取消",
		},
		LanguageEnglish: {
			Tooltip:     "Jeemi tray",
			Show:        "Show Jeemi",
			ShowTooltip: "Show window",
			Quit:        "Quit Jeemi",
			QuitTooltip: "Quit application",
			Reload:      "Reload interface", ReloadTooltip: "Recover interface", ReloadMessage: "Discard drafts?", ReloadConfirm: "Reload", ReloadCancel: "Cancel",
		},
	}
	native := &fakeBackend{}
	controller := newController([]byte{1, 2, 3}, labels, native)
	showCount := 0
	quitCount := 0
	reloadCount := 0

	controller.Start(func() { showCount++ }, func() { reloadCount++ }, func() { quitCount++ })
	controller.Start(nil, nil, nil)

	if !controller.Ready() {
		t.Fatal("tray was not marked ready")
	}
	if native.startCount != 1 || !native.menuCreated || !native.menuDetached || !native.rightClickEnabled {
		t.Fatalf("tray was not initialised once: %#v", native)
	}
	if native.separatorCount != 1 || len(native.items) != 3 {
		t.Fatalf("unexpected tray menu: separators=%d items=%d", native.separatorCount, len(native.items))
	}
	if native.items[0].title != "显示 Jeemi" || native.items[1].title != "重新加载界面" || native.items[2].title != "退出 Jeemi" {
		t.Fatal("unexpected default labels")
	}

	native.doubleClick()
	native.items[0].click()
	native.items[1].click()
	if reloadCount != 1 || quitCount != 0 || native.endCount != 0 {
		t.Fatal("UI reload should dispatch independently without quitting or stopping the tray")
	}
	native.items[2].click()
	if showCount != 2 || quitCount != 1 {
		t.Fatalf("unexpected action counts: show=%d quit=%d", showCount, quitCount)
	}

	controller.SetLanguage(LanguageEnglish)
	if native.tooltip != "Jeemi tray" || native.items[0].title != "Show Jeemi" || native.items[1].title != "Reload interface" || native.items[2].title != "Quit Jeemi" || controller.CurrentLabels().ReloadCancel != "Cancel" {
		t.Fatal("English labels were not applied to menu and recovery dialog")
	}

	controller.Stop()
	controller.Stop()
	if controller.Ready() {
		t.Fatal("tray remained ready after Stop()")
	}
	if native.endCount != 1 {
		t.Fatalf("tray stopped %d times", native.endCount)
	}
}

func TestNewRejectsIncompleteLocalizedLabels(t *testing.T) {
	_, err := New([]byte{1}, []byte(`{"zh-CN":{"tooltip":"Jeemi"},"en-US":{"tooltip":"Jeemi"}}`))
	if err == nil {
		t.Fatal("New() accepted incomplete tray labels")
	}
}

func TestControllerWaitsForNativeTrayRemoval(t *testing.T) {
	labels := map[string]Labels{
		LanguageChinese: {Tooltip: "tray", Show: "show", ShowTooltip: "show window", Quit: "quit", QuitTooltip: "quit app"},
		LanguageEnglish: {Tooltip: "tray", Show: "show", ShowTooltip: "show window", Quit: "quit", QuitTooltip: "quit app"},
	}
	native := &delayedExitBackend{
		fakeBackend:   &fakeBackend{},
		exitRequested: make(chan struct{}),
		releaseExit:   make(chan struct{}),
	}
	controller := newController([]byte{1}, labels, native)
	controller.Start(nil, nil, nil)

	stopReturned := make(chan struct{})
	go func() {
		controller.Stop()
		close(stopReturned)
	}()

	select {
	case <-native.exitRequested:
	case <-time.After(time.Second):
		t.Fatal("native tray stop was not requested")
	}
	select {
	case <-stopReturned:
		t.Fatal("Stop returned before the native tray exit callback")
	default:
	}

	close(native.releaseExit)
	select {
	case <-stopReturned:
	case <-time.After(time.Second):
		t.Fatal("Stop did not return after native tray removal")
	}
}
