package configdocument

import (
	"encoding/json"
	"reflect"
	"testing"

	"jeemi/internal/configfile"
)

type oldItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Flag bool   `json:"flag,omitempty"`
}
type oldDocument struct {
	Version  int       `json:"version"`
	Items    []oldItem `json:"items"`
	Settings struct {
		Enabled bool     `json:"enabled"`
		List    []string `json:"list"`
	} `json:"settings"`
}

func TestOlderWriterPreservesNewerFieldsAcrossRepeatedEdits(t *testing.T) {
	format := Format{VersionField: "version", Version: 1, ArrayKeys: map[string]string{"items": "id"}}
	raw := []byte(`{"version":1,"future":{"large":9007199254740993},"settings":{"enabled":false,"list":[],"future":null},"items":[{"id":"a","name":"A","flag":true,"future":"alpha"},{"id":"b","name":"B","future":{"x":1}},{"id":"removed","name":"old","future":true}]}`)
	for i := 0; i < 4; i++ {
		var older oldDocument
		if err := format.Decode(raw, &older); err != nil {
			t.Fatal(err)
		}
		if older.Settings.Enabled || older.Settings.List == nil || len(older.Settings.List) != 0 {
			t.Fatal("explicit defaults changed")
		}
		items := map[string]oldItem{}
		for _, item := range older.Items {
			items[item.ID] = item
		}
		a, b := items["a"], items["b"]
		a.Name = "edited"
		a.Flag = false
		older.Items = []oldItem{b, a, {ID: "new", Name: "new"}}
		var err error
		raw, err = format.Marshal(raw, older)
		if err != nil {
			t.Fatal(err)
		}
		var newer map[string]json.RawMessage
		if err := json.Unmarshal(raw, &newer); err != nil {
			t.Fatal(err)
		}
		if !equalJSON(newer["future"], []byte(`{"large":9007199254740993}`)) {
			t.Fatal("root extension lost precision")
		}
		var saved []map[string]json.RawMessage
		json.Unmarshal(newer["items"], &saved)
		if len(saved) != 3 || string(saved[0]["id"]) != `"b"` || !equalJSON(saved[0]["future"], []byte(`{"x":1}`)) || string(saved[1]["future"]) != `"alpha"` {
			t.Fatal("extensions did not follow identities")
		}
		if _, ok := saved[1]["flag"]; ok {
			t.Fatal("deleted known field was resurrected")
		}
		if _, ok := saved[2]["future"]; ok {
			t.Fatal("new item inherited removed item's extension")
		}
	}
}

func TestStorageRejectsInvalidTypesDuplicateKeysAndRequiredFeatures(t *testing.T) {
	format := Format{VersionField: "version", Version: 1}
	for _, raw := range []string{`{"version":1,"version":1}`, `{"version":1,"settings":{"enabled":"false"}}`, `{"version":1} {}`, `null`, `{"version":1,"requiredFeatures":42}`} {
		var out oldDocument
		if err := format.Decode([]byte(raw), &out); err == nil || configfile.IsUnsupported(err) {
			t.Fatalf("invalid input not classified correctly: %s / %v", raw, err)
		}
	}
	for _, raw := range []string{`{"version":2}`, `{"version":1,"requiredFeatures":["future-routing"]}`, `{"version":1,"items":[{"requiredFeatures":["future-routing"]}]}`} {
		var out oldDocument
		if err := format.Decode([]byte(raw), &out); !configfile.IsUnsupported(err) {
			t.Fatalf("future format was not distinguished: %v", err)
		}
	}
}

func TestFutureEnumIsUnsupportedWhileWrongTypeIsInvalid(t *testing.T) {
	format := Format{Enums: map[string][]string{"items[].name": {"A", "B"}}}
	var value oldDocument
	if err := format.Decode([]byte(`{"items":[{"id":"x","name":"future"}]}`), &value); !configfile.IsUnsupported(err) {
		t.Fatal("future enum was treated as corruption", err)
	}
	for _, raw := range []string{`{"items":[{"name":false}]}`, `{"settings":{"enabled":null}}`} {
		if err := format.Decode([]byte(raw), &value); err == nil || configfile.IsUnsupported(err) {
			t.Fatal("wrong field type classified as a future feature", err)
		}
	}
}

func TestUnkeyedExtensionsAreNeverMatchedByArrayIndex(t *testing.T) {
	format := Format{}
	raw := []byte(`{"items":[{"id":"a","name":"A","extra":1},{"id":"b","name":"B","extra":2}]}`)
	var value oldDocument
	if err := format.Decode(raw, &value); err != nil {
		t.Fatal(err)
	}
	unchanged, err := format.Marshal(raw, value)
	if err != nil {
		t.Fatal(err)
	}
	var original, saved map[string]any
	json.Unmarshal(raw, &original)
	json.Unmarshal(unchanged, &saved)
	if !reflect.DeepEqual(original["items"], saved["items"]) {
		t.Fatal("unchanged array lost extensions")
	}
	value.Items[0], value.Items[1] = value.Items[1], value.Items[0]
	if _, err := format.Marshal(raw, value); err == nil {
		t.Fatal("unkeyed reorder silently discarded extensions")
	}
}
