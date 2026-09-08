package application

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jeemi/internal/dnsquery"
	"jeemi/internal/settings"
)

func TestDNSQuerySavesOnQueryAndRestoresAfterRestartEvenWhenProxyFails(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "jeemi_data")
	service, err := NewService(directory)
	if err != nil {
		t.Fatal(err)
	}
	// Invalid direct endpoint and a stopped proxy ensure no network requests.
	response, err := service.QueryDNS(dnsquery.Request{
		ID: "query-1", Domain: "https://Example.COM:8443/private-path?token=target-secret#part",
		ProxyDNS: "1.1.1.1", DirectDNS: "invalid", CustomProxy: true,
		CustomDNS: " https://resolver.example/private?token=saved ",
	})
	if err != nil || response.Domain != "example.com" || response.Proxy.Error != "proxy_invalid" || response.Custom.Error != "proxy_invalid" || response.Direct.Error != "invalid_server" {
		t.Fatalf("QueryDNS() = %+v, %v", response, err)
	}
	reopened, err := NewService(directory)
	if err != nil {
		t.Fatal(err)
	}
	preferences, err := reopened.DNSQueryPreferences()
	if err != nil || preferences.CustomDNS != "https://resolver.example/private?token=saved" {
		t.Fatalf("restored preferences = %+v, %v", preferences, err)
	}
	contents, err := os.ReadFile(filepath.Join(directory, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, unwanted := range []string{"example.com", "private-path", "target-secret", "proxyDNS", "directDNS", "customProxy", "queriedAt"} {
		if strings.Contains(strings.ToLower(string(contents)), strings.ToLower(unwanted)) {
			t.Fatalf("saved unwanted query data: %s", unwanted)
		}
	}
}

func TestDNSQuerySavesBeforeValidationAndCancellation(t *testing.T) {
	service, err := NewService(filepath.Join(t.TempDir(), "jeemi_data"))
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ id, domain, address, code string }{
		{"invalid", "*.example.com", "8.8.8.8", "invalid_domain"},
		{"cancelled", "example.com", "9.9.9.9", "cancelled"},
	} {
		if test.code == "cancelled" {
			service.CancelDNSQuery(test.id)
		}
		_, err := service.QueryDNS(dnsquery.Request{ID: test.id, Domain: test.domain, CustomDNS: test.address, CustomProxy: true})
		if err == nil || err.Error() != test.code {
			t.Fatalf("QueryDNS() error = %v, want %s", err, test.code)
		}
		preferences, err := service.DNSQueryPreferences()
		if err != nil || preferences.CustomDNS != test.address {
			t.Fatal("a failed or cancelled query lost the submitted address")
		}
	}
}

func TestDNSQuerySettingsErrorsDoNotExposePathsOrContinueQuery(t *testing.T) {
	blocked := filepath.Join(t.TempDir(), "private-settings-location.json")
	if err := os.WriteFile(blocked, []byte("{private-credentials}"), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := settings.NewStore(blocked)
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{settingsStore: store}
	if _, err := service.DNSQueryPreferences(); err == nil || err.Error() != "preferences_load_failed" {
		t.Fatalf("load error = %v", err)
	}
	if _, err := service.QueryDNS(dnsquery.Request{CustomDNS: "1.1.1.1"}); err == nil || err.Error() != "preferences_save_failed" {
		t.Fatalf("save error = %v", err)
	}
}
