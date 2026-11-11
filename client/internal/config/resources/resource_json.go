package resources

import (
	"bytes"
	"encoding/json"
)

// Descriptions have been removed from resource metadata. Accept that one
// obsolete display field on input, without restoring it in the model or output.
// All other unknown fields are still rejected. Reads never rewrite a file.
func decodeResource(data []byte, target any) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	delete(fields, "description")
	clean, err := json.Marshal(fields)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(clean))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func (value *StrategyGroup) UnmarshalJSON(data []byte) error {
	type plain StrategyGroup
	var result plain
	if err := decodeResource(data, &result); err != nil {
		return err
	}
	*value = StrategyGroup(result)
	return nil
}

func (value *RuleSet) UnmarshalJSON(data []byte) error {
	type plain RuleSet
	var result plain
	if err := decodeResource(data, &result); err != nil {
		return err
	}
	*value = RuleSet(result)
	return nil
}
