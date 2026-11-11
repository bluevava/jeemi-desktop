package presets

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
	"jeemi/internal/config/inspect"
	"jeemi/internal/config/resources"
)

type auditConfiguration struct {
	Rules     []string `yaml:"rules"`
	Providers map[string]struct {
		Behavior string
		Payload  []string
	} `yaml:"rule-providers"`
	Groups []struct{ Name string } `yaml:"proxy-groups"`
}

func TestPresetDisabledResourcesProduceValidFinalRouting(t *testing.T) {
	source := []byte("proxies: [{name: JP, type: ss}]\nproxy-groups: [{name: Origin, type: select, proxies: [JP]}]\nrules: ['PROCESS-NAME,subscription-process,DIRECT', 'MATCH,Origin']\n")
	for _, kind := range []string{"general", "process"} {
		for _, platform := range []string{"windows", "darwin", "linux"} {
			for _, mode := range []string{resources.RuleOutputInline, resources.RuleOutputRuleSet} {
				for _, off := range []string{"Video", "Instagram", "all-rules", "optional-groups", "current-platform", "plan"} {
					t.Run(strings.Join([]string{kind, platform, mode, off}, "/"), func(t *testing.T) {
						preset, err := Load(kind, platform)
						if err != nil {
							t.Fatal(err)
						}
						store, _ := resources.NewStore(resources.StoreOptions{DataDirectory: t.TempDir()})
						state, err := store.PreparePreset(preset.Plan, preset.Groups, preset.Rules)
						if err != nil {
							t.Fatal(err)
						}
						var omittedNames []string
						for i := range state.StrategyGroups {
							group := &state.StrategyGroups[i]
							group.RuleOutput = mode
							if (off == "Video" && group.Name == "Video") || (off == "optional-groups" && group.Name != "Jeemi") {
								group.Disabled = true
							}
						}
						for i := range state.RuleSets {
							rule := &state.RuleSets[i]
							if (off == "Instagram" && strings.Contains(rule.Name, "Instagram")) || off == "all-rules" || (off == "current-platform" && rule.Platform == platform) {
								rule.Disabled = true
								omittedNames = append(omittedNames, rule.Name)
							}
						}
						preset.Plan.RulesDisabled = off == "plan"
						base, err := resources.PrepareSource(source, preset.Plan)
						if err != nil {
							t.Fatal(err)
						}
						result, err := resources.Apply(base, state, preset.Plan, resources.ApplyOptions{Platform: platform})
						if err != nil {
							t.Fatal(err)
						}
						if err := inspect.ValidateReferences(result); err != nil {
							t.Fatalf("disabled resources left invalid references: %v", err)
						}
						if off == "plan" {
							if !bytes.Equal(source, result) {
								t.Fatal("disabled plan changed source")
							}
							return
						}
						var config auditConfiguration
						if err := yaml.Unmarshal(result, &config); err != nil {
							t.Fatal(err)
						}
						if config.Rules[len(config.Rules)-1] != "MATCH,🚀 Jeemi" {
							t.Fatal("fallback changed")
						}
						if strings.Contains(string(result), "subscription-process") {
							t.Fatal("old routing survived reconstruction")
						}
						if count, err := resources.RuleProviderCount(preset.Plan, state, platform); err != nil || count != len(config.Providers) {
							t.Fatal("provider count disagrees with generated output", count, err)
						}
						for _, name := range omittedNames {
							if _, found := config.Providers[name]; found {
								t.Fatal("disabled rule emitted provider", name)
							}
						}
						if off == "Video" || off == "optional-groups" {
							if strings.Contains(string(result), "🎬 Video") || strings.Contains(string(result), "instagram.com") || strings.Contains(string(result), "netflix.com") {
								t.Fatal("disabled Video emitted rules or selectors")
							}
						}
						if off == "Instagram" && strings.Contains(string(result), "instagram.com") {
							t.Fatal("disabled Instagram still matched")
						}
						if off == "all-rules" && (len(config.Rules) != 1 || len(config.Providers) != 0 || len(config.Groups) != 8) {
							t.Fatal("disabling all rule sets did not retain usable selectors and MATCH")
						}
						if off == "current-platform" && kind == "process" && strings.Contains(string(result), "PROCESS-") {
							t.Fatal("other platform used as a fallback")
						}
					})
				}
			}
		}
	}
}

func TestProcessPresetEmitsExactlyItsPlatformPayload(t *testing.T) {
	for _, platform := range []string{"windows", "darwin", "linux"} {
		t.Run(platform, func(t *testing.T) {
			preset, _ := Load("process", platform)
			store, _ := resources.NewStore(resources.StoreOptions{DataDirectory: t.TempDir()})
			state, err := store.PreparePreset(preset.Plan, preset.Groups, preset.Rules)
			if err != nil {
				t.Fatal(err)
			}

			result, err := resources.Apply([]byte("proxies: [{name: JP, type: ss}]"), state, preset.Plan, resources.ApplyOptions{Platform: platform})
			if err != nil {
				t.Fatal(err)
			}
			var config auditConfiguration
			if err := yaml.Unmarshal(result, &config); err != nil {
				t.Fatal(err)
			}
			var got, want []string
			for _, rule := range config.Rules {
				if strings.HasPrefix(rule, "PROCESS-") {
					got = append(got, rule)
				}
			}
			for _, ruleSet := range state.RuleSets {
				if ruleSet.Platform == platform {
					for _, rule := range ruleSet.Payload {
						want = append(want, rule+",DIRECT")
					}
				}
			}
			if len(want) == 0 || !reflect.DeepEqual(got, want) {
				t.Fatalf("platform rules leaked, disappeared or changed order: got %v; want %v", got, want)
			}
		})
	}
}

func TestVideoHasFiveIndependentApplicationRuleSets(t *testing.T) {
	preset, _ := Load("general", "windows")
	store, _ := resources.NewStore(resources.StoreOptions{DataDirectory: t.TempDir()})
	state, err := store.PreparePreset(preset.Plan, preset.Groups, preset.Rules)
	if err != nil {
		t.Fatal(err)
	}
	result, err := resources.Apply([]byte("proxies: [{name: JP, type: ss}]"), state, preset.Plan)
	if err != nil {
		t.Fatal(err)
	}
	var config auditConfiguration
	if err := yaml.Unmarshal(result, &config); err != nil {
		t.Fatal(err)
	}
	var video []string
	for _, rule := range config.Rules {
		if strings.HasSuffix(rule, ",🎬 Video") {
			video = append(video, rule)
		}
	}
	want := []string{"RULE-SET,Jeemi · Netflix,🎬 Video", "RULE-SET,Jeemi · Spotify 域名,🎬 Video", "RULE-SET,Jeemi · X-com 域名,🎬 Video", "RULE-SET,Jeemi · Instagram 域名,🎬 Video", "RULE-SET,Jeemi · YouTube 域名,🎬 Video"}
	if !reflect.DeepEqual(video, want) {
		t.Fatalf("Video must have five app rules: %v", video)
	}
	netflix := config.Providers["Jeemi · Netflix"]
	if netflix.Behavior != "classical" {
		t.Fatal("Netflix must combine domains and IPs in classical format")
	}
	for _, sample := range []string{"DOMAIN-SUFFIX,netflix.com", "IP-CIDR,23.246.0.0/18,no-resolve", "IP-CIDR6,2a03:5640:f800::/37,no-resolve"} {
		found := false
		for _, rule := range netflix.Payload {
			if rule == sample {
				found = true
			}
		}
		if !found {
			t.Fatal("merged Netflix lost rule", sample)
		}
	}
}
