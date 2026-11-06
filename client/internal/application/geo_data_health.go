package application

import (
	"jeemi/internal/geodata"
	mihomoruntime "jeemi/internal/mihomo/runtime"
)

// Only dependency metadata is retained; status polling does not compose YAML,
// execute scripts, fetch providers, or wait for the runtime operation lock.
type runtimeGeoRequirements struct {
	preferences geodata.Preferences
	kinds       []geodata.Kind
}

func (s *Service) setDesiredGeoRequirements(requirements runtimeGeoRequirements) {
	s.mu.Lock()
	s.desiredGeoRequirements = requirements
	s.mu.Unlock()
}

func (s *Service) applyGeoRequirements() {
	s.mu.Lock()
	s.activeGeoRequirements = s.desiredGeoRequirements
	s.mu.Unlock()
}

func (s *Service) runtimeGeoDataHealth(live mihomoruntime.Status) geodata.Health {
	s.mu.RLock()
	requirements := s.desiredGeoRequirements
	if live.PID > 0 || live.State == mihomoruntime.StateRunning || live.State == mihomoruntime.StateRecovering {
		requirements = s.activeGeoRequirements
	}
	s.mu.RUnlock()
	health, err := s.geoDataManager.HealthForKinds(requirements.preferences, requirements.kinds)
	if err != nil {
		return geodata.Health{Status: geodata.HealthInvalid, MissingKinds: []geodata.Kind{}, InvalidKinds: append([]geodata.Kind{}, requirements.kinds...)}
	}
	return health
}
