package subscription

import "jeemi/internal/configdocument"

var storageFormat = configdocument.Format{
	VersionField: "manifestVersion", Version: manifestVersion,
	ArrayKeys: map[string]string{"revisions": "id"},
	Enums: map[string][]string{
		"sourceKind":         {"url", "file"},
		"importMethod":       {"url", "file", "qr_image", "qr_screen"},
		"revisions[].format": {"yaml", "json", "txt"},
	},
}
