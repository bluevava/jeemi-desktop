package geodata

import (
	"fmt"
	"net/url"
	"strings"
)

type Kind string

const (
	KindGeoIPMMDB Kind = "geoip-mmdb"
	KindGeoIPDAT  Kind = "geoip-dat"
	KindGeoSite   Kind = "geosite"
	KindASN       Kind = "asn"

	GeoIPModeMMDB = "mmdb"
	GeoIPModeDAT  = "dat"

	LoaderMemoryConservative = "memconservative"
	LoaderStandard           = "standard"

	SourceOfficial = "official"
	SourceCustom   = "custom"

	HealthReady   = "ready"
	HealthPending = "pending"
	HealthMissing = "missing"
	HealthInvalid = "invalid"

	OfficialReleasesURL = "https://github.com/MetaCubeX/meta-rules-dat/releases"
)

type URLs struct {
	GeoIPMMDB string `json:"geoIpMmdb"`
	GeoIPDAT  string `json:"geoIpDat"`
	GeoSite   string `json:"geoSite"`
	ASN       string `json:"asn"`
}

type Preferences struct {
	GeoIPMode  string `json:"geoIpMode"`
	Loader     string `json:"loader"`
	Source     string `json:"source"`
	CustomURLs URLs   `json:"customUrls"`
}

func OfficialURLs() URLs {
	const base = "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/"
	return URLs{
		GeoIPMMDB: base + "geoip.metadb",
		GeoIPDAT:  base + "geoip.dat",
		GeoSite:   base + "geosite.dat",
		ASN:       base + "GeoLite2-ASN.mmdb",
	}
}

func DefaultPreferences() Preferences {
	return Preferences{
		GeoIPMode:  GeoIPModeMMDB,
		Loader:     LoaderMemoryConservative,
		Source:     SourceOfficial,
		CustomURLs: OfficialURLs(),
	}
}

func Normalize(preferences Preferences) Preferences {
	defaults := DefaultPreferences()
	if preferences.GeoIPMode != GeoIPModeMMDB && preferences.GeoIPMode != GeoIPModeDAT {
		preferences.GeoIPMode = defaults.GeoIPMode
	}
	if preferences.Loader != LoaderMemoryConservative && preferences.Loader != LoaderStandard {
		preferences.Loader = defaults.Loader
	}
	if preferences.Source != SourceOfficial && preferences.Source != SourceCustom {
		preferences.Source = defaults.Source
	}
	allURLsMissing := strings.TrimSpace(preferences.CustomURLs.GeoIPMMDB) == "" &&
		strings.TrimSpace(preferences.CustomURLs.GeoIPDAT) == "" &&
		strings.TrimSpace(preferences.CustomURLs.GeoSite) == "" &&
		strings.TrimSpace(preferences.CustomURLs.ASN) == ""
	fillMissingURLs := preferences.Source == SourceOfficial || allURLsMissing
	if fillMissingURLs && strings.TrimSpace(preferences.CustomURLs.GeoIPMMDB) == "" {
		preferences.CustomURLs.GeoIPMMDB = defaults.CustomURLs.GeoIPMMDB
	}
	if fillMissingURLs && strings.TrimSpace(preferences.CustomURLs.GeoIPDAT) == "" {
		preferences.CustomURLs.GeoIPDAT = defaults.CustomURLs.GeoIPDAT
	}
	if fillMissingURLs && strings.TrimSpace(preferences.CustomURLs.GeoSite) == "" {
		preferences.CustomURLs.GeoSite = defaults.CustomURLs.GeoSite
	}
	if fillMissingURLs && strings.TrimSpace(preferences.CustomURLs.ASN) == "" {
		preferences.CustomURLs.ASN = defaults.CustomURLs.ASN
	}
	return preferences
}

func ValidatePreferences(preferences Preferences) error {
	if preferences.GeoIPMode != GeoIPModeMMDB && preferences.GeoIPMode != GeoIPModeDAT {
		return fmt.Errorf("GEO GeoIP mode is invalid")
	}
	if preferences.Loader != LoaderMemoryConservative && preferences.Loader != LoaderStandard {
		return fmt.Errorf("GEO data loader is invalid")
	}
	if preferences.Source != SourceOfficial && preferences.Source != SourceCustom {
		return fmt.Errorf("GEO data source is invalid")
	}
	if preferences.Source == SourceCustom {
		for _, kind := range allKinds() {
			value, err := preferences.URLFor(kind)
			if err != nil {
				return err
			}
			parsed, err := url.Parse(value)
			if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
				return fmt.Errorf("custom GEO data URL for %s must be an HTTPS URL without user information", kind)
			}
		}
	}
	return nil
}

