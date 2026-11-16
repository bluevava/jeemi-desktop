package settings

import (
	"encoding/json"
	"fmt"
	"reflect"

	"jeemi/internal/configfile"
	"jeemi/internal/geodata"
	"jeemi/internal/runtimeconfig"
)

func validateStoredPreferences(raw []byte, document Document) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	if value, ok := fields["runtime"]; ok {
		if string(value) == "null" {
			return fmt.Errorf("runtime preferences must be an object")
		}
		// Earlier v1 writers could persist an entirely zero runtime object when
		// only selecting a core. Preserve that existing all-default meaning.
		if !reflect.DeepEqual(document.Runtime, runtimeconfig.Preferences{}) {
			if err := runtimeconfig.ValidateStored(document.Runtime); err != nil {
				return err
			}
		}
	}
	geo := document.GeoData
	if (geo.GeoIPMode != "" && geo.GeoIPMode != geodata.GeoIPModeMMDB && geo.GeoIPMode != geodata.GeoIPModeDAT) ||
		(geo.Loader != "" && geo.Loader != geodata.LoaderMemoryConservative && geo.Loader != geodata.LoaderStandard) ||
		(geo.Source != "" && geo.Source != geodata.SourceOfficial && geo.Source != geodata.SourceCustom) {
		return configfile.UnsupportedFormat("stored GEO preference is unsupported")
	}
	return nil
}
