package resources

import "jeemi/internal/config/compose"

const documentVersion = 1

const (
	StrategyGroupKindRule     = "rule"
	StrategyGroupKindSelector = "selector"

	RuleOutputRuleSet = "rule-set"
	RuleOutputInline  = "inline"

	PolicyProxy  = "proxy"
	PolicyDirect = "direct"
	PolicyReject = "reject"
)

type NodeFilter struct {
	ProxyTypes   []string `json:"proxyTypes"`
	NamePatterns []string `json:"namePatterns"`
}

// RuleSetReference belongs to a reusable strategy group. The local
// configuration only orders strategy-group IDs; it never binds rule sets
// directly.
type RuleSetReference struct {
	RuleSetID string `json:"ruleSetId"`
}

// PolicyTarget is used by logical rule groups. Proxy is intentionally an
// abstract direction: the concrete selector belongs to each local
// configuration's routing plan, not to this reusable resource.
type PolicyTarget struct {
	Mode string `json:"mode"`
}

// StrategyGroup represents one reusable local policy group. Selector groups
// materialize a real mihomo proxy-group and may also own rules. Rule groups are
// Jeemi-only ordered rule bundles and never materialize a proxy-group.
type StrategyGroup struct {
	ID                string             `json:"id"`
	Name              string             `json:"name"`
	Description       string             `json:"description"`
	Kind              string             `json:"kind"`
	RuleOutput        string             `json:"ruleOutput"`
	RuleSetReferences []RuleSetReference `json:"ruleSetReferences"`
	Policy            PolicyTarget       `json:"policy"`

	// Selector-only fields.
	Type      string     `json:"type"`
	Emoji     string     `json:"emoji"`
	Icon      string     `json:"icon"`
	URL       string     `json:"url"`
	Interval  int        `json:"interval"`
	Tolerance int        `json:"tolerance"`
	Strategy  string     `json:"strategy"`
	Lazy      bool       `json:"lazy"`
	Filter    NodeFilter `json:"filter"`

	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type RuleSet struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	SourceType  string   `json:"sourceType"`
	Behavior    string   `json:"behavior"`
	Format      string   `json:"format"`
	URL         string   `json:"url"`
	Interval    int      `json:"interval"`
	NoResolve   bool     `json:"noResolve,omitempty"`
	PayloadYAML string   `json:"payloadYaml"`
	Payload     []string `json:"payload"`
	CreatedAt   string   `json:"createdAt"`
	UpdatedAt   string   `json:"updatedAt"`
}

type State struct {
	Directory      string          `json:"directory"`
	Revision       int             `json:"revision"`
	UpdatedAt      string          `json:"updatedAt"`
	StrategyGroups []StrategyGroup `json:"strategyGroups"`
	RuleSets       []RuleSet       `json:"ruleSets"`
}

const CurrentPlanVersion = 1

// LocalMatch is applied before the subscription's independent fallback override.
// SelectorID binds a referenced local selector by stable identity.
type LocalMatch struct {
	Mode       string `json:"mode"`
	SelectorID string `json:"selectorId"`
}

// Plan is the first-release local routing model. RuleStrategy controls the
// complete routing layer, including selector and provider composition.
type Plan struct {
	Version                  int              `json:"version"`
	Match                    LocalMatch       `json:"match"`
	StrategyGroupIDs         []string         `json:"strategyGroupIds"`
	DisabledStrategyGroupIDs []string         `json:"disabledStrategyGroupIds"`
	RulesDisabled            bool             `json:"rulesDisabled"`
	RuleStrategy             compose.Strategy `json:"ruleStrategy"`
	DefaultProxySelectorID   string           `json:"defaultProxySelectorId"`
}

func DefaultPlan() Plan {
	return Plan{
		Version:                  CurrentPlanVersion,
		Match:                    LocalMatch{Mode: "none"},
		StrategyGroupIDs:         []string{},
		DisabledStrategyGroupIDs: []string{},
		RuleStrategy:             compose.StrategyAppendBeforeTerminal,
	}
}

type libraryDocument struct {
	Version        int             `json:"version"`
	Revision       int             `json:"revision"`
	UpdatedAt      string          `json:"updatedAt"`
	StrategyGroups []StrategyGroup `json:"strategyGroups"`
	RuleSets       []RuleSet       `json:"ruleSets"`
}
