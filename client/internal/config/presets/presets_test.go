package presets

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
	"jeemi/internal/config/resources"
	"jeemi/internal/subscriptionformat"
)

func TestOfflinePresetsComposeSharedNodesAndPairedAuto(t *testing.T) {
	for _, kind := range []string{"general", "process"} {
		t.Run(kind, func(t *testing.T) {
			p, err := Load(kind)
			if err != nil {
				t.Fatal(err)
			}
			if len(p.Plan.StrategyGroupIDs) == 0 || p.Plan.StrategyGroupIDs[0] != p.Plan.DefaultProxySelectorID {
				t.Fatal("default proxy selector must lead the preset plan")
			}
			store, err := resources.NewStore(resources.StoreOptions{DataDirectory: t.TempDir()})
			if err != nil {
				t.Fatal(err)
			}
			state, err := store.PreparePreset(p.Plan, p.Groups, p.Rules)
			if err != nil {
				t.Fatal(err)
			}
			source := "proxies:\n"
			names := []string{"🇯🇵 Japan", "🇹🇼 Taiwan", "🇭🇰 Hongkong", "香港 02", "HK fast", "hong kong 03", "Hong-Kong 04"}
			for _, name := range names {
				source += fmt.Sprintf("  - {name: %q, type: ss, server: example.invalid, port: 443, cipher: aes-128-gcm, password: fixture}\n", name)
			}
			source += "proxy-groups: [{name: old, type: select, proxies: [DIRECT]}]\nrules: [MATCH,old]\nsub-rules: {old: [MATCH,DIRECT]}\n"
			input, err := resources.PrepareSource([]byte(source), p.Plan)
			if err != nil {
				t.Fatal(err)
			}
			result, err := resources.Apply(input, state, p.Plan, resources.ApplyOptions{BuiltInBrowserRules: []string{"PROCESS-NAME,msedge.exe"}})
			if err != nil {
				t.Fatal(err)
			}
			var config struct {
				Proxies []map[string]any `yaml:"proxies"`
				Groups  []struct {
					Name, Type string
					Proxies    []string
					Hidden     bool
				} `yaml:"proxy-groups"`
				Rules     []string `yaml:"rules"`
				Providers map[string]struct {
					Type, URL, Behavior string
					Payload             []string
				} `yaml:"rule-providers"`
			}
			if err := yaml.Unmarshal(result, &config); err != nil {
				t.Fatal(err)
			}
			if len(config.Proxies) != len(names) || len(config.Groups) != 8 || strings.Contains(string(result), "sub-rules:") {
				t.Fatal("routing reconstruction lost nodes or retained old routing")
			}
			byName := map[string][]string{}
			visibleSelectors := []string{}
			for _, group := range config.Groups {
				byName[group.Name] = group.Proxies
				if !group.Hidden {
					visibleSelectors = append(visibleSelectors, group.Name)
				}
				if strings.HasSuffix(group.Name, " Auto") && (group.Type != "url-test" || !group.Hidden) {
					t.Fatal("Auto must be a hidden url-test group")
				}
			}
			if !reflect.DeepEqual(visibleSelectors, []string{"🚀 Jeemi", "🤖 AI", "✈️ Telegram", "🎬 Video"}) {
				t.Fatalf("preset selector order changed: %v", visibleSelectors)
			}
			for _, name := range []string{"🚀 Jeemi", "🤖 AI", "✈️ Telegram", "🎬 Video"} {
				main, auto := byName[name], byName[name+" Auto"]
				if len(main) < 1 || main[0] != name+" Auto" || !reflect.DeepEqual(main[1:], auto) {
					t.Fatalf("%s candidates differ", name)
				}
				want := names
				if name == "🤖 AI" {
					want = names[:2]
				}
				if !reflect.DeepEqual(auto, want) {
					t.Fatalf("%s filter mismatch: %v", name, auto)
				}
			}
			for _, provider := range config.Providers {
				if provider.Type != "inline" || provider.URL != "" || len(provider.Payload) == 0 {
					t.Fatal("rules must be populated and offline")
				}
			}
			last := config.Rules[len(config.Rules)-1]
			if last != "MATCH,🚀 Jeemi" {
				t.Fatal(last)
			}
			if !strings.HasSuffix(config.Rules[0], ",🤖 AI") || !strings.HasSuffix(config.Rules[1], ",✈️ Telegram") || !strings.HasSuffix(config.Rules[3], ",🎬 Video") {
				t.Fatal("business rule precedence changed", config.Rules)
			}
			if kind == "process" {
				if config.Rules[len(config.Rules)-2] != "PROCESS-NAME,msedge.exe,DIRECT" || strings.Contains(string(result), "GEOIP,") || strings.Contains(string(result), "GEOSITE,") {
					t.Fatal("process default order changed")
				}
				if _, err = resources.Apply(input, state, p.Plan); err == nil {
					t.Fatal("missing platform browser rules must block composition")
				}
				for _, rule := range state.RuleSets {
					if rule.DefaultBrowser && len(rule.Payload) != 0 {
						t.Fatal("composition mutated saved browser rules")
					}
				}
			} else {
				want := []string{"DOMAIN-SUFFIX,windowsupdate.com,DIRECT", "DOMAIN-SUFFIX,dl.delivery.mp.microsoft.com,DIRECT", "GEOSITE,CN,DIRECT", "GEOIP,CN,DIRECT", "MATCH,🚀 Jeemi"}
				if !reflect.DeepEqual(config.Rules[5:], want) {
					t.Fatalf("general inline routing mismatch: %v", config.Rules)
				}
				if strings.Contains(string(result), "PROCESS-") {
					t.Fatal("general preset contains process routing")
				}
			}
			if len(config.Providers) != 5 {
				t.Fatal("direct rules were not expanded inline")
			}
			for _, group := range state.StrategyGroups {
				if group.Emoji == "" {
					t.Fatal("preset group has no Emoji")
				}
			}
			for _, rule := range state.RuleSets {
				if strings.HasPrefix(rule.Name, "Jeemi") {
					t.Fatal("preset rule set has no Emoji")
				}
			}
		})
	}
}

