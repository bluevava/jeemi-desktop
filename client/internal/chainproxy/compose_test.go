package chainproxy

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"jeemi/internal/config/document"
	"jeemi/internal/configfile"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

const landingURI = "anytls://test-password@landing.example.invalid:443#Landing"
const sourceYAML = `proxies:
  - {name: HK GM, type: ss, server: 192.0.2.1, port: 443, cipher: aes-128-gcm, password: fixture}
  - {name: JP GM, type: ss, server: 192.0.2.2, port: 443, cipher: aes-128-gcm, password: fixture}
  - {name: JP EV GM, type: ss, server: 192.0.2.3, port: 443, cipher: aes-128-gcm, password: fixture}
proxy-groups:
  - {name: Main, type: select, proxies: [HK GM, DIRECT]}
  - {name: Japan, type: select, proxies: [JP GM, JP EV GM]}
rules: [MATCH,Main]
`

func fixtureGroup(t *testing.T) Group {
	t.Helper()
	imported, err := Parse([]byte(landingURI), "")
	if err != nil {
		t.Fatal(err)
	}
	return Group{ID: strings.Repeat("a", 32), Name: "Landing group", Kind: "manual", Nodes: imported.Nodes, Sources: []Source{}}
}

func TestNameFilterBuckets(t *testing.T) {
	queries := []string{" hk | jp & gm & !ev ", "hk& gm & !ev  | jp", "!ev|hk&gm|jp", "hk     | jp      &gm&!ev"}
	for _, q := range queries {
		for name, want := range map[string]bool{"HK GM": true, "JP gm": true, "HK EV GM": false, "SG GM": false, "JP": false} {
			if ParseNameFilter(q).Match(name) != want {
				t.Errorf("%q / %q", q, name)
			}
		}
	}
	if !ParseNameFilter("!ev").Match("hk") || !ParseNameFilter("& gm").Match("sg gm") || !ParseNameFilter("|& ! ").Empty() {
		t.Fatal("empty/negative-only semantics")
	}
}

func TestBulkFormatsPreserveUnknownFieldsAndDNS(t *testing.T) {
	for _, input := range []string{landingURI + "\n" + strings.ReplaceAll(landingURI, "Landing", "Other"), base64.StdEncoding.EncodeToString([]byte(landingURI)),
		"{name: B, type: ss, server: example.invalid, port: 443, password: fixture, custom-options: {nested: [true, 42]}}",
		"- {name: B, type: ss, server: example.invalid, port: 443, password: fixture}\n- {name: C, type: ss, server: example.invalid, port: 443, password: fixture}",
		"[{name: B, type: ss, server: example.invalid, port: 443, password: fixture}]",
		"[Proxy]\nB = socks5, example.invalid, 443, username=test, password=fixture",
	} {
		imported, err := Parse([]byte(input), "")
		if err != nil || len(imported.Nodes) == 0 {
			t.Fatalf("format rejected: %v", err)
		}
		if strings.Contains(input, "custom-options") && !strings.Contains(imported.Nodes[0].YAML, "custom-options") {
			t.Fatal("unknown field lost")
		}
	}
	input := "host://landing.example.invalid?server=https%3A%2F%2Fdns.example.invalid%2Fdns-query\n" + landingURI
	imported, err := Parse([]byte(input), "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(imported.Nodes[0].DNSYAML, "dns.example.invalid") {
		t.Fatal("node DNS lost")
	}
	edited, err := Parse([]byte(EditableNode(imported.Nodes[0])), "")
	if err != nil || len(edited.Nodes) != 1 || edited.Nodes[0].DNSYAML != imported.Nodes[0].DNSYAML {
		t.Fatal("opening and saving a node lost its custom DNS", err)
	}
	for _, input := range []string{"name: B\nname: C", "proxies: []", "proxies:\n- {name: B, type: ss, port: 443}", "proxies:\n- &node {name: B, type: ss, server: 1.1.1.1, port: 443}\n- *node", "proxies: []\n---\nproxies: []"} {
		if _, err := Parse([]byte(input), ""); err == nil {
			t.Fatalf("bad input accepted: %q", input)
		}
	}
	filtered, err := Parse([]byte(landingURI), "missing")
	if err != nil || len(filtered.Nodes) != 0 {
		t.Fatal("valid empty URL filter must remain refreshable")
	}
}

