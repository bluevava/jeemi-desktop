package runtimeoverride

import (
	"strings"
	"testing"

	"jeemi/internal/config/document"
	"jeemi/internal/runtimeconfig"
)

func TestExternalUIAlwaysOverridesSourceWithoutMutatingIt(t *testing.T) {
	source := []byte("external-ui: /unmanaged/ui\nexternal-ui-url: https://example.test/unmanaged.zip\nexternal-ui-name: custom\nkeep-alive-idle: 30\n")
	preferences := runtimeconfig.DefaultPreferences()
	for _, enabled := range []bool{false, true} {
		preferences.ExternalUIEnabled = enabled
		preferences.ExternalUIVersion = "v3.26.0"
		result, err := Apply(source, preferences)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(source), "/unmanaged/ui") || strings.Contains(string(result), "unmanaged") || strings.Contains(string(result), "external-ui-name") {
			t.Fatal("source override failed")
		}
		parsed, _ := document.Parse(result)
		assertValue(t, parsed, "/keep-alive-idle", "30")
		if enabled {
			assertValue(t, parsed, "/external-ui", "external-ui/zashboard/v3.26.0/dist")
			assertValue(t, parsed, "/external-ui-url", "https://github.com/Zephyruso/zashboard/releases/download/v3.26.0/dist.zip")
		} else if strings.Contains(string(result), "external-ui") {
			t.Fatal("disabled external UI survived")
		}
	}
}
