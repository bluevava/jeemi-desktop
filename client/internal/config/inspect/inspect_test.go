package inspect

import "testing"

func TestConfigurationProjectsSelectorsAndProvidersWithoutRuntime(t *testing.T) {
	contents := []byte(`
proxies:
  - name: direct-a
    type: ss
proxy-providers:
  inline:
    type: inline
    payload:
      - name: provider-a
        type: trojan
  remote:
    type: http
    url: https://example.invalid/private
proxy-groups:
  - name: Primary
    icon: https://assets.example.invalid/primary.png
    hidden: true
    type: select
    proxies: [direct-a, DIRECT]
    use: [inline, remote]
    filter: "(?i)a"
  - name: Unused Auto
    type: url-test
    proxies: [direct-a]
rules:
  - AND,((DOMAIN,example.com),(NETWORK,TCP)),Primary
rule-providers:
  private:
    type: http
    behavior: domain
    format: yaml
`)
	result, err := Configuration(contents)
	if err != nil {
		t.Fatal(err)
	}
	if result.ProxyCount != 2 || result.ProxyProviderCount != 2 || len(result.Proxies) != 2 || len(result.Selectors) != 2 || len(result.RuleProviders) != 1 {
		t.Fatalf("unexpected projection summary: %+v", result)
	}
	selector := result.Selectors[0]
	if selector.Icon != "https://assets.example.invalid/primary.png" {
		t.Fatalf("selector icon was not projected: %+v", selector)
	}
	if !selector.Hidden {
		t.Fatalf("selector hidden flag was not projected: %+v", selector)
	}
	if selector.DefaultSelection != "direct-a" || len(selector.Members) != 3 {
		t.Fatalf("unexpected selector: %+v", selector)
	}
	if len(selector.UnresolvedProviderNames) != 1 || selector.UnresolvedProviderNames[0] != "remote" {
		t.Fatalf("remote provider was not marked unresolved: %+v", selector)
	}
	if len(result.Warnings) != 1 || result.Warnings[0] != "selector_dynamic_filter_not_evaluated" {
		t.Fatalf("dynamic selector limitation is missing: %+v", result.Warnings)
	}
	if !selector.ReferencedByRules || result.Selectors[1].ReferencedByRules {
		t.Fatalf("rule target usage was not projected: %+v", result.Selectors)
	}
	if result.Proxies[0].Name != "direct-a" || result.Proxies[1].Name != "provider-a" || result.Proxies[1].ProviderName != "inline" {
		t.Fatalf("global proxy members were not flattened in source order: %+v", result.Proxies)
	}
}

func TestConfigurationRejectsMalformedNamedSelectorItems(t *testing.T) {
	_, err := Configuration([]byte("proxy-groups:\n  - type: select\n"))
	if err == nil {
		t.Fatal("Configuration() accepted a proxy group without a name")
	}
}
