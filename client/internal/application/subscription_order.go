package application

import "fmt"

// ReorderSubscriptions changes presentation order only. It must not reconcile
// runtime configuration or change the selected subscription.
func (s *Service) ReorderSubscriptions(ids []string) error {
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()
	state, err := s.subscriptions.State()
	if err != nil {
		return err
	}
	if len(ids) != len(state.Subscriptions) {
		return fmt.Errorf("subscription list changed; reload before reordering")
	}
	remaining := make(map[string]bool, len(state.Subscriptions))
	for _, item := range state.Subscriptions {
		remaining[item.ID] = true
	}
	for _, id := range ids {
		if !remaining[id] {
			return fmt.Errorf("subscription order must contain every current ID exactly once")
		}
		delete(remaining, id)
	}
	return s.settingsStore.SetSubscriptionOrder(ids)
}
