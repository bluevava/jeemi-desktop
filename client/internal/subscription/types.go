package subscription

import "jeemi/internal/config/fallbackoverride"

type SourceKind string

const (
	SourceURL  SourceKind = "url"
	SourceFile SourceKind = "file"
)

type ImportMethod string

const (
	ImportURL      ImportMethod = "url"
	ImportFile     ImportMethod = "file"
	ImportQRImage  ImportMethod = "qr_image"
	ImportQRScreen ImportMethod = "qr_screen"
)

type Format string

const (
	FormatYAML Format = "yaml"
	FormatJSON Format = "json"
	FormatText Format = "txt"
)

type IconKind string

const (
	IconNone  IconKind = ""
	IconEmoji IconKind = "emoji"
	IconURL   IconKind = "url"
)

// RemoteProfile contains normalised, non-sensitive metadata advertised by a
// subscription HTTP response. Raw header values are never persisted.
type RemoteProfile struct {
	HasTraffic          bool   `json:"hasTraffic"`
	UploadBytes         int64  `json:"uploadBytes"`
	DownloadBytes       int64  `json:"downloadBytes"`
	TotalBytes          int64  `json:"totalBytes"`
	ExpiresAt           int64  `json:"expiresAt"`
	UpdateIntervalHours int    `json:"updateIntervalHours"`
	ObservedAt          string `json:"observedAt"`
}

type CompositionSummary struct {
	Status             string `json:"status"`
	ProxyCount         int    `json:"proxyCount"`
	SelectorCount      int    `json:"selectorCount"`
	ProxyProviderCount int    `json:"proxyProviderCount"`
	RuleProviderCount  int    `json:"ruleProviderCount"`
}

type Summary struct {
	Fallback                     fallbackoverride.Selection `json:"fallback"`
	FallbackResetTarget          string                     `json:"fallbackResetTarget"`
	ID                           string                     `json:"id"`
	Name                         string                     `json:"name"`
	Description                  string                     `json:"description"`
	SourceKind                   SourceKind                 `json:"sourceKind"`
	ImportMethod                 ImportMethod               `json:"importMethod"`
	SourceLabel                  string                     `json:"sourceLabel"`
	Format                       Format                     `json:"format"`
	IconKind                     IconKind                   `json:"iconKind"`
	Icon                         string                     `json:"icon"`
	LocalConfigID                string                     `json:"localConfigId"`
	LocalScriptID                string                     `json:"localScriptId"`
	RuleProviderOverrideRevision int                        `json:"ruleProviderOverrideRevision"`
	FallbackOverrideRevision     int                        `json:"fallbackOverrideRevision"`
	DisabledRuleProviders        []string                   `json:"disabledRuleProviders"`
	CurrentRevisionID            string                     `json:"currentRevisionId"`
	RevisionCount                int                        `json:"revisionCount"`
	SizeBytes                    int64                      `json:"sizeBytes"`
	CreatedAt                    string                     `json:"createdAt"`
	UpdatedAt                    string                     `json:"updatedAt"`
	LastFetchedAt                string                     `json:"lastFetchedAt"`
	RemoteProfile                RemoteProfile              `json:"remoteProfile"`
	Composition                  CompositionSummary         `json:"composition"`
}

type Detail struct {
	Summary
	SourceURL string `json:"sourceUrl"`
}

type State struct {
	Directory              string      `json:"directory"`
	Subscriptions          []Summary   `json:"subscriptions"`
	SelectedSubscriptionID string      `json:"selectedSubscriptionId"`
	Preferences            Preferences `json:"preferences"`
	Projection             *Projection `json:"projection"`
}

type Preferences struct {
	SelectorDensity      string `json:"selectorDensity"`
	DelayTestConcurrency int    `json:"delayTestConcurrency"`
	ConnectionResetMode  string `json:"connectionResetMode"`
	SelectorSortMode     string `json:"selectorSortMode"`
	SelectorViewMode     string `json:"selectorViewMode"`
}

type PreferencesInput struct {
	SelectorDensity      string `json:"selectorDensity"`
	DelayTestConcurrency int    `json:"delayTestConcurrency"`
	ConnectionResetMode  string `json:"connectionResetMode"`
}

type SelectorDisplayPreferencesInput struct {
	SelectorSortMode string `json:"selectorSortMode"`
	SelectorViewMode string `json:"selectorViewMode"`
}

type SelectorMember struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	Source       string `json:"source"`
	ProviderName string `json:"providerName"`
}

type Selector struct {
	Name                    string           `json:"name"`
	Icon                    string           `json:"icon"`
	Hidden                  bool             `json:"hidden"`
	Type                    string           `json:"type"`
	DefaultSelection        string           `json:"defaultSelection"`
	Members                 []SelectorMember `json:"members"`
	ProviderNames           []string         `json:"providerNames"`
	UnresolvedProviderNames []string         `json:"unresolvedProviderNames"`
	ReferencedByRules       bool             `json:"referencedByRules"`
}

