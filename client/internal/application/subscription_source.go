package application

func (s *Service) SubscriptionSourceAddress(id string) (string, error) {
	return s.subscriptions.SourceAddress(id)
}
