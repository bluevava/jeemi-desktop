package core

import (
	"fmt"
	"regexp"
)

var (
	stableVersionPattern  = regexp.MustCompile(`^v[0-9]+(?:\.[0-9]+){2}$`)
	unknownVersionPattern = regexp.MustCompile(`^unknown-[a-f0-9]{16}$`)
	versionInNamePattern  = regexp.MustCompile(`(?:^|[^0-9A-Za-z])(v[0-9]+(?:\.[0-9]+){2})(?:[^0-9A-Za-z]|$)`)
)

type PlatformTarget struct {
	OS               string
	Architecture     string
	ID               string
	ArchiveExtension string
	ExecutableName   string
	assetPattern     string
}

func ResolvePlatformTarget(goos, goarch string) (PlatformTarget, error) {
	key := goos + "/" + goarch
	targets := map[string]PlatformTarget{
		"windows/amd64": {
			OS:               "windows",
			Architecture:     "amd64",
			ID:               "windows-amd64",
			ArchiveExtension: ".zip",
			ExecutableName:   "mihomo.exe",
			assetPattern:     "mihomo-windows-amd64-compatible-%s.zip",
		},
		"windows/arm64": {
			OS:               "windows",
			Architecture:     "arm64",
			ID:               "windows-arm64",
			ArchiveExtension: ".zip",
			ExecutableName:   "mihomo.exe",
			assetPattern:     "mihomo-windows-arm64-%s.zip",
		},
		"linux/amd64": {
			OS:               "linux",
			Architecture:     "amd64",
			ID:               "linux-amd64",
			ArchiveExtension: ".gz",
			ExecutableName:   "mihomo",
			assetPattern:     "mihomo-linux-amd64-compatible-%s.gz",
		},
		"linux/arm64": {
			OS:               "linux",
			Architecture:     "arm64",
			ID:               "linux-arm64",
			ArchiveExtension: ".gz",
			ExecutableName:   "mihomo",
			assetPattern:     "mihomo-linux-arm64-%s.gz",
		},
		"darwin/amd64": {
			OS:               "darwin",
			Architecture:     "amd64",
			ID:               "darwin-amd64",
			ArchiveExtension: ".gz",
			ExecutableName:   "mihomo",
			assetPattern:     "mihomo-darwin-amd64-compatible-%s.gz",
		},
		"darwin/arm64": {
			OS:               "darwin",
			Architecture:     "arm64",
			ID:               "darwin-arm64",
			ArchiveExtension: ".gz",
			ExecutableName:   "mihomo",
			assetPattern:     "mihomo-darwin-arm64-%s.gz",
		},
	}
	target, ok := targets[key]
	if !ok {
		return PlatformTarget{}, fmt.Errorf("unsupported mihomo platform: %s", key)
	}
	return target, nil
}

func (t PlatformTarget) AssetName(version string) (string, error) {
	if !stableVersionPattern.MatchString(version) {
		return "", fmt.Errorf("invalid stable mihomo version: %q", version)
	}
	return fmt.Sprintf(t.assetPattern, version), nil
}
