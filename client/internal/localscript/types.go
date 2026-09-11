package localscript

type SaveInput struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Description      string `json:"description"`
	Contents         string `json:"contents"`
	SourceURL        string `json:"sourceUrl,omitempty"`
	ExpectedRevision int    `json:"expectedRevision,omitempty"`
}

type Summary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Revision    int    `json:"revision"`
	LineCount   int    `json:"lineCount"`
	SizeBytes   int64  `json:"sizeBytes"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
	SourceType  string `json:"sourceType"`
}

type Script struct {
	Summary
	Contents  string `json:"contents"`
	SourceURL string `json:"sourceUrl"`
}

type State struct {
	Directory string    `json:"directory"`
	Scripts   []Summary `json:"scripts"`
}

type TestInput struct {
	SaveInput
	SubscriptionID string `json:"subscriptionId"`
}

type TestResult struct {
	SubscriptionID   string `json:"subscriptionId"`
	SubscriptionName string `json:"subscriptionName"`
	Contents         string `json:"contents"`
	CoreValidated    bool   `json:"coreValidated"`
}

type manifestDocument struct {
	ManifestVersion int    `json:"manifestVersion"`
	ID              string `json:"id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	Revision        int    `json:"revision"`
	LineCount       int    `json:"lineCount"`
	SizeBytes       int64  `json:"sizeBytes"`
	CreatedAt       string `json:"createdAt"`
	UpdatedAt       string `json:"updatedAt"`
	SourceURL       string `json:"sourceUrl,omitempty"`
}
