package subscription

import (
	"encoding/json"

	"jeemi/internal/config/fallbackoverride"
	"jeemi/internal/configfile"
)

// CheckStartupConfiguration uses the same readers as the subscription list,
// without fetching subscriptions, creating directories, or rewriting metadata.
func CheckStartupConfiguration(root string) error {
	store, err := NewStore(StoreOptions{DataDirectory: root})
	if err != nil {
		return err
	}
	_, err = store.State()
	return err
}

func invalidMetadata(path string, contents []byte, id string, cause error) error {
	repaired := repairMetadata(contents, id)
	if repaired == nil {
		return configfile.Invalid(path, contents, cause)
	}
	return configfile.InvalidWithRepair(path, contents, repaired, cause)
}

// Only the current metadata schema is recoverable. Unknown fields are removed
// in an explicit, confirmed repair, never silently during ordinary reads.
// Source identity, revisions, associations and valid preferences must survive
// the complete current-schema validation before a repair can be offered.
func repairMetadata(contents []byte, id string) []byte {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(contents, &fields); err != nil || fields == nil {
		return nil
	}
	// An incompatible optional interval can safely use the current default.
	// Decode it separately so a wrong JSON type does not destroy the source URL.
	var interval int
	_ = json.Unmarshal(fields["autoUpdateIntervalMinutes"], &interval)
	delete(fields, "autoUpdateIntervalMinutes")
	known, err := json.Marshal(fields)
	if err != nil {
		return nil
	}
	var metadata metadataDocument
	if err := json.Unmarshal(known, &metadata); err != nil {
		return nil
	}
	if metadata.ManifestVersion != manifestVersion || metadata.ID != id {
		return nil
	}
	if metadata.SourceKind == SourceFile {
		interval = 0
	}
	metadata.AutoUpdateIntervalMinutes, err = normalizeAutoUpdateInterval(metadata.SourceKind, interval)
	if err != nil {
		metadata.AutoUpdateIntervalMinutes = DefaultAutoUpdateIntervalMinutes
	}
	if metadata.Fallback == (fallbackoverride.Selection{}) {
		metadata.Fallback = fallbackoverride.Selection{Mode: fallbackoverride.ModeNone}
	}
	if err := validateStoredMetadata(metadata, id); err != nil {
		return nil
	}
	repaired, err := marshalMetadata(metadata)
	if err != nil {
		return nil
	}
	return repaired
}
