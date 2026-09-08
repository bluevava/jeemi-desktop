package compose

import (
	"errors"
	"strings"
	"testing"

	"jeemi/internal/config/document"
)

func TestComposeAppliesTypedStrategiesAndPreservesUnknownFields(t *testing.T) {
	subscription := []byte(`mode: rule
future-field:
  enabled: true
dns:
  enable: false
  nameserver:
    - 1.1.1.1
rules:
  - DOMAIN,example.com,DIRECT
  - MATCH,Proxy
`)
	overlay := []byte(`mode: direct
dns:
  enable: true
  nameserver:
    - tls://dns.example
rules:
  - DOMAIN-SUFFIX,local.test,DIRECT
`)
	result, err := Compose(subscription, overlay, []Rule{
		{Path: "/mode", Strategy: StrategyReplace},
		{Path: "/dns/enable", Strategy: StrategyReplace},
		{Path: "/dns/nameserver", Strategy: StrategyPrepend},
		{Path: "/rules", Strategy: StrategyAppendBeforeTerminal},
	})
	if err != nil {
		t.Fatalf("Compose() error = %v", err)
	}
	for _, expected := range []string{"mode: direct", "future-field:", "tls://dns.example", "MATCH,Proxy"} {
		if !strings.Contains(result.YAML, expected) {
			t.Fatalf("result missing %q:\n%s", expected, result.YAML)
		}
	}
	localRule := strings.Index(result.YAML, "DOMAIN-SUFFIX,local.test,DIRECT")
	terminal := strings.Index(result.YAML, "MATCH,Proxy")
	if localRule < 0 || terminal < 0 || localRule > terminal {
		t.Fatalf("local rule was not inserted before MATCH:\n%s", result.YAML)
	}
}

func TestComposeNamedCollectionRequiresExplicitConflictResolution(t *testing.T) {
	subscription := []byte("proxies:\n  - name: Shared\n    type: direct\n")
	overlay := []byte("proxies:\n  - name: Shared\n    type: reject\n")
	rule := Rule{Path: "/proxies", Strategy: StrategyMergeByName}

	_, err := Compose(subscription, overlay, []Rule{rule})
	var conflict *Conflict
	if !errors.As(err, &conflict) || conflict.Name != "Shared" {
		t.Fatalf("Compose() error = %v, want named conflict", err)
	}

	rule.ConflictPolicy = ConflictUseLocal
	result, err := Compose(subscription, overlay, []Rule{rule})
	if err != nil {
		t.Fatalf("Compose(use local) error = %v", err)
	}
	if !strings.Contains(result.YAML, "type: reject") || strings.Contains(result.YAML, "type: direct") {
		t.Fatalf("local replacement was not applied:\n%s", result.YAML)
	}
}

func TestComposeMergesMappingsAndDeduplicatesAppendedSequences(t *testing.T) {
	subscription := []byte(`hosts:
  old.example: 127.0.0.1
  shared.example: 127.0.0.2
dns:
  nameserver: [1.1.1.1, 8.8.8.8]
`)
	overlay := []byte(`hosts:
  shared.example: 127.0.0.3
  new.example: 127.0.0.4
dns:
  nameserver: [8.8.8.8, 9.9.9.9]
`)
	result, err := Compose(subscription, overlay, []Rule{
		{Path: "/hosts", Strategy: StrategyMerge},
		{Path: "/dns/nameserver", Strategy: StrategyAppend},
	})
	if err != nil {
		t.Fatalf("Compose() error = %v", err)
	}
	for _, expected := range []string{"old.example: 127.0.0.1", "shared.example: 127.0.0.3", "new.example: 127.0.0.4", "9.9.9.9"} {
		if !strings.Contains(result.YAML, expected) {
			t.Fatalf("result missing %q:\n%s", expected, result.YAML)
		}
	}
	if strings.Count(result.YAML, "8.8.8.8") != 1 {
		t.Fatalf("sequence value was not deduplicated:\n%s", result.YAML)
	}
}

func TestComposeNamedMappingConflictUsesTheSameExplicitPolicy(t *testing.T) {
	subscription := []byte("rule-providers:\n  shared:\n    type: http\n    url: https://example.invalid/a\n")
	overlay := []byte("rule-providers:\n  shared:\n    type: file\n    path: ./shared.yaml\n")
	rule := Rule{Path: "/rule-providers", Strategy: StrategyMergeByName}
	if _, err := Compose(subscription, overlay, []Rule{rule}); err == nil {
		t.Fatal("Compose() accepted a named mapping conflict without a policy")
	}
	rule.ConflictPolicy = ConflictUseLocal
	result, err := Compose(subscription, overlay, []Rule{rule})
	if err != nil {
		t.Fatalf("Compose(use local) error = %v", err)
	}
	if !strings.Contains(result.YAML, "type: file") || strings.Contains(result.YAML, "example.invalid") {
		t.Fatalf("named mapping replacement was not applied:\n%s", result.YAML)
	}
}

func TestValidatePlanRejectsOverlappingPaths(t *testing.T) {
	overlay := []byte("dns:\n  enable: true\n")
	overlayDocument, err := document.Parse(overlay)
	if err != nil {
		t.Fatal(err)
	}
	err = ValidatePlan(document.Root(overlayDocument), []Rule{
		{Path: "/dns", Strategy: StrategyMerge},
		{Path: "/dns/enable", Strategy: StrategyReplace},
	})
	if err == nil || !strings.Contains(err.Error(), "overlapping") {
		t.Fatalf("ValidatePlan() error = %v, want overlap", err)
	}
}

func TestValidatePlanRejectsUndeclaredOverlayValues(t *testing.T) {
	overlayDocument, err := document.Parse([]byte("mode: rule\nipv6: true\n"))
	if err != nil {
		t.Fatal(err)
	}
	err = ValidatePlan(document.Root(overlayDocument), []Rule{{
		Path: "/mode", Strategy: StrategyReplace,
	}})
	if err == nil || !strings.Contains(err.Error(), "not declared") {
		t.Fatalf("ValidatePlan() error = %v, want undeclared value", err)
	}
}
