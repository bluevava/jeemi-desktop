package application

import (
	"errors"

	"jeemi/internal/dnsquery"
)

func (s *Service) DNSQueryPreferences() (dnsquery.Preferences, error) {
	document, err := s.settingsStore.Load()
	if err != nil {
		return dnsquery.Preferences{}, errors.New("preferences_load_failed")
	}
	preferences, err := dnsquery.PreparePreferences(*document.DNSQuery)
	if err != nil {
		return dnsquery.Preferences{}, errors.New("preferences_load_failed")
	}
	return preferences, nil
}

func (s *Service) QueryDNS(input dnsquery.Request) (dnsquery.Response, error) {
	preferences, err := dnsquery.PreparePreferences(dnsquery.Preferences{CustomDNS: input.CustomDNS})
	if err != nil {
		return dnsquery.Response{}, err
	}
	// Save on the explicit query action, before validation or network IO. A
	// failed or cancelled query must not lose the submitted custom address.
	if err := s.settingsStore.SetDNSQueryPreferences(preferences); err != nil {
		return dnsquery.Response{}, errors.New("preferences_save_failed")
	}
	input.CustomDNS = preferences.CustomDNS
	ctx, cancel := s.operationContext(dnsquery.Timeout)
	defer cancel()
	return s.runtimeManager.QueryDNS(ctx, input)
}

func (s *Service) CancelDNSQuery(id string) { s.runtimeManager.CancelDNSQuery(id) }
