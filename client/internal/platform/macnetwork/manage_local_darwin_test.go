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
)

func TestLocalCodeHashReadsActualMacSignature(t *testing.T) {
	file, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "x86_64"
	}
	hash, err := codeHash(context.Background(), file, arch)
	if err != nil || len(hash) != 40 {
		t.Fatalf("could not read local code signature: %v", err)
	}
	hashes, err := codeHashes(context.Background(), file)
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
