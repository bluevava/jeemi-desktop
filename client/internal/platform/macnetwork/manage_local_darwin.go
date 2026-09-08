//go:build darwin && jeemi_local_test

package macnetwork

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"jeemi/internal/platform/requirements"
)

var cdHashLine = regexp.MustCompile(`(?m)^CDHash=([a-f0-9]{40})$`)

func codeHashes(ctx context.Context, file string) ([]string, error) {
	architectures, err := codeArchitectures(file)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(architectures))
	for _, arch := range architectures {
		hash, err := codeHash(ctx, file, arch)
		if err != nil {
			return nil, err
		}
		result = append(result, hash)
	}
	return result, nil
}

func codeHash(ctx context.Context, file, arch string) (string, error) {
	text, err := exec.CommandContext(ctx, "/usr/bin/codesign", "-d", "--arch", arch, "-vvv", file).CombinedOutput()
	match := cdHashLine.FindSubmatch(text)
	if err != nil || len(match) != 2 {
		return "", Failure("local_signature_invalid")
	}
	return string(match[1]), nil
}

func InstallLocalFromBundle() error { return localManage("register") }

func localManage(action string) error {
	if os.Geteuid() == 0 || (action != "register" && action != "unregister") {
		return Failure("invalid_request")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	input := localInstallation{UID: uint32(os.Getuid())}
	// Removal uses the current bundle's verified helper for recovery, so a
	// missing/crashing installed helper is never a prerequisite for cleanup.
	{
		executable, err := os.Executable()
		if err != nil {
			return Failure("unsafe_path")
		}
		executable, err = filepath.EvalSymlinks(executable)
		if err != nil {
			return Failure("unsafe_path")
		}
		directory := filepath.Dir(executable)
		if filepath.Base(directory) != "MacOS" || filepath.Base(filepath.Dir(directory)) != "Contents" {
			return Failure("helper_missing")
		}
		gui := filepath.Join(directory, "Jeemi")
		input.Source = filepath.Join(directory, HelperName)
		for _, p := range []string{gui, input.Source} {
			info, err := os.Lstat(p)
			if err != nil || !info.Mode().IsRegular() {
				return Failure("unsafe_path")
			}
			if exec.CommandContext(ctx, "/usr/bin/codesign", "--verify", "--strict", p).Run() != nil {
				return Failure("local_signature_invalid")
			}
		}
		if action == "register" {
			input.GUI, err = codeHashes(ctx, gui)
			if err != nil {
				return err
			}
			input.Helper, err = codeHashes(ctx, input.Source)
			if err != nil {
				return err
			}
		}
		file, err := os.Open(input.Source)
		if err != nil {
			return Failure("unsafe_path")
		}
		hash := sha256.New()
		size, err := io.Copy(hash, io.LimitReader(file, 256<<20+1))
		file.Close()
		if err != nil || size > 256<<20 {
			return Failure("unsafe_path")
		}
		input.SHA256 = hex.EncodeToString(hash.Sum(nil))
	}
	script, err := localInstallScript(input, action == "unregister")
	if err != nil {
		return err
	}
	// AppleScript performs one visible administrator authorization. No password
	// is read, cached or taken from the SSH connection. The privileged shell is
	// fixed except for quoted paths and validated hashes/UID from this bundle.
	command := exec.CommandContext(ctx, "/usr/bin/osascript", "-")
	command.Stdin = strings.NewReader(authorizationAppleScript(script))
	if output, err := command.CombinedOutput(); err != nil {
		code := "unknown"
		if match := regexp.MustCompile(`\((-?[0-9]{1,6})\)`).FindSubmatch(output); len(match) == 2 {
			code = string(match[1])
		}
		if code == "-60007" {
			return Failure("local_interaction_required")
		}
		if code == "-128" || code == "-60006" {
			return Failure("cancelled")
		}
		return &requirements.Error{Code: "macos_network_local_installation_failed", Message: fmt.Sprintf("macos_network_local_installation_failed (system code %s)", code)}
	}
	nativeDisconnect()
	return nil
}

func authorizationAppleScript(script string) string {
	quoted := strings.ReplaceAll(strings.ReplaceAll(script, "\\", "\\\\"), "\"", "\\\"")
	return "do shell script \"" + quoted + "\" with administrator privileges\n"
}
