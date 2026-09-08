// Package subscriptionformat converts subscription documents in memory. It has
// no network, filesystem, process, or application settings dependencies.
package subscriptionformat

import "fmt"

const (
	Version             = 1
	MaxBytes            = 4 << 20
	maxLines            = 20000
	maxLineBytes        = 384 << 10 // cert:// can carry a bounded PEM chain.
	maxFieldBytes       = 8192
	maxCertificateBytes = 256 << 10
)

// Diagnostic contains only fixed codes and positions, never source values.
type Diagnostic struct {
	Code  string `json:"code"`
	Line  int    `json:"line"`
	Field string `json:"field"`
}

type Report struct {
	Format       string       `json:"format"`
	Encoding     string       `json:"encoding"`
	Fingerprint  string       `json:"fingerprint"`
	RequiredCore string       `json:"requiredCore,omitempty"`
	ProxyCount   int          `json:"proxyCount"`
	SkippedNodes int          `json:"skippedNodes"`
	Diagnostics  []Diagnostic `json:"diagnostics"`
}

type Result struct {
	Contents     []byte
	Report       Report
	RequiredCore string
}

type Error struct {
	Code  string
	Line  int
	Field string
}

func (e *Error) Error() string {
	return fmt.Sprintf("subscription normalization: %s (line %d, field %s)", e.Code, e.Line, e.Field)
}

func failure(code string, line int, field string) error { return &Error{code, line, field} }

type tlsOptions struct {
	Enabled           bool
	ServerName        string
	LeafSHA256        string
	ClientFingerprint string
	ALPN              []string
	SkipVerify        *bool
	RealityPublicKey  string
	RealityShortID    string
}

type transportOptions struct{ Network, Path, Host, ServiceName, Mode string }

type proxyNode struct {
	Name, Protocol, Server                string
	Port, Line                            int
	Username, Password, UUID, Cipher, PSK string
	UDP                                   *bool
	TLS                                   tlsOptions
	Transport                             transportOptions
	IdleCheck, IdleTimeout, MinIdle       *int64
	Congestion, UDPRelayMode              string
	ReduceRTT                             *bool
	Heartbeat                             *int64
	Flow, Encryption, PacketEncoding      string
	SnellVersion                          int
	ObfsMode, ObfsHost                    string
}

type dnsPolicy struct {
	Pattern string
	Servers []string
	Line    int
	Nodes   bool
}
type certificate struct {
	Pattern string
	PEM     []byte
	Line    int
}
type parsedDocument struct {
	Nodes                  []proxyNode
	Policies               []dnsPolicy
	Certificates           []certificate
	Nameservers, Bootstrap []string
	Hosts                  map[string][]string
	Diagnostics            []Diagnostic
	Skipped                int
	ProxyCount             int
}

func (p *parsedDocument) skip(code string, line int, field string) {
	p.Skipped++
	p.Diagnostics = append(p.Diagnostics, Diagnostic{Code: code, Line: line, Field: field})
}

func boolValue(v bool) *bool { return &v }
