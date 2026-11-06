package presets

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
	"jeemi/internal/config/resources"
)

func (d *Definition) addDirectGroup(key, emoji string) {
	g := resources.StrategyGroup{ID: id("group:" + key), Name: "Jeemi · " + key, Emoji: emoji, Kind: resources.StrategyGroupKindRule,
		RuleOutput: resources.RuleOutputInline, Policy: resources.PolicyTarget{Mode: resources.PolicyDirect}, RuleSetReferences: []resources.RuleSetReference{}}
	d.Groups = append(d.Groups, g)
	d.Plan.StrategyGroupIDs = append(d.Plan.StrategyGroupIDs, g.ID)
}

func (d *Definition) addRule(owner, name, emoji, behavior, payload string) {
	r := resources.RuleSet{ID: id("rule:" + name), Name: emoji + " " + name, Description: "Jeemi 本地规则；可视化编辑，不从规则仓库更新。",
		SourceType: "inline", Behavior: behavior, Format: "yaml", PayloadYAML: payload, NoResolve: behavior == "ipcidr"}
	d.Rules = append(d.Rules, r)
	for i := range d.Groups {
		if d.Groups[i].ID == id("group:"+owner) {
			d.Groups[i].RuleSetReferences = append(d.Groups[i].RuleSetReferences, resources.RuleSetReference{RuleSetID: r.ID})
		}
	}
}

func (d *Definition) addListRule(owner, name, emoji, behavior string, sources []string) error {
	payload := []string{}
	seen := map[string]bool{}
	for _, source := range sources {
		contents, err := data.ReadFile("data/" + source + ".list")
		if err != nil {
			return fmt.Errorf("read bundled rule baseline")
		}
		for _, line := range strings.Split(string(contents), "\n") {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") && !seen[line] {
				payload = append(payload, line)
				seen[line] = true
			}
		}
	}
	contents, err := yaml.Marshal(struct {
		Payload []string `yaml:"payload"`
	}{payload})
	if err != nil {
		return err
	}
	d.addRule(owner, name, emoji, behavior, string(contents))
	return nil
}
