package tray

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

const (
	LanguageChinese   = "zh-CN"
	LanguageEnglish   = "en-US"
	nativeStopTimeout = 2 * time.Second
)

type Labels struct {
	Tooltip        string `json:"tooltip"`
	Show           string `json:"show"`
	ShowTooltip    string `json:"showTooltip"`
	Quit           string `json:"quit"`
	QuitTooltip    string `json:"quitTooltip"`
	Reload         string `json:"reload"`
	ReloadTooltip  string `json:"reloadTooltip"`
	ReloadMessage  string `json:"reloadMessage"`
	ReloadConfirm  string `json:"reloadConfirm"`
	ReloadCancel   string `json:"reloadCancel"`
	StartProxy     string `json:"startProxy"`
	StartTooltip   string `json:"startProxyTooltip"`
	StopProxy      string `json:"stopProxy"`
	StopTooltip    string `json:"stopProxyTooltip"`
	RestartProxy   string `json:"restartProxy"`
	RestartTooltip string `json:"restartProxyTooltip"`
	ProxyFailure   string `json:"proxyFailure"`
	Close          string `json:"close"`
}

type menuItem interface {
	Click(func())
	SetTitle(string)
	SetTooltip(string)
	Enable()
	Disable()
}

type backend interface {
	Start(onReady, onExit func()) (start, end func())
	SetIcon([]byte)
	SetTooltip(string)
	SetOnDoubleClick(func())
	SetContextMenuOnRightClick()
	CreateMenu()
	DetachMenu()
	AddMenuItem(title, tooltip string) menuItem
	AddSeparator()
}

// Controller owns the native tray lifecycle while keeping Wails window actions
// behind the callbacks supplied by the application layer.
type Controller struct {
	mu           sync.Mutex
	menuUpdateMu sync.Mutex

	backend  backend
	icon     []byte
	labels   map[string]Labels
	language string

	showWindow   func()
	reloadUI     func()
	quitApp      func()
	showItem     menuItem
	reloadItem   menuItem
	quitItem     menuItem
	end          func()
	stopped      chan struct{}
	started      bool
	ready        bool
	proxy        ProxyControls
	proxyItems   [3]menuItem
	proxyEnabled [3]bool
	proxyBusy    bool
}

func New(icon, labelsJSON []byte) (*Controller, error) {
	if len(icon) == 0 {
		return nil, fmt.Errorf("tray icon is empty")
	}
	labels := make(map[string]Labels)
	if err := json.Unmarshal(labelsJSON, &labels); err != nil {
		return nil, fmt.Errorf("parse tray labels: %w", err)
	}
	for _, language := range []string{LanguageChinese, LanguageEnglish} {
		if err := validateLabels(language, labels[language]); err != nil {
			return nil, err
		}
	}
	icon, err := prepareTrayIcon(icon)
	if err != nil {
		return nil, err
	}

	return newController(icon, labels, newSystemBackend()), nil
}

func newController(icon []byte, labels map[string]Labels, native backend) *Controller {
	return &Controller{
		backend:  native,
		icon:     append([]byte(nil), icon...),
		labels:   labels,
		language: LanguageChinese,
	}
}

func validateLabels(language string, labels Labels) error {
	if labels.Tooltip == "" || labels.Show == "" || labels.ShowTooltip == "" || labels.Quit == "" || labels.QuitTooltip == "" ||
		labels.Reload == "" || labels.ReloadTooltip == "" || labels.ReloadMessage == "" || labels.ReloadConfirm == "" || labels.ReloadCancel == "" ||
		labels.StartProxy == "" || labels.StartTooltip == "" || labels.StopProxy == "" || labels.StopTooltip == "" ||
		labels.RestartProxy == "" || labels.RestartTooltip == "" || labels.ProxyFailure == "" || labels.Close == "" {
		return fmt.Errorf("tray labels for %s are incomplete", language)
	}
	return nil
}

func (c *Controller) Start(showWindow, reloadUI, quitApp func(), proxy ProxyControls) {
	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return
	}
	if showWindow == nil {
		showWindow = func() {}
	}
	if quitApp == nil {
		quitApp = func() {}
	}
	if reloadUI == nil {
		reloadUI = func() {}
	}
	stopped := make(chan struct{})
	exitOnce := &sync.Once{}
	c.started = true
	c.stopped = stopped
	c.showWindow = showWindow
	c.reloadUI = reloadUI
	c.quitApp = quitApp
	c.proxy = proxy
	c.mu.Unlock()

	if native, ok := c.backend.(interface{ SetOnUnavailable(func()) }); ok {
		native.SetOnUnavailable(c.onUnavailable)
	}
	start, end := c.backend.Start(c.onReady, func() {
		c.onExit(stopped, exitOnce)
	})
	c.mu.Lock()
	c.end = end
	c.mu.Unlock()
	if start != nil {
		start()
	}
}