func TestBundledSnapshotIntegrityAndCoverage(t *testing.T) {
	var manifest struct {
		Files []struct {
			File, SHA256 string
			Bytes        int
		}
	}
	raw, err := data.ReadFile("data/sources.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Files) != 7 {
		t.Fatalf("missing source records: %s", raw)
	}
	for _, f := range manifest.Files {
		content, err := data.ReadFile("data/" + f.File)
		if err != nil {
			t.Fatal(err)
		}
		if fmt.Sprintf("%x", sha256.Sum256(content)) != f.SHA256 || len(content) != f.Bytes {
			t.Fatalf("snapshot mismatch: %s", f.File)
		}
	}
	p, err := Load("general")
	if err != nil {
		t.Fatal(err)
	}
	all := ""
	for _, r := range p.Rules {
		all += r.PayloadYAML
	}
	for _, domain := range []string{"+.openai.com", "+.claude.ai", "+.gemini.google.com", "+.grok.com", "+.perplexity.ai", "+.cursor.com", "+.instagram.com", "+.youtube.com", "+.netflix.com"} {
		if !strings.Contains(all, domain) {
			t.Errorf("baseline misses %s", domain)
		}
	}
	if _, err := Load("unknown"); err == nil {
		t.Fatal("unknown preset accepted")
	}
}

func TestEmptyAICandidatesNeverFallBackToDirect(t *testing.T) {
	p, _ := Load("process")
	s, _ := resources.NewStore(resources.StoreOptions{DataDirectory: t.TempDir()})
	state, err := s.PreparePreset(p.Plan, p.Groups, p.Rules)
	if err != nil {
		t.Fatal(err)
	}
	result, err := resources.Apply([]byte("proxies: [{name: HK, type: ss}]"), state, p.Plan, resources.ApplyOptions{BuiltInBrowserRules: []string{"PROCESS-NAME,msedge.exe"}})
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Groups []struct {
			Name    string
			Proxies []string
		} `yaml:"proxy-groups"`
	}
	if err = yaml.Unmarshal(result, &config); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, g := range config.Groups {
		if g.Name == "🤖 AI Auto" {
			found = true
		}
		if g.Name == "🤖 AI Auto" && !reflect.DeepEqual(g.Proxies, []string{"REJECT"}) {
			t.Fatalf("empty AI permits traffic: %+v", g)
		}
	}
	if !found {
		t.Fatal("AI Auto disappeared")
	}
}

