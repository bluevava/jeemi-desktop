package settings

import "jeemi/internal/dnsquery"

func (s *Store) SetDNSQueryPreferences(input dnsquery.Preferences) error {
	preferences, err := dnsquery.PreparePreferences(input)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	document, err := s.loadUnlocked()
	if err != nil {
		return err
	}
	if document.DNSQuery != nil && *document.DNSQuery == preferences {
		return nil
	}
	document.DNSQuery = &preferences
	return s.saveUnlocked(document)
}
