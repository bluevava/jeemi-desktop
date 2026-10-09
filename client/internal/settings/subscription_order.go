package settings

import "fmt"

func (s *Store) SetSubscriptionOrder(ids []string) error {
	if err := validateSubscriptionOrder(ids); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	document, err := s.loadUnlocked()
	if err != nil {
		return err
	}
	document.Subscriptions.Order = append([]string(nil), ids...)
	return s.saveUnlocked(document)
}

func validateSubscriptionOrder(ids []string) error {
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if !subscriptionIDPattern.MatchString(id) || seen[id] {
			return fmt.Errorf("subscription order contains invalid or duplicate IDs")
		}
		seen[id] = true
	}
	return nil
}
