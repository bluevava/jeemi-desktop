//go:build linux

package coreauth

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dop251/goja"
	"jeemi/internal/platform/requirements"
)

func TestResolverRuleScope(t *testing.T) {
	vm := goja.New()
	var rule goja.Callable
	if err := vm.Set("polkit", map[string]any{
		"Result": map[string]any{"YES": "yes"},
		"addRule": func(call goja.FunctionCall) goja.Value {
			rule, _ = goja.AssertFunction(call.Argument(0))
			return goja.Undefined()
		},
	}); err != nil {
		t.Fatal(err)
	}
	p := makeResolverPolicy(1000, "lx")
	if _, err := vm.RunString(string(p.rule)); err != nil || rule == nil {
		t.Fatalf("rule syntax: %v", err)
	}
	check := func(action, user string, iface any, want bool) {
		t.Helper()
		for _, active := range []bool{true, false} {
			for _, local := range []bool{true, false} {
				value, err := rule(goja.Undefined(), vm.ToValue(map[string]any{"id": action, "lookup": func(string) any { return iface }}),
					vm.ToValue(map[string]any{"user": user, "active": active, "local": local}))
				if err != nil || (value.String() == "yes") != want {
					t.Fatalf("%s %s %v: %v %v", action, user, iface, value, err)
				}
			}
		}
	}
	for _, action := range resolverActions {
		check(action, "lx", "JeemiTun", true)
		for _, iface := range []any{"ens33", "lo", "JeemiTun2", "jeemitun", "", nil} {
			check(action, "lx", iface, false)
		}
		for _, user := range []string{"root", "another-user", "LX", ""} {
			check(action, user, "JeemiTun", false)
		}
	}
	for _, action := range []string{"org.freedesktop.policykit.exec", "org.freedesktop.resolve1.set-dnssec", "org.freedesktop.resolve1.set-dns-over-tls", "org.freedesktop.resolve1.revert-all", ""} {
		check(action, "lx", "JeemiTun", false)
	}
	if _, err := vm.RunString(string(makeResolverPolicy(1000, "user\"\\\n; polkit.bad()").rule)); err != nil {
		t.Fatal("username was not escaped", err)
	}
}

func TestResolverPolicyRequiresVerifiedReceiptAndInstallsIdempotently(t *testing.T) {
	dir, uid := t.TempDir(), uint32(os.Getuid())
	p := makeResolverPolicy(1000, "lx")
	if code := requirements.Code(p.check(dir, uid), ""); code != "linux_core_permission_required" {
		t.Fatal(code)
	}
	checks := 0
	verify := func(context.Context) error { checks++; return nil }
	if err := p.install(context.Background(), dir, uid, verify); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(filepath.Join(dir, p.name+".rules"))
	if err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if err := p.check(dir, uid); err != nil {
			t.Fatal(err)
		}
	}
	if checks != 1 {
		t.Fatal("read-only checks performed authorization")
	}
	if err := p.install(context.Background(), dir, uid, verify); err != nil {
		t.Fatal(err)
	}
	after, _ := os.Stat(filepath.Join(dir, p.name+".rules"))
	if !os.SameFile(before, after) || checks != 2 {
		t.Fatal("reinstalled an identical rule")
	}
	if err := os.Remove(filepath.Join(dir, p.name+".verified")); err != nil {
		t.Fatal(err)
	}
	if code := requirements.Code(p.check(dir, uid), ""); code != "linux_core_permission_required" {
		t.Fatal("unverified rule accepted", code)
	}
	if err := p.install(context.Background(), dir, uid, verify); err != nil {
		t.Fatal("interrupted install cannot resume", err)
	}
}

