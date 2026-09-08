package runtimecontrol

import (
	"strings"
	"testing"

	"jeemi/internal/config/document"
)

func TestApplyReplacesAllControlEntrypointsWithLoopbackSession(t *testing.T) {
	source := []byte(`external-controller: 0.0.0.0:9090
secret: leaked
external-controller-unix: /tmp/mihomo.sock
external-controller-pipe: \\.\pipe\mihomo
external-doh-server: /dns-query
proxies: []
`)
	result, err := Apply(source, Options{
		Address:        "127.0.0.1:49152",
		Secret:         strings.Repeat("a", 64),
		AllowedOrigins: []string{"http://wails.localhost"},
	})
	if err != nil {
		t.Fatal(err)
	}
	configuration, err := document.Parse(result)
	if err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{
		"/external-controller": "127.0.0.1:49152",
		"/secret":              strings.Repeat("a", 64),
	} {
		value, found, findErr := document.Find(document.Root(configuration), path)
		if findErr != nil || !found || value.Value != want {
			t.Fatalf("%s = %+v found=%v err=%v", path, value, found, findErr)
		}
	}
	for _, path := range []string{"/external-controller-unix", "/external-controller-pipe", "/external-doh-server"} {
		if _, found, findErr := document.Find(document.Root(configuration), path); findErr != nil || found {
			t.Fatalf("alternate controller %s remains", path)
		}
	}
	origins, found, err := document.Find(document.Root(configuration), "/external-controller-cors/allow-origins")
	if err != nil || !found || len(origins.Content) != 1 || origins.Content[0].Value != "http://wails.localhost" {
		t.Fatalf("unexpected CORS origins: %+v found=%v err=%v", origins, found, err)
	}
}

func TestApplyRejectsNonLoopbackWeakOrWildcardControlSettings(t *testing.T) {
	tests := []Options{
		{Address: "0.0.0.0:9090", Secret: strings.Repeat("a", 64), AllowedOrigins: []string{"http://wails.localhost"}},
		{Address: "127.0.0.1:9090", Secret: "short", AllowedOrigins: []string{"http://wails.localhost"}},
		{Address: "127.0.0.1:9090", Secret: strings.Repeat("a", 64), AllowedOrigins: []string{"*"}},
	}
	for _, options := range tests {
		if result, err := Apply([]byte("proxies: []\n"), options); err == nil || result != nil {
			t.Fatalf("Apply() accepted unsafe options %+v", options)
		}
	}
}

func TestStripSessionFieldsKeepsResolvedSnapshotFreeOfControllerCredentials(t *testing.T) {
	result, err := StripSessionFields([]byte(`external-controller: 127.0.0.1:9090
secret: stale-secret
external-controller-cors:
  allow-origins: ['*']
external-controller-pipe: jeemi-pipe
mode: rule
proxies: []
`))
	if err != nil {
		t.Fatal(err)
	}
	configuration, err := document.Parse(result)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range ManagedPaths() {
		if _, found, findErr := document.Find(document.Root(configuration), path); findErr != nil || found {
			t.Fatalf("session field %s remains in resolved snapshot", path)
		}
	}
	mode, found, err := document.Find(document.Root(configuration), "/mode")
	if err != nil || !found || mode.Value != "rule" {
		t.Fatalf("non-session field was not preserved: mode=%+v found=%v err=%v", mode, found, err)
	}
}

func TestFormatForDisplayRestoresUnicodeScalars(t *testing.T) {
	result, err := FormatForDisplay([]byte(`proxies:
  - name: "\U0001F1EF\U0001F1F5Jeeyio SS2022.jp"
    note: "可用 \U0001F680"
    sequence: "\U0001F469\u200D\U0001F4BB"
    literal: "\\U0001F680"
`))
	if err != nil {
		t.Fatal(err)
	}
	text := string(result)
	if !strings.Contains(text, "🇯🇵Jeeyio SS2022.jp") || !strings.Contains(text, "可用 🚀") || !strings.Contains(text, "👩‍💻") {
		t.Fatalf("display YAML did not restore Unicode scalars:\n%s", text)
	}
	if strings.Contains(text, `\U0001F1EF`) {
		t.Fatalf("display YAML still contains a flag escape:\n%s", text)
	}
	if !strings.Contains(text, `literal: "\\U0001F680"`) {
		t.Fatalf("display YAML changed a literal Unicode escape:\n%s", text)
	}
	if _, err := document.Parse(result); err != nil {
		t.Fatalf("display YAML is no longer valid: %v", err)
	}
}
