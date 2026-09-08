package profile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"jeemi/internal/config/compose"
	"jeemi/internal/platform/paths"
)

const fixedLocalConfigID = "0123456789abcdef0123456789abcdef"

func TestLocalConfigStoreSavesLoadsAndDeletesManagedFiles(t *testing.T) {
	store := newTestLocalConfigStore(t)
	created, err := store.Save(SaveLocalConfigInput{
		Name:        "办公网络",
		Description: "DNS 与本地域名规则",
		Fields: []LocalConfigField{
			{Path: "/tcp-concurrent", ValueYAML: "true", Strategy: compose.StrategyReplace},
			{Path: "/dns/fallback", ValueYAML: "- 1.1.1.1\n- tls://dns.example", Strategy: compose.StrategyPrepend},
		},
	})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if created.ID != fixedLocalConfigID || created.Revision != 1 || created.EnabledFieldCount != 2 {
		t.Fatalf("unexpected created config: %+v", created)
	}
	for _, name := range []string{"manifest.json", "overlay.yaml", "merge-plan.json"} {
		path := filepath.Join(store.Directory(), fixedLocalConfigID, name)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("managed file %s missing: %v", name, err)
		}
	}
	if !strings.Contains(created.OverlayYAML, "fallback:") {
		t.Fatalf("overlay YAML was not generated:\n%s", created.OverlayYAML)
	}

	state, err := store.State()
	if err != nil {
		t.Fatalf("State() error = %v", err)
	}
	if len(state.Configs) != 1 || state.Configs[0].ID != created.ID {
		t.Fatalf("saved configuration is not listed: %+v", state)
	}
	state, err = store.Delete(created.ID)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if len(state.Configs) != 0 {
		t.Fatalf("deleted configuration still listed: %+v", state.Configs)
	}
}

