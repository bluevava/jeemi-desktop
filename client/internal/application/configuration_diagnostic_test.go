package application

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestCandidateDiagnosticRedactsSecretsAndKeepsLocation(t *testing.T) {
	err := errors.New("rules[3]: URL https://example.test/sub?token=private-url; password: private-password; secret=private-secret; Authorization: Bearer private-bearer")
	message := safeConfigurationError(err)
	if !strings.Contains(message, "rules[3]") || strings.Contains(message, "private-") || strings.Contains(message, "example.test") {
		t.Fatal("diagnostic lost its location or exposed a credential")
	}
	message = safeConfigurationError(errors.New(strings.Repeat("配置错误", 300)))
	if len(message) > 1024 || !utf8.ValidString(message) {
		t.Fatal("diagnostic truncation broke UTF-8 or exceeded its bound")
	}
}
