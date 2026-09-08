package application

import (
	"fmt"
	"time"

	"jeemi/internal/config/fallbackoverride"
	configinspect "jeemi/internal/config/inspect"
	"jeemi/internal/subscription"
)

type SubscriptionFallbackResult struct {
	State                 subscription.State `json:"state"`
	ConnectionResetFailed bool               `json:"connectionResetFailed"`
}

func (s *Service) SetSubscriptionFallback(id string, input fallbackoverride.Selection, expectedRevision int) (SubscriptionFallbackResult, error) {
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()
	selection, err := fallbackoverride.Normalize(input)
	if err != nil {
		return SubscriptionFallbackResult{}, err
	}
	target, err := s.subscriptions.Get(id)
	if err != nil {
		return SubscriptionFallbackResult{}, err
	}
	if target.FallbackOverrideRevision != expectedRevision {
		return SubscriptionFallbackResult{}, fmt.Errorf("subscription fallback changed; refresh before saving")
	}
	settings, err := s.settingsStore.Load()
	if err != nil {
		return SubscriptionFallbackResult{}, err
	}
	target.Fallback = selection
	composed, _, err := s.composeSubscription(target.Summary, settings.Runtime, settings.GeoData)
	if err != nil {
		return SubscriptionFallbackResult{}, err
	}
	if composed.Fallback.Reset {
		return SubscriptionFallbackResult{}, fmt.Errorf("fallback selector is unavailable in the composed subscription")
	}
	if _, err := configinspect.Configuration(composed.Contents); err != nil {
		return SubscriptionFallbackResult{}, err
	}
	reset := s.runtimeManager.PrepareFallbackConnectionReset(id, composed.Contents, settings.Subscriptions.ConnectionResetMode)
	saved, err := s.subscriptions.SetFallback(id, selection, expectedRevision, false)
	if err != nil {
		return SubscriptionFallbackResult{}, err
	}
	resetFailed := false
	if err := s.reconcileSelectedWithTimeoutLocked(reconcileTriggerSubscription, nil, reconcileAutomatic); err == nil {
		resetContext, cancel := s.operationContext(8 * time.Second)
		resetFailed = s.runtimeManager.CompleteFallbackConnectionReset(resetContext, reset, saved.FallbackOverrideRevision) != nil
		cancel()
	}
	state, err := s.SubscriptionState()
	return SubscriptionFallbackResult{State: state, ConnectionResetFailed: resetFailed}, err
}

// Called only by the serial coordinator after persisted inputs change. Pure
// getters, candidate script tests and failed downloads never enter this path.
func (s *Service) normalizeSubscriptionFallbacks() error {
	state, err := s.subscriptions.State()
	if err != nil {
		return err
	}
	settings, err := s.settingsStore.Load()
	if err != nil {
		return err
	}
	for _, summary := range state.Subscriptions {
		if summary.Fallback.Mode != fallbackoverride.ModeSelector {
			continue
		}
		composed, _, err := s.composeSubscription(summary, settings.Runtime, settings.GeoData)
		if err != nil || !composed.Fallback.Reset {
			continue
		}
		if _, err := configinspect.Configuration(composed.Contents); err != nil {
			continue
		}
		if _, err := s.subscriptions.SetFallback(summary.ID, composed.Fallback.Selection, summary.FallbackOverrideRevision, true); err != nil {
			return err
		}
	}
	return nil
}
