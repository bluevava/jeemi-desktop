package daemon

import (
	"path/filepath"
	"strconv"
	"strings"

	"jeemi/internal/platform/macnetwork"
)

// A trusted core still parses user-controlled YAML. Root privileges must not
// make arbitrary certificate/provider/UI paths readable or writable. Sandbox
// enforcement follows resolved filesystem paths, including later symlinks.
func SandboxProfile(data, runtime, binary string) (string, error) {
	for _, path := range []string{data, runtime, binary} {
		if !filepath.IsAbs(path) || strings.ContainsAny(path, "\x00\r\n") {
			return "", macnetwork.Failure("unsafe_path")
		}
	}
	q := strconv.Quote
	return `(version 1)
(allow default)
(deny file-read-data)
(allow file-read-data (literal "/") (subpath "/System/Library")
 (subpath "/System/Volumes/Preboot/Cryptexes/OS/System/Library")
 (subpath "/System/Volumes/Preboot/Cryptexes/OS/usr/lib") (subpath "/usr/lib") (subpath "/usr/share")
 (subpath "/private/var/db/timezone")
 (literal "/private/etc/resolv.conf") (literal "/private/etc/hosts") (literal "/private/etc/localtime")
 (literal "/dev/urandom") (literal "/dev/random") (literal "/dev/null")
 (subpath ` + q(data) + `) (literal ` + q(binary) + `))
(deny file-write*)
(allow file-write* (subpath ` + q(runtime) + `) (literal "/dev/null"))
(deny process-exec)
(allow process-exec (literal ` + q(binary) + `))
(deny process-fork)
(deny mach-lookup)
(allow mach-lookup (global-name "com.apple.system.logger") (global-name "com.apple.system.opendirectoryd.libinfo")
 (global-name "com.apple.SystemConfiguration.configd") (global-name "com.apple.mDNSResponder")
 (global-name "com.apple.SecurityServer"))
(allow sysctl-read)
(deny sysctl-write)
(deny signal)
(allow signal (target self))
`, nil
}
