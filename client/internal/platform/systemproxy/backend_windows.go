//go:build windows

package systemproxy

import (
	"context"
	"fmt"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

const internetSettingsKey = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`

const defaultProxyBypass = "localhost;127.*;192.168.*;10.*;172.16.*;172.17.*;172.18.*;172.19.*;172.20.*;172.21.*;172.22.*;172.23.*;172.24.*;172.25.*;172.26.*;172.27.*;172.28.*;172.29.*;172.30.*;172.31.*;<local>"

type windowsBackend struct {
	SID      string
	notifier *serviceNotifier
}

func (b windowsBackend) openKey(access uint32) (registry.Key, error) {
	if b.SID != "" {
		return registry.OpenKey(registry.USERS, b.SID+`\`+internetSettingsKey, access)
	}
	return registry.OpenKey(registry.CURRENT_USER, internetSettingsKey, access)
}

func newBackend() backend { return windowsBackend{} }

func (b windowsBackend) Capture(context.Context) (snapshot, error) {
	key, err := b.openKey(registry.QUERY_VALUE)
	if err != nil {
		return snapshot{}, err
	}
	defer key.Close()
	result := snapshot{Backend: "windows"}
	if value, _, err := key.GetIntegerValue("ProxyEnable"); err == nil {
		result.ProxyEnableExists = true
		result.ProxyEnable = value
	} else if err != registry.ErrNotExist {
		return snapshot{}, err
	}
	if value, _, err := key.GetStringValue("ProxyServer"); err == nil {
		result.ProxyServerExists = true
		result.ProxyServer = value
	} else if err != registry.ErrNotExist {
		return snapshot{}, err
	}
	result.ProxyOverrideCaptured = true
	if value, _, err := key.GetStringValue("ProxyOverride"); err == nil {
		result.ProxyOverrideExists = true
		result.ProxyOverride = value
	} else if err != registry.ErrNotExist {
		return snapshot{}, err
	}
	return result, nil
}

func (b windowsBackend) Apply(_ context.Context, proxyServer string) error {
	key, err := b.openKey(registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	if err := key.SetDWordValue("ProxyEnable", 1); err != nil {
		return err
	}
	if err := key.SetStringValue("ProxyServer", proxyServer); err != nil {
		return err
	}
	return key.SetStringValue("ProxyOverride", defaultProxyBypass)
}

func (b windowsBackend) Restore(_ context.Context, previous snapshot) error {
	if previous.Backend != "" && previous.Backend != "windows" {
		return fmt.Errorf("system proxy recovery record does not belong to the Windows backend")
	}
	key, err := b.openKey(registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	if previous.ProxyEnableExists {
		if err := key.SetDWordValue("ProxyEnable", uint32(previous.ProxyEnable)); err != nil {
			return err
		}
	} else if err := key.DeleteValue("ProxyEnable"); err != nil && err != registry.ErrNotExist {
		return err
	}
	if previous.ProxyServerExists {
		if err := key.SetStringValue("ProxyServer", previous.ProxyServer); err != nil {
			return err
		}
	} else if err := key.DeleteValue("ProxyServer"); err != nil && err != registry.ErrNotExist {
		return err
	}
	// Version 1 recovery records did not capture ProxyOverride and the old
	// implementation never changed it. Preserve that value when recovering a
	// stale v1 record instead of guessing or deleting a user setting.
	if previous.ProxyOverrideCaptured {
		if previous.ProxyOverrideExists {
			return key.SetStringValue("ProxyOverride", previous.ProxyOverride)
		}
		if err := key.DeleteValue("ProxyOverride"); err != nil && err != registry.ErrNotExist {
			return err
		}
	}
	return nil
}

func (b windowsBackend) Notify(context.Context) error {
	if b.notifier != nil {
		return b.notifier.notify(context.Background())
	}
	if b.SID != "" {
		return nil
	}

	wininet := syscall.NewLazyDLL("wininet.dll")
	procedure := wininet.NewProc("InternetSetOptionW")
	for _, option := range []uintptr{39, 37} { // SETTINGS_CHANGED, REFRESH
		result, _, callErr := procedure.Call(0, option, 0, 0)
		if result == 0 {
			return fmt.Errorf("InternetSetOptionW(%d): %w", option, callErr)
		}
	}
	return nil
}
