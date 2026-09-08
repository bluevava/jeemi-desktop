package macnetwork

import (
	"context"
	"errors"
	"testing"
)

func TestUnregisterStoppedLocalHelperAndVerifyRemoval(t *testing.T) {
	for _, retained := range []bool{false, true} {
		calls := 0
		inspect := func(context.Context) Status {
			return Status{LocalTest: true, Present: calls == 0 || retained, Code: "unreachable"}
		}
		manage := func(action string) error {
			if action != "unregister" {
				t.Fatal(action)
			}
			calls++
			return nil
		}
		call := func(context.Context, Request) (Response, error) {
			t.Fatal("required broken helper to be running")
			return Response{}, nil
		}
		_, err := unregister(context.Background(), inspect, manage, call)
		if calls != 1 || (err != nil) != retained {
			t.Fatalf("calls=%d err=%v", calls, err)
		}
	}
}
func TestUnregisterDoesNotDiscardFailedNetworkRecovery(t *testing.T) {
	inspect := func(context.Context) Status { return Status{LocalTest: true, Ready: true, Present: true} }
	manage := func(string) error { t.Fatal("removed before recovery succeeded"); return nil }
	call := func(context.Context, Request) (Response, error) { return Response{}, errors.New("recovery failed") }
	if _, err := unregister(context.Background(), inspect, manage, call); err == nil {
		t.Fatal("recovery failure hidden")
	}
}
func TestUnregisterAbsentHelperDoesNotRequestAuthentication(t *testing.T) {
	inspect := func(context.Context) Status { return Status{LocalTest: true, Code: "local_installation_required"} }
	manage := func(string) error { t.Fatal("authenticated absent helper"); return nil }
	call := func(context.Context, Request) (Response, error) {
		t.Fatal("contacted absent helper")
		return Response{}, nil
	}
	if _, err := unregister(context.Background(), inspect, manage, call); err != nil {
		t.Fatal(err)
	}
}

func TestPresenceSurvivesStoppedBrokenAndUnapprovedInstallations(t *testing.T) {
	for _, f := range []Facts{
		{Supported: true, LocalTest: true, Present: true},
		{Supported: true, LocalTest: true, Present: true, Packaged: true, Signed: true, Service: "not_registered"},
		{Supported: true, Installed: true, Packaged: true, Signed: true, Service: "approval"},
		{Supported: true, Installed: true, Packaged: true, Signed: true, Service: "enabled"},
	} {
		if s := Evaluate(f, nil); !s.Present || s.Ready {
			t.Fatalf("lost installed state: %+v", s)
		}
	}
	f := Facts{Supported: true, LocalTest: true, Present: true, Packaged: true, Signed: true, Service: "enabled"}
	if s := Evaluate(f, nil); s.Code != "local_update_required" || s.Action != "repair" {
		t.Fatalf("old trust mistaken for absent: %+v", s)
	}
}

func TestLocalBuildNeedsPinnedInstallationAndHealthyConnection(t *testing.T) {
	f := Facts{LocalTest: true, Supported: true, Packaged: true, Signed: true}
	if s := Evaluate(f, nil); s.Ready || s.Code != "local_installation_required" || s.Action != "register" || !s.LocalTest {
		t.Fatalf("%+v", s)
	}
	f.Installed = true
	f.Service = "enabled"
	if s := Evaluate(f, nil); s.Ready || s.Code != "unreachable" {
		t.Fatalf("%+v", s)
	}
	if Evaluate(f, &Response{Protocol: 99, Code: "ok"}).Ready {
		t.Fatal("wrong protocol accepted")
	}
	if Evaluate(f, &Response{Protocol: ProtocolVersion, Code: "recovery_failed"}).Ready {
		t.Fatal("unrestored network accepted")
	}
	if !Evaluate(f, &Response{Protocol: ProtocolVersion, Code: "ok"}).Ready {
		t.Fatal("healthy pinned helper rejected")
	}
}

func TestAuthorizationChecksGateInOrder(t *testing.T) {
	f := Facts{Supported: true}
	stages := []struct {
		code, action string
		change       func()
	}{
		{"helper_missing", "", func() {}},
		{"signing_required", "", func() { f.Packaged = true }},
		{"installation_required", "applications", func() { f.Signed = true }},
		{"registration_required", "register", func() { f.Installed = true; f.Service = "not_registered" }},
		{"approval_required", "settings", func() { f.Service = "approval" }},
		{"unreachable", "repair", func() { f.Service = "enabled" }},
	}
	for _, stage := range stages {
		stage.change()
		got := Evaluate(f, nil)
		if got.Ready || got.Code != stage.code || got.Action != stage.action {
			t.Fatalf("%s: %+v", stage.code, got)
		}
	}
	for _, reply := range []Response{{Protocol: 99, Code: "ok"}, {Protocol: ProtocolVersion, Code: "recovery_failed"}} {
		if Evaluate(f, &reply).Ready {
			t.Fatal("unhealthy helper was authorized")
		}
	}
	got := Evaluate(f, &Response{Protocol: ProtocolVersion, Code: "ok"})
	if !got.Ready {
		t.Fatalf("%+v", got)
	}
	for _, step := range got.Steps {
		if !step.Complete {
			t.Fatal(step.ID)
		}
	}
}
func TestTargetRejectsArbitraryExecutables(t *testing.T) {
	target := Target{Version: "v1.19.0", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Size: 1024}
	if target.Validate() != nil {
		t.Fatal("valid target rejected")
	}
	for _, version := range []string{"../v1.0.0", "unknown-abc", "latest", "v1.2.3;id"} {
		candidate := target
		candidate.Version = version
		if candidate.Validate() == nil {
			t.Fatal(version)
		}
	}
}
