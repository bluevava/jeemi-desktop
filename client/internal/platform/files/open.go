package files

import (
	"fmt"
	"os"
	"path/filepath"
)

// OpenDirectory opens one already-existing absolute directory in the platform
// file manager. The frontend never supplies this path; application services
// pass a fixed Jeemi-managed directory.
func OpenDirectory(path string) error {
	clean := filepath.Clean(path)
	if path == "" || !filepath.IsAbs(clean) {
		return fmt.Errorf("directory path must be absolute")
	}
	info, err := os.Stat(clean)
	if err != nil {
		return fmt.Errorf("inspect directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory")
	}
	if err := openDirectory(clean); err != nil {
		return fmt.Errorf("open directory: %w", err)
	}
	return nil
}