func (preferences Preferences) URLFor(kind Kind) (string, error) {
	if !validKind(kind) {
		return "", fmt.Errorf("GEO data kind is invalid")
	}
	urls := OfficialURLs()
	if preferences.Source == SourceCustom {
		urls = preferences.CustomURLs
	}
	var value string
	switch kind {
	case KindGeoIPMMDB:
		value = urls.GeoIPMMDB
	case KindGeoIPDAT:
		value = urls.GeoIPDAT
	case KindGeoSite:
		value = urls.GeoSite
	case KindASN:
		value = urls.ASN
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("GEO data URL for %s is empty", kind)
	}
	return value, nil
}

func SelectedKinds(preferences Preferences) []Kind {
	geoIP := KindGeoIPMMDB
	if preferences.GeoIPMode == GeoIPModeDAT {
		geoIP = KindGeoIPDAT
	}
	return []Kind{geoIP, KindGeoSite, KindASN}
}

func allKinds() []Kind {
	return []Kind{KindGeoIPMMDB, KindGeoIPDAT, KindGeoSite, KindASN}
}

func validKind(kind Kind) bool {
	for _, candidate := range allKinds() {
		if kind == candidate {
			return true
		}
	}
	return false
}

func CanonicalName(kind Kind) (string, error) {
	switch kind {
	case KindGeoIPMMDB:
		return "geoip.metadb", nil
	case KindGeoIPDAT:
		return "GeoIP.dat", nil
	case KindGeoSite:
		return "GeoSite.dat", nil
	case KindASN:
		return "ASN.mmdb", nil
	default:
		return "", fmt.Errorf("GEO data kind is invalid")
	}
}

type Revision struct {
	Kind                 Kind   `json:"kind"`
	SHA256               string `json:"sha256"`
	Size                 int64  `json:"size"`
	InstalledAt          string `json:"installedAt"`
	Source               string `json:"source"`
	SourceURL            string `json:"sourceUrl"`
	SourceName           string `json:"sourceName"`
	ValidatedCoreVersion string `json:"validatedCoreVersion"`
	PublisherVerified    bool   `json:"publisherVerified"`
}

type AssetState struct {
	Kind                 Kind   `json:"kind"`
	CanonicalName        string `json:"canonicalName"`
	Installed            bool   `json:"installed"`
	Active               bool   `json:"active"`
	Pending              bool   `json:"pending"`
	SHA256               string `json:"sha256"`
	ActiveSHA256         string `json:"activeSha256"`
	AvailableSHA256      string `json:"availableSha256"`
	Size                 int64  `json:"size"`
	InstalledAt          string `json:"installedAt"`
	Source               string `json:"source"`
	SourceURL            string `json:"sourceUrl"`
	ValidatedCoreVersion string `json:"validatedCoreVersion"`
	PublisherVerified    bool   `json:"publisherVerified"`
}

type State struct {
	Directory        string       `json:"directory"`
	RuntimeDirectory string       `json:"runtimeDirectory"`
	Preferences      Preferences  `json:"preferences"`
	ActiveGeoIPMode  string       `json:"activeGeoIpMode"`
	ActiveLoader     string       `json:"activeLoader"`
	LastCheckedAt    string       `json:"lastCheckedAt"`
	DownloadingKind  string       `json:"downloadingKind"`
	RestartRequired  bool         `json:"restartRequired"`
	Health           Health       `json:"health"`
	Assets           []AssetState `json:"assets"`
}

type Health struct {
	Status       string `json:"status"`
	MissingKinds []Kind `json:"missingKinds"`
	InvalidKinds []Kind `json:"invalidKinds"`
}

type DialogLabels struct {
	Title      string `json:"title"`
	FilterName string `json:"filterName"`
}

type DialogImportResult struct {
	Cancelled bool  `json:"cancelled"`
	State     State `json:"state"`
}
