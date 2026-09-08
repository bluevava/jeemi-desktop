package desktop

import "jeemi/internal/dnsquery"

func (a *App) GetDNSQueryPreferences() (dnsquery.Preferences, error) {
	return a.service.DNSQueryPreferences()
}

func (a *App) QueryDNS(input dnsquery.Request) (dnsquery.Response, error) {
	return a.service.QueryDNS(input)
}

func (a *App) CancelDNSQuery(id string) { a.service.CancelDNSQuery(id) }
