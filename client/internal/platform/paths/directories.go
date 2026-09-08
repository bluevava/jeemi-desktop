package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	DataDirectoryName    = "jeemi_data"
	WebViewDirectoryName = "webview2"
)

// ProgramDirectory returns the directory containing the running executable.
// It is an application resource location and must not be used for mutable data.
func ProgramDirectory() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable: %w", err)
	}
	absolute, err := filepath.Abs(executable)
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}
	return programDirectoryFromExecutable(absolute)
}

func programDirectoryFromExecutable(executable string) (string, error) {
	if strings.TrimSpace(executable) == "" {
		return "", fmt.Errorf("executable path is empty")
	}
	clean := filepath.Clean(executable)
	if !filepath.IsAbs(clean) {
		return "", fmt.Errorf("executable path is not absolute: %s", executable)
	}
	return filepath.Dir(clean), nil
}

// DataDirectory returns the platform user configuration root joined with
// jeemi_data. Go maps this root to APPDATA on Windows, Application Support on
// macOS, and XDG_CONFIG_HOME (or ~/.config) on Linux.
func DataDirectory() (string, error) {
	configRoot, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user configuration directory: %w", err)
	}
	return dataDirectoryFromConfigRoot(configRoot)
}

func dataDirectoryFromConfigRoot(configRoot string) (string, error) {
	if strings.TrimSpace(configRoot) == "" {
		return "", fmt.Errorf("user configuration directory is empty")
	}
	clean := filepath.Clean(configRoot)
	if !filepath.IsAbs(clean) {
		return "", fmt.Errorf("user configuration directory is not absolute: %s", configRoot)
	}
	return filepath.Join(clean, DataDirectoryName), nil
}

// WebViewDataDirectory returns the WebView runtime storage below jeemi_data.
// Setting this path explicitly prevents Wails/WebView2 from creating a second
// %APPDATA%/<binary-name>.exe directory on Windows.
func WebViewDataDirectory() (string, error) {
	dataRoot, err := DataDirectory()
	if err != nil {
		return "", err
	}
	return WebViewDataDirectoryFromRoot(dataRoot)
}

// WebViewDataDirectoryFromRoot resolves the WebView directory for an injected
// data root. It is exported so application assembly and tests share one rule.
func WebViewDataDirectoryFromRoot(dataRoot string) (string, error) {
	clean, err := validateDataRoot(dataRoot)
	if err != nil {
		return "", err
	}
	return filepath.Join(clean, WebViewDirectoryName), nil
}

// MihomoDirectoryFromRoot returns the only managed root for downloaded mihomo
// binaries and their metadata.
func MihomoDirectoryFromRoot(dataRoot string) (string, error) {
	clean, err := validateDataRoot(dataRoot)
	if err != nil {
		return "", err
	}
	return filepath.Join(clean, "core", "mihomo"), nil
}

// GeoDataDirectoryFromRoot returns the managed root for versioned GeoIP,
// GeoSite, and ASN database revisions. Active files are materialized into the
// mihomo runtime home separately so a failed update can be rolled back without
// changing the immutable revision store.
func GeoDataDirectoryFromRoot(dataRoot string) (string, error) {
	clean, err := validateDataRoot(dataRoot)
	if err != nil {
		return "", err
	}
	return filepath.Join(clean, "geodata"), nil
}

// LocalConfigsDirectoryFromRoot returns the managed root for sparse local
// configurations. It never points into the program or build directories.
func LocalConfigsDirectoryFromRoot(dataRoot string) (string, error) {
	clean, err := validateDataRoot(dataRoot)
	if err != nil {
		return "", err
	}
	return filepath.Join(clean, "local-configs"), nil
}

// LocalScriptsDirectoryFromRoot returns the managed root for reusable local
// JavaScript transforms. Scripts are user data and never live beside the
// executable or inside an immutable subscription revision.
func LocalScriptsDirectoryFromRoot(dataRoot string) (string, error) {
	clean, err := validateDataRoot(dataRoot)
	if err != nil {
		return "", err
	}
	return filepath.Join(clean, "local-scripts"), nil
}

// LocalConfigResourcesFileFromRoot returns the Go-owned library for reusable
// local strategy groups and rule sets. It stays beside local configurations,
// never in the program directory or a subscription revision.
func LocalConfigResourcesFileFromRoot(dataRoot string) (string, error) {
	root, err := LocalConfigsDirectoryFromRoot(dataRoot)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "resources", "library.json"), nil
}

// SubscriptionsDirectoryFromRoot returns the managed root for imported
// subscription sources and their immutable revisions.
func SubscriptionsDirectoryFromRoot(dataRoot string) (string, error) {
	clean, err := validateDataRoot(dataRoot)
	if err != nil {
		return "", err
	}
	return filepath.Join(clean, "subscriptions"), nil
}

// MihomoRuntimeDirectoryFromRoot returns the managed home used for candidate
// generations, the active runtime configuration, provider caches, and
// lifecycle recovery state. It is never placed beside the executable.
func MihomoRuntimeDirectoryFromRoot(dataRoot string) (string, error) {
	clean, err := validateDataRoot(dataRoot)
	if err != nil {
		return "", err
	}
	return filepath.Join(clean, "runtime", "mihomo"), nil
}

// MihomoResolvedDirectoryFromRoot returns the managed root for long-lived,
// per-subscription resolved runtime snapshots. These snapshots deliberately
// exclude the controller address and secret used by a concrete process
// session.
func MihomoResolvedDirectoryFromRoot(dataRoot string) (string, error) {
	runtimeRoot, err := MihomoRuntimeDirectoryFromRoot(dataRoot)
	if err != nil {
		return "", err
	}
	return filepath.Join(runtimeRoot, "resolved"), nil
}

// MihomoProxyDelayCacheDirectoryFromRoot returns the managed cache for
// per-subscription proxy delay observations. It is deliberately separate
// from immutable subscription revisions and runtime session generations.
func MihomoProxyDelayCacheDirectoryFromRoot(dataRoot string) (string, error) {
	clean, err := validateDataRoot(dataRoot)
	if err != nil {
		return "", err
	}
	return filepath.Join(clean, "cache", "mihomo", "proxy-delays"), nil
}

// SettingsFileFromRoot returns Jeemi's Go-owned settings document.
func SettingsFileFromRoot(dataRoot string) (string, error) {
	clean, err := validateDataRoot(dataRoot)
	if err != nil {
		return "", err
	}
	return filepath.Join(clean, "settings.json"), nil
}

func validateDataRoot(dataRoot string) (string, error) {
	if strings.TrimSpace(dataRoot) == "" {
		return "", fmt.Errorf("data directory is empty")
	}
	clean := filepath.Clean(dataRoot)
	if !filepath.IsAbs(clean) {
		return "", fmt.Errorf("data directory is not absolute: %s", dataRoot)
	}
	return clean, nil
}
