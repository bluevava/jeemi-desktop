package presets

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
	"jeemi/internal/config/resources"
)

func TestPresetBusinessCategoriesAndGoogleMediaPrecedence(t *testing.T) {
	p, err := Load("general", "windows")
	if err != nil {
		t.Fatal(err)
	}
	store, _ := resources.NewStore(resources.StoreOptions{DataDirectory: t.TempDir()})
	state, err := store.PreparePreset(p.Plan, p.Groups, p.Rules)
	if err != nil {
		t.Fatal(err)
	}
	result, err := resources.Apply([]byte("proxies: [{name: JP, type: ss}]"), state, p.Plan)
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Rules     []string `yaml:"rules"`
		Providers map[string]struct {
			Behavior string
			Payload  []string
		} `yaml:"rule-providers"`
	}
	if err := yaml.Unmarshal(result, &config); err != nil {
		t.Fatal(err)
	}
	for domain, want := range map[string]string{
		"www.instagram.com": "🎬 Video", "www.youtube.com": "🎬 Video", "r1.googlevideo.com": "🎬 Video", "i.ytimg.com": "🎬 Video",
		"www.netflix.com": "🎬 Video", "open.spotify.com": "🎬 Video", "x.com": "🎬 Video",
		"api.telegram.org": "✈️ IM", "discord.com": "✈️ IM", "discord-attachments-uploads-prd.storage.googleapis.com": "✈️ IM", "whatsapp.com": "✈️ IM", "line.me": "✈️ IM",
		"www.google.com": "🤖 AI+Google", "claude.ai": "🤖 AI+Google", "github.com": "🚀 Jeemi", "www.apple.com": "🚀 Jeemi",
	} {
		t.Run(domain, func(t *testing.T) {
			for _, rule := range config.Rules {
				parts := strings.Split(rule, ",")
				if parts[0] != "RULE-SET" {
					continue
				}
				provider := config.Providers[parts[1]]
				if provider.Behavior != "domain" && provider.Behavior != "classical" {
					continue
				}
				for _, match := range provider.Payload {
					if provider.Behavior == "classical" {
						if strings.HasPrefix(match, "DOMAIN-SUFFIX,") {
							match = "+." + strings.TrimPrefix(match, "DOMAIN-SUFFIX,")
						} else if strings.HasPrefix(match, "DOMAIN,") {
							match = strings.TrimPrefix(match, "DOMAIN,")
						} else {
							continue
						}
					}
					base := strings.TrimPrefix(match, "+.")
					if match == domain || (strings.HasPrefix(match, "+.") && (domain == base || strings.HasSuffix(domain, "."+base))) {
						if parts[2] != want {
							t.Fatalf("first rule targets %s, want %s", parts[2], want)
						}
						return
					}
				}
			}
			t.Fatal("business domain has no dedicated rule")
		})
	}
	for _, rule := range state.RuleSets {
		if strings.Contains(rule.Name, "Google") {
			for _, entry := range rule.Payload {
				if strings.Contains(entry, "youtube") || strings.Contains(entry, "googlevideo") || strings.Contains(entry, "ytimg") {
					t.Fatalf("Google repeated a YouTube entry: %s", entry)
				}
			}
		}
		if strings.Contains(rule.Name, "Instagram") {
			if (rule.Name != "Jeemi · Instagram 域名" || rule.Emoji != "📷") || strings.Contains(rule.PayloadYAML, "+.telegram.org") {
				t.Fatal("Instagram is not an independent rule set")
			}
		}
	}
}
