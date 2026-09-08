// Package authorization exposes platform facts without granting the WebView
// access to service names, executable paths, commands or credentials.
package authorization

type Step struct {
	ID       string `json:"id"`
	Complete bool   `json:"complete"`
}
type Status struct {
	Platform  string `json:"platform"`
	Kind      string `json:"kind"`
	Ready     bool   `json:"ready"`
	Present   bool   `json:"present"`
	LocalTest bool   `json:"localTest"`
	Code      string `json:"code"`
	Action    string `json:"action"`
	Steps     []Step `json:"steps"`
}
