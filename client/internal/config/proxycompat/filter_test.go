package proxycompat_test

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
	configinspect "jeemi/internal/config/inspect"
	"jeemi/internal/config/proxycompat"
)

func apply(t *testing.T, source string) (map[string]any, []byte) {
	t.Helper()
	got, err := proxycompat.Apply([]byte(source))
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := yaml.Unmarshal(got, &config); err != nil {
		t.Fatal(err)
	}
	again, err := proxycompat.Apply(got)
	if err != nil || !bytes.Equal(got, again) {
		t.Fatal("filter is not idempotent", err)
	}
	return config, got
}

func names(value any) []string {
	var result []string
	if entries, ok := value.([]any); ok {
		for _, entry := range entries {
			result = append(result, entry.(map[string]any)["name"].(string))
		}
	}
	return result
}

func TestApplyPreservesSupportedAndMalformedInputForNormalValidation(t *testing.T) {
	for _, source := range []string{
		"# untouched comments\nproxies: [{name: Node, type: anytls, password: fixture, extension: {future: true}}]\n",
		`{"proxies":[{"name":"Node","type":"ss"}],"rules":["MATCH,Node"]}`,
		"proxies: [{name: Node, type: reject}, {name: Easy, type: easytier}]\n",
		"proxies: [{name: Broken, type: 123}]\n",
		"proxies: [{name: Broken, type: [snell]}]\n",
		"proxies: [{name: Broken}]\n",
		"proxies: [{name: Broken, type: snell, version: nonsense}]\n",
		"proxy-providers: {remote: {type: http, url: 'https://example.invalid/nodes'}}\n",
	} {
		_, result := apply(t, source)
		if !bytes.Equal(result, []byte(source)) {
			t.Fatal("unrelated configuration was rewritten")
		}
	}
	for version := 0; version <= 5; version++ {
		source := fmt.Sprintf("proxies: [{name: Supported, type: snell, version: %d}]\n", version)
		_, got := apply(t, source)
		if string(got) != source {
			t.Fatalf("supported Snell version %d was filtered", version)
		}
	}
}

func TestApplyFiltersOnlyUnsupportedCapabilitiesAndDependentNodes(t *testing.T) {
	config, result := apply(t, `# keep source comments
proxies:
  - {name: Snell6, type: snell, version: 6}
  - {name: Future, type: unsupported-fixture}
  - {name: UserKey, type: snell, version: 5, userkey: fixture}
  - {name: Mode, type: snell, version: 5, mode: unsafe-raw}
  - {name: Chained, type: anytls, dialer-proxy: Snell6}
  - {name: TwiceChained, type: anytls, dialer-proxy: Chained}
  - {name: AnyTLS, type: anytls, extension: {future: true}}
  - {name: Snell5, type: snell, version: 5}
proxy-groups:
  - {name: Jeemi, type: select, proxies: [Snell6, AnyTLS, Snell5, Future], default-selected: Snell6}
  - {name: Auto, type: url-test, proxies: [Snell6, Chained, TwiceChained], url: 'https://example.invalid/test'}
  - {name: Nested, type: select, proxies: [Auto, Jeemi]}
  - {name: Dynamic, type: select, include-all: true, filter: '^Snell6$'}
  - {name: ExplicitFallback, type: select, include-all: true, empty-fallback: DIRECT}
  - {name: InvalidUnrelated, type: select, proxies: []}
rules: ['MATCH,Jeemi']
`)
	if got := names(config["proxies"]); !reflect.DeepEqual(got, []string{"AnyTLS", "Snell5"}) {
		t.Fatalf("unexpected survivors: %v", got)
	}
	groups := config["proxy-groups"].([]any)
	jeemi := groups[0].(map[string]any)
	if !reflect.DeepEqual(jeemi["proxies"], []any{"AnyTLS", "Snell5"}) || jeemi["default-selected"] != nil {
		t.Fatal("selector still references removed nodes")
	}
	if auto := groups[1].(map[string]any); !reflect.DeepEqual(auto["proxies"], []any{"REJECT"}) || auto["empty-fallback"] != "REJECT" {
		t.Fatal("empty auto selector is not blocked")
	}
	if nested := groups[2].(map[string]any); !reflect.DeepEqual(nested["proxies"], []any{"Auto", "Jeemi"}) || nested["empty-fallback"] != nil {
		t.Fatal("nested unaffected group was changed")
	}
	if groups[3].(map[string]any)["empty-fallback"] != "REJECT" || groups[4].(map[string]any)["empty-fallback"] != "DIRECT" {
		t.Fatal("dynamic filter fallback lost fail-closed or explicit user intent")
	}
	if len(groups[5].(map[string]any)["proxies"].([]any)) != 0 {
		t.Fatal("unrelated invalid group was repaired")
	}
	if !strings.Contains(string(result), "# keep source comments") || config["proxies"].([]any)[0].(map[string]any)["extension"] == nil {
		t.Fatal("unrelated fields or comments were lost")
	}
}

