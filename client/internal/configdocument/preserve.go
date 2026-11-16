package configdocument

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
)

func (format Format) preserve(old, next []byte, typ reflect.Type, path string) ([]byte, error) {
	typ = indirect(typ)
	if bytes.Equal(bytes.TrimSpace(next), []byte("null")) {
		return next, nil
	}
	switch typ.Kind() {
	case reflect.Struct:
		var before, after map[string]json.RawMessage
		if err := json.Unmarshal(old, &before); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(next, &after); err != nil {
			return nil, err
		}
		known := objectFields(typ)
		for key, value := range before {
			fieldType, ok := known[key]
			fieldPath := childPath(path, key)
			if format.Discarded[fieldPath] {
				continue
			}
			if !ok {
				after[key] = value
				continue
			}
			if changed, exists := after[key]; exists {
				merged, err := format.preserve(value, changed, fieldType, fieldPath)
				if err != nil {
					return nil, err
				}
				after[key] = merged
			}
		}
		return json.Marshal(after)
	case reflect.Map:
		var before, after map[string]json.RawMessage
		if err := json.Unmarshal(old, &before); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(next, &after); err != nil {
			return nil, err
		}
		for key, value := range after {
			if prior, ok := before[key]; ok {
				merged, err := format.preserve(prior, value, typ.Elem(), path+"{}")
				if err != nil {
					return nil, err
				}
				after[key] = merged
			}
		}
		return json.Marshal(after)
	case reflect.Slice, reflect.Array:
		if indirect(typ.Elem()).Kind() != reflect.Struct {
			return next, nil
		}
		var before, after []json.RawMessage
		if err := json.Unmarshal(old, &before); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(next, &after); err != nil {
			return nil, err
		}
		identity := format.ArrayKeys[path]
		if identity == "" {
			// A collection unchanged in the typed view can keep its complete raw
			// form, including extensions, without matching elements by position.
			projected, err := format.project(old, typ, path)
			if err != nil {
				return nil, err
			}
			if equalJSON(projected, next) {
				return old, nil
			}
			// An unkeyed collection may be replaced only if no unknown fields
			// would be lost. Never attach one object's metadata to another index.
			for _, item := range before {
				projected, err := format.project(item, typ.Elem(), path+"[]")
				if err != nil {
					return nil, err
				}
				var a, b any
				left, right := json.NewDecoder(bytes.NewReader(item)), json.NewDecoder(bytes.NewReader(projected))
				left.UseNumber()
				right.UseNumber()
				if left.Decode(&a) != nil || right.Decode(&b) != nil || !reflect.DeepEqual(a, b) {
					return nil, fmt.Errorf("cannot preserve unkeyed configuration extensions")
				}
			}
			return next, nil
		}
		indexed := map[string]json.RawMessage{}
		for _, item := range before {
			key, err := itemKey(item, identity)
			if err != nil {
				return nil, err
			}
			if _, duplicate := indexed[key]; duplicate {
				return nil, fmt.Errorf("duplicate configuration identity")
			}
			indexed[key] = item
		}
		for i, item := range after {
			key, err := itemKey(item, identity)
			if err != nil {
				return nil, err
			}
			if prior, ok := indexed[key]; ok {
				merged, err := format.preserve(prior, item, typ.Elem(), path+"[]")
				if err != nil {
					return nil, err
				}
				after[i] = merged
			}
		}
		return json.Marshal(after)
	default:
		return next, nil
	}
}

func equalJSON(left, right []byte) bool {
	var a, b any
	x, y := json.NewDecoder(bytes.NewReader(left)), json.NewDecoder(bytes.NewReader(right))
	x.UseNumber()
	y.UseNumber()
	return x.Decode(&a) == nil && y.Decode(&b) == nil && reflect.DeepEqual(a, b)
}

func itemKey(raw []byte, field string) (string, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return "", err
	}
	var key string
	if err := json.Unmarshal(object[field], &key); err != nil || key == "" {
		return "", fmt.Errorf("invalid configuration identity")
	}
	return key, nil
}
