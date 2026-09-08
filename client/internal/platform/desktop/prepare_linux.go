//go:build linux && !bindings

package desktop

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"jeemi/internal/platform/icons"
)

const managedMarker = "X-Jeemi-Managed=true"

// Prepare runs before Wails creates any windows. Registration is per-user,
// refreshes portable executable paths, and never installs an autostart or service.
func Prepare(icon []byte) error {
	prepareNativeIdentity()
	// A sudo launch must not write root-owned launchers into the user's desktop.
	if os.Geteuid() == 0 {
		return nil
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	return register(dataHome(os.Getenv("XDG_DATA_HOME"), home), executable, icon)
}

func dataHome(configured, home string) string {
	if filepath.IsAbs(configured) {
		return filepath.Clean(configured)
	}
	return filepath.Join(home, ".local", "share")
}

func register(root, executable string, source []byte) error {
	if !filepath.IsAbs(root) || !filepath.IsAbs(executable) || strings.ContainsAny(root+executable, "\x00\r\n") || strings.Contains(executable, "=") {
		return fmt.Errorf("invalid desktop registration path")
	}
	entryPath := filepath.Join(root, "applications", AppID+".desktop")
	if info, err := os.Lstat(entryPath); err == nil {
		if !info.Mode().IsRegular() || info.Size() > 64<<10 {
			return fmt.Errorf("desktop entry is not a managed regular file")
		}
		existing, err := os.ReadFile(entryPath)
		if err != nil {
			return err
		}
		if !strings.Contains("\n"+string(existing), "\n"+managedMarker+"\n") {
			return fmt.Errorf("desktop entry already exists and is not managed by Jeemi")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	icon, err := icons.ResizePNG(source, 256)
	if err != nil {
		return err
	}
	themeRoot := filepath.Join(root, "icons", "hicolor")
	iconPath := filepath.Join(themeRoot, "256x256", "apps", AppID+".png")
	changed, err := writeIfChanged(iconPath, icon)
	if err != nil {
		return fmt.Errorf("register desktop icon: %w", err)
	}
	if changed {
		// Invalidate an existing icon-theme cache without running shell utilities.
		now := time.Now()
		if err := os.Chtimes(themeRoot, now, now); err != nil {
			return err
		}
	}
	entry := "[Desktop Entry]\nType=Application\nVersion=1.0\nName=" + Name +
		"\nExec=" + desktopExec(executable) + "\nIcon=" + AppID +
		"\nTerminal=false\nStartupNotify=false\nStartupWMClass=" + AppID +
		"\nCategories=Network;\n" + managedMarker + "\n"
	_, err = writeIfChanged(entryPath, []byte(entry))
	return err
}

// Desktop entry values and Exec arguments have separate escaping layers. GIO
// checks the first executable before expanding '%%', so a fixed env executable
// preserves portable paths containing literal percentages. No shell is involved;
// paths with '=' are rejected so env cannot treat the path as an assignment.
func desktopExec(executable string) string {
	argument := strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "`", "\\`", "$", "\\$", "%", "%%").Replace(executable)
	argument = strings.NewReplacer("\\", "\\\\", "\t", "\\t").Replace(argument)
	return "/usr/bin/env -- \"" + argument + "\""
}

func writeIfChanged(path string, content []byte) (bool, error) {
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() || info.Size() > 8<<20 {
			return false, fmt.Errorf("desktop resource is not a regular file")
		}
		existing, err := os.ReadFile(path)
		if err != nil {
			return false, err
		}
		if bytes.Equal(existing, content) {
			return false, nil
		}
	} else if !os.IsNotExist(err) {
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".jeemi-*")
	if err != nil {
		return false, err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if err := file.Chmod(0o644); err != nil {
		return false, err
	}
	if _, err := file.Write(content); err != nil {
		return false, err
	}
	if err := file.Close(); err != nil {
		return false, err
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return false, err
	}
	return true, nil
}
