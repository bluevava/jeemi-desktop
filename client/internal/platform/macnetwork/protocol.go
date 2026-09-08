// Package macnetwork defines the narrow macOS network-helper protocol. No UI
// method exposes its process, file or IPC primitives.
package macnetwork

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"jeemi/internal/platform/processmemory"
	"jeemi/internal/platform/requirements"
)

const ProtocolVersion = 2 // Includes helper-owned memory and scoped WebKit fallback.
const HelperName = "jeemi-authorizer"
const MaxMessageBytes = 1 << 20

type Target struct {
	Version string `json:"version"`
	SHA256  string `json:"sha256"`
	Size    int64  `json:"size"`
}

type Request struct {
	Protocol      int     `json:"protocol"`
	Operation     string  `json:"operation"`
	Target        *Target `json:"target,omitempty"`
	Configuration string  `json:"configuration,omitempty"`
	Endpoint      string  `json:"endpoint,omitempty"`
	DNS           string  `json:"dns,omitempty"`
	WebViewMemory bool    `json:"webViewMemory,omitempty"`
}

type Response struct {
	Configuration string                        `json:"configuration,omitempty"`
	Protocol      int                           `json:"protocol"`
	Code          string                        `json:"code"`
	PID           int                           `json:"pid,omitempty"`
	Running       bool                          `json:"running,omitempty"`
	Device        string                        `json:"device,omitempty"`
	Memory        *processmemory.HelperSnapshot `json:"memory,omitempty"`
}

var versionPattern = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)
var digestPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

func (t Target) Validate() error {
	if !versionPattern.MatchString(t.Version) || !digestPattern.MatchString(t.SHA256) || t.Size < 64 || t.Size > 512<<20 {
		return Failure("core_unverified")
	}
	return nil
}

func Failure(code string) error {
	return &requirements.Error{Code: "macos_network_" + code, Message: "macos_network_" + code}
}

// Call never returns native error text or request contents: either can contain
// a managed path, subscription URL or controller credential.
func Call(ctx context.Context, request Request) (Response, error) {
	request.Protocol = ProtocolVersion
	data, err := json.Marshal(request)
	if err != nil || len(data) > MaxMessageBytes {
		return Response{}, Failure("invalid_request")
	}
	type result struct {
		data []byte
		err  error
	}
	if err := ctx.Err(); err != nil {
		return Response{}, err
	}
	// Keep a start request tied to its connection until native completion. On
	// timeout/cancellation disconnect it so the daemon cannot leave an orphan.
	if request.Operation == "start" {
		data, err := nativeCall(data)
		if ctx.Err() != nil || err != nil {
			nativeDisconnect()
			if ctx.Err() != nil {
				return Response{}, ctx.Err()
			}
			return Response{}, Failure("unreachable")
		}
		var reply Response
		if len(data) > MaxMessageBytes || json.Unmarshal(data, &reply) != nil || reply.Protocol != ProtocolVersion {
			nativeDisconnect()
			return Response{}, Failure("version_mismatch")
		}
		if reply.Code != "ok" {
			return reply, Failure(knownCode(reply.Code))
		}
		return reply, nil
	}
	finished := make(chan result, 1)
	go func() { data, err := nativeCall(data); finished <- result{data, err} }()
	select {
	case <-ctx.Done():
		if request.Operation == "start" {
			nativeDisconnect()
		}
		return Response{}, ctx.Err()
	case value := <-finished:
		if value.err != nil {
			return Response{}, Failure("unreachable")
		}
		var reply Response
		if len(value.data) > MaxMessageBytes || json.Unmarshal(value.data, &reply) != nil || reply.Protocol != ProtocolVersion {
			return Response{}, Failure("version_mismatch")
		}
		if reply.Code != "ok" {
			return reply, Failure(knownCode(reply.Code))
		}
		return reply, nil
	}
}

func knownCode(code string) string {
	switch code {
	case "busy", "core_unverified", "core_missing", "invalid_request", "unsafe_path", "core_failed", "network_failed", "recovery_failed", "dns_in_use", "dns_invalid", "tun_unavailable", "cancelled", "sandbox_unavailable", "integrity_failed":
		return code
	default:
		return "helper_failed"
	}
}

func RequireReady(ctx context.Context) error {
	state := Inspect(ctx)
	if !state.Ready {
		return Failure(state.Code)
	}
	return nil
}

func Prepare(ctx context.Context, target Target) error {
	if err := target.Validate(); err != nil {
		return err
	}
	if err := RequireReady(ctx); err != nil {
		return err
	}
	_, err := Call(ctx, Request{Operation: "prepare", Target: &target})
	if ctx.Err() != nil {
		cancelCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = Call(cancelCtx, Request{Operation: "cancel_prepare"})
	}
	return err
}

func CheckTarget(ctx context.Context, target Target) error {
	if err := target.Validate(); err != nil {
		return err
	}
	_, err := Call(ctx, Request{Operation: "check", Target: &target})
	return err
}

func Start(ctx context.Context, target Target, configuration string) (Response, error) {
	if err := target.Validate(); err != nil {
		return Response{}, err
	}
	return Call(ctx, Request{Operation: "start", Target: &target, Configuration: configuration})
}

func ErrorCode(err error) string {
	if err == nil {
		return "ok"
	}
	const prefix = "macos_network_"
	code := requirements.Code(err, "")
	if len(code) > len(prefix) && code[:len(prefix)] == prefix {
		return knownCode(code[len(prefix):])
	}
	return "helper_failed"
}

func (r Response) Marshal() []byte {
	r.Protocol = ProtocolVersion
	data, err := json.Marshal(r)
	if err != nil {
		panic(fmt.Sprintf("marshal fixed helper response: %T", err))
	}
	return data
}
