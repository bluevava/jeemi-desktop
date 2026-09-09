package tray

import "time"

// State must return a cheap in-memory snapshot, independent of the renderer.
// Actions run off the native event loop and may block until they finish.
type ProxyControls struct {
	State                func() ProxyState
	Start, Stop, Restart func()
}

type ProxyState struct {
	Running bool
	Busy    bool
}

func (c *Controller) proxyAvailabilityLocked() [3]bool {
	if !c.started || !c.ready || c.proxyBusy || c.proxy.State == nil {
		return [3]bool{}
	}
	state := c.proxy.State()
	return [3]bool{
		!state.Busy && !state.Running && c.proxy.Start != nil,
		!state.Busy && state.Running && c.proxy.Stop != nil,
		!state.Busy && state.Running && c.proxy.Restart != nil,
	}
}

func (c *Controller) refreshProxy() {
	c.menuUpdateMu.Lock()
	defer c.menuUpdateMu.Unlock()
	c.mu.Lock()
	if !c.started || !c.ready {
		c.mu.Unlock()
		return
	}
	enabled := c.proxyAvailabilityLocked()
	previous, items := c.proxyEnabled, c.proxyItems
	c.proxyEnabled = enabled
	c.mu.Unlock()
	// Native updates can synchronously marshal to the platform event loop.
	// Never hold the lifecycle mutex while waiting for that loop.
	for index, item := range items {
		if item == nil || enabled[index] == previous[index] {
			continue
		}
		if enabled[index] {
			item.Enable()
		} else {
			item.Disable()
		}
	}
}

func (c *Controller) watchProxy(stopped <-chan struct{}) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-stopped:
			return
		case <-ticker.C:
			c.mu.Lock()
			if !c.started || !c.ready || c.stopped != stopped {
				c.mu.Unlock()
				return
			}
			c.mu.Unlock()
			c.refreshProxy()
		}
	}
}

func (c *Controller) runProxyAction(index int) {
	c.mu.Lock()
	// Recheck the source at click time: the core may have exited since the last
	// menu refresh, or another action may already be pending in the window.
	enabled := c.proxyAvailabilityLocked()
	if index < 0 || index >= len(enabled) || !enabled[index] {
		c.mu.Unlock()
		return
	}
	operation := [3]func(){c.proxy.Start, c.proxy.Stop, c.proxy.Restart}[index]
	c.proxyBusy = true
	c.mu.Unlock()
	c.refreshProxy()
	defer func() {
		c.mu.Lock()
		c.proxyBusy = false
		c.mu.Unlock()
		c.refreshProxy()
	}()
	operation()
}
