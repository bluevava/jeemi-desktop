package profile

import "jeemi/internal/configdocument"

var manifestFormat = configdocument.Format{
	VersionField: "manifestVersion", Version: manifestVersion,
	Rejected: map[string]bool{"targetCoreVersion": true},
}
var planFormat = configdocument.Format{
	VersionField: "version", Version: mergePlanVersion,
	ArrayKeys: map[string]string{"rules": "path", "transforms": "path"},
	Rejected: map[string]bool{
		"resources.selectorStrategy": true, "resources.ruleProviderStrategy": true, "resources.ruleSetBindings": true,
	},
	Enums: map[string][]string{
		"resources.ruleStrategy": {"", "prepend", "append_before_terminal", "replace"},
		"resources.match.mode":   {"", "none", "direct", "selector"},
		"rules[].strategy":       {"replace", "merge", "prepend", "append", "merge_by_name", "append_before_terminal"},
		"rules[].conflictPolicy": {"", "error", "use_local"},
	},
}
