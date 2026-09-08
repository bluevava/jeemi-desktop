package application

import (
	"fmt"
	"os"
	"time"

	"jeemi/internal/geodata"
	platformfiles "jeemi/internal/platform/files"
)

func (s *Service) GeoDataState() (geodata.State, error) {
	document, err := s.settingsStore.Load()
	if err != nil {
		return geodata.State{}, err
	}
	return s.geoDataManager.State(document.GeoData)
}

func (s *Service) SaveGeoDataPreferences(preferences geodata.Preferences) (geodata.State, error) {
	preferences = geodata.Normalize(preferences)
	if err := geodata.ValidatePreferences(preferences); err != nil {
		return geodata.State{}, err
	}
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()
	document, err := s.settingsStore.SetGeoDataPreferences(preferences)
	if err != nil {
		return geodata.State{}, err
	}
	// The saved settings are desired state. Running cores keep their current
	// memory-mapped files until an explicit restart; stopped cores activate a
	// revision transaction only while rebuilding and validating their snapshot.
	_ = s.reconcileSelectedWithTimeoutLocked(reconcileTriggerGeoData, nil, reconcileAutomatic)
	return s.geoDataManager.State(document.GeoData)
}

func (s *Service) CheckGeoDataUpdates() (geodata.State, error) {
	document, err := s.settingsStore.Load()
	if err != nil {
		return geodata.State{}, err
	}
	ctx, cancel := s.operationContext(45 * time.Second)
	defer cancel()
	return s.geoDataManager.Check(ctx, document.GeoData)
}

func (s *Service) DownloadGeoData(kind string) (geodata.State, error) {
	installation, err := s.selectedMihomoInstallation()
	if err != nil {
		return geodata.State{}, err
	}
	document, err := s.settingsStore.Load()
	if err != nil {
		return geodata.State{}, err
	}
	ctx, cancel := s.operationContext(15 * time.Minute)
	defer cancel()
	if _, err := s.geoDataManager.Download(ctx, geodata.Kind(kind), document.GeoData, installation.ExecutablePath, installation.Version); err != nil {
		return geodata.State{}, err
	}
	return s.reconcileAndReadGeoData(document.GeoData)
}

func (s *Service) DownloadAllGeoData() (geodata.State, error) {
	installation, err := s.selectedMihomoInstallation()
	if err != nil {
		return geodata.State{}, err
	}
	document, err := s.settingsStore.Load()
	if err != nil {
		return geodata.State{}, err
	}
	ctx, cancel := s.operationContext(30 * time.Minute)
	defer cancel()
	if _, err := s.geoDataManager.DownloadAll(ctx, document.GeoData, installation.ExecutablePath, installation.Version); err != nil {
		return geodata.State{}, err
	}
	return s.reconcileAndReadGeoData(document.GeoData)
}

func (s *Service) CancelGeoDataDownload() {
	s.geoDataManager.CancelDownload()
}

func (s *Service) ImportGeoData(kind, path string) (geodata.State, error) {
	installation, err := s.selectedMihomoInstallation()
	if err != nil {
		return geodata.State{}, err
	}
	document, err := s.settingsStore.Load()
	if err != nil {
		return geodata.State{}, err
	}
	ctx, cancel := s.operationContext(2 * time.Minute)
	defer cancel()
	if _, err := s.geoDataManager.Import(ctx, geodata.Kind(kind), path, installation.ExecutablePath, installation.Version, document.GeoData); err != nil {
		return geodata.State{}, err
	}
	return s.reconcileAndReadGeoData(document.GeoData)
}

func (s *Service) ApplyGeoDataUpdates() (RuntimeStatus, error) {
	return s.runAuthorizedProxy(reconcileTriggerGeoData, reconcileRestart)
}

func (s *Service) CleanOldGeoDataRevisions() (geodata.State, error) {
	document, err := s.settingsStore.Load()
	if err != nil {
		return geodata.State{}, err
	}
	return s.geoDataManager.CleanOldRevisions(document.GeoData)
}

func (s *Service) OpenGeoDataDirectory() error {
	directory := s.geoDataManager.Directory()
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create GEO data directory: %w", err)
	}
	return platformfiles.OpenDirectory(directory)
}

func (s *Service) reconcileAndReadGeoData(preferences geodata.Preferences) (geodata.State, error) {
	s.reconcileMu.Lock()
	_ = s.reconcileSelectedWithTimeoutLocked(reconcileTriggerGeoData, nil, reconcileAutomatic)
	s.reconcileMu.Unlock()
	return s.geoDataManager.State(preferences)
}
