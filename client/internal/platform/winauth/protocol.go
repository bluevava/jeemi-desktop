// Package winauth implements the fixed Windows authorization service protocol.
package winauth

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"regexp"
	"strings"

	"jeemi/internal/platform/coresnapshot"
	"jeemi/internal/platform/processmemory"
	"jeemi/internal/platform/requirements"
	"jeemi/internal/runtimeconfig"
)

const HelperName = "jeemi-authorizer.exe"
const Protocol = 7 // Retains HTTP rule-provider caches across core sessions.
const maxRequestBytes = 1 << 20

var helperSHA256 string // pinned after building the standalone helper
var installedVersion = regexp.MustCompile(`^(v[0-9]+\.[0-9]+\.[0-9]+|unknown-[a-f0-9]{16})$`)
var digestPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

type Target struct {
	Version        string `json:"version"`
	SHA256         string `json:"sha256"`
	Size           int64  `json:"size"`
	ExecutablePath string `json:"executablePath"`
}

func (t Target) valid() bool {
	return installedVersion.MatchString(t.Version) && digestPattern.MatchString(t.SHA256) && t.Size >= 64 && t.Size <= 512<<20 && t.ExecutablePath != "" && !strings.ContainsAny(t.ExecutablePath, "\x00\r\n")
}

type Request struct {
	Protocol      int                        `json:"protocol"`
	Operation     string                     `json:"operation"`
	Target        *Target                    `json:"target,omitempty"`
	Configuration string                     `json:"configuration,omitempty"`
	SourceRoot    string                     `json:"sourceRoot,omitempty"`
	SnapshotSize  int64                      `json:"snapshotSize,omitempty"`
	Preferences   *runtimeconfig.Preferences `json:"preferences,omitempty"`
	WebViewMemory bool                       `json:"webViewMemory,omitempty"`
}
type Reply struct {
	Protocol      int                           `json:"protocol"`
	Code          string                        `json:"code"`
	Digest        string                        `json:"digest,omitempty"`
	PID           int                           `json:"pid,omitempty"`
	Running       bool                          `json:"running,omitempty"`
	Configuration string                        `json:"configuration,omitempty"`
	Memory        *processmemory.HelperSnapshot `json:"memory,omitempty"`
}

func failure(code string) error { return &requirements.Error{Code: code, Message: code} }
func decodeRequest(data []byte) (Request, error) {
	var r Request
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	var extra any
	if len(data) > maxRequestBytes || d.Decode(&r) != nil || d.Decode(&extra) != io.EOF {
		return r, failure("authorization_invalid_request")
	}
	snapshot := r.Configuration != "" || r.SourceRoot != "" || r.SnapshotSize != 0
	if r.WebViewMemory && r.Operation != "memory" {
		return r, failure("authorization_invalid_request")
	}
	switch r.Operation {
	case "status", "state", "stop", "restore", "remove", "memory":
		if r.Target != nil || snapshot || r.Preferences != nil {
			return r, failure("authorization_invalid_request")
		}
	case "check", "prepare":
		if r.Target == nil || !r.Target.valid() || snapshot || r.Preferences != nil {
			return r, failure("authorization_invalid_request")
		}
	case "start", "stage":
		if (r.Operation == "start" && (r.Target == nil || !r.Target.valid())) || (r.Operation == "stage" && r.Target != nil) || !coresnapshot.ConfigurationName(r.Configuration) || r.SourceRoot == "" || r.SnapshotSize <= 0 || r.SnapshotSize > 1100<<20 || r.Preferences != nil {
			return r, failure("authorization_invalid_request")
		}
	case "proxy":
		if r.Preferences == nil || runtimeconfig.Validate(*r.Preferences) != nil || r.Preferences.ProxyMode != runtimeconfig.ProxyModeSystemProxy || r.Target != nil || snapshot {
			return r, failure("authorization_invalid_request")
		}
	default:
		return r, failure("authorization_invalid_request")
	}
	return r, nil
}

type Network interface {
	Apply(context.Context, runtimeconfig.Preferences) error
	Restore(context.Context) error
}
type NetworkFactory func(string, string) (Network, error)

func (t Target) Valid() bool { return t.valid() }
