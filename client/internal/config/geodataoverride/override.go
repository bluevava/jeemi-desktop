package geodataoverride

import (
	"fmt"

	"jeemi/internal/config/document"
	"jeemi/internal/geodata"

	"gopkg.in/yaml.v3"
)

var managedPaths = []string{
	"/geodata-mode",
	"/geodata-loader",
	"/geo-auto-update",
	"/geo-update-interval",
	"/geox-url",
}

func ManagedPaths() []string {
	return append([]string{}, managedPaths...)
}

// Apply gives Jeemi sole ownership of mihomo's GEO loading policy. Database
// downloads are performed by the independent GEO manager, so mihomo's own
// updater and source map are removed from the final configuration.
func Apply(configurationYAML []byte, preferences geodata.Preferences) ([]byte, error) {
	preferences = geodata.Normalize(preferences)
	if err := geodata.ValidatePreferences(preferences); err != nil {
		return nil, fmt.Errorf("validate GEO data preferences: %w", err)
	}
	configuration, err := document.Parse(configurationYAML)
	if err != nil {
		return nil, fmt.Errorf("parse composed configuration: %w", err)
	}
	root := document.Root(configuration)
	for _, path := range []string{"/geo-update-interval", "/geox-url"} {
		if err := document.DeleteMappingPath(root, path); err != nil {
			return nil, err
		}
	}
	mode := preferences.GeoIPMode == geodata.GeoIPModeDAT
	values := []struct {
		path  string
		value *yaml.Node
	}{
		{path: "/geodata-mode", value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: fmt.Sprintf("%t", mode)}},
		{path: "/geodata-loader", value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: preferences.Loader}},
		{path: "/geo-auto-update", value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "false"}},
	}
	for _, item := range values {
		if err := document.SetMappingPath(root, item.path, item.value); err != nil {
			return nil, fmt.Errorf("inject GEO data field %s: %w", item.path, err)
		}
	}
	return document.Encode(configuration)
}
