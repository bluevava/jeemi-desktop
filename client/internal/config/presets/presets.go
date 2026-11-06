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
	sum := sha256.Sum256([]byte("jeemi-offline-preset-v1:" + key))
	return fmt.Sprintf("%x", sum[:16])
}

func Load(kind string) (Definition, error) {
	if kind != "general" && kind != "process" {
		return Definition{}, fmt.Errorf("unknown built-in configuration")
	}
	d := Definition{Groups: []resources.StrategyGroup{}, Rules: []resources.RuleSet{}, Plan: resources.DefaultPlan()}
	d.Plan.RuleStrategy = compose.StrategyReplace
	for _, spec := range []struct{ name, emoji string }{{"Jeemi", "🚀"}, {"AI", "🤖"}, {"Telegram", "✈️"}, {"Video", "🎬"}} {
		g := resources.StrategyGroup{ID: id("group:" + spec.name), Name: spec.name, Emoji: spec.emoji, Kind: resources.StrategyGroupKindSelector, Type: "select", AutoSelect: true,
			RuleOutput: resources.RuleOutputRuleSet, RuleSetReferences: []resources.RuleSetReference{}, URL: "http://www.google.com/generate_204", Interval: 300, Tolerance: 50, Lazy: true,
			Filter: resources.NodeFilter{ProxyTypes: []string{}, NamePatterns: []string{}}}
		if spec.name == "AI" {
			g.Filter.NamePatterns = []string{"!🇭🇰", "!香港", "!HK", "!Hong Kong", "!HongKong", "!Hong-Kong"}
		}
		d.Groups = append(d.Groups, g)
	}
	for _, spec := range []struct {
		owner, name, emoji, behavior string
		sources                      []string
	}{
		{"AI", "Jeemi · AI 域名", "🤖", "domain", []string{"ai"}},
		{"Telegram", "Jeemi · Telegram / Instagram 域名", "✈️", "domain", []string{"telegram-domain", "instagram"}},
		{"Telegram", "Jeemi · Telegram IP", "✈️", "ipcidr", []string{"telegram-ip"}},
		{"Video", "Jeemi · YouTube / Netflix 域名", "🎬", "domain", []string{"youtube", "netflix-domain"}},
		{"Video", "Jeemi · Netflix IP", "🎬", "ipcidr", []string{"netflix-ip"}},
	} {
		if err := d.addListRule(spec.owner, spec.name, spec.emoji, spec.behavior, spec.sources); err != nil {
			return Definition{}, err
		}
	}
	// Jeemi leads the selector list; its fallback rule is emitted separately last.
	d.Plan.StrategyGroupIDs = []string{id("group:Jeemi"), id("group:AI"), id("group:Telegram"), id("group:Video")}
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
		d.Description = "专用站点优先，系统更新、国内应用进程与系统内置浏览器直连，其余走 Jeemi；不添加 CN 地域规则，需要可用的进程查询。"
		d.addDirectGroup("国内应用直连", "💻")
		payload, err := data.ReadFile("data/process-direct.yaml")
		if err != nil {
			return Definition{}, fmt.Errorf("read bundled process rules")
		}
		d.addRule("国内应用直连", "Jeemi · 国内应用进程", "💻", "classical", string(payload))
		// Keep resource IDs stable while updating the displayed browser name.
		d.addDirectGroup("默认浏览器", "🌐")
		d.Groups[len(d.Groups)-1].Name = "Jeemi · 系统浏览器直连"
		d.addRule("默认浏览器", "Jeemi · 默认浏览器", "🌐", "classical", "# 固定匹配系统内置浏览器：Windows Edge、Linux Firefox、macOS Safari。\n# 不读取系统默认浏览器设置；关闭内置匹配后可手填进程规则。\npayload: []\n")
		d.Rules[len(d.Rules)-1].Name = "🌐 Jeemi · 系统内置浏览器"
		d.Rules[len(d.Rules)-1].DefaultBrowser = true
	}
	d.Plan.DefaultProxySelectorID = id("group:Jeemi")
	d.Plan.Match = resources.LocalMatch{Mode: "selector", SelectorID: id("group:Jeemi")}
	return d, nil
}