func TestApplyRejectsDanglingReferencesWithoutChangingRuleOrder(t *testing.T) {
	config, result := apply(t, `proxies:
  - {name: Bad Node, type: snell, version: 6}
  - {name: Good, type: anytls}
proxy-groups:
  - {name: Proxy, type: select, proxies: [Bad Node, Good], empty-fallback: Bad Node}
  - {name: Relay, type: relay, proxies: [Good, Bad Node], empty-fallback: DIRECT}
proxy-providers:
  remote: {type: http, url: 'https://example.invalid/nodes', proxy: Bad Node, override: {dialer-proxy: Bad Node}}
rule-providers:
  domains: {type: http, behavior: domain, url: 'https://example.invalid/rules', proxy: Bad Node}
rules:
  - DOMAIN,first.example.invalid,Bad Node
  - AND,((DOMAIN,second.example.invalid),(NETWORK,TCP)),Bad Node
  - IP-CIDR,192.0.2.0/24,Bad Node,no-resolve
  - SUB-RULE,(NETWORK,TCP),Bad Node
  - RULE-SET,domains,Bad Node
  - MATCH,Proxy
sub-rules:
  Bad Node: ['DOMAIN,nested.example.invalid,Bad Node', 'MATCH,Good']
listeners: [{name: mixed, type: mixed, port: 7891, proxy: Bad Node}]
tunnels: [{network: tcp, address: '127.0.0.1:7892', target: '192.0.2.1:80', proxy: Bad Node}]
dns:
  nameserver: ['https://example.invalid/dns-query#Bad%20Node&h3=true', 'https://example.invalid/Good#Good']
  nameserver-policy: {'Bad Node': 'https://example.invalid/dns-query#Bad Node'}
  proxy-server-nameserver-policy: {'+.example.invalid': ['https://example.invalid/dns-query#Bad%20Node&skip-cert-verify=true']}
`)
	if err := configinspect.ValidateReferences(result); err != nil {
		t.Fatal("filtered reference graph is invalid", err)
	}
	wantRules := []any{
		"DOMAIN,first.example.invalid,REJECT",
		"AND,((DOMAIN,second.example.invalid),(NETWORK,TCP)),REJECT",
		"IP-CIDR,192.0.2.0/24,REJECT,no-resolve",
		"SUB-RULE,(NETWORK,TCP),Bad Node",
		"RULE-SET,domains,REJECT", "MATCH,Proxy",
	}
	if !reflect.DeepEqual(config["rules"], wantRules) || !reflect.DeepEqual(config["sub-rules"].(map[string]any)["Bad Node"], []any{"DOMAIN,nested.example.invalid,REJECT", "MATCH,Good"}) {
		t.Fatal("rule targets/order or sub-rule names were changed incorrectly")
	}
	groups := config["proxy-groups"].([]any)
	if groups[0].(map[string]any)["empty-fallback"] != "REJECT" {
		t.Fatal("dangling empty-fallback target remains")
	}
	relay := groups[1].(map[string]any)
	if relay["type"] != "select" || relay["empty-fallback"] != "REJECT" || !reflect.DeepEqual(relay["proxies"], []any{"REJECT"}) {
		t.Fatal("partial relay would bypass its removed hop")
	}
	for _, key := range []string{"proxy-providers", "rule-providers"} {
		for _, entry := range config[key].(map[string]any) {
			if entry.(map[string]any)["proxy"] != "REJECT" {
				t.Fatal("dangling provider download route", key)
			}
		}
	}
	if config["proxy-providers"].(map[string]any)["remote"].(map[string]any)["override"].(map[string]any)["dialer-proxy"] != "REJECT" {
		t.Fatal("provider override bypasses removed dialer")
	}
	for _, key := range []string{"listeners", "tunnels"} {
		if config[key].([]any)[0].(map[string]any)["proxy"] != "REJECT" {
			t.Fatal("dangling inbound route", key)
		}
	}
	dns := config["dns"].(map[string]any)
	if !reflect.DeepEqual(dns["nameserver"], []any{"https://example.invalid/dns-query#REJECT&h3=true", "https://example.invalid/Good#Good"}) ||
		dns["nameserver-policy"].(map[string]any)["Bad Node"] != "https://example.invalid/dns-query#REJECT" ||
		!reflect.DeepEqual(dns["proxy-server-nameserver-policy"].(map[string]any)["+.example.invalid"], []any{"https://example.invalid/dns-query#REJECT&skip-cert-verify=true"}) {
		t.Fatal("DNS references or fragment options were not preserved")
	}
}

