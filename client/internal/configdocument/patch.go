package configdocument

import (
	"encoding/json"
	"os"
	"strings"
)

// PatchFile changes only explicitly owned paths. Defaulted display values from
// the read model must not overwrite unrelated stored values during another edit.
func (format Format) PatchFile(path string, next any, fields ...string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	merged, err := format.Marshal(raw, next)
	if err != nil || len(raw) == 0 || len(fields) == 0 {
		return merged, err
	}
	var before, after map[string]json.RawMessage
	if err := json.Unmarshal(raw, &before); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(merged, &after); err != nil {
		return nil, err
	}
	for _, path := range fields {
		if err := patch(before, after, strings.Split(path, ".")); err != nil {
			return nil, err
		}
	}
	return json.MarshalIndent(before, "", "  ")
}

func patch(before, after map[string]json.RawMessage, path []string) error {
	key := path[0]
	if len(path) == 1 {
		if value, ok := after[key]; ok {
			before[key] = value
		} else {
			delete(before, key)
		}
		return nil
	}
	left, right := map[string]json.RawMessage{}, map[string]json.RawMessage{}
	if raw, ok := before[key]; ok && string(raw) != "null" {
		if err := json.Unmarshal(raw, &left); err != nil {
			return err
		}
	}
	if raw, ok := after[key]; ok {
		if err := json.Unmarshal(raw, &right); err != nil {
			return err
		}
	}
	if err := patch(left, right, path[1:]); err != nil {
		return err
	}
	raw, err := json.Marshal(left)
	before[key] = raw
	return err
}