func TestURISubscriptionAcquiresOfflineRouting(t *testing.T) {
	raw := []byte("anytls://fixture-password@192.0.2.10:443?sni=tls.example.invalid#JP%20Example\n")
	normalized, err := subscriptionformat.Normalize(raw)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := Load("general")
	store, _ := resources.NewStore(resources.StoreOptions{DataDirectory: t.TempDir()})
	state, err := store.PreparePreset(p.Plan, p.Groups, p.Rules)
	if err != nil {
		t.Fatal(err)
	}
	base, err := resources.PrepareSource(normalized.Contents, p.Plan)
	if err != nil {
		t.Fatal(err)
	}
	result, err := resources.Apply(base, state, p.Plan)
	if err != nil {
		t.Fatal(err)
	}
	var routing struct {
		Rules []string `yaml:"rules"`
	}
	if err := yaml.Unmarshal(result, &routing); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(routing.Rules, "\n")
	if !strings.Contains(joined, "MATCH,🚀 Jeemi") || !strings.Contains(joined, "RULE-SET,") || !strings.Contains(joined, "GEOSITE,CN,DIRECT") || !strings.Contains(joined, "GEOIP,CN,DIRECT") {
		t.Fatal("URI lost offline business routing or local CN GEO references")
	}
	if strings.Count(string(result), "password:") != 1 || !strings.Contains(string(result), "JP Example") {
		t.Fatal("URI node definition was duplicated or lost")
	}
}

func TestDomesticProcessRulesMatchAppBoundaries(t *testing.T) {
	p, _ := Load("process")
	s, _ := resources.NewStore(resources.StoreOptions{DataDirectory: t.TempDir()})
	state, err := s.PreparePreset(p.Plan, p.Groups, p.Rules)
	if err != nil {
		t.Fatal(err)
	}
	patterns := map[string][]*regexp.Regexp{}
	for _, rule := range state.RuleSets {
		if !strings.Contains(rule.Name, "国内应用进程") {
			continue
		}
		for _, line := range rule.Payload {
			kind, expr, _ := strings.Cut(line, ",")
			re, err := regexp.Compile(expr)
			if err != nil {
				t.Fatal(err)
			}
			patterns[kind] = append(patterns[kind], re)
		}
	}
	for _, tt := range []struct {
		kind, input string
		want        bool
	}{
		{"PROCESS-PATH-REGEX", `C:\Program Files\Tencent\WeChat\WeChatAppEx.exe`, true},
		{"PROCESS-PATH-REGEX", `C:\apps\Feishu\1.0\helper.exe`, true},
		{"PROCESS-PATH-REGEX", `C:\apps\Huorong\bin\helper.exe`, true},
		{"PROCESS-PATH-REGEX", `/Applications/WeChat.app/Contents/Frameworks/Helper.app/Contents/MacOS/Helper`, true},
		{"PROCESS-NAME-REGEX", "Weixin.exe", true},
		{"PROCESS-NAME-REGEX", "cloudmusic", true},
		{"PROCESS-NAME-REGEX", "SunloginClient.exe", true},
		{"PROCESS-NAME-REGEX", "Telegram.exe", false},
		{"PROCESS-NAME-REGEX", "cursor.exe", false},
		{"PROCESS-NAME-REGEX", "mywechattool.exe", false},
		{"PROCESS-PATH-REGEX", `C:\apps\NotTencent\other.exe`, false},
		{"PROCESS-PATH-REGEX", `C:\apps\Microsoft\Edge\msedge.exe`, false},
	} {
		matched := false
		for _, pattern := range patterns[tt.kind] {
			matched = matched || pattern.MatchString(tt.input)
		}
		if matched != tt.want {
			t.Errorf("%s = %v, want %v", tt.input, matched, tt.want)
		}
	}
}

func TestPresetCreationRenamesEmojiAndAutoCollisions(t *testing.T) {
	s, _ := resources.NewStore(resources.StoreOptions{DataDirectory: t.TempDir()})
	if _, err := s.SaveStrategyGroup(resources.StrategyGroup{Name: "AI Auto", Emoji: "🤖", Kind: "selector", Type: "select"}); err != nil {
		t.Fatal(err)
	}
	p, _ := Load("general")
	state, err := s.PreparePreset(p.Plan, p.Groups, p.Rules)
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range state.StrategyGroups {
		if g.ID == id("group:AI") && resources.SelectorDisplayName(g) != "🤖 AI (2)" {
			t.Fatal("generated Auto name collision was not resolved")
		}
	}
}