func TestApplyFiltersInlineProvidersAndKeepsNameScopesSeparate(t *testing.T) {
	config, result := apply(t, `proxies:
  - {name: Same, type: anytls}
  - {name: Bad, type: snell, version: 6}
proxy-providers:
  mixed:
    type: inline
    payload: [{name: Same, type: snell, version: 6}, {name: Good, type: ss}]
  empty:
    type: inline
    payload: [{name: Bad, type: snell, version: 6}, {name: Dependent, type: anytls, dialer-proxy: Bad}]
  untouched: {type: http, url: 'https://example.invalid/nodes'}
proxy-groups:
  - {name: Mixed, type: select, proxies: [Same], use: [mixed, empty]}
  - {name: Empty, type: select, use: [empty]}
  - {name: Filtered, type: select, use: [mixed], filter: '^Same$'}
rules: ['MATCH,Mixed']
`)
	if err := configinspect.ValidateReferences(result); err != nil {
		t.Fatal(err)
	}
	providers := config["proxy-providers"].(map[string]any)
	if providers["empty"] != nil || !reflect.DeepEqual(names(providers["mixed"].(map[string]any)["payload"]), []string{"Good"}) {
		t.Fatal("inline provider nodes were not filtered")
	}
	groups := config["proxy-groups"].([]any)
	if !reflect.DeepEqual(names(config["proxies"]), []string{"Same"}) || !reflect.DeepEqual(groups[0].(map[string]any)["proxies"], []any{"Same"}) || !reflect.DeepEqual(groups[0].(map[string]any)["use"], []any{"mixed"}) {
		t.Fatal("inline node shadowed a valid global target")
	}
	if !reflect.DeepEqual(groups[1].(map[string]any)["proxies"], []any{"REJECT"}) || groups[2].(map[string]any)["empty-fallback"] != "REJECT" {
		t.Fatal("provider filtering left an unsafe empty group")
	}
}

func TestApplyHandlesAliasesWithoutChangingUnrelatedCopies(t *testing.T) {
	config, result := apply(t, `proxies:
  - &removed {name: Bad, type: snell, version: 6}
  - {name: Good, type: anytls}
proxy-groups:
  - {name: One, type: select, proxies: &members [Bad, Good]}
  - {name: Two, type: select, proxies: *members}
x-reference: *removed
x-members: *members
rules: ['MATCH,One']
`)
	if err := configinspect.ValidateReferences(result); err != nil {
		t.Fatal(err)
	}
	if config["x-reference"].(map[string]any)["name"] != "Bad" || !reflect.DeepEqual(config["x-members"], []any{"Bad", "Good"}) {
		t.Fatal("pruning changed unrelated aliased data")
	}
	for _, group := range config["proxy-groups"].([]any) {
		if !reflect.DeepEqual(group.(map[string]any)["proxies"], []any{"Good"}) {
			t.Fatal("alias reference was not pruned")
		}
	}
}

func TestApplyAllUnsupportedFailsClosedAndPreservesExplicitFallback(t *testing.T) {
	for _, version := range []string{"6", "'6'", "0x6", "-1"} {
		config, result := apply(t, "proxies: [{name: Bad, type: snell, version: "+version+"}]\n")
		if len(names(config["proxies"])) != 0 || !reflect.DeepEqual(config["rules"], []any{"MATCH,REJECT"}) {
			t.Fatal("all unsupported nodes did not fail closed", version)
		}
		if err := configinspect.ValidateReferences(result); err != nil {
			t.Fatal(err)
		}
	}
	for _, target := range []string{"DIRECT", "REJECT"} {
		config, _ := apply(t, "proxies: [{name: Bad, type: snell, version: 6}]\nrules: ['MATCH,"+target+"']\n")
		if !reflect.DeepEqual(config["rules"], []any{"MATCH," + target}) {
			t.Fatal("explicit source fallback was overwritten")
		}
	}
}

func TestApplyRejectsAmbiguousNamesAndInvalidDocuments(t *testing.T) {
	for _, source := range []string{
		"proxies: [{name: Same, type: snell, version: 6}, {name: Same, type: anytls}]",
		"proxies: [{name: Same, type: snell, version: 6}]\nproxy-groups: [{name: Same, type: select, proxies: [DIRECT]}]",
		"proxies: [{name: DIRECT, type: snell, version: 6}]",
		"proxies: [\n",
		"proxies: []\n---\nproxies: []\n",
	} {
		if _, err := proxycompat.Apply([]byte(source)); err == nil {
			t.Fatal("invalid or ambiguous configuration was silently accepted")
		}
	}
}