func TestDefaultMatchAndPerSelectorCandidates(t *testing.T) {
	group := fixtureGroup(t)
	input := []byte(strings.Replace(sourceYAML, "rules: [MATCH,Main]", "rules: ['MATCH,Main']", 1))
	before := bytes.Clone(input)
	output, report, err := Apply(input, []Group{group}, 1, "")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(input, before) || report.Generated != 1 {
		t.Fatalf("mutation/count: %+v", report)
	}
	ast, _ := document.Parse(output)
	root := document.Root(ast)
	nodes := value(root, "proxies").Content
	if scalar(value(nodes[3], "dialer-proxy")) != "HK GM" {
		t.Fatal("wrong chain direction")
	}
	if len(value(value(root, "proxy-groups").Content[1], "proxies").Content) != 2 {
		t.Fatal("B leaked into unmatched group")
	}
	group.SelectorFilter = "Main | Japan"
	group.NodeFilter = "hk | jp & gm & !ev"
	output, report, err = Apply(input, []Group{group}, 1, "")
	if err != nil || report.Generated != 2 {
		t.Fatalf("per-selector: %+v %v", report, err)
	}
	ast, _ = document.Parse(output)
	selectors := value(document.Root(ast), "proxy-groups").Content
	if len(value(selectors[0], "proxies").Content) != 3 || len(value(selectors[1], "proxies").Content) != 3 {
		t.Fatal("candidate union leaked across selectors")
	}
	for _, target := range []string{"DIRECT", "HK GM", "Missing"} {
		group.SelectorFilter = ""
		changed := strings.Replace(string(input), "MATCH,Main", "MATCH,"+target, 1)
		output, report, err := Apply([]byte(changed), []Group{group}, 1, "")
		if err != nil || report.Generated != 0 || string(output) != changed || report.Diagnostics[0].Code != "match_not_selector" {
			t.Fatalf("missing selector: %+v %v", report, err)
		}
	}
}

func TestIncludeAllIsFrozenAndGeneratedNodesNeverBecomeIntermediaries(t *testing.T) {
	group := fixtureGroup(t)
	group.SelectorFilter = "Main"
	input := strings.Replace(sourceYAML, "type: select, proxies: [JP GM, JP EV GM]", "type: select, include-all: true, filter: 'JP'", 1)
	output, report, err := Apply([]byte(input), []Group{group, group}, 1, "")
	if err != nil {
		t.Fatal(err)
	}
	if report.Generated != 1 {
		t.Fatalf("duplicate/recursive generation: %+v", report)
	}
	ast, _ := document.Parse(output)
	other := value(document.Root(ast), "proxy-groups").Content[1]
	if value(other, "include-all") != nil || scalar(value(other, "include-all-providers")) != "true" || len(value(other, "proxies").Content) != 2 {
		t.Fatal("include-all picked up generated nodes")
	}
	// Existing dialer links are skipped rather than forming a cycle via Main.
	input = strings.Replace(input, "name: HK GM, type: ss", "name: HK GM, dialer-proxy: Main, type: ss", 1)
	_, report, err = Apply([]byte(input), []Group{group}, 1, "")
	if err != nil || report.Generated != 0 {
		t.Fatal("chained intermediary accepted")
	}
}

