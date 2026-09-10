package subscriptionformat

import (
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"jeemi/internal/config/document"
)

type nativeFieldKind uint8

const (
	nativeString nativeFieldKind = iota
	nativeMultiline
	nativeBool
	nativeInteger
	nativeSignedInteger
	nativeObject
	nativeArray
	nativeStrings
	nativeReserved
)

type nativeFields map[string]nativeFieldKind

func fieldNames(kind nativeFieldKind, names string) nativeFields {
	result := nativeFields{}
	for _, name := range strings.Fields(names) {
		result[name] = kind
	}
	return result
}

func combineFields(sets ...nativeFields) nativeFields {
	result := nativeFields{}
	for _, set := range sets {
		for name, kind := range set {
			result[name] = kind
		}
	}
	return result
}

func parseNativeValue(raw string, kind nativeFieldKind) (any, bool) {
	if len(raw) > 256<<10 || !utf8.ValidString(raw) {
		return nil, false
	}
	if kind != nativeObject && kind != nativeArray && kind != nativeStrings {
		if strings.ContainsFunc(raw, func(r rune) bool {
			return unicode.IsControl(r) && !(kind == nativeMultiline && (r == '\n' || r == '\r' || r == '\t'))
		}) {
			return nil, false
		}
	}
	switch kind {
	case nativeString, nativeMultiline:
		return raw, true
	case nativeBool:
		return normalizeField(raw, "bool")
	case nativeInteger, nativeSignedInteger:
		n, err := strconv.ParseInt(raw, 10, 64)
		return int(n), err == nil && (kind == nativeSignedInteger || n >= 0)
	case nativeReserved:
		if !strings.HasPrefix(raw, "[") {
			decoded, ok := decodeBase64(raw)
			return raw, ok && len(decoded) == 3
		}
		value, ok := parseNativeValue(raw, nativeArray)
		if !ok {
			return nil, false
		}
		items := value.([]any)
		if len(items) != 3 {
			return nil, false
		}
		for _, item := range items {
			n, ok := item.(int)
			if !ok || n < 0 || n > 255 {
				return nil, false
			}
		}
		return items, true
	case nativeObject, nativeArray, nativeStrings:
		if !json.Valid([]byte(raw)) {
			// Existing comma-separated ALPN and string lists remain accepted.
			if kind == nativeStrings {
				return normalizeField(raw, "list")
			}
			return nil, false
		}
		ast, err := document.ParseValue(raw)
		if err != nil {
			return nil, false
		}
		var result any
		if ast.Decode(&result) != nil || !validNativeJSON(result, 0) {
			return nil, false
		}
		if kind == nativeObject {
			_, ok := result.(map[string]any)
			return result, ok
		}
		items, ok := result.([]any)
		if !ok {
			return nil, false
		}
		if kind == nativeStrings {
			stringsOnly := make([]string, len(items))
			for i, value := range items {
				text, ok := value.(string)
				if !ok || !validText(text, maxFieldBytes) || text == "" {
					return nil, false
				}
				stringsOnly[i] = text
			}
			return stringsOnly, true
		}
		return items, true
	}
	return nil, false
}

func validNativeJSON(value any, depth int) bool {
	if depth > 24 {
		return false
	}
	switch v := value.(type) {
	case map[string]any:
		if len(v) > 4096 {
			return false
		}
		for key, child := range v {
			if !validText(key, 256) || !validNativeJSON(child, depth+1) {
				return false
			}
		}
	case []any:
		if len(v) > 4096 {
			return false
		}
		for _, child := range v {
			if !validNativeJSON(child, depth+1) {
				return false
			}
		}
	case string:
		return !strings.ContainsRune(v, 0)
	}
	return true
}

// Canonical fields, authority values and compatibility aliases have equal
// authority. Conflicting values reject one node; none silently wins.
func mergeNativeField(target map[string]any, key string, value any) bool {
	previous, exists := target[key]
	if !exists {
		target[key] = value
		return true
	}
	left, leftOK := previous.(map[string]any)
	right, rightOK := value.(map[string]any)
	if leftOK && rightOK {
		for child, v := range right {
			if !mergeNativeField(left, child, v) {
				return false
			}
		}
		return true
	}
	return reflect.DeepEqual(previous, value)
}

func nativeText(node map[string]any, key string) string {
	value, _ := node[key].(string)
	return value
}

func nativeFlag(node map[string]any, key string) bool {
	value, _ := node[key].(bool)
	return value
}
