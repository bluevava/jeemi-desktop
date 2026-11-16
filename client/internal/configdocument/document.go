// Package configdocument implements forward-compatible private JSON storage.
// It is deliberately separate from strict UI/import DTO decoding and YAML merging.
package configdocument

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"

	"jeemi/internal/configfile"
)

type Format struct {
	VersionField string
	Version      int
	// ArrayKeys declares stable identities; array indexes never identify objects.
	// Paths use [] for an element, for example groups[].nodes.
	ArrayKeys map[string]string
	// Rejected fields used incompatible development-era semantics. They must not
	// become silently ignored optional fields in the released format.
	Rejected  map[string]bool
	Discarded map[string]bool
	// Unknown values for semantic enums indicate a future capability, not damage.
	Enums map[string][]string
}

func (format Format) Decode(raw []byte, target any) error {
	if err := validateJSON(raw); err != nil {
		return err
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil || envelope == nil {
		return fmt.Errorf("configuration must be a JSON object")
	}
	if format.VersionField != "" {
		var version int
		if err := json.Unmarshal(envelope[format.VersionField], &version); err != nil {
			return fmt.Errorf("invalid configuration version: %w", err)
		}
		if version != format.Version {
			return configfile.UnsupportedFormat("configuration version is unsupported")
		}
	}
	projected, err := format.project(raw, reflect.TypeOf(target), "")
	if err != nil {
		return err
	}
	return json.Unmarshal(projected, target)
}

// Marshal preserves only unknown fields from the original document. Known
// fields (including deliberate deletions and empty values) come from next.
func (format Format) Marshal(original []byte, next any) ([]byte, error) {
	encoded, err := json.Marshal(next)
	if err != nil {
		return nil, err
	}
	if len(original) == 0 {
		original = []byte(`{}`)
	}
	if err := validateJSON(original); err != nil {
		return nil, err
	}
	merged, err := format.preserve(original, encoded, reflect.TypeOf(next), "")
	if err != nil {
		return nil, err
	}
	var output bytes.Buffer
	if err := json.Indent(&output, merged, "", "  "); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

// MarshalFile is called under the owning store's transaction lock. It never
// writes a file, and never treats an unreadable existing file as empty.
func (format Format) MarshalFile(path string, next any) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return format.Marshal(raw, next)
}

func objectFields(t reflect.Type) map[string]reflect.Type {
	result := map[string]reflect.Type{}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" {
			continue
		}
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if name == "-" {
			continue
		}
		if field.Anonymous && name == "" {
			for key, typ := range objectFields(indirect(field.Type)) {
				result[key] = typ
			}
			continue
		}
		if name == "" {
			name = field.Name
		}
		result[name] = field.Type
	}
	return result
}

func indirect(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}

func childPath(parent, child string) string {
	if parent == "" {
		return child
	}
	return parent + "." + child
}

func checkFeatures(fields map[string]json.RawMessage) error {
	if raw, ok := fields["requiredFeatures"]; ok {
		var features []string
		if err := json.Unmarshal(raw, &features); err != nil {
			return fmt.Errorf("invalid required configuration features")
		}
		// The initial contract implements no optional required extensions. Future
		// releases must explicitly register capabilities before accepting them.
		if len(features) != 0 {
			return configfile.UnsupportedFormat("required configuration feature is unsupported")
		}
	}
	return nil
}

func (format Format) project(raw []byte, typ reflect.Type, path string) ([]byte, error) {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		switch typ.Kind() {
		case reflect.Pointer, reflect.Map, reflect.Slice, reflect.Interface:
			return raw, nil
		default:
			return nil, fmt.Errorf("configuration field cannot be null")
		}
	}
	typ = indirect(typ)
	if options, ok := format.Enums[path]; ok {
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, err
		}
		supported := false
		for _, option := range options {
			if value == option {
				supported = true
				break
			}
		}
		if !supported {
			return nil, configfile.UnsupportedFormat("configuration option is unsupported")
		}
	}
	switch typ.Kind() {
	case reflect.Struct:
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			return nil, err
		}
		if err := checkFeatures(fields); err != nil {
			return nil, err
		}
		known := objectFields(typ)
		for name, value := range fields {
			key := childPath(path, name)
			if format.Rejected[key] {
				return nil, configfile.UnsupportedFormat("configuration field uses an unsupported format")
			}
			fieldType, ok := known[name]
			if !ok || format.Discarded[key] {
				delete(fields, name)
				continue
			}
			projected, err := format.project(value, fieldType, key)
			if err != nil {
				return nil, err
			}
			fields[name] = projected
		}
		return json.Marshal(fields)
	case reflect.Slice, reflect.Array:
		if typ.Elem().Kind() == reflect.Uint8 {
			return raw, nil
		}
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil, err
		}
		for i := range items {
			value, err := format.project(items[i], typ.Elem(), path+"[]")
			if err != nil {
				return nil, err
			}
			items[i] = value
		}
		return json.Marshal(items)
	case reflect.Map:
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			return nil, err
		}
		for key, value := range fields {
			projected, err := format.project(value, typ.Elem(), path+"{}")
			if err != nil {
				return nil, err
			}
			fields[key] = projected
		}
		return json.Marshal(fields)
	default:
		return raw, nil
	}
}

// Reject duplicate keys before either the typed or raw decoder can choose a
// different value. Errors do not include configuration values or credentials.
func validateJSON(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value func(int) error
	value = func(depth int) error {
		if depth > 100 {
			return fmt.Errorf("configuration nesting exceeds limit")
		}
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delim, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		if delim == '{' {
			seen := map[string]bool{}
			for decoder.More() {
				key, err := decoder.Token()
				if err != nil {
					return err
				}
				name, ok := key.(string)
				if !ok || seen[name] {
					return fmt.Errorf("duplicate configuration field")
				}
				seen[name] = true
				if err := value(depth + 1); err != nil {
					return err
				}
			}
		} else if delim == '[' {
			for decoder.More() {
				if err := value(depth + 1); err != nil {
					return err
				}
			}
		} else {
			return fmt.Errorf("invalid configuration delimiter")
		}
		_, err = decoder.Token()
		return err
	}
	if err := value(0); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return fmt.Errorf("configuration has trailing data")
	}
	return nil
}
