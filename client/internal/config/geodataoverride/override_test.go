package geodataoverride

import (
	"strings"
	"testing"

	"jeemi/internal/geodata"
)

func TestApplyOwnsGeoFields(t *testing.T) {
	input := []byte("geodata-mode: true\ngeodata-loader: standard\ngeo-auto-update: true\ngeo-update-interval: 2\ngeox-url:\n  geoip: https://example.invalid/GeoIP.dat\nrules:\n  - MATCH,DIRECT\n")
	preferences := geodata.DefaultPreferences()
	output, err := Apply(input, preferences)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	text := string(output)
	for _, expected := range []string{"geodata-mode: false", "geodata-loader: memconservative", "geo-auto-update: false"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("Apply() output does not contain %q:\n%s", expected, text)
		}
	}
	for _, removed := range []string{"geo-update-interval", "geox-url"} {
		if strings.Contains(text, removed) {
			t.Fatalf("Apply() output retained %q:\n%s", removed, text)
		}
	}
}

func TestApplyDATMode(t *testing.T) {
	preferences := geodata.DefaultPreferences()
	preferences.GeoIPMode = geodata.GeoIPModeDAT
	preferences.Loader = geodata.LoaderStandard
	output, err := Apply([]byte("rules:\n  - MATCH,DIRECT\n"), preferences)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if !strings.Contains(string(output), "geodata-mode: true") || !strings.Contains(string(output), "geodata-loader: standard") {
		t.Fatalf("Apply() output = %s", output)
	}
}
