//go:build jeemi_local_test

package macnetwork

import (
	"strings"
	"testing"
)

func localFixture() localInstallation {
	return localInstallation{UID: 501, Source: "/Users/test/App's $(touch nope)/Jeemi.app/Contents/MacOS/jeemi-authorizer", SHA256: strings.Repeat("a", 64), GUI: []string{strings.Repeat("a", 40), strings.Repeat("b", 40)}, Helper: []string{strings.Repeat("c", 40), strings.Repeat("d", 40)}}
}

func TestLocalInstallerAcceptsThinAndUniversalSignatures(t *testing.T) {
	for _, count := range []int{1, 2} {
		input := localFixture()
		input.GUI, input.Helper = input.GUI[:count], input.Helper[:count]
		script, err := localInstallScript(input, false)
		if err != nil {
			t.Fatalf("%d signature(s): %v", count, err)
		}
		for _, hashes := range [][]string{input.GUI, input.Helper} {
			for _, hash := range hashes {
				if !strings.Contains(script, hash) {
					t.Fatal("installer omitted a present architecture's signature")
				}
			}
		}
	}
	for _, hashes := range [][]string{nil, {strings.Repeat("a", 40), strings.Repeat("a", 40)}, {strings.Repeat("a", 40), strings.Repeat("b", 40), strings.Repeat("c", 40)}, {strings.Repeat("z", 40)}, {strings.Repeat("a", 39)}} {
		for _, gui := range []bool{true, false} {
			input := localFixture()
			if gui {
				input.GUI = hashes
			} else {
				input.Helper = hashes
			}
			if _, err := localInstallScript(input, false); err == nil {
				t.Fatal("invalid signature allowlist accepted")
			}
		}
	}
}

func TestLocalInstallerQuotesBundleAndValidatesBeforeReplacing(t *testing.T) {
	input := localFixture()
	script, err := localInstallScript(input, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(script, "/bin/cp "+shellLiteral(input.Source)+" ") {
		t.Fatal("bundle path not shell quoted")
	}
	verify := strings.Index(script, "/usr/bin/codesign --verify --strict")
	stop := strings.Index(script, "/bin/launchctl bootout")
	replace := strings.Index(script, "/bin/mv -f")
	if verify < 0 || verify > stop || stop > replace {
		t.Fatal("unverified input can replace the helper")
	}
	if !strings.Contains(script, `"uid":501`) || !strings.Contains(script, "check_file") || !strings.Contains(script, "test ! -L") {
		t.Fatal("missing owner/identity checks")
	}
	for _, value := range []localInstallation{{UID: 0}, {UID: 501, Source: input.Source, SHA256: input.SHA256, GUI: []string{";id"}, Helper: input.Helper}, {UID: 501, Source: "relative/Contents/MacOS/jeemi-authorizer", SHA256: input.SHA256, GUI: input.GUI, Helper: input.Helper}} {
		if _, err := localInstallScript(value, false); err == nil {
			t.Fatal("unsafe installer input accepted")
		}
	}
}
func TestLocalRemovalRetainsRecoveryAndHasNoRecursiveRuntimeDeletion(t *testing.T) {
	script, err := localInstallScript(localInstallation{}, true)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(script, "rm -rf") || strings.Contains(script, "bootstrap system") {
		t.Fatal("removal can discard recovery or start helper")
	}
	if !strings.Contains(script, "bootout") || !strings.Contains(script, "/trust.json") {
		t.Fatal("removal incomplete")
	}
}

func TestRemovalUsesFreshVerifiedRecoveryBeforeDeletingRegistration(t *testing.T) {
	script, err := localInstallScript(localFixture(), true)
	if err != nil {
		t.Fatal(err)
	}
	verify := strings.Index(script, "/usr/bin/codesign --verify")
	stop := strings.Index(script, "/bin/launchctl bootout")
	recover := strings.Index(script, "--recover-local")
	remove := strings.Index(script, "/bin/rm -f '/Library/LaunchDaemons/")
	if verify < 0 || stop < verify || recover < stop || remove < recover {
		t.Fatal("unsafe cleanup order")
	}
	if strings.Contains(script, "/bin/mv -f") || strings.Contains(script, "bootstrap system") {
		t.Fatal("cleanup reinstalls helper")
	}
}

func TestLocalHelperRenameKeepsServiceIdentityAndRecoveryDirectory(t *testing.T) {
	script, err := localInstallScript(localFixture(), false)
	if err != nil {
		t.Fatal(err)
	}
	if HelperName != "jeemi-authorizer" || !strings.Contains(script, "<string>/Library/PrivilegedHelperTools/jeemi-authorizer</string>") {
		t.Fatal("helper basename not unified")
	}
	oldHelper := shellLiteral("/Library/PrivilegedHelperTools/" + ServiceName)
	cleanup := strings.Index(script, "/bin/rm -f "+oldHelper)
	bootstrap := strings.Index(script, "/bin/launchctl bootstrap system")
	if cleanup < bootstrap || bootstrap < 0 {
		t.Fatal("legacy binary removed before replacement was registered")
	}
	if !strings.Contains(script, LocalTrustDirectory) {
		t.Fatal("recovery directory changed")
	}
}
