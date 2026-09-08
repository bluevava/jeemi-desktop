package profile

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"jeemi/internal/config/resources"
	"jeemi/internal/config/schema"
)

func TestFirstReleaseConfigRoundTripDoesNotRewriteFiles(t *testing.T) {
	store := newTestLocalConfigStore(t)
	plan := resources.DefaultPlan()
	plan.Match = resources.LocalMatch{Mode: "direct"}
	created, err := store.Save(SaveLocalConfigInput{
		Name: "首发配置", ResourcePlan: plan,
		Fields: []LocalConfigField{
			{Path: "/tcp-concurrent", ValueYAML: "true"},
			{Path: "/sniffer/enable", ValueYAML: "true"},
			{Path: "/proxies/*/udp", ValueYAML: "true"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(store.Directory(), created.ID)
	before := savedConfigBytes(t, directory)
	var manifest manifestDocument
	var mergePlan mergePlanDocument
	if err := json.Unmarshal(before["manifest.json"], &manifest); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(before["merge-plan.json"], &mergePlan); err != nil {
		t.Fatal(err)
	}
	if manifest.ManifestVersion != 1 || mergePlan.Version != 1 || mergePlan.Resources.Version != 1 || manifest.SchemaVersion != schema.Version {
		t.Fatal("new configuration did not use the first-release formats")
	}
	if len(mergePlan.Transforms) != 1 || mergePlan.Resources.Match.Mode != "direct" {
		t.Fatal("current transforms or local MATCH were lost")
	}
	for _, field := range []string{"targetCoreVersion", "selectorStrategy", "ruleProviderStrategy", "ruleSetBindings"} {
		if bytes.Contains(before["manifest.json"], []byte(field)) || bytes.Contains(before["merge-plan.json"], []byte(field)) {
			t.Fatal("first-release configuration retained a historical field", field)
		}
	}
	for range 2 {
		loaded, err := store.Get(created.ID)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(loaded, created) {
			t.Fatal("read changed the saved configuration")
		}
		if _, err := store.State(); err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(before, savedConfigBytes(t, directory)) {
		t.Fatal("reading rewrote saved files")
	}
}

func TestFirstReleaseConfigRejectsUnsupportedFormatsWithoutRewriting(t *testing.T) {
	cases := map[string]struct {
		file   string
		mutate func(map[string]any)
	}{
		"manifest version":     {"manifest.json", func(value map[string]any) { value["manifestVersion"] = 2 }},
		"catalog version":      {"manifest.json", func(value map[string]any) { value["schemaVersion"] = "mihomo-2026.09-runtime-6" }},
		"core binding":         {"manifest.json", func(value map[string]any) { value["targetCoreVersion"] = "v1.19.30" }},
		"merge version":        {"merge-plan.json", func(value map[string]any) { value["version"] = 7 }},
		"rule version":         {"merge-plan.json", func(value map[string]any) { value["resources"].(map[string]any)["version"] = 2 }},
		"missing rule version": {"merge-plan.json", func(value map[string]any) { delete(value["resources"].(map[string]any), "version") }},
		"independent mode":     {"merge-plan.json", func(value map[string]any) { value["resources"].(map[string]any)["selectorStrategy"] = "append" }},
		"old disabled mode":    {"merge-plan.json", func(value map[string]any) { value["resources"].(map[string]any)["ruleStrategy"] = "none" }},
	}
	for name, fixture := range cases {
		t.Run(name, func(t *testing.T) {
			store := newTestLocalConfigStore(t)
			created, err := store.Save(SaveLocalConfigInput{Name: "首发配置"})
			if err != nil {
				t.Fatal(err)
			}
			directory := filepath.Join(store.Directory(), created.ID)
			before := savedConfigBytes(t, directory)
			var value map[string]any
			if err := json.Unmarshal(before[fixture.file], &value); err != nil {
				t.Fatal(err)
			}
			fixture.mutate(value)
			changed, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(directory, fixture.file), changed, 0o600); err != nil {
				t.Fatal(err)
			}
			before[fixture.file] = changed
			if _, err := store.Get(created.ID); err == nil {
				t.Fatal("unsupported format was accepted")
			}
			if !reflect.DeepEqual(before, savedConfigBytes(t, directory)) {
				t.Fatal("rejected read rewrote files")
			}
		})
	}
}

func savedConfigBytes(t *testing.T, directory string) map[string][]byte {
	t.Helper()
	result := map[string][]byte{}
	for _, name := range []string{"manifest.json", "overlay.yaml", "merge-plan.json"} {
		data, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil {
			t.Fatal(err)
		}
		result[name] = data
	}
	return result
}