func TestLocalConfigStoreUpdatesRevisionAndNormalizesFieldValues(t *testing.T) {
	store := newTestLocalConfigStore(t)
	created, err := store.Save(SaveLocalConfigInput{
		Name: "Initial",
		Fields: []LocalConfigField{{
			Path: "/dns/use-system-hosts", ValueYAML: "true", Strategy: compose.StrategyReplace,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := store.Save(SaveLocalConfigInput{
		ID:   created.ID,
		Name: "Updated",
		Fields: []LocalConfigField{{
			Path: "/dns/use-system-hosts", ValueYAML: "false\n", Strategy: compose.StrategyReplace,
		}},
	})
	if err != nil {
		t.Fatalf("Save(update) error = %v", err)
	}
	if updated.Revision != 2 || updated.Name != "Updated" || updated.Fields[0].ValueYAML != "false" {
		t.Fatalf("unexpected update: %+v", updated)
	}
}

func TestLocalConfigStoreRejectsRuntimeOwnedAndWrongTypeFields(t *testing.T) {
	store := newTestLocalConfigStore(t)
	for _, field := range []LocalConfigField{
		{Path: "/secret", ValueYAML: "unsafe", Strategy: compose.StrategyReplace},
		{Path: "/mode", ValueYAML: "rule", Strategy: compose.StrategyReplace},
		{Path: "/mixed-port", ValueYAML: "7890", Strategy: compose.StrategyReplace},
		{Path: "/allow-lan", ValueYAML: "true", Strategy: compose.StrategyReplace},
		{Path: "/tun/stack", ValueYAML: "mixed", Strategy: compose.StrategyReplace},
		{Path: "/tun/device", ValueYAML: "OtherTun", Strategy: compose.StrategyReplace},
		{Path: "/tun/auto-route", ValueYAML: "false", Strategy: compose.StrategyReplace},
		{Path: "/tun/route-exclude-address", ValueYAML: "- 192.168.0.0/16", Strategy: compose.StrategyAppend},
		{Path: "/log-level", ValueYAML: "info", Strategy: compose.StrategyReplace},
		{Path: "/find-process-mode", ValueYAML: "always", Strategy: compose.StrategyReplace},
		{Path: "/dns/enable", ValueYAML: "true", Strategy: compose.StrategyReplace},
		{Path: "/dns/nameserver", ValueYAML: "- 1.1.1.1", Strategy: compose.StrategyAppend},
		{Path: "/dns/fallback", ValueYAML: "1.1.1.1", Strategy: compose.StrategyAppend},
	} {
		_, err := store.Preview(SaveLocalConfigInput{Name: "Invalid", Fields: []LocalConfigField{field}})
		if err == nil {
			t.Fatalf("Preview() accepted invalid field %+v", field)
		}
	}
}

func TestLocalConfigStoreDoesNotEchoSensitiveFieldContentsInErrors(t *testing.T) {
	store := newTestLocalConfigStore(t)
	_, err := store.Preview(SaveLocalConfigInput{
		Name: "Sensitive",
		Fields: []LocalConfigField{{
			Path: "/authentication", ValueYAML: "[top-secret", Strategy: compose.StrategyAppend,
		}},
	})
	if err == nil {
		t.Fatal("Preview() accepted invalid sensitive YAML")
	}
	if strings.Contains(err.Error(), "top-secret") {
		t.Fatalf("sensitive field contents leaked in error: %v", err)
	}
}

func TestLocalConfigPreviewRedactsSensitiveValuesByDefault(t *testing.T) {
	store := newTestLocalConfigStore(t)
	preview, err := store.Preview(SaveLocalConfigInput{
		Name: "Sensitive",
		Fields: []LocalConfigField{{
			Path: "/authentication", ValueYAML: `- "alice:top-secret"`, Strategy: compose.StrategyAppend,
		}},
	})
	if err != nil {
		t.Fatalf("Preview() error = %v", err)
	}
	if !strings.Contains(preview.OverlayYAML, "top-secret") {
		t.Fatalf("full preview unexpectedly lost the edited value: %s", preview.OverlayYAML)
	}
	if strings.Contains(preview.RedactedOverlayYAML, "top-secret") || !strings.Contains(preview.RedactedOverlayYAML, "***") {
		t.Fatalf("redacted preview leaked or omitted its marker: %s", preview.RedactedOverlayYAML)
	}
}

func TestLocalConfigStorePersistsProxyFieldTransformsOutsideOverlay(t *testing.T) {
	store := newTestLocalConfigStore(t)
	created, err := store.Save(SaveLocalConfigInput{
		Name: "代理节点通用字段",
		Fields: []LocalConfigField{{
			Path:      "/proxies/*/client-fingerprint",
			ValueYAML: "chrome",
			Strategy:  compose.StrategyReplace,
		}},
	})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if strings.Contains(created.OverlayYAML, "proxies") {
		t.Fatalf("wildcard transform leaked into mihomo overlay YAML:\n%s", created.OverlayYAML)
	}
	if len(created.Fields) != 1 || created.Fields[0].Path != "/proxies/*/client-fingerprint" {
		t.Fatalf("proxy transform was not returned: %+v", created.Fields)
	}
	planBytes, err := os.ReadFile(filepath.Join(store.Directory(), created.ID, "merge-plan.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(planBytes), `"transforms"`) || !strings.Contains(string(planBytes), `client-fingerprint`) {
		t.Fatalf("proxy transform was not persisted as Jeemi metadata:\n%s", planBytes)
	}
	preview, err := store.Preview(SaveLocalConfigInput{Name: created.Name, Fields: created.Fields})
	if err != nil {
		t.Fatal(err)
	}
	if preview.ProxyOverrideCount != 1 || len(preview.MergePlan) != 0 {
		t.Fatalf("unexpected proxy transform preview: %+v", preview)
	}
}

func newTestLocalConfigStore(t *testing.T) *LocalConfigStore {
	t.Helper()
	dataDirectory := filepath.Join(t.TempDir(), paths.DataDirectoryName)
	store, err := NewLocalConfigStore(LocalConfigStoreOptions{
		DataDirectory: dataDirectory,
		Now: func() time.Time {
			return time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)
		},
		NewID: func() (string, error) { return fixedLocalConfigID, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	return store
}
