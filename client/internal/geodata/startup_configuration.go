package geodata

import "jeemi/internal/platform/paths"

// CheckStartupConfiguration validates only the persisted manifest. In particular
// it must not adopt runtime assets before the user has approved startup recovery.
func CheckStartupConfiguration(root string) error {
	directory, err := paths.GeoDataDirectoryFromRoot(root)
	if err != nil {
		return err
	}
	manager := &Manager{directory: directory}
	_, err = manager.loadManifest()
	return err
}
