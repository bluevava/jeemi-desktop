package application

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

var configurationDiagnosticURL = regexp.MustCompile(`(?i)(?:https?|wss?)://[^\s"'<>]+`)
var configurationDiagnosticSecret = regexp.MustCompile(`(?im)((?:"|')?(?:secret|token|password|passwd|uuid|private-key|username)(?:"|')?\s*[:=]\s*)(?:"[^"\r\n]*"|'[^'\r\n]*'|[^\s,}]+)`)
var configurationDiagnosticBearer = regexp.MustCompile(`(?i)(bearer\s+)[^\s,}]+`)

// Candidate failures reach editable UI as bounded diagnostics, never YAML or
// a fetched subscription URL. Keep locations and stages useful for repair.
func safeConfigurationError(err error) string {
	if err == nil {
		return "runtime configuration operation failed"
	}
	message := strings.TrimSpace(err.Error())
	message = configurationDiagnosticURL.ReplaceAllString(message, "<redacted-url>")
	message = configurationDiagnosticSecret.ReplaceAllString(message, "${1}<redacted>")
	message = configurationDiagnosticBearer.ReplaceAllString(message, "${1}<redacted>")
	if len(message) > 1024 {
		message = message[:1024]
		for !utf8.ValidString(message) {
			message = message[:len(message)-1]
		}
	}
	return message
}
