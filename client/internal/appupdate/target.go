package appupdate

import (
	"errors"
	"path/filepath"
	"regexp"
)

var (
	ErrAssetUnavailable = errors.New("jeemi_update_asset_unavailable")
	ErrBusy             = errors.New("jeemi_update_busy")
	ErrExpired          = errors.New("jeemi_update_check_again")
	ErrDownload         = errors.New("jeemi_update_download_failed")
	ErrChecksum         = errors.New("jeemi_update_checksum_failed")
	ErrPackage          = errors.New("jeemi_update_package_invalid")
	ErrLocation         = errors.New("jeemi_update_location_unwritable")
	ErrRestart          = errors.New("jeemi_update_restart_failed")
	ErrReplace          = errors.New("jeemi_update_replace_failed")
	ErrRollback         = errors.New("jeemi_update_rollback_failed")
	ErrCancelled        = errors.New("jeemi_update_cancelled")
	ErrStop             = errors.New("jeemi_update_stop_failed")
	digestPattern       = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	idPattern           = regexp.MustCompile(`^[0-9a-f]{32}$`)
)

const maxArchiveBytes int64 = 512 << 20
const maxExtractBytes int64 = 2 << 30

type target struct {
	platform, architecture, directory, extension, executable, helper string
}

func targetFor(goos, arch string) (target, error) {
	if arch != "amd64" && arch != "arm64" {
		return target{}, ErrAssetUnavailable
	}
	switch goos {
	case "windows":
		return target{"windows", arch, "Windows-" + arch, ".zip", "Jeemi.exe", "jeemi-authorizer.exe"}, nil
	case "linux":
		return target{"linux", arch, "Linux-" + arch, ".tar.gz", "Jeemi", "jeemi-authorizer"}, nil
	case "darwin":
		return target{"macos", arch, "macOS-" + arch, ".zip", "Jeemi.app/Contents/MacOS/Jeemi", "Jeemi.app/Contents/MacOS/jeemi-authorizer"}, nil
	default:
		return target{}, ErrAssetUnavailable
	}
}

func (t target) installItems() []string {
	if t.platform == "macos" {
		return []string{"Jeemi.app"}
	}
	return []string{t.executable, t.helper, "release.json", "SHA256SUMS"}
}

func (t target) installRoot(executable string) (string, error) {
	if !filepath.IsAbs(executable) {
		return "", ErrLocation
	}
	root := filepath.Dir(executable)
	if t.platform == "macos" {
		root = filepath.Dir(filepath.Dir(filepath.Dir(root)))
	}
	if filepath.Clean(filepath.Join(root, filepath.FromSlash(t.executable))) != filepath.Clean(executable) {
		return "", ErrLocation
	}
	return root, nil
}
