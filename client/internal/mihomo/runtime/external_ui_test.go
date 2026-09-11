package runtime

import (
	"net/url"
	"strings"
	"testing"

	"jeemi/internal/runtimeconfig"
)

func TestExternalUILoginUsesOnlyCurrentLoopbackFragment(t *testing.T) {
	for _, base := range []string{"http://127.0.0.1:45678", "http://[::1]:45679"} {
		secret := "test+secret&with=# special 字符"
		address, err := externalUIURL(ControllerSession{BaseURL: base, Secret: secret})
		if err != nil {
			t.Fatal(err)
		}
		parsed, _ := url.Parse(address)
		query, _ := url.ParseQuery(strings.TrimPrefix(parsed.Fragment, "/setup?"))
		if parsed.RawQuery != "" || parsed.Path != "/ui/" || query.Get("secret") != secret || query.Get("port") != parsed.Port() || query.Get("hostname") != parsed.Hostname() {
			t.Fatal("bad local setup parameters")
		}
	}
	for _, base := range []string{"https://example.org", "http://example.org:1234", "http://127.0.0.1", "http://127.0.0.1:1/remote", "http://127.0.0.1:1?x=1", "http://127.0.0.1:99999"} {
		if _, err := externalUIURL(ControllerSession{BaseURL: base, Secret: "fixture"}); err == nil {
			t.Fatal("non-local or invalid controller accepted")
		}
	}
}

func TestExternalUIRestartDecisionAndOfflineOpen(t *testing.T) {
	old := runtimeconfig.DefaultPreferences()
	m := &Manager{active: &Generation{Preferences: old}, status: Status{State: StateRunning, ControllerReady: true}}
	next := old
	next.ExternalUIVersion = "v3.26.0"
	if m.ExternalUIRequiresRestart(next) {
		t.Fatal("staging a disabled UI restarted core")
	}
	next.ExternalUIEnabled = true
	if !m.ExternalUIRequiresRestart(next) {
		t.Fatal("enabling UI requires router recreation")
	}
	m.active.Preferences = next
	if m.ExternalUIRequiresRestart(next) {
		t.Fatal("unchanged UI restarted core")
	}
	next.ExternalUIVersion = "v3.27.0"
	if !m.ExternalUIRequiresRestart(next) {
		t.Fatal("version switch must recreate route")
	}
	m.status.State = StateStopped
	called := false
	if err := m.OpenExternalUI(func(string) { called = true }); err == nil || called {
		t.Fatal("opened a stopped session")
	}
}
