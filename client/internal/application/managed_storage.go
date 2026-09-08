package application

// ManagedStorageDirectories exposes only Jeemi-owned domain roots needed by
// read-only settings views. It does not grant arbitrary filesystem access.
type ManagedStorageDirectories struct {
	Subscriptions string `json:"subscriptions"`
	LocalConfigs  string `json:"localConfigs"`
	LocalScripts  string `json:"localScripts"`
}

func (s *Service) ManagedStorageDirectories() ManagedStorageDirectories {
	return ManagedStorageDirectories{
		Subscriptions: s.subscriptions.Directory(),
		LocalConfigs:  s.localConfigs.Directory(),
		LocalScripts:  s.localScripts.Directory(),
	}
}
