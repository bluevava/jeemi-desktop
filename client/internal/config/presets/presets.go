// Package presets contains editable local starting points. Nodes always come
// from the bound subscription; creating or composing presets never fetches rules.
package presets

import (
	"crypto/sha256"
	"embed"
	"fmt"

	"jeemi/internal/config/compose"
	"jeemi/internal/config/resources"
)

//go:embed data/*
var data embed.FS

type Definition struct {
	Name        string
	Description string
	Plan        resources.Plan
	Groups      []resources.StrategyGroup
	Rules       []resources.RuleSet
}

func id(key string) string {
	// A new template edition must not attach new rules to previously edited groups.
	// This is a resource identity, not a storage schema version or migration.
	sum := sha256.Sum256([]byte("jeemi-offline-preset-v1:media-im:" + key))
	return fmt.Sprintf("%x", sum[:16])
}

func Load(kind string, platform string) (Definition, error) {
	if kind != "general" && kind != "process" {
		return Definition{}, fmt.Errorf("unknown built-in configuration")
	}
	d := Definition{Groups: []resources.StrategyGroup{}, Rules: []resources.RuleSet{}, Plan: resources.DefaultPlan()}
	d.Plan.RuleStrategy = compose.StrategyReplace
	for _, spec := range []struct{ name, emoji string }{{"Jeemi", "🚀"}, {"AI+Google", "🤖"}, {"IM", "✈️"}, {"Video", "🎬"}} {
		g := resources.StrategyGroup{ID: id("group:" + spec.name), Name: spec.name, Emoji: spec.emoji, Kind: resources.StrategyGroupKindSelector, Type: "select", AutoSelect: true,
			RuleOutput: resources.RuleOutputRuleSet, RuleSetReferences: []resources.RuleSetReference{}, URL: "http://www.google.com/generate_204", Interval: 300, Tolerance: 50, Lazy: true,
			Filter: resources.NodeFilter{ProxyTypes: []string{}, NamePatterns: []string{}}}
		if spec.name == "AI+Google" {
			g.Filter.NamePatterns = []string{"!🇭🇰", "!香港", "!HK", "!Hong Kong", "!HongKong", "!Hong-Kong"}
		}
		d.Groups = append(d.Groups, g)
	}
	for _, spec := range []struct {
		owner, name, emoji, behavior string
		sources                      []string
	}{
		{"Jeemi", "Jeemi · GitHub 域名", "🚀", "domain", []string{"github"}},
		{"Jeemi", "Jeemi · Apple 域名", "🍎", "domain", []string{"apple"}},
		{"AI+Google", "Jeemi · AI 域名", "🤖", "domain", []string{"ai"}},
		{"AI+Google", "Jeemi · Google 域名", "🔍", "domain", []string{"google"}},
		{"IM", "Jeemi · Telegram 域名", "✈️", "domain", []string{"telegram-domain"}},
		{"IM", "Jeemi · Telegram IP", "✈️", "ipcidr", []string{"telegram-ip"}},
		{"IM", "Jeemi · Discord 域名", "💬", "domain", []string{"discord"}},
		{"IM", "Jeemi · WhatsApp 域名", "💬", "domain", []string{"whatsapp"}},
		{"IM", "Jeemi · LINE 域名", "💬", "domain", []string{"line"}},
		{"Video", "Jeemi · Netflix", "🎬", "classical", []string{"netflix-domain", "netflix-ip"}},
		{"Video", "Jeemi · Spotify 域名", "🎵", "domain", []string{"spotify"}},
		{"Video", "Jeemi · X-com 域名", "📱", "domain", []string{"twitter"}},
		{"Video", "Jeemi · Instagram 域名", "📷", "domain", []string{"instagram"}},
		{"Video", "Jeemi · YouTube 域名", "🎬", "domain", []string{"youtube"}},
	} {
		var except []string
		if spec.name == "Jeemi · Google 域名" {
			except = []string{"youtube"}
		}
		if err := d.addListRule(spec.owner, spec.name, spec.emoji, spec.behavior, spec.sources, except...); err != nil {
			return Definition{}, err
		}
	}
	// Specific media/chat domains precede Google's broader API domains.
	// Jeemi stays first and MATCH is emitted last.
	d.Plan.StrategyGroupIDs = []string{id("group:Jeemi"), id("group:IM"), id("group:Video"), id("group:AI+Google")}
	// Both reference configurations explicitly exempt these update endpoints.
	d.addDirectGroup("系统更新直连", "☁️")
	d.addRule("系统更新直连", "Jeemi · 系统更新域名", "☁️", "classical", "# 系统更新直连；专用站点规则优先。\npayload:\n  - DOMAIN-SUFFIX,windowsupdate.com\n  - DOMAIN-SUFFIX,dl.delivery.mp.microsoft.com\n")
	if kind == "general" {
		d.Name = "🌐 Jeemi · 常规通用"
		d.Description = "专用站点优先，系统更新与 CN 国内流量直连，其余走 Jeemi；国内规则 inline 展开，使用主页安装的本地 GEO 库。"
		d.addDirectGroup("国内直连", "🏠")
		d.addRule("国内直连", "Jeemi · 国内地域（CN）", "🏠", "classical", "# 使用主页 GEO 管理中的本地数据库；按域名后 IP 的顺序匹配。\npayload:\n  - GEOSITE,CN\n  - GEOIP,CN\n")
	} else {
		d.Name = "💻 Jeemi · 进程规则"
		d.Description = "专用站点优先，系统更新、国内应用与指定浏览器进程直连（Windows Edge、Linux Firefox、macOS Chrome），其余走 Jeemi；不添加 CN 地域规则，需要可用的进程查询。"
		platformName := map[string]string{"windows": "Windows", "darwin": "macOS", "linux": "Linux"}[platform]
		if platformName == "" {
			return Definition{}, fmt.Errorf("unsupported process preset platform")
		}
		owner := platformName + " 应用直连"
		d.addDirectGroup(owner, "💻")
		payload, err := data.ReadFile("data/process-direct-" + platform + ".yaml")
		if err != nil {
			return Definition{}, fmt.Errorf("read bundled process rules")
		}
		d.addRule(owner, "Jeemi · "+platformName+" 应用进程", "💻", "classical", string(payload))
		// Platform-scoped group AND rule identities avoid attaching a resource
		// owned by a previously created multi-platform group to a second owner.
		rule := &d.Rules[len(d.Rules)-1]
		rule.ID = id("platform-process-rule:" + platform)
		rule.Platform = platform
		d.Groups[len(d.Groups)-1].RuleSetReferences = []resources.RuleSetReference{{RuleSetID: rule.ID}}
	}
	d.Plan.DefaultProxySelectorID = id("group:Jeemi")
	d.Plan.Match = resources.LocalMatch{Mode: "selector", SelectorID: id("group:Jeemi")}
	return d, nil
}
