package dnstool_test

import (
	"net"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"jeemi/internal/config/dnstool"
	"jeemi/internal/config/document"
	"jeemi/internal/config/runtimecontrol"
)

func TestToolsCreateMissingCollectionsAndKeepBothRoutesOnRegeneration(t *testing.T) {
	for _, test := range []struct{ name, source string }{
		{"missing listeners", "proxies: []\nproxy-groups:\n  - name: Proxy\n    type: select\n    proxies: [DIRECT]\nrules:\n  - MATCH,Proxy\n"},
		{"missing listeners and groups", "proxies: []\nrules:\n  - MATCH,DIRECT\n"},
		{"empty collections", "proxies: []\nlisteners: []\nproxy-groups: []\nrules:\n  - MATCH,DIRECT\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			configuration := []byte(test.source)
			for generation := 0; generation < 2; generation++ {
				generated, session, err := dnstool.Prepare(configuration, strings.Repeat("a", 64))
				if err != nil {
					t.Fatal(err)
				}
				assertToolSessionMaterialized(t, generated, session)
				configuration = generated
			}
		})
	}
}

func assertToolSessionMaterialized(t *testing.T, configuration []byte, session dnstool.Session) {
	t.Helper()
	parsed, err := document.Parse(configuration)
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Listeners []struct {
			Name   string
			Type   string
			Listen string
			Port   int
			UDP    bool
			Proxy  string
			Users  []struct{ Username, Password string }
		}
		Groups []struct {
			Name    string
			Type    string
			Hidden  bool
			Proxies []string
		} `yaml:"proxy-groups"`
	}
	if err := document.Root(parsed).Decode(&config); err != nil {
		t.Fatal(err)
	}
	if len(config.Listeners) != 2 {
		t.Fatalf("expected both DNS listeners, got %d", len(config.Listeners))
	}
	for _, expected := range []struct{ name, address, target string }{
		{dnstool.Prefix + "proxy", session.ProxyAddress, dnstool.Group},
		{dnstool.Prefix + "direct", session.DirectAddress, "DIRECT"},
	} {
		found := false
		for _, listener := range config.Listeners {
			if listener.Name != expected.name {
				continue
			}
			found = true
			if listener.Type != "socks" || listener.Listen != "127.0.0.1" || listener.UDP || listener.Proxy != expected.target || net.JoinHostPort(listener.Listen, strconv.Itoa(listener.Port)) != expected.address {
				t.Fatalf("listener %s does not match its session route", expected.name)
			}
			if len(listener.Users) != 1 || listener.Users[0].Username != session.Username || listener.Users[0].Password != session.Password {
				t.Fatalf("listener %s has incorrect authentication", expected.name)
			}
		}
		if !found {
			t.Fatalf("missing listener %s", expected.name)
		}
	}
	groups := 0
	for _, group := range config.Groups {
		if group.Name == dnstool.Group {
			groups++
			if group.Type != "select" || !group.Hidden || len(group.Proxies) != 1 || group.Proxies[0] != "REJECT" {
				t.Fatal("invalid private selector")
			}
		}
	}
	if groups != 1 {
		t.Fatalf("expected one private selector, got %d", groups)
	}
}

func TestToolsAreEphemeralAndPreserveUserRouting(t *testing.T) {
	source := []byte("proxies: []\nproxy-groups:\n  - name: Proxy\n    type: select\n    proxies: [DIRECT]\nlisteners:\n  - name: user-in\n    type: socks\n    port: 8900\nrules:\n  - DOMAIN,example.com,DIRECT\n  - MATCH,Proxy\ndns:\n  nameserver: [223.5.5.5]\nx-unknown: preserved\n")
	generated, session, err := dnstool.Prepare(source, strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	if session.MatchTarget != "Proxy" || session.DirectAddress == session.ProxyAddress {
		t.Fatalf("invalid session: %+v", session)
	}
	if session.Password == strings.Repeat("a", 64) {
		t.Fatal("SOCKS endpoint would receive the controller bearer secret")
	}
	parsed, _ := document.Parse(generated)
	var values map[string]any
	_ = document.Root(parsed).Decode(&values)
	listeners := values["listeners"].([]any)
	for _, listener := range listeners[1:] {
		item := listener.(map[string]any)
		if item["listen"] != "127.0.0.1" || item["udp"] != false || item["users"] == nil {
			t.Fatalf("unsafe inbound: %+v", item)
		}
	}
	if listeners[1].(map[string]any)["proxy"] != dnstool.Group || listeners[2].(map[string]any)["proxy"] != "DIRECT" {
		t.Fatal("routes not forced")
	}
	safe, err := runtimecontrol.StripSessionFields(generated)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(safe), session.Password) || strings.Contains(string(safe), dnstool.Prefix) {
		t.Fatal("tool session leaked into persistent/view config")
	}
	before, _ := document.Parse(source)
	after, _ := document.Parse(safe)
	var expected, actual any
	_ = document.Root(before).Decode(&expected)
	_ = document.Root(after).Decode(&actual)
	if !reflect.DeepEqual(expected, actual) {
		t.Fatalf("source routing changed: %s", safe)
	}
	regenerated, _, err := dnstool.Prepare(generated, strings.Repeat("b", 64))
	if err != nil || strings.Count(string(regenerated), "name: "+dnstool.Group) != 1 {
		t.Fatal("duplicate tool objects after restart")
	}
}

func TestFirstTopLevelTerminatorDefinesMATCH(t *testing.T) {
	parsed, _ := document.Parse([]byte("sub-rules:\n  nested: [MATCH,Wrong]\nrules:\n  - FINAL,First\n  - MATCH,Second\n"))
	if got := dnstool.MatchTarget(document.Root(parsed)); got != "First" {
		t.Fatalf("MATCH = %q", got)
	}
}
