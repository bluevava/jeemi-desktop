package runtime

import (
	"regexp"
	"strings"
)

var (
	credentialQueryPattern = regexp.MustCompile(`(?i)([?&](?:token|secret|password|passwd|auth|key)=)[^&\s]+`)
	bearerPattern          = regexp.MustCompile(`(?i)(authorization\s*[:=]\s*bearer\s+)[^\s]+`)
	yamlSecretPattern      = regexp.MustCompile(`(?im)^(\s*(?:secret|token|password|passwd|uuid|private-key|username)\s*:\s*).+$`)
	urlUserInfoPattern     = regexp.MustCompile(`(?i)(https?://)[^/@\s]+@`)
)

// sanitizeDiagnostic keeps enough bounded core output to diagnose a failed
// start or crash without reflecting common subscription credentials or the
// controller bearer token into the renderer.
func sanitizeDiagnostic(value string) string {
	value = strings.TrimSpace(value)
	value = credentialQueryPattern.ReplaceAllString(value, `${1}<redacted>`)
	value = bearerPattern.ReplaceAllString(value, `${1}<redacted>`)
	value = yamlSecretPattern.ReplaceAllString(value, `${1}<redacted>`)
	value = urlUserInfoPattern.ReplaceAllString(value, `${1}<redacted>@`)
	if len(value) > 4096 {
		value = value[len(value)-4096:]
	}
	return strings.TrimSpace(value)
}
