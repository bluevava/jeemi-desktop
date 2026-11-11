package presets

import (
	"fmt"
	"net/netip"
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
	r := resources.RuleSet{ID: id("rule:" + name), Name: name, Emoji: emoji,
		SourceType: "inline", Behavior: behavior, Format: "yaml", PayloadYAML: payload, NoResolve: behavior == "ipcidr"}
	d.Rules = append(d.Rules, r)
	for i := range d.Groups {
		if d.Groups[i].ID == id("group:"+owner) {
			d.Groups[i].RuleSetReferences = append(d.Groups[i].RuleSetReferences, resources.RuleSetReference{RuleSetID: r.ID})
		}
	}
}

func (d *Definition) addListRule(owner, name, emoji, behavior string, sources []string, except ...string) error {
	payload := []string{}
	seen := map[string]bool{}
	var excluded []string
	for _, source := range except {
		contents, err := data.ReadFile("data/" + source + ".list")
		if err != nil {
			return fmt.Errorf("read bundled rule exclusions")
		}
		excluded = append(excluded, strings.Fields(string(contents))...)
	}
	for _, source := range sources {
		contents, err := data.ReadFile("data/" + source + ".list")
		if err != nil {
			return fmt.Errorf("read bundled rule baseline")
		}
		for _, line := range strings.Split(string(contents), "\n") {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") && !seen[line] && !coveredDomain(line, excluded) {
				seen[line] = true
				if behavior == "classical" {
					line, err = classicalListEntry(line)
					if err != nil {
						return err
					}
				}
				payload = append(payload, line)
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

// A single application can own domain and IP snapshots in one editable set.
// Keep no-resolve on IP entries only, as required by classical providers.
func classicalListEntry(line string) (string, error) {
	if prefix, err := netip.ParsePrefix(line); err == nil {
		kind := "IP-CIDR"
		if prefix.Addr().Is6() {
			kind = "IP-CIDR6"
		}
		return kind + "," + line + ",no-resolve", nil
	}
	if strings.ContainsAny(strings.TrimPrefix(line, "+."), "+*?,/\\") {
		return "", fmt.Errorf("unsupported domain pattern in bundled classical rules")
	}
	if strings.HasPrefix(line, "+.") {
		return "DOMAIN-SUFFIX," + strings.TrimPrefix(line, "+."), nil
	}
	return "DOMAIN," + line, nil
}

// Remove dedicated YouTube entries from Google's upstream superset while
// preserving the original, checksummed snapshots. Broader Google API domains
// remain useful, so IM/Video are also placed before Google in the routing plan.
func coveredDomain(candidate string, exclusions []string) bool {
	name := strings.TrimPrefix(candidate, "+.")
	for _, rule := range exclusions {
		if candidate == rule {
			return true
		}
		if strings.HasPrefix(rule, "+.") {
			suffix := strings.TrimPrefix(rule, "+.")
			if name == suffix || strings.HasSuffix(name, "."+suffix) {
				return true
			}
		}
	}
	return false
}
