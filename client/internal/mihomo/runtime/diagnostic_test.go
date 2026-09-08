package runtime

import (
	"strings"
	"testing"
)

func TestSanitizeDiagnosticRedactsCommonCredentials(t *testing.T) {
	input := "fetch https://alice:secret@example.test/sub?token=abc&x=1\nsecret: controller-key\nAuthorization: Bearer live-key"
	output := sanitizeDiagnostic(input)
	for _, sensitive := range []string{"alice:secret", "token=abc", "controller-key", "live-key"} {
		if strings.Contains(output, sensitive) {
			t.Fatalf("diagnostic still contains %q: %s", sensitive, output)
		}
	}
	if !strings.Contains(output, "example.test") || !strings.Contains(output, "<redacted>") {
		t.Fatalf("diagnostic lost useful context: %s", output)
	}
}
