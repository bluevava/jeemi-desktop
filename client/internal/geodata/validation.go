package geodata

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	mihomoruntime "jeemi/internal/mihomo/runtime"
)

type coreValidator struct {
	stagingRoot string
}

func (validator coreValidator) Validate(ctx context.Context, executablePath string, kind Kind, candidatePath string) error {
	if err := os.MkdirAll(validator.stagingRoot, 0o700); err != nil {
		return fmt.Errorf("create GEO validation directory: %w", err)
	}
	home, err := os.MkdirTemp(validator.stagingRoot, ".validation-")
	if err != nil {
		return fmt.Errorf("create GEO validation home: %w", err)
	}
	defer os.RemoveAll(home)
	name, err := CanonicalName(kind)
	if err != nil {
		return err
	}
	if err := copyFile(candidatePath, filepath.Join(home, name), maxAssetBytes); err != nil {
		return fmt.Errorf("stage GEO validation asset: %w", err)
	}
	configuration := probeConfiguration(kind)
	configurationPath := filepath.Join(home, "config.yaml")
	if err := os.WriteFile(configurationPath, configuration, 0o600); err != nil {
		return fmt.Errorf("write GEO validation configuration: %w", err)
	}
	return mihomoruntime.ValidateConfigurationAtHome(ctx, executablePath, home, configurationPath)
}

func probeConfiguration(kind Kind) []byte {
	mode := "false"
	rule := "GEOIP,CN,DIRECT,no-resolve"
	switch kind {
	case KindGeoIPDAT:
		mode = "true"
	case KindGeoSite:
		mode = "true"
		rule = "GEOSITE,CN,DIRECT"
	case KindASN:
		rule = "IP-ASN,4134,DIRECT,no-resolve"
	}
	return []byte("mode: rule\nlog-level: silent\ngeodata-mode: " + mode + "\ngeodata-loader: memconservative\nrules:\n  - " + rule + "\n  - MATCH,DIRECT\n")
}
