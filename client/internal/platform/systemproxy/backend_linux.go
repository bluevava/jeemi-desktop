//go:build linux

package systemproxy

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"jeemi/internal/platform/requirements"
)

const gnomeSchema = "org.gnome.system.proxy"

// Only these keys are read or changed. PAC URLs and authentication credentials
// are never read, logged, or overwritten. mode is restored last.
var gnomeKeys = []string{
	"mode", "use-same-proxy", "ignore-hosts", "http/use-authentication",
	"http/host", "http/port", "https/host", "https/port",
	"ftp/host", "ftp/port", "socks/host", "socks/port",
}

const gnomeBypass = "['localhost', '127.0.0.0/8', '::1', '10.0.0.0/8', '172.16.0.0/12', '192.168.0.0/16', '169.254.0.0/16', 'fc00::/7', 'fe80::/10']"

type gnomeBackend struct {
	run     func(context.Context, ...string) (string, error)
	session func() bool
}

func newBackend() backend {
	return gnomeBackend{run: runGSettings, session: gnomeSession}
}

func gnomeSession() bool {
	if os.Geteuid() == 0 || os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		return false
	}
	for _, desktop := range strings.Split(strings.ToLower(os.Getenv("XDG_CURRENT_DESKTOP")), ":") {
		if desktop == "gnome" || desktop == "ubuntu" || desktop == "unity" {
			return true
		}
	}
	return false
}

func runGSettings(ctx context.Context, arguments ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "gsettings", arguments...)
	// Never return command output on error: desktop settings may contain private
	// hosts. Fresh processes also verify that dconf actually persisted a write.
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("gsettings operation failed; check the desktop session, writable settings and libglib2.0-bin")
	}
	if len(output) > 64<<10 {
		return "", fmt.Errorf("desktop proxy setting exceeds the size limit")
	}
	return strings.TrimSpace(string(output)), nil
}

func gnomeKey(key string) (string, string) {
	parts := strings.SplitN(key, "/", 2)
	if len(parts) == 2 {
		return gnomeSchema + "." + parts[0], parts[1]
	}
	return gnomeSchema, key
}

func (b gnomeBackend) Check(ctx context.Context) error {
	if !b.session() {
		return &requirements.Error{Code: "linux_system_proxy_unavailable", Message: "system proxy requires the current user's Ubuntu/GNOME desktop session; do not launch the GUI with sudo"}
	}
	for _, key := range gnomeKeys {
		schema, name := gnomeKey(key)
		writable, err := b.run(ctx, "writable", schema, name)
		if err != nil || writable != "true" {
			return &requirements.Error{Code: "linux_system_proxy_unavailable", Message: "GNOME proxy settings are unavailable or locked; check the session bus and libglib2.0-bin"}
		}
	}
	return nil
}

func (b gnomeBackend) Capture(ctx context.Context) (snapshot, error) {
	if err := b.Check(ctx); err != nil {
		return snapshot{}, err
	}
	values := make(map[string]string, len(gnomeKeys))
	for _, key := range gnomeKeys {
		schema, name := gnomeKey(key)
		value, err := b.run(ctx, "get", schema, name)
		if err != nil || value == "" {
			return snapshot{}, fmt.Errorf("read GNOME proxy setting %s", key)
		}
		values[key] = value
	}
	return snapshot{Backend: "gnome", DesktopValues: values}, nil
}

func (b gnomeBackend) Apply(ctx context.Context, server string) error {
	values, err := gnomeProxyValues(server)
	if err != nil {
		return err
	}
	if err := b.Check(ctx); err != nil {
		return err
	}
	return b.writeValues(ctx, values)
}

func gnomeProxyValues(server string) (map[string]string, error) {
	values := map[string]string{
		"mode": "'manual'", "use-same-proxy": "false", "ignore-hosts": gnomeBypass,
		"http/use-authentication": "false",
	}
	for _, protocol := range []string{"http", "https", "ftp", "socks"} {
		values[protocol+"/host"] = "''"
		values[protocol+"/port"] = "0"
	}
	for _, entry := range strings.Split(server, ";") {
		protocol, endpoint, ok := strings.Cut(entry, "=")
		if !ok || (protocol != "http" && protocol != "https" && protocol != "socks") {
			return nil, fmt.Errorf("invalid system proxy protocol")
		}
		host, port, err := net.SplitHostPort(endpoint)
		number, parseErr := strconv.Atoi(port)
		if err != nil || parseErr != nil || host != "127.0.0.1" || number < 1 || number > 65535 {
			return nil, fmt.Errorf("invalid system proxy endpoint")
		}
		values[protocol+"/host"] = "'127.0.0.1'"
		values[protocol+"/port"] = strconv.Itoa(number)
	}
	return values, nil
}

func (b gnomeBackend) Restore(ctx context.Context, previous snapshot) error {
	if previous.Backend != "gnome" || len(previous.DesktopValues) != len(gnomeKeys) {
		return fmt.Errorf("system proxy recovery record does not belong to the GNOME backend")
	}
	for _, key := range gnomeKeys {
		if previous.DesktopValues[key] == "" {
			return fmt.Errorf("GNOME proxy recovery record is incomplete")
		}
	}
	if err := b.Check(ctx); err != nil {
		return err
	}
	return b.writeValues(ctx, previous.DesktopValues)
}

func (b gnomeBackend) writeValues(ctx context.Context, values map[string]string) error {
	// Disable the previous mode while replacing its endpoints. Enable the new
	// mode only after every setting has been written and read back successfully.
	if err := b.set(ctx, "mode", "'none'"); err != nil {
		return err
	}
	for _, key := range gnomeKeys[1:] {
		if err := b.set(ctx, key, values[key]); err != nil {
			return err
		}
	}
	return b.set(ctx, "mode", values["mode"])
}

func (b gnomeBackend) set(ctx context.Context, key, value string) error {
	schema, name := gnomeKey(key)
	if _, err := b.run(ctx, "set", schema, name, value); err != nil {
		return fmt.Errorf("write GNOME proxy setting %s: %w", key, err)
	}
	actual, err := b.run(ctx, "get", schema, name)
	if err != nil || actual != value {
		return fmt.Errorf("GNOME proxy setting %s was not persisted", key)
	}
	return nil
}

func (gnomeBackend) Notify(context.Context) error { return nil } // GSettings emits change notifications.
