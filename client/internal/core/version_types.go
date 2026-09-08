package core

type AvailableVersion struct {
	Version     string `json:"version"`
	PublishedAt string `json:"publishedAt"`
	AssetName   string `json:"assetName"`
	ArchiveSize int64  `json:"archiveSize"`
	SHA256      string `json:"sha256"`
	Installed   bool   `json:"installed"`
	Selected    bool   `json:"selected"`
}

type InstalledVersion struct {
	Version        string `json:"version"`
	Target         string `json:"target"`
	ExecutablePath string `json:"executablePath"`
	BinarySize     int64  `json:"binarySize"`
	BinarySHA256   string `json:"binarySha256"`
	InstalledAt    string `json:"installedAt"`
	Source         string `json:"source"`
	Selected       bool   `json:"selected"`
}

type VersionManagerState struct {
	DataDirectory      string             `json:"dataDirectory"`
	CoreDirectory      string             `json:"coreDirectory"`
	Target             string             `json:"target"`
	SelectedVersion    string             `json:"selectedVersion"`
	LatestVersion      string             `json:"latestVersion"`
	LastCheckedAt      string             `json:"lastCheckedAt"`
	DownloadingVersion string             `json:"downloadingVersion"`
	InstalledVersions  []InstalledVersion `json:"installedVersions"`
	AvailableVersions  []AvailableVersion `json:"availableVersions"`
}

type releaseAsset struct {
	Version     string
	PublishedAt string
	Name        string
	URL         string
	Size        int64
	SHA256      string
}

type installationMetadata struct {
	Version       string `json:"version"`
	Target        string `json:"target"`
	AssetName     string `json:"assetName"`
	ArchiveSHA256 string `json:"archiveSha256"`
	BinarySHA256  string `json:"binarySha256"`
	BinarySize    int64  `json:"binarySize"`
	InstalledAt   string `json:"installedAt"`
	Source        string `json:"source"`
}

type DialogLabels struct {
	Title      string `json:"title"`
	FilterName string `json:"filterName"`
}

type DialogImportResult struct {
	Cancelled       bool                `json:"cancelled"`
	ImportedVersion string              `json:"importedVersion"`
	State           VersionManagerState `json:"state"`
}
