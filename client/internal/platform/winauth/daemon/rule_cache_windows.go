//go:build windows

package daemon

import (
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/sys/windows"
	"jeemi/internal/platform/rulecache"
)

// Called only after OpenInstalledCore has verified the target and locked its
// parents. All persistent cache IO uses the authenticated ordinary-user token,
// retained until session cleanup even if that GUI process has already exited.
func (c *serviceCores) prepareRuleCache(caller windows.Handle, target Target, workspace, configuration string) error {
	var token windows.Token
	if windows.OpenProcessToken(caller, windows.TOKEN_QUERY|windows.TOKEN_DUPLICATE, &token) != nil {
		return failure("authorization_identity_failed")
	}
	dataRoot := filepath.Clean(target.ExecutablePath)
	for range 5 {
		dataRoot = filepath.Dir(dataRoot)
	}
	if !strings.EqualFold(filepath.Base(dataRoot), "jeemi_data") {
		token.Close()
		return failure("authorization_core_unverified")
	}
	c.cacheToken = token
	c.ruleCache = rulecache.NewSession(workspace, rulecache.Store{
		Root: dataRoot, Path: "cache/mihomo/rule-providers",
		Access: func(action func() error) error { return asCacheOwner(token, action) },
	})
	if err := c.prepareRuleCacheFiles(configuration); err != nil {
		c.closeRuleCache()
		return err
	}
	return nil
}

func asCacheOwner(token windows.Token, action func() error) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	result, _, _ := windows.NewLazySystemDLL("advapi32.dll").NewProc("ImpersonateLoggedOnUser").Call(uintptr(token))
	if result == 0 {
		return failure("authorization_identity_failed")
	}
	defer func() {
		if windows.RevertToSelf() != nil {
			os.Exit(1)
		}
	}()
	return action()
}

func (c *serviceCores) prepareRuleCacheFiles(configuration string) error {
	if c.ruleCache == nil {
		return nil
	}
	for _, name := range []string{"bootstrap.yaml", "config.yaml"} {
		if err := c.ruleCache.Prepare(path.Join(path.Dir(configuration), name)); err != nil && !os.IsNotExist(err) {
			return failure("authorization_snapshot_unsafe_path")
		}
	}
	return nil
}

func (c *serviceCores) closeRuleCache() {
	c.ruleCache = nil
	if c.cacheToken != 0 {
		c.cacheToken.Close()
		c.cacheToken = 0
	}
}