func (c *Controller) Stop() {
	c.mu.Lock()
	if !c.started {
		c.mu.Unlock()
		return
	}
	c.started = false
	c.ready = false
	end := c.end
	stopped := c.stopped
	c.end = nil
	c.mu.Unlock()

	if end != nil {
		end()
	}
	if stopped != nil {
		select {
		case <-stopped:
		case <-time.After(nativeStopTimeout):
		}
	}

	c.mu.Lock()
	if c.stopped == stopped {
		c.stopped = nil
	}
	c.mu.Unlock()
}

func (c *Controller) Ready() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if availability, ok := c.backend.(interface{ Available() bool }); ok {
		return c.ready && availability.Available()
	}
	return c.ready
}

func (c *Controller) SetLanguage(language string) {
	if language != LanguageEnglish {
		language = LanguageChinese
	}

	c.mu.Lock()
	c.language = language
	labels := c.labels[language]
	showItem := c.showItem
	reloadItem := c.reloadItem
	quitItem := c.quitItem
	proxyItems := c.proxyItems
	ready := c.ready
	c.mu.Unlock()

	if ready {
		c.applyLabels(labels, showItem, reloadItem, quitItem, proxyItems)
	}
}

func (c *Controller) CurrentLabels() Labels {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.labels[c.language]
}

func (c *Controller) onReady() {
	c.mu.Lock()
	if !c.started {
		c.mu.Unlock()
		return
	}
	labels := c.labels[c.language]
	showWindow := c.showWindow
	reloadUI := c.reloadUI
	quitApp := c.quitApp
	c.mu.Unlock()

	c.backend.SetIcon(c.icon)
	c.backend.SetTooltip(labels.Tooltip)
	c.backend.SetOnDoubleClick(showWindow)
	c.backend.SetContextMenuOnRightClick()
	c.backend.CreateMenu()

	showItem := c.backend.AddMenuItem(labels.Show, labels.ShowTooltip)
	showItem.Click(showWindow)
	reloadItem := c.backend.AddMenuItem(labels.Reload, labels.ReloadTooltip)
	reloadItem.Click(reloadUI)
	c.backend.AddSeparator()
	proxyItems := [3]menuItem{
		c.backend.AddMenuItem(labels.StartProxy, labels.StartTooltip),
		c.backend.AddMenuItem(labels.StopProxy, labels.StopTooltip),
		c.backend.AddMenuItem(labels.RestartProxy, labels.RestartTooltip),
	}
	for index, item := range proxyItems {
		item.Disable()
		item.Click(func() { go c.runProxyAction(index) })
	}
	c.backend.AddSeparator()
	quitItem := c.backend.AddMenuItem(labels.Quit, labels.QuitTooltip)
	quitItem.Click(quitApp)

	// macOS needs the menu detached from the status item so click and double-click
	// events remain available; the right-click callback opens the same menu.
	c.backend.DetachMenu()

	c.mu.Lock()
	if !c.started {
		c.mu.Unlock()
		return
	}
	c.showItem = showItem
	c.reloadItem = reloadItem
	c.quitItem = quitItem
	c.proxyItems = proxyItems
	c.proxyEnabled = [3]bool{}
	c.ready = true
	latestLabels := c.labels[c.language]
	stopped := c.stopped
	c.mu.Unlock()

	if latestLabels != labels {
		c.applyLabels(latestLabels, showItem, reloadItem, quitItem, proxyItems)
	}
	c.refreshProxy()
	go c.watchProxy(stopped)
}

func (c *Controller) onUnavailable() {
	c.mu.Lock()
	show := c.showWindow
	shouldShow := c.started && c.ready
	c.mu.Unlock()
	if shouldShow && show != nil {
		go show()
	}
}

func (c *Controller) onExit(stopped chan struct{}, exitOnce *sync.Once) {
	c.mu.Lock()
	var showWindow func()
	if c.stopped == stopped {
		if c.started && c.ready {
			showWindow = c.showWindow
		}
		c.ready = false
	}
	c.mu.Unlock()

	exitOnce.Do(func() {
		close(stopped)
	})
	// A desktop panel/extension can disappear while the window is hidden.
	// Recover the existing window; normal Stop has already cleared started.
	if showWindow != nil {
		go showWindow()
	}
}

func (c *Controller) applyLabels(labels Labels, showItem, reloadItem, quitItem menuItem, proxyItems [3]menuItem) {
	c.menuUpdateMu.Lock()
	defer c.menuUpdateMu.Unlock()
	c.backend.SetTooltip(labels.Tooltip)
	showItem.SetTitle(labels.Show)
	showItem.SetTooltip(labels.ShowTooltip)
	reloadItem.SetTitle(labels.Reload)
	reloadItem.SetTooltip(labels.ReloadTooltip)
	quitItem.SetTitle(labels.Quit)
	quitItem.SetTooltip(labels.QuitTooltip)
	for index, text := range [3][2]string{{labels.StartProxy, labels.StartTooltip}, {labels.StopProxy, labels.StopTooltip}, {labels.RestartProxy, labels.RestartTooltip}} {
		proxyItems[index].SetTitle(text[0])
		proxyItems[index].SetTooltip(text[1])
	}
}
