package resources

import "jeemi/internal/configdocument"

var storageFormat = configdocument.Format{
	VersionField: "version", Version: documentVersion,
	ArrayKeys: map[string]string{
		"strategyGroups": "id", "ruleSets": "id", "strategyGroups[].ruleSetReferences": "ruleSetId",
	},
	Rejected: map[string]bool{
		"strategyGroups[].filter.countries":              true,
		"strategyGroups[].ruleSetReferences[].noResolve": true,
		"strategyGroups[].policy.selectorId":             true,
		"legacyRuleProxyTargets":                         true,
	},
	Discarded: map[string]bool{"strategyGroups[].description": true, "ruleSets[].description": true},
	Enums: map[string][]string{
		"strategyGroups[].kind":        {"rule", "selector"},
		"strategyGroups[].ruleOutput":  {"", "rule-set", "inline"},
		"strategyGroups[].type":        {"", "select", "url-test", "fallback", "load-balance"},
		"strategyGroups[].policy.mode": {"", "proxy", "direct", "reject"},
		"ruleSets[].sourceType":        {"inline", "http"},
		"ruleSets[].behavior":          {"domain", "ipcidr", "classical"},
		"ruleSets[].format":            {"", "yaml", "text", "mrs"},
		"ruleSets[].platform":          {"", "windows", "darwin", "linux"},
	},
}
