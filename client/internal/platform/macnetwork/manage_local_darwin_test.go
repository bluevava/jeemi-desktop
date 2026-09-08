//go:build darwin && jeemi_local_test

package macnetwork

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestLocalCodeHashReadsActualMacSignature(t *testing.T) {
	file, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	// Intel Go test binaries need not be signed. Prepare a signed fixture on
	// both architectures without changing the running test binary or bundle.
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	file = filepath.Join(t.TempDir(), "signed-test-fixture")
	if err := os.WriteFile(file, data, 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	if output, err := exec.CommandContext(ctx, "/usr/bin/codesign", "--force", "--sign", "-", "--timestamp=none", file).CombinedOutput(); err != nil {
		t.Fatalf("could not ad-hoc sign test fixture: %v %s", err, output)
	}
	if output, err := exec.CommandContext(ctx, "/usr/bin/codesign", "--verify", "--strict", file).CombinedOutput(); err != nil {
		t.Fatalf("invalid test fixture signature: %v %s", err, output)
	}
	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "x86_64"
	}
	hash, err := codeHash(ctx, file, arch)
	if err != nil || len(hash) != 40 {
		t.Fatalf("could not read local code signature: %v", err)
	}
	hashes, err := codeHashes(ctx, file)
	if err != nil || len(hashes) != 1 || hashes[0] != hash {
		t.Fatalf("thin native test binary signatures = %v, %v", hashes, err)
	}
	input := localFixture()
	input.GUI, input.Helper = hashes, hashes
	if _, err := localInstallScript(input, false); err != nil {
		t.Fatalf("single-architecture signatures cannot reach administrator authorization: %v", err)
	}
}

func TestLocalInstallerShellSyntaxWithoutExecuting(t *testing.T) {
	for _, remove := range []bool{false, true} {
		script, err := localInstallScript(localFixture(), remove)
		if err != nil {
			t.Fatal(err)
		}
		command := exec.Command("/bin/sh", "-n")
		command.Stdin = strings.NewReader(script)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("invalid installer syntax: %v %s", err, output)
		}
		compiler := exec.Command("/usr/bin/osacompile", "-o", filepath.Join(t.TempDir(), "installer.scpt"), "-")
		compiler.Stdin = strings.NewReader(authorizationAppleScript(script))
		if output, err := compiler.CombinedOutput(); err != nil {
			t.Fatalf("invalid authorization script: %v %s", err, output)
		}
	}
}
