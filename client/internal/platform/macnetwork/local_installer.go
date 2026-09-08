//go:build jeemi_local_test

package macnetwork

import (
	"bytes"
	"encoding/json"
	"path"
	"regexp"
	"strings"
	"text/template"
)

type localInstallation struct {
	UID            uint32
	Source, SHA256 string
	GUI, Helper    []string
}

func shellLiteral(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }

func localInstallScript(input localInstallation, remove bool) (string, error) {
	if !remove || input.Source != "" {
		if input.UID < 500 || !path.IsAbs(input.Source) || !strings.HasSuffix(input.Source, "/Contents/MacOS/jeemi-authorizer") || strings.ContainsAny(input.Source, "\x00\r\n") || !digestPattern.MatchString(input.SHA256) {
			return "", Failure("invalid_request")
		}
		if !remove {
			for _, hashes := range [][]string{input.GUI, input.Helper} {
				if len(hashes) < 1 || len(hashes) > 2 || (len(hashes) == 2 && hashes[0] == hashes[1]) {
					return "", Failure("invalid_request")
				}
				for _, hash := range hashes {
					if !regexp.MustCompile(`^[a-f0-9]{40}$`).MatchString(hash) {
						return "", Failure("invalid_request")
					}
				}
			}
		}
	}
	trust, _ := json.Marshal(struct {
		Version int      `json:"version"`
		UID     uint32   `json:"uid"`
		GUI     []string `json:"gui"`
		Helper  []string `json:"helper"`
	}{1, input.UID, input.GUI, input.Helper})
	helper := "/Library/PrivilegedHelperTools/" + HelperName
	plistPath := "/Library/LaunchDaemons/" + ServiceName + ".plist"
	plist := `<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><dict><key>Label</key><string>` + ServiceName + `</string><key>ProgramArguments</key><array><string>` + helper + `</string></array><key>MachServices</key><dict><key>` + ServiceName + `</key><true/></dict><key>AssociatedBundleIdentifiers</key><array><string>com.jeemi.desktop</string></array><key>RunAtLoad</key><true/><key>KeepAlive</key><dict><key>SuccessfulExit</key><false/></dict><key>AbandonProcessGroup</key><false/><key>ExitTimeOut</key><integer>15</integer><key>ThrottleInterval</key><integer>5</integer></dict></plist>`
	values := map[string]any{"Remove": remove, "HasSource": input.Source != "", "Base": shellLiteral(LocalTrustDirectory), "Helper": shellLiteral(helper), "LegacyHelper": shellLiteral("/Library/PrivilegedHelperTools/" + ServiceName), "PlistPath": shellLiteral(plistPath), "Service": shellLiteral("system/" + ServiceName), "Source": shellLiteral(input.Source), "SHA256": shellLiteral(input.SHA256), "Trust": shellLiteral(string(trust)), "Plist": shellLiteral(plist)}
	var out bytes.Buffer
	err := template.Must(template.New("install").Parse(localInstallerTemplate)).Execute(&out, values)
	return out.String(), err
}

const localInstallerTemplate = `set -eu
PATH=/usr/bin:/bin:/usr/sbin:/sbin
export PATH
umask 077
test "$(/usr/bin/id -u)" = 0
check_dir() {
 test ! -L "$1" && test -d "$1"
 test "$(/usr/bin/stat -f %u "$1")" = 0
 mode=$(/usr/bin/stat -f %Lp "$1")
 test "$((8#$mode & 022))" = 0
}
check_file() {
 test ! -L "$1"
 if test -e "$1"; then
  test -f "$1" && test "$(/usr/bin/stat -f %u "$1")" = 0
  mode=$(/usr/bin/stat -f %Lp "$1")
  test "$((8#$mode & 022))" = 0
 fi
}
for directory in / /Library '/Library/Application Support' /Library/LaunchDaemons; do check_dir "$directory"; done
for directory in /Library/PrivilegedHelperTools {{.Base}}; do
 test ! -L "$directory"
 if test ! -e "$directory"; then /bin/mkdir -m 755 "$directory"; fi
 check_dir "$directory"
done
check_file {{.Helper}}
check_file {{.LegacyHelper}}
check_file {{.PlistPath}}
check_file {{.Base}}/trust.json
check_file {{.Base}}/cleanup-complete
{{if .HasSource}}
stage=$(/usr/bin/mktemp -d {{.Base}}/.install.XXXXXX)
trap '/bin/rm -rf "$stage"' EXIT
/bin/cp {{.Source}} "$stage/helper"
test "$(/usr/bin/shasum -a 256 "$stage/helper" | /usr/bin/awk '{print $1}')" = {{.SHA256}}
/usr/bin/codesign --verify --strict "$stage/helper"
/bin/chmod 755 "$stage/helper"
{{end}}
{{if not .Remove}}
/usr/bin/printf '%s\n' {{.Trust}} > "$stage/trust.json"
/usr/bin/printf '%s\n' {{.Plist}} > "$stage/service.plist"
/usr/bin/plutil -lint "$stage/service.plist" >/dev/null
/usr/sbin/chown root:wheel "$stage/helper" "$stage/trust.json" "$stage/service.plist"
/bin/chmod 755 "$stage/helper"
/bin/chmod 644 "$stage/trust.json" "$stage/service.plist"
{{end}}
if /bin/launchctl print {{.Service}} >/dev/null 2>&1; then
 /bin/launchctl bootout {{.Service}}
 count=0
 while /bin/launchctl print {{.Service}} >/dev/null 2>&1; do
  count=$((count+1)); test "$count" -lt 20; /bin/sleep 1
 done
fi
{{if .Remove}}
if test -e {{.Base}}/runtime/network-recovery.json; then
 {{if .HasSource}}"$stage/helper" --recover-local{{else}}exit 1{{end}}
fi
/bin/rm -f {{.PlistPath}} {{.Helper}} {{.LegacyHelper}} {{.Base}}/trust.json
/usr/bin/printf 'Jeemi authorization cleanup v1\n' > {{.Base}}/cleanup-complete
/bin/chmod 644 {{.Base}}/cleanup-complete
{{else}}
/bin/rm -f {{.Base}}/cleanup-complete
/bin/mv -f "$stage/helper" {{.Helper}}
/bin/mv -f "$stage/trust.json" {{.Base}}/trust.json
/bin/mv -f "$stage/service.plist" {{.PlistPath}}
/bin/launchctl bootstrap system {{.PlistPath}}
/bin/rm -f {{.LegacyHelper}}
{{end}}
`
