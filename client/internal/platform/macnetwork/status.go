package macnetwork

import (
	"context"
	"jeemi/internal/platform/requirements"
)

type Facts struct {
	LocalTest bool   `json:"localTest"`
	Supported bool   `json:"supported"`
	Packaged  bool   `json:"packaged"`
	Signed    bool   `json:"signed"`
	Installed bool   `json:"installed"`
	Present   bool   `json:"present"`
	Service   string `json:"service"`
}

type Step struct {
	ID       string `json:"id"`
	Complete bool   `json:"complete"`
}

type Status struct {
	LocalTest  bool   `json:"localTest"`
	Applicable bool   `json:"applicable"`
	Ready      bool   `json:"ready"`
	Present    bool   `json:"present"`
	Code       string `json:"code"`
	Action     string `json:"action"`
	Steps      []Step `json:"steps"`
}

func Evaluate(f Facts, reply *Response) Status {
	if f.LocalTest {
		return evaluateLocal(f, reply)
	}
	state := Status{Applicable: f.Supported, Present: hasRegistration(f), Code: "unsupported", Steps: []Step{}}
	if !f.Supported {
		return state
	}
	state.Steps = []Step{{"package", f.Packaged}, {"signature", f.Signed}, {"location", f.Installed}, {"approval", f.Service == "enabled"}, {"connection", reply != nil && reply.Protocol == ProtocolVersion && reply.Code == "ok"}}
	switch {
	case !f.Packaged:
		state.Code = "helper_missing"
	case !f.Signed:
		state.Code = "signing_required"
	case !f.Installed:
		state.Code = "installation_required"
		state.Action = "applications"
	case f.Service == "approval":
		state.Code = "approval_required"
		state.Action = "settings"
	case f.Service == "not_registered" || f.Service == "not_found":
		state.Code = "registration_required"
		state.Action = "register"
	case f.Service != "enabled":
		state.Code = "inspection_failed"
	case reply == nil:
		state.Code = "unreachable"
		state.Action = "repair"
	case reply.Protocol != ProtocolVersion:
		state.Code = "version_mismatch"
		state.Action = "repair"
	case reply.Code != "ok":
		state.Code = knownCode(reply.Code)
	default:
		state.Ready = true
		state.Code = "ready"
	}
	return state
}

func evaluateLocal(f Facts, reply *Response) Status {
	state := Status{Applicable: f.Supported, Present: hasRegistration(f), LocalTest: true, Code: "unsupported", Steps: []Step{}}
	if !f.Supported {
		return state
	}
	state.Steps = []Step{{"package", f.Packaged}, {"local_signature", f.Signed}, {"local_installation", f.Installed && f.Service == "enabled"}, {"connection", reply != nil && reply.Protocol == ProtocolVersion && reply.Code == "ok"}}
	switch {
	case !f.Packaged:
		state.Code = "helper_missing"
	case !f.Signed:
		state.Code = "local_signature_invalid"
	case !f.Installed || f.Service != "enabled":
		state.Code = "local_installation_required"
		state.Action = "register"
		if state.Present {
			state.Action = "repair"
			if !f.Installed && f.Service == "enabled" {
				state.Code = "local_update_required"
			}
		}
	case reply == nil:
		state.Code = "unreachable"
		state.Action = "repair"
	case reply.Protocol != ProtocolVersion:
		state.Code = "version_mismatch"
		state.Action = "repair"
	case reply.Code != "ok":
		state.Code = knownCode(reply.Code)
	default:
		state.Ready = true
		state.Code = "ready"
	}
	return state
}

// Presence is registration or owned artifacts, not a live process. A disabled,
// broken or old installation must still offer repair and removal.
func hasRegistration(f Facts) bool {
	return f.Present || f.Service == "enabled" || f.Service == "approval" || f.Service == "unknown"
}

func Inspect(ctx context.Context) Status {
	facts := nativeFacts()
	state := Evaluate(facts, nil)
	if facts.Packaged && facts.Signed && facts.Installed && facts.Service == "enabled" {
		if reply, err := Call(ctx, Request{Operation: "ping"}); err == nil || reply.Protocol == ProtocolVersion {
			state = Evaluate(facts, &reply)
		} else if requirements.Code(err, "") == "macos_network_version_mismatch" {
			state = Evaluate(facts, &Response{})
		}
	}
	return state
}

// Setup and settings navigation only run after an explicit UI action. Inspect
// is passive and never presents authentication or registration UI.
func Setup(ctx context.Context) (Status, error) {
	state := Inspect(ctx)
	if state.Ready {
		return state, nil
	}
	if state.Action != "register" && state.Action != "repair" {
		return state, Failure(state.Code)
	}
	if state.Action == "repair" && !LocalTesting {
		// A stale or unreachable helper cannot acknowledge recovery. Unregister
		// asks launchd to terminate it; its durable journal survives replacement
		// and the new helper must restore it before reporting ready.
		_, _ = Call(ctx, Request{Operation: "stop"})
		if err := nativeManage("unregister"); err != nil {
			return state, err
		}
	}
	if err := nativeManage("register"); err != nil {
		return Inspect(ctx), err
	}
	return Inspect(ctx), nil
}

func OpenSettings() error     { return nativeManage("settings") }
func OpenApplications() error { return nativeManage("applications") }
func Unregister(ctx context.Context) (Status, error) {
	return unregister(ctx, Inspect, nativeManage, Call)
}

func unregister(ctx context.Context, inspect func(context.Context) Status, manage func(string) error, call func(context.Context, Request) (Response, error)) (Status, error) {
	state := inspect(ctx)
	if !state.Present && (state.Code == "registration_required" || state.Code == "local_installation_required") {
		return state, nil
	}
	if state.Ready {
		if _, err := call(ctx, Request{Operation: "stop"}); err != nil {
			return state, err
		}
	}
	// Only the local installer has an independently elevated recovery command.
	// A production helper must acknowledge restored networking before removal.
	if !state.LocalTest && state.Present && !state.Ready {
		return state, Failure("recovery_failed")
	}
	if err := manage("unregister"); err != nil {
		return state, err
	}
	state = inspect(ctx)
	if state.Present {
		return state, Failure("cleanup_incomplete")
	}
	return state, nil
}
