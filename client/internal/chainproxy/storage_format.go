package chainproxy

import "jeemi/internal/configdocument"

var storageFormat = configdocument.Format{
	VersionField: "version", Version: Version,
	ArrayKeys: map[string]string{
		"groups": "id", "groups[].nodes": "id", "groups[].sources": "id", "groups[].sources[].nodes": "id",
	},
	Enums: map[string][]string{"groups[].kind": {"manual", "subscription"}},
}
