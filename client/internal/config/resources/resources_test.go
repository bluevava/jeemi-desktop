package resources

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"jeemi/internal/config/compose"
	"jeemi/internal/platform/paths"
)

const (
	selectorID   = "11111111111111111111111111111111"
	ruleSetID    = "22222222222222222222222222222222"
	ruleGroupID  = "33333333333333333333333333333333"
	secondRuleID = "44444444444444444444444444444444"
)

func TestStorePersistsReusableGroupsAndRuleSets(t *testing.T) {
	ids := []string{ruleSetID, selectorID}
	store, err := NewStore(StoreOptions{
		DataDirectory: filepath.Join(t.TempDir(), paths.DataDirectoryName),
		Now: func() time.Time {
			return time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC)
		},
		NewID: func() (string, error) {
			id := ids[0]
			ids = ids[1:]
			return id, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	state, err := store.SaveRuleSet(RuleSet{
		Name: "本地域名", SourceType: "inline", Behavior: "domain", PayloadYAML: "payload:\n  - +.example.com\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if state.Revision != 1 || len(state.RuleSets) != 1 || state.RuleSets[0].ID != ruleSetID {
		t.Fatalf("unexpected rule set state: %+v", state)
	}
	state, err = store.SaveStrategyGroup(StrategyGroup{
		Name: "德国自动选择", Kind: StrategyGroupKindSelector, Type: "url-test",
		Emoji:     "🐱",
		CreatedAt: "2000-01-01T00:00:00Z", UpdatedAt: "2000-01-01T00:00:00Z",
		RuleOutput:        RuleOutputRuleSet,
		RuleSetReferences: []RuleSetReference{{RuleSetID: ruleSetID}},
		Filter:            NodeFilter{ProxyTypes: []string{"VLESS"}, NamePatterns: []string{"NB", "!test"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if state.Revision != 2 || len(state.StrategyGroups) != 1 || state.StrategyGroups[0].ID != selectorID ||
		state.StrategyGroups[0].Filter.ProxyTypes[0] != "vless" || state.StrategyGroups[0].Interval != 300 ||
		state.StrategyGroups[0].URL != defaultSelectorTestURL || strategyGroupDisplayName(state.StrategyGroups[0]) != "🐱 德国自动选择" {
		t.Fatalf("unexpected strategy group state: %+v", state)
	}
	if state.StrategyGroups[0].CreatedAt != "2026-09-03T10:00:00Z" || state.StrategyGroups[0].UpdatedAt != "2026-09-03T10:00:00Z" {
		t.Fatalf("client-controlled timestamps were persisted: %+v", state.StrategyGroups[0])
	}
	reloaded, err := store.State()
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Directory == "" || len(reloaded.StrategyGroups) != 1 || len(reloaded.RuleSets) != 1 {
		t.Fatalf("resource library did not persist: %+v", reloaded)
	}
}

func TestInlineRuleSetRequiresPayloadYAMLAndPreservesComments(t *testing.T) {
	now := time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC)
	if _, err := normalizeRuleSet(RuleSet{
		Name: "Bare array", SourceType: "inline", Behavior: "domain", Payload: []string{"+.example.com"},
	}, now); err == nil {
		t.Fatal("new rule-set input accepted a bare payload array without YAML")
	}
	input := `# Google rules
payload:
  - DOMAIN-SUFFIX,google.com
  - IP-CIDR,142.250.0.0/15,no-resolve
`
	normalized, err := normalizeRuleSet(RuleSet{
		Name: "Google", SourceType: "inline", Behavior: "classical", PayloadYAML: input,
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if normalized.PayloadYAML != input || len(normalized.Payload) != 2 || normalized.Payload[1] != "IP-CIDR,142.250.0.0/15,no-resolve" {
		t.Fatalf("payload YAML was not preserved and derived correctly: %+v", normalized)
	}

	for _, test := range []struct {
		name string
		yaml string
	}{
		{name: "sequence root", yaml: "- DOMAIN,example.com\n"},
		{name: "extra field", yaml: "payload: [DOMAIN,example.com]\nextra: true\n"},
		{name: "non sequence", yaml: "payload: DOMAIN,example.com\n"},
		{name: "alias", yaml: "payload: &rules\n  - DOMAIN,example.com\ncopy: *rules\n"},
		{name: "second document", yaml: "payload: [DOMAIN,example.com]\n---\npayload: [DOMAIN,example.org]\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := normalizeRuleSet(RuleSet{
				Name: "Invalid", SourceType: "inline", Behavior: "classical", PayloadYAML: test.yaml,
			}, now); err == nil {
				t.Fatalf("invalid payload YAML was accepted: %s", test.yaml)
			}
		})
	}
	if _, err := normalizeRuleSet(RuleSet{
		Name: "Bad classical", SourceType: "inline", Behavior: "classical",
		PayloadYAML: "payload:\n  - UNKNOWN,example.com\n",
	}, now); err == nil {
		t.Fatal("unsupported classical rule type was accepted")
	}
}

func TestRuleSetCanOnlyBelongToOneStrategyGroup(t *testing.T) {
	ids := []string{ruleSetID, selectorID, ruleGroupID}
	store, err := NewStore(StoreOptions{
		DataDirectory: filepath.Join(t.TempDir(), paths.DataDirectoryName),
		Now:           func() time.Time { return time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC) },
		NewID: func() (string, error) {
			id := ids[0]
			ids = ids[1:]
			return id, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SaveRuleSet(RuleSet{
		Name: "共享规则", SourceType: "inline", Behavior: "domain", PayloadYAML: "payload:\n  - +.example.com\n",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SaveStrategyGroup(StrategyGroup{
		Name: "第一个归属", Kind: StrategyGroupKindSelector, Type: "select", RuleOutput: RuleOutputRuleSet,
		RuleSetReferences: []RuleSetReference{{RuleSetID: ruleSetID}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SaveStrategyGroup(StrategyGroup{
		Name: "重复归属", Kind: StrategyGroupKindRule, RuleOutput: RuleOutputRuleSet,
		RuleSetReferences: []RuleSetReference{{RuleSetID: ruleSetID}}, Policy: PolicyTarget{Mode: PolicyDirect},
	}); err == nil || !strings.Contains(err.Error(), "only be referenced by one strategy group") {
		t.Fatalf("duplicate rule-set ownership was not rejected: %v", err)
	}
}

func TestSelectorEmojiMustUseControlledCatalog(t *testing.T) {
	now := time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC)
	if len(selectorEmojiValues) != 91 || len(selectorEmojiSet) != len(selectorEmojiValues) {
		t.Fatalf("selector Emoji catalog is incomplete or duplicated: %d/%d", len(selectorEmojiValues), len(selectorEmojiSet))
	}
	group, err := normalizeStrategyGroup(StrategyGroup{
		Name: "自动选择", Kind: StrategyGroupKindSelector, Type: "select", Emoji: "🚀",
	}, now)
	if err != nil || strategyGroupDisplayName(group) != "🚀 自动选择" {
		t.Fatalf("supported selector Emoji was not accepted: %+v %v", group, err)
	}
	if _, err := normalizeStrategyGroup(StrategyGroup{
		Name: "自动选择", Kind: StrategyGroupKindSelector, Type: "select", Emoji: "😀",
	}, now); err == nil {
		t.Fatal("unsupported selector Emoji was accepted")
	}
}

func TestProtocolAndNameFiltersApplyExclusionsLast(t *testing.T) {
	filter := NodeFilter{ProxyTypes: []string{"vless"}, NamePatterns: []string{"NB", "!test"}}
	if !matchesFilter(filter, proxyRecord{Name: "🇩🇪Base.NB.01.de", Type: "vless"}) {
		t.Fatal("matching proxy was rejected")
	}
	if !matchesFilter(filter, proxyRecord{Name: "🇯🇵Base.NB.01.jp", Type: "vless"}) {
		t.Fatal("an implicit country filter was applied")
	}
	for _, proxy := range []proxyRecord{
		{Name: "🇩🇪Base.NB.test", Type: "vless"},
		{Name: "🇩🇪Base.NB.01.de", Type: "ss"},
		{Name: "🇩🇪Base.GM.01.de", Type: "vless"},
	} {
		if matchesFilter(filter, proxy) {
			t.Fatalf("non-matching proxy was accepted: %+v", proxy)
		}
	}
}

func TestApplyMaterializesSelectorAndRuleGroup(t *testing.T) {
	input := []byte(`proxies:
  - name: 🇩🇪Base.NB.01.de
    type: vless
    server: de.example
    port: 443
    uuid: id
  - name: 🇯🇵Base.NB.01.jp
    type: vless
    server: jp.example
    port: 443
    uuid: id
proxy-groups:
  - name: Existing
    type: select
    proxies: [DIRECT]
rules:
  - MATCH,Existing
`)
	state := State{
		StrategyGroups: []StrategyGroup{
			{
				ID: selectorID, Name: "德国选择", Emoji: "🐱", Kind: StrategyGroupKindSelector,
				Type: "select", RuleOutput: RuleOutputRuleSet,
				RuleSetReferences: []RuleSetReference{},
				Filter:            NodeFilter{NamePatterns: []string{"🇩🇪"}},
			},
			{
				ID: ruleGroupID, Name: "德国域名规则", Kind: StrategyGroupKindRule,
				RuleOutput:        RuleOutputRuleSet,
				RuleSetReferences: []RuleSetReference{{RuleSetID: ruleSetID}},
				Policy:            PolicyTarget{Mode: PolicyProxy},
			},
		},
		RuleSets: []RuleSet{{ID: ruleSetID, Name: "本地域名", SourceType: "inline", Behavior: "domain", Format: "yaml", Payload: []string{"+.example.com"}}},
	}
	result, err := Apply(input, state, Plan{
		StrategyGroupIDs:       []string{ruleGroupID},
		RuleStrategy:           compose.StrategyAppendBeforeTerminal,
		DefaultProxySelectorID: selectorID,
	})
	if err != nil {
		t.Fatal(err)
	}
	output := string(result)
	for _, expected := range []string{
		`name: "\U0001F431 德国选择"`,
		"Base.NB.01.de",
		"本地域名:",
		"payload:",
		"- +.example.com",
		`- "RULE-SET,本地域名,\U0001F431 德国选择"`,
		"- MATCH,Existing",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("materialized configuration is missing %q:\n%s", expected, output)
		}
	}
	if strings.Count(output, "Base.NB.01.de") != 2 || strings.Count(output, "Base.NB.01.jp") != 1 {
		t.Fatalf("flag name filter produced the wrong selector membership:\n%s", output)
	}
	if strings.Contains(output, "name: 德国域名规则") {
		t.Fatalf("logical rule group was incorrectly materialized as a proxy-group:\n%s", output)
	}
	if strings.Index(output, `RULE-SET,本地域名,\U0001F431 德国选择`) > strings.Index(output, "MATCH,Existing") {
		t.Fatalf("rule group was not inserted before the terminal rule:\n%s", output)
	}
}

func TestApplyRuleSetOutputForSelectorAndDeduplicatesProvider(t *testing.T) {
	input := []byte("proxies:\n  - name: node-a\n    type: ss\nrules:\n  - MATCH,DIRECT\n")
	state := State{
		StrategyGroups: []StrategyGroup{
			{
				ID: selectorID, Name: "本地选择", Kind: StrategyGroupKindSelector,
				Type: "select", RuleOutput: RuleOutputRuleSet,
				RuleSetReferences: []RuleSetReference{{RuleSetID: ruleSetID}},
			},
			{
				ID: ruleGroupID, Name: "直连规则", Kind: StrategyGroupKindRule,
				RuleOutput:        RuleOutputRuleSet,
				RuleSetReferences: []RuleSetReference{{RuleSetID: ruleSetID}},
				Policy:            PolicyTarget{Mode: PolicyDirect},
			},
		},
		RuleSets: []RuleSet{{
			ID: ruleSetID, Name: "共享域名", SourceType: "inline", Behavior: "domain", Payload: []string{"+.example.com"},
		}},
	}
	result, err := Apply(input, state, Plan{
		StrategyGroupIDs: []string{ruleGroupID, selectorID},
		RuleStrategy:     compose.StrategyAppendBeforeTerminal,
	})
	if err != nil {
		t.Fatal(err)
	}
	output := string(result)
	if strings.Count(output, "共享域名:") != 1 || strings.Count(output, "RULE-SET,共享域名") != 2 {
		t.Fatalf("shared provider was not deduplicated across rule and selector groups:\n%s", output)
	}
	if strings.Index(output, "RULE-SET,共享域名,DIRECT") > strings.Index(output, "RULE-SET,共享域名,本地选择") {
		t.Fatalf("strategy group order was not preserved:\n%s", output)
	}
}

func TestApplyExpandsInlineRulesForRuleAndSelectorGroups(t *testing.T) {
	input := []byte(`proxies:
  - name: node-a
    type: ss
proxy-groups:
  - name: Existing
    type: select
    proxies: [node-a]
rules:
  - DOMAIN-SUFFIX,subscription.example,DIRECT
  - MATCH,Existing
`)
	state := State{
		StrategyGroups: []StrategyGroup{
			{
				ID: selectorID, Name: "本地选择", Kind: StrategyGroupKindSelector,
				Type: "select", RuleOutput: RuleOutputInline,
				RuleSetReferences: []RuleSetReference{{RuleSetID: secondRuleID}},
				Filter:            NodeFilter{},
			},
			{
				ID: ruleGroupID, Name: "直连规则", Kind: StrategyGroupKindRule,
				RuleOutput:        RuleOutputInline,
				RuleSetReferences: []RuleSetReference{{RuleSetID: ruleSetID}},
				Policy:            PolicyTarget{Mode: PolicyDirect},
			},
		},
		RuleSets: []RuleSet{
			{ID: ruleSetID, Name: "直连域名", SourceType: "inline", Behavior: "domain", Payload: []string{"+.direct.example"}},
			{ID: secondRuleID, Name: "代理域名", SourceType: "inline", Behavior: "classical", Payload: []string{"DOMAIN,proxy.example", "IP-CIDR,203.0.113.0/24,no-resolve"}},
		},
	}
	result, err := Apply(input, state, Plan{
		Version:          CurrentPlanVersion,
		StrategyGroupIDs: []string{ruleGroupID, selectorID},
		RuleStrategy:     compose.StrategyReplace,
		Match:            LocalMatch{Mode: "direct"},
	})
	if err != nil {
		t.Fatal(err)
	}
	output := string(result)
	for _, expected := range []string{
		"- DOMAIN-SUFFIX,direct.example,DIRECT",
		"- DOMAIN,proxy.example,本地选择",
		"- IP-CIDR,203.0.113.0/24,本地选择,no-resolve",
		"- MATCH,DIRECT",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("inline rules are missing %q:\n%s", expected, output)
		}
	}
	if strings.Contains(output, "subscription.example") || strings.Contains(output, "RULE-SET,") {
		t.Fatalf("replace/inline output retained unwanted rules or providers:\n%s", output)
	}
	if strings.Index(output, "DOMAIN-SUFFIX,direct.example,DIRECT") > strings.Index(output, "DOMAIN,proxy.example,本地选择") {
		t.Fatalf("inline strategy group order was not preserved:\n%s", output)
	}
}

func TestIPCIDRRuleSetOwnsNoResolveForProviderAndInlineOutput(t *testing.T) {
	input := []byte("proxies: []\nrules:\n  - MATCH,DIRECT\n")
	ruleSet := RuleSet{
		ID: ruleSetID, Name: "私网地址", SourceType: "inline", Behavior: "ipcidr",
		NoResolve: true, Payload: []string{"10.0.0.0/8"},
	}
	for _, test := range []struct {
		name       string
		outputMode string
		expected   string
	}{
		{name: "provider", outputMode: RuleOutputRuleSet, expected: "RULE-SET,私网地址,DIRECT,no-resolve"},
		{name: "inline", outputMode: RuleOutputInline, expected: "IP-CIDR,10.0.0.0/8,DIRECT,no-resolve"},
	} {
		t.Run(test.name, func(t *testing.T) {
			state := State{
				StrategyGroups: []StrategyGroup{{
					ID: ruleGroupID, Name: "私网直连", Kind: StrategyGroupKindRule,
					RuleOutput: test.outputMode, RuleSetReferences: []RuleSetReference{{RuleSetID: ruleSetID}},
					Policy: PolicyTarget{Mode: PolicyDirect},
				}},
				RuleSets: []RuleSet{ruleSet},
			}
			result, err := Apply(input, state, Plan{
				StrategyGroupIDs: []string{ruleGroupID}, RuleStrategy: compose.StrategyAppendBeforeTerminal,
			})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(result), test.expected) {
				t.Fatalf("rule-set-owned no-resolve was not emitted:\n%s", result)
			}
		})
	}
}

func TestUnusedDefaultSelectorDoesNotMaterializeOrChangeTerminal(t *testing.T) {
	input := []byte("proxies:\n  - name: node-a\n    type: ss\nrules:\n  - DOMAIN,example.org,DIRECT\n  - FINAL,DIRECT\n")
	state := State{StrategyGroups: []StrategyGroup{{
		ID: selectorID, Name: "本地选择", Kind: StrategyGroupKindSelector,
		Type: "select", RuleOutput: RuleOutputRuleSet, RuleSetReferences: []RuleSetReference{},
	}}}
	result, err := Apply(input, state, Plan{
		Version:                CurrentPlanVersion,
		StrategyGroupIDs:       []string{},
		RuleStrategy:           compose.StrategyAppendBeforeTerminal,
		DefaultProxySelectorID: selectorID,
	})
	if err != nil {
		t.Fatal(err)
	}
	output := string(result)
	if !strings.Contains(output, "DOMAIN,example.org,DIRECT") || strings.Contains(output, "name: 本地选择") || !strings.Contains(output, "FINAL,DIRECT") {
		t.Fatalf("unused default selector was materialized or terminal was modified:\n%s", output)
	}
}

func TestApplyRequiresDefaultSelectorForProxyRuleGroups(t *testing.T) {
	input := []byte("proxies: []\nrules:\n  - MATCH,DIRECT\n")
	state := State{
		StrategyGroups: []StrategyGroup{
			{
				ID: ruleGroupID, Name: "代理规则", Kind: StrategyGroupKindRule,
				RuleOutput: RuleOutputRuleSet, RuleSetReferences: []RuleSetReference{{RuleSetID: ruleSetID}},
				Policy: PolicyTarget{Mode: PolicyProxy},
			},
		},
		RuleSets: []RuleSet{{
			ID: ruleSetID, Name: "代理域名", SourceType: "inline", Behavior: "domain", Format: "yaml",
			PayloadYAML: "payload:\n  - +.example.com\n", Payload: []string{"+.example.com"},
		}},
	}
	_, err := Apply(input, state, Plan{
		StrategyGroupIDs: []string{ruleGroupID},
		RuleStrategy:     compose.StrategyAppendBeforeTerminal,
	})
	if err == nil || !strings.Contains(err.Error(), "default proxy selector") {
		t.Fatalf("proxy rule group without a config-level selector returned %v", err)
	}
}

func TestApplyRejectsDuplicateSelectorNames(t *testing.T) {
	input := []byte("proxies: []\nproxy-groups:\n  - name: Duplicate\n    type: select\n    proxies: [DIRECT]\n")
	_, err := Apply(input, State{StrategyGroups: []StrategyGroup{{
		ID: selectorID, Name: "Duplicate", Kind: StrategyGroupKindSelector,
		Type: "select", RuleOutput: RuleOutputRuleSet, RuleSetReferences: []RuleSetReference{},
	}}}, Plan{
		StrategyGroupIDs: []string{selectorID}, RuleStrategy: compose.StrategyAppendBeforeTerminal,
	})
	if err == nil {
		t.Fatal("duplicate selector name was accepted")
	}
}

func TestStrategyGroupTypeSpecificFieldsAreNormalized(t *testing.T) {
	now := time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC)
	fallback, err := normalizeStrategyGroup(StrategyGroup{
		Name: "Fallback", Kind: StrategyGroupKindSelector, RuleOutput: RuleOutputRuleSet,
		Type: "fallback", Tolerance: 50, Strategy: "round-robin",
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if fallback.Tolerance != 0 || fallback.Strategy != "" {
		t.Fatalf("fallback retained fields owned by other selector types: %+v", fallback)
	}
	balanced, err := normalizeStrategyGroup(StrategyGroup{
		Name: "Balanced", Kind: StrategyGroupKindSelector, RuleOutput: RuleOutputRuleSet, Type: "load-balance",
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if balanced.Strategy != "consistent-hashing" {
		t.Fatalf("load-balance default strategy is wrong: %+v", balanced)
	}
}

func TestInlineOutputRejectsRemoteRuleSets(t *testing.T) {
	now := time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC)
	state := State{
		StrategyGroups: []StrategyGroup{{
			ID: ruleGroupID, Name: "远程内联", Kind: StrategyGroupKindRule,
			RuleOutput:        RuleOutputInline,
			RuleSetReferences: []RuleSetReference{{RuleSetID: ruleSetID}},
			Policy:            PolicyTarget{Mode: PolicyDirect}, CreatedAt: now.Format(time.RFC3339Nano), UpdatedAt: now.Format(time.RFC3339Nano),
		}},
		RuleSets: []RuleSet{{
			ID: ruleSetID, Name: "远程规则", SourceType: "http", Behavior: "domain", Format: "yaml",
			URL: "https://example.invalid/rules.yaml", Interval: 86400,
			CreatedAt: now.Format(time.RFC3339Nano), UpdatedAt: now.Format(time.RFC3339Nano),
		}},
	}
	if err := validateState(state.StrategyGroups, state.RuleSets); err == nil {
		t.Fatal("remote rule set was accepted for inline output")
	}
}
