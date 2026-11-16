package profile

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalConfigurationExtensionsSurviveEditing(t *testing.T) {
	store := newTestLocalConfigStore(t)
	created, err := store.Save(SaveLocalConfigInput{Name: "local", Fields: []LocalConfigField{{Path: "/tcp-concurrent", ValueYAML: "true"}}})
	if err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(store.Directory(), created.ID)
	for _, name := range []string{"manifest.json", "merge-plan.json"} {
		path := filepath.Join(directory, name)
		raw, _ := os.ReadFile(path)
		var fields map[string]any
		json.Unmarshal(raw, &fields)
		fields["futureDocument"] = "keep"
		if name == "merge-plan.json" {
			fields["rules"].([]any)[0].(map[string]any)["futureRule"] = "keep-rule"
			fields["resources"].(map[string]any)["futureResources"] = "keep-resources"
		}
		raw, _ = json.Marshal(fields)
		os.WriteFile(path, raw, 0600)
	}
	for i := 0; i < 3; i++ {
		loaded, err := store.Get(created.ID)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := store.Save(SaveLocalConfigInput{ID: loaded.ID, Name: "edited", Fields: loaded.Fields, ResourcePlan: loaded.ResourcePlan}); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"manifest.json", "merge-plan.json"} {
			raw, _ := os.ReadFile(filepath.Join(directory, name))
			if !bytes.Contains(raw, []byte("futureDocument")) {
				t.Fatal("manifest extension lost")
			}
			if name == "merge-plan.json" && (!bytes.Contains(raw, []byte("futureRule")) || !bytes.Contains(raw, []byte("futureResources"))) {
				t.Fatal("nested plan extension lost")
			}
		}
	}
}
