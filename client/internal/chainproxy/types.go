// Package chainproxy owns reusable landing nodes and pure dialer-proxy composition.
package chainproxy

import "jeemi/internal/subscriptionformat"

const Version = 1
const MaxGeneratedNodes = 4096

// Library is private persistence data. YAML and full URLs never appear in List.
type Library struct {
	Version  int     `json:"version"`
	Revision int     `json:"revision"`
	Groups   []Group `json:"groups"`
}

type Group struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Kind           string   `json:"kind"`
	SelectorFilter string   `json:"selectorFilter"`
	NodeFilter     string   `json:"nodeFilter"`
	Nodes          []Node   `json:"nodes"`
	Sources        []Source `json:"sources"`
}

type Node struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	YAML         string `json:"yaml"`
	DNSYAML      string `json:"dnsYaml,omitempty"`
	HostsYAML    string `json:"hostsYaml,omitempty"`
	RequiredCore string `json:"requiredCore,omitempty"`
}

type Source struct {
	ID        string                    `json:"id"`
	URL       string                    `json:"url"`
	Filter    string                    `json:"filter"`
	UpdatedAt string                    `json:"updatedAt"`
	Nodes     []Node                    `json:"nodes"`
	Report    subscriptionformat.Report `json:"report"`
}

type State struct {
	Revision int            `json:"revision"`
	Groups   []GroupSummary `json:"groups"`
}

type GroupSummary struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	Kind           string          `json:"kind"`
	SelectorFilter string          `json:"selectorFilter"`
	NodeFilter     string          `json:"nodeFilter"`
	Nodes          []NodeSummary   `json:"nodes"`
	Sources        []SourceSummary `json:"sources"`
}

type NodeSummary struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	SourceID string `json:"sourceId"`
}

type SourceSummary struct {
	ID        string                    `json:"id"`
	Label     string                    `json:"label"`
	Filter    string                    `json:"filter"`
	UpdatedAt string                    `json:"updatedAt"`
	NodeCount int                       `json:"nodeCount"`
	Report    subscriptionformat.Report `json:"report"`
}

type SaveGroupInput struct {
	Revision       int    `json:"revision"`
	ID             string `json:"id"`
	Name           string `json:"name"`
	Kind           string `json:"kind"`
	SelectorFilter string `json:"selectorFilter"`
	NodeFilter     string `json:"nodeFilter"`
}

type ImportInput struct {
	Revision int    `json:"revision"`
	GroupID  string `json:"groupId"`
	NodeID   string `json:"nodeId"`
	Contents string `json:"contents"`
}

type SourceInput struct {
	Revision int    `json:"revision"`
	GroupID  string `json:"groupId"`
	ID       string `json:"id"`
	URL      string `json:"url"`
	Filter   string `json:"filter"`
}

type MutationResult struct {
	State    State                      `json:"state"`
	Report   *subscriptionformat.Report `json:"report,omitempty"`
	Failures []RefreshFailure           `json:"failures"`
}

type RefreshFailure struct {
	GroupID  string `json:"groupId"`
	SourceID string `json:"sourceId"`
	Code     string `json:"code"`
}

type Diagnostic struct {
	GroupID  string `json:"groupId"`
	Selector string `json:"selector"`
	Code     string `json:"code"`
	Count    int    `json:"count"`
}

type Composition struct {
	Fingerprint string       `json:"fingerprint"`
	Generated   int          `json:"generated"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

func (g Group) LandingNodes() []Node {
	result := append([]Node{}, g.Nodes...)
	for _, source := range g.Sources {
		result = append(result, source.Nodes...)
	}
	return result
}
