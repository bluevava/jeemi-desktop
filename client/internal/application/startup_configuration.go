package application

import (
	"jeemi/internal/chainproxy"
	"jeemi/internal/config/resources"
	"jeemi/internal/geodata"
	"jeemi/internal/platform/paths"
	"jeemi/internal/settings"
)

// CheckStartupConfiguration reads the documents needed to construct the client.
// It does not adopt GEO files, recover networking, or rewrite stored preferences.
func CheckStartupConfiguration(root string) error {
	path, err := paths.SettingsFileFromRoot(root)
	if err != nil {
		return err
	}
	store, err := settings.NewStore(path)
	if err != nil {
		return err
	}
	if _, err := store.Load(); err != nil {
		return err
	}
	library, err := resources.NewStore(resources.StoreOptions{DataDirectory: root})
	if err != nil {
		return err
	}
	if _, err := library.State(); err != nil {
		return err
	}
	chains, err := chainproxy.NewStore(root)
	if err != nil {
		return err
	}
	if _, err := chains.Load(); err != nil {
		return err
	}
	return geodata.CheckStartupConfiguration(root)
}
