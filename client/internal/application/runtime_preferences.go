package application

import "jeemi/internal/runtimeconfig"

func (s *Service) RuntimePreferences() (runtimeconfig.Preferences, error) {
	document, err := s.settingsStore.Load()
	if err != nil {
		return runtimeconfig.Preferences{}, err
	}
	return document.Runtime, nil
}

func (s *Service) DefaultLANBypassRules() []string {
	return runtimeconfig.DefaultLANBypassRules()
}

func (s *Service) ValidateRuntimeYAMLFragment(input runtimeconfig.YAMLFragmentInput) runtimeconfig.YAMLFragmentValidation {
	return runtimeconfig.ValidateYAMLFragment(input)
}

func (s *Service) SaveRuntimePreferences(input runtimeconfig.Preferences) (runtimeconfig.Preferences, error) {
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()
	if err := runtimeconfig.Validate(input); err != nil {
		_ = s.failRuntimeConfiguration(reconcileTriggerRuntimePreferences, "preferences", "runtime_preferences_invalid", err, nil)
		return runtimeconfig.Preferences{}, err
	}
	previous, err := s.RuntimePreferences()
	if err != nil {
		return runtimeconfig.Preferences{}, err
	}
	document, err := s.settingsStore.SetRuntimePreferences(input)
	if err != nil {
		return runtimeconfig.Preferences{}, err
	}
	// Persisted preferences are the desired state. Activation failures are
	// reported through RuntimeStatus.Configuration while the previous healthy
	// generation remains active; they do not roll back the user's saved intent.
	_ = s.reconcileSelectedWithTimeoutLocked(reconcileTriggerRuntimePreferences, &previous, reconcileAutomatic)
	return document.Runtime, nil
}
