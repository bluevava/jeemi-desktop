// Package iconcatalog owns the offline Emoji options shared by Go and React.
package iconcatalog

import (
	_ "embed"
	"encoding/json"
)

//go:embed emojis.json
var catalogJSON []byte

type Category struct {
	ID    string      `json:"id"`
	ZH    string      `json:"zh"`
	EN    string      `json:"en"`
	Items [][3]string `json:"items"` // Emoji, Chinese keywords, English keywords.
}

func Values() []string {
	var categories []Category
	if err := json.Unmarshal(catalogJSON, &categories); err != nil {
		panic("invalid bundled Emoji catalog")
	}
	values := []string{}
	for _, category := range categories {
		for _, item := range category.Items {
			values = append(values, item[0])
		}
	}
	return values
}