type RuleProvider struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Behavior string `json:"behavior"`
	Format   string `json:"format"`
	Enabled  bool   `json:"enabled"`
}

// Projection is an offline view of subscription + local configuration
// composition. It is not a mihomo-validated or active runtime generation.
type Projection struct {
	Fallback                     fallbackoverride.State `json:"fallback"`
	SubscriptionID               string                 `json:"subscriptionId"`
	RevisionID                   string                 `json:"revisionId"`
	LocalConfigID                string                 `json:"localConfigId"`
	LocalConfigRevision          int                    `json:"localConfigRevision"`
	LocalScriptID                string                 `json:"localScriptId"`
	LocalScriptRevision          int                    `json:"localScriptRevision"`
	RuleProviderOverrideRevision int                    `json:"ruleProviderOverrideRevision"`
	FallbackOverrideRevision     int                    `json:"fallbackOverrideRevision"`
	GeoDataFingerprint           string                 `json:"geoDataFingerprint"`
	Status                       string                 `json:"status"`
	CoreValidationStatus         string                 `json:"coreValidationStatus"`
	Summary                      CompositionSummary     `json:"summary"`
	Selectors                    []Selector             `json:"selectors"`
	Proxies                      []SelectorMember       `json:"proxies"`
	RuleProviders                []RuleProvider         `json:"ruleProviders"`
	Warnings                     []string               `json:"warnings"`
}

type ImportURLInput struct {
	Name         string       `json:"name"`
	Description  string       `json:"description"`
	SourceURL    string       `json:"sourceUrl"`
	ImportMethod ImportMethod `json:"importMethod"`
	IconKind     IconKind     `json:"iconKind"`
	Icon         string       `json:"icon"`
}

type UpdateInput struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	SourceURL   string   `json:"sourceUrl"`
	IconKind    IconKind `json:"iconKind"`
	Icon        string   `json:"icon"`
}

type IconDetectionResult struct {
	Found    bool     `json:"found"`
	IconKind IconKind `json:"iconKind"`
	Icon     string   `json:"icon"`
}

type TextView struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Format     Format `json:"format"`
	RevisionID string `json:"revisionId"`
	Contents   string `json:"contents"`
}

type DialogLabels struct {
	Title      string `json:"title"`
	FilterName string `json:"filterName"`
}

type DialogImportResult struct {
	Cancelled bool  `json:"cancelled"`
	State     State `json:"state"`
}

// Cancelled/failed capture leaves the existing subscription state untouched.
// Screenshot data, local paths and decoded URLs never cross this boundary.
type ScreenImportResult struct {
	Cancelled bool   `json:"cancelled"`
	ErrorCode string `json:"errorCode,omitempty"`
	State     *State `json:"state,omitempty"`
}

type revisionDocument struct {
	ID        string `json:"id"`
	FileName  string `json:"fileName"`
	Format    Format `json:"format"`
	SHA256    string `json:"sha256"`
	SizeBytes int64  `json:"sizeBytes"`
	CreatedAt string `json:"createdAt"`
}

type metadataDocument struct {
	Fallback                     fallbackoverride.Selection `json:"fallback"`
	FallbackResetTarget          string                     `json:"fallbackResetTarget,omitempty"`
	ManifestVersion              int                        `json:"manifestVersion"`
	ID                           string                     `json:"id"`
	Name                         string                     `json:"name"`
	Description                  string                     `json:"description"`
	SourceKind                   SourceKind                 `json:"sourceKind"`
	ImportMethod                 ImportMethod               `json:"importMethod"`
	SourceURL                    string                     `json:"sourceUrl"`
	OriginalFileName             string                     `json:"originalFileName"`
	IconKind                     IconKind                   `json:"iconKind"`
	Icon                         string                     `json:"icon"`
	LocalConfigID                string                     `json:"localConfigId"`
	LocalScriptID                string                     `json:"localScriptId"`
	RuleProviderOverrideRevision int                        `json:"ruleProviderOverrideRevision"`
	FallbackOverrideRevision     int                        `json:"fallbackOverrideRevision"`
	DisabledRuleProviders        []string                   `json:"disabledRuleProviders"`
	CurrentRevisionID            string                     `json:"currentRevisionId"`
	Revisions                    []revisionDocument         `json:"revisions"`
	CreatedAt                    string                     `json:"createdAt"`
	UpdatedAt                    string                     `json:"updatedAt"`
	LastFetchedAt                string                     `json:"lastFetchedAt"`
	RemoteProfile                RemoteProfile              `json:"remoteProfile"`
}
