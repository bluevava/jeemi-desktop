package profile

import (
	"jeemi/internal/config/compose"
	configresources "jeemi/internal/config/resources"
)

const (
	manifestVersion  = 1
	mergePlanVersion = 1
)

type LocalConfigField struct {
	Path           string                 `json:"path"`
	ValueYAML      string                 `json:"valueYaml"`
	Strategy       compose.Strategy       `json:"strategy"`
	ConflictPolicy compose.ConflictPolicy `json:"conflictPolicy"`
}

type SaveLocalConfigInput struct {
	ID           string               `json:"id"`
	Name         string               `json:"name"`
	Description  string               `json:"description"`
	Fields       []LocalConfigField   `json:"fields"`
	ResourcePlan configresources.Plan `json:"resourcePlan"`
}

type LocalConfigSummary struct {
	ID                     string `json:"id"`
	Name                   string `json:"name"`
	Description            string `json:"description"`
	SchemaVersion          string `json:"schemaVersion"`
	Revision               int    `json:"revision"`
	CreatedAt              string `json:"createdAt"`
	UpdatedAt              string `json:"updatedAt"`
	EnabledFieldCount      int    `json:"enabledFieldCount"`
	RuleProviderCount      int    `json:"ruleProviderCount"`
	StrategyGroupCount     int    `json:"strategyGroupCount"`
	StaticValidationStatus string `json:"staticValidationStatus"`
}

type LocalConfig struct {
	LocalConfigSummary
	Fields       []LocalConfigField   `json:"fields"`
	OverlayYAML  string               `json:"overlayYaml"`
	ResourcePlan configresources.Plan `json:"resourcePlan"`
}

type LocalConfigState struct {
	Directory string               `json:"directory"`
	Configs   []LocalConfigSummary `json:"configs"`
}

type LocalConfigPreview struct {
	SchemaVersion       string               `json:"schemaVersion"`
	EnabledFieldCount   int                  `json:"enabledFieldCount"`
	ProxyOverrideCount  int                  `json:"proxyOverrideCount"`
	OverlayYAML         string               `json:"overlayYaml"`
	RedactedOverlayYAML string               `json:"redactedOverlayYaml"`
	MergePlan           []compose.Rule       `json:"mergePlan"`
	Fields              []LocalConfigField   `json:"fields"`
	ResourcePlan        configresources.Plan `json:"resourcePlan"`
}

type manifestDocument struct {
	ManifestVersion        int    `json:"manifestVersion"`
	ID                     string `json:"id"`
	Name                   string `json:"name"`
	Description            string `json:"description"`
	SchemaVersion          string `json:"schemaVersion"`
	Revision               int    `json:"revision"`
	CreatedAt              string `json:"createdAt"`
	UpdatedAt              string `json:"updatedAt"`
	EnabledFieldCount      int    `json:"enabledFieldCount"`
	RuleProviderCount      int    `json:"ruleProviderCount"`
	StrategyGroupCount     int    `json:"strategyGroupCount"`
	StaticValidationStatus string `json:"staticValidationStatus"`
}

type mergePlanDocument struct {
	Version    int                      `json:"version"`
	Rules      []compose.Rule           `json:"rules"`
	Transforms []localFieldTransformDoc `json:"transforms,omitempty"`
	Resources  configresources.Plan     `json:"resources"`
}

type localFieldTransformDoc struct {
	Path      string `json:"path"`
	ValueYAML string `json:"valueYaml"`
}
