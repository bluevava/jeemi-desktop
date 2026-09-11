// Package configtransfer defines the portable, versioned .json format.
// Packages contain saved local inputs, never subscription sources or runtime state.
package configtransfer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"jeemi/internal/config/resources"
	"jeemi/internal/localscript"
	"jeemi/internal/profile"
)

const (
	Extension  = ".json"
	ConfigKind = "local-config"
	ScriptKind = "local-script"
	MaxBytes   = 16 << 20
	format     = "jeemi-local-package"
	version    = 1
)

type Config struct {
	Name           string                     `json:"name"`
	Description    string                     `json:"description"`
	Fields         []profile.LocalConfigField `json:"fields"`
	ResourcePlan   resources.Plan             `json:"resourcePlan"`
	StrategyGroups []resources.StrategyGroup  `json:"strategyGroups"`
	RuleSets       []resources.RuleSet        `json:"ruleSets"`
}

type Script struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Contents    string `json:"contents"`
	SourceURL   string `json:"sourceUrl,omitempty"`
}

type Package struct {
	Format  string  `json:"format"`
	Version int     `json:"version"`
	Kind    string  `json:"kind"`
	Config  *Config `json:"config,omitempty"`
	Script  *Script `json:"script,omitempty"`
}

func FromConfig(local profile.LocalConfig, state resources.State) (Package, error) {
	groups, rules, err := resources.ExportGraph(local.ResourcePlan, state)
	if err != nil {
		return Package{}, err
	}
	return Package{Format: format, Version: version, Kind: ConfigKind, Config: &Config{
		Name: local.Name, Description: local.Description, Fields: local.Fields, ResourcePlan: local.ResourcePlan, StrategyGroups: groups, RuleSets: rules,
	}}, nil
}

func FromScript(script localscript.Script) Package {
	return Package{Format: format, Version: version, Kind: ScriptKind, Script: &Script{Name: script.Name, Description: script.Description, Contents: script.Contents, SourceURL: script.SourceURL}}
}

func Encode(p Package) ([]byte, error) {
	if err := p.validate(p.Kind); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode Jeemi JSON package")
	}
	data = append(data, '\n')
	if len(data) > MaxBytes {
		return nil, fmt.Errorf("Jeemi JSON package exceeds 16 MiB")
	}
	return data, nil
}

func Decode(data []byte, expectedKind string) (Package, error) {
	if len(data) == 0 || len(data) > MaxBytes || !utf8.Valid(data) {
		return Package{}, fmt.Errorf("Jeemi JSON package must be UTF-8 JSON within 16 MiB")
	}
	// Reject duplicate keys (including case variants) before struct decoding.
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := uniqueJSONValue(decoder, 0); err != nil {
		return Package{}, fmt.Errorf("invalid Jeemi package JSON: %w", err)
	}
	if _, err := decoder.Token(); err != io.EOF {
		return Package{}, fmt.Errorf("Jeemi JSON package contains trailing data")
	}
	var p Package
	decoder = json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&p); err != nil {
		return Package{}, fmt.Errorf("Jeemi JSON package contains invalid or unsupported fields")
	}
	if err := p.validate(expectedKind); err != nil {
		return Package{}, err
	}
	return p, nil
}

func (p Package) validate(expectedKind string) error {
	if p.Format != format || p.Version != version {
		return fmt.Errorf("unsupported Jeemi JSON package format or version")
	}
	if p.Kind != expectedKind {
		return fmt.Errorf("Jeemi JSON package type does not match the selected card")
	}
	switch p.Kind {
	case ConfigKind:
		if p.Config == nil || p.Script != nil {
			return fmt.Errorf("Jeemi JSON configuration package is incomplete")
		}
		if p.Config.ResourcePlan.Version != resources.CurrentPlanVersion {
			return fmt.Errorf("unsupported local rule plan version in Jeemi JSON package")
		}
	case ScriptKind:
		if p.Script == nil || p.Config != nil {
			return fmt.Errorf("Jeemi JSON script package is incomplete")
		}
	default:
		return fmt.Errorf("unsupported Jeemi JSON package type")
	}
	return nil
}

func uniqueJSONValue(decoder *json.Decoder, depth int) error {
	if depth > 48 {
		return fmt.Errorf("nesting is too deep")
	}
	token, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("malformed JSON")
	}
	delimiter, compound := token.(json.Delim)
	if !compound {
		return nil
	}
	if delimiter != '{' && delimiter != '[' {
		return fmt.Errorf("unexpected delimiter")
	}
	keys := map[string]bool{}
	for decoder.More() {
		if delimiter == '{' {
			key, err := decoder.Token()
			if err != nil {
				return fmt.Errorf("malformed object")
			}
			name, ok := key.(string)
			if !ok {
				return fmt.Errorf("invalid object key")
			}
			name = strings.ToLower(name)
			if keys[name] {
				return fmt.Errorf("duplicate object key")
			}
			keys[name] = true
		}
		if err := uniqueJSONValue(decoder, depth+1); err != nil {
			return err
		}
	}
	if _, err := decoder.Token(); err != nil {
		return fmt.Errorf("unclosed JSON value")
	}
	return nil
}