func TestStableNamesDedupAndDNSConflicts(t *testing.T) {
	group := fixtureGroup(t)
	group.SelectorFilter = "Main|Japan"
	input := strings.Replace(sourceYAML, "proxies: [JP GM, JP EV GM]", "proxies: [HK GM]", 1)
	a, report, err := Apply([]byte(input), []Group{group}, 1, "")
	if err != nil || report.Generated != 1 {
		t.Fatalf("dedup: %+v %v", report, err)
	}
	group.Nodes[0].YAML = strings.Replace(group.Nodes[0].YAML, "test-password", "changed-password", 1)
	b, other, err := Apply([]byte(input), []Group{group}, 1, "")
	if err != nil {
		t.Fatal(err)
	}
	first, _ := document.Parse(a)
	second, _ := document.Parse(b)
	if scalar(value(value(document.Root(first), "proxies").Content[3], "name")) != scalar(value(value(document.Root(second), "proxies").Content[3], "name")) || report.Fingerprint == other.Fingerprint {
		t.Fatal("identity must survive password changes, fingerprint must change")
	}
	group.Nodes[0].DNSYAML = "landing.example.invalid: [1.1.1.1]"
	input += "dns:\n  proxy-server-nameserver-policy:\n    landing.example.invalid: [8.8.8.8]\n"
	if _, _, err := Apply([]byte(input), []Group{group}, 1, ""); err == nil {
		t.Fatal("conflicting DNS silently overwritten")
	}
}

func TestStoreReadOnlyCASAndPublicDTO(t *testing.T) {
	root := t.TempDir()
	store, err := NewStore(root)
	if err != nil {
		t.Fatal(err)
	}
	library, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "chain-proxies")); !os.IsNotExist(err) {
		t.Fatal("read created directory")
	}
	library.Groups = append(library.Groups, fixtureGroup(t))
	saved, err := store.Commit(0, library)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(store.file)
	if _, err := store.Commit(0, library); err == nil {
		t.Fatal("stale write accepted")
	}
	loaded, err := store.Load()
	if err != nil || !reflect.DeepEqual(saved, loaded) {
		t.Fatal("round trip", err)
	}
	after, _ := os.ReadFile(store.file)
	if !bytes.Equal(before, after) {
		t.Fatal("read/CAS rewrote file")
	}
	public, _ := json.Marshal(List(loaded))
	if strings.Contains(string(public), "test-password") || strings.Contains(string(public), "landing.example.invalid") {
		t.Fatal("credentials leaked in list")
	}
	if err := os.WriteFile(store.file, []byte("bad"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err = store.Load()
	var invalid *configfile.Failure
	if !errors.As(err, &invalid) || invalid.Reason != "invalid" {
		t.Fatal("corruption not recoverable")
	}
	if err := os.WriteFile(store.file, bytes.Repeat([]byte(" "), maxLibraryBytes+2), 0600); err != nil {
		t.Fatal(err)
	}
	_, err = store.Load()
	if !errors.As(err, &invalid) || invalid.Reason != "read" {
		t.Fatal("partial oversized read must not authorize deleting unverified content")
	}
}

func TestGeneratedNamesRemainValidWithLongUnicodeNames(t *testing.T) {
	name := chainName(strings.Repeat("落", 80), strings.Repeat("🇯🇵", 30), "stable-identity")
	if !utf8.ValidString(name) || len(name) > 256 {
		t.Fatal("generated name exceeds proxy selection persistence constraints")
	}
}

func TestCoreRequirementOnlyAppliesToGeneratedLandingNodes(t *testing.T) {
	group := fixtureGroup(t)
	group.Nodes[0].RequiredCore = "v1.19.30"
	input := []byte(strings.Replace(sourceYAML, "rules: [MATCH,Main]", "rules: ['MATCH,DIRECT']", 1))
	if _, report, err := Apply(input, []Group{group}, 1, "v1.19.0"); err != nil || report.Generated != 0 {
		t.Fatal("ignored chain blocked a direct MATCH", err)
	}
	group.SelectorFilter = "Main"
	if _, _, err := Apply(input, []Group{group}, 1, "v1.19.0"); err == nil {
		t.Fatal("generated node bypassed its core requirement")
	}
}