func TestResolverPolicyPreservesAdministratorFiles(t *testing.T) {
	for _, kind := range []string{"different rule", "different receipt", "symlink", "hardlink", "writable rule", "writable directory", "wrong owner", "directory symlink"} {
		t.Run(kind, func(t *testing.T) {
			dir, uid := t.TempDir(), uint32(os.Getuid())
			p := makeResolverPolicy(1000, "lx")
			name := filepath.Join(dir, p.name+".rules")
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			switch kind {
			case "different rule":
				must(os.WriteFile(name, []byte("administrator rule"), 0o644))
			case "different receipt":
				must(os.WriteFile(filepath.Join(dir, p.name+".verified"), []byte("other grant"), 0o644))
			case "symlink":
				must(os.Symlink("/nonexistent", name))
			case "hardlink":
				must(os.WriteFile(name, p.rule, 0o644))
				must(os.Link(name, name+".link"))
			case "writable rule":
				must(os.WriteFile(name, p.rule, 0o644))
				must(os.Chmod(name, 0o666))
			case "writable directory":
				must(os.Chmod(dir, 0o777))
			case "wrong owner":
				must(os.WriteFile(name, p.rule, 0o644))
				uid++
			case "directory symlink":
				actual := dir
				dir = filepath.Join(t.TempDir(), "linked")
				must(os.Symlink(actual, dir))
			}
			called := false
			err := p.install(context.Background(), dir, uid, func(context.Context) error { called = true; return nil })
			if err == nil || called {
				t.Fatal("unsafe policy accepted", err)
			}
			if kind == "different rule" {
				data, _ := os.ReadFile(name)
				if string(data) != "administrator rule" {
					t.Fatal("overwrote administrator rule")
				}
			}
		})
	}
}

func TestResolverDeniedOrCancelledGrantDoesNotLeaveNewPolicy(t *testing.T) {
	for _, cancelBefore := range []bool{true, false} {
		dir, uid := t.TempDir(), uint32(os.Getuid())
		p := makeResolverPolicy(1000, "lx")
		ctx, cancel := context.WithCancel(context.Background())
		if cancelBefore {
			cancel()
		}
		err := p.install(ctx, dir, uid, func(context.Context) error { cancel(); return errors.New("policy denied") })
		cancel()
		if err == nil {
			t.Fatal("failed grant reported success")
		}
		files, _ := os.ReadDir(dir)
		if len(files) != 0 {
			t.Fatal("failed grant left new files", files)
		}
	}
}

func TestResolverPolicyReceiptBindsIdentityAndRule(t *testing.T) {
	p := makeResolverPolicy(1000, "lx")
	if strings.Contains(string(p.rule), "subject.active") || strings.Contains(string(p.rule), "subject.local") {
		t.Fatal("background lifecycle restricted")
	}
	if string(p.receipt) == string(makeResolverPolicy(1001, "lx").receipt) || string(p.receipt) == string(makeResolverPolicy(1000, "other").receipt) {
		t.Fatal("receipt does not bind account")
	}
	for _, test := range []struct {
		version string
		want    bool
	}{{"257.9-0ubuntu2.5", true}, {"259", true}, {"256", false}, {"255.4", false}, {"", false}, {"unknown", false}} {
		if supportsResolverInterfaceScope(test.version) != test.want {
			t.Fatal(test.version)
		}
	}
}

func TestCombinedAuthorizationChecksAllPermissionsBeforeOnePrompt(t *testing.T) {
	missing := failure("linux_core_permission_required")
	blocked := failure("linux_core_privileges_blocked")
	conflict := failure("linux_resolver_authorization_conflict")
	for _, test := range []struct {
		core, resolver error
		code           string
	}{
		{nil, nil, ""},
		{missing, nil, "linux_core_permission_required"},
		{nil, missing, "linux_core_permission_required"},
		{missing, missing, "linux_core_permission_required"},
		{missing, conflict, "linux_resolver_authorization_conflict"},
		{blocked, missing, "linux_core_privileges_blocked"},
	} {
		if code := requirements.Code(combinedAuthorizationError(test.core, test.resolver), ""); code != test.code {
			t.Fatal(code, test.code)
		}
	}
}
