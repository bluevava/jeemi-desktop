//go:build linux

package coreauth

import (
	"context"
	"os"
	"os/user"
	"strconv"

	"jeemi/internal/platform/requirements"
)

// This directory is root-owned but readable to the GUI on Ubuntu. In contrast,
// /etc/polkit-1/rules.d is normally 0750 root:polkitd. Never relax its permissions.
const resolverRulesDirectory = "/usr/share/polkit-1/rules.d"

func CheckResolverAuthorization() error {
	if !resolverAvailable() {
		return nil
	}
	policy, err := resolverPolicyFor(uint32(os.Getuid()))
	if err != nil {
		return err
	}
	return policy.check(resolverRulesDirectory, 0)
}

func resolverAvailable() bool {
	info, err := os.Stat("/usr/bin/resolvectl")
	return err == nil && info.Mode().IsRegular() && info.Mode()&0o111 != 0
}

// Check is the common read-only preflight for both modes, including recovery.
// It never invokes pkexec or requests interactive authentication.
func Check(executable string, runningPID int) error {
	coreErr := requirements.CheckCoreCapabilities(executable, runningPID)
	if !resolverAvailable() {
		return coreErr
	}
	policy, err := resolverPolicyFor(uint32(os.Getuid()))
	if err != nil {
		return err
	}
	return combinedAuthorizationError(coreErr, policy.check(resolverRulesDirectory, 0))
}

func combinedAuthorizationError(coreErr, resolverErr error) error {
	// A missing permission can be repaired by the one prompt. An unsafe path,
	// conflicting administrator rule or unsupported environment cannot.
	for _, err := range []error{coreErr, resolverErr} {
		if err != nil && requirements.Code(err, "") != "linux_core_permission_required" {
			return err
		}
	}
	if coreErr != nil || resolverErr != nil {
		return failure("linux_core_permission_required")
	}
	return nil
}

func resolverPolicyFor(uid uint32) (resolverPolicy, error) {
	account, err := user.LookupId(strconv.FormatUint(uint64(uid), 10))
	if err != nil || uid == 0 || account.Username == "" || len(account.Username) > 256 {
		return resolverPolicy{}, failure("linux_resolver_authorization_unavailable")
	}
	return makeResolverPolicy(uid, account.Username), nil
}

func authorizeResolver(ctx context.Context, uid uint32, parent int) error {
	if !resolverAvailable() {
		return nil
	}
	policy, err := resolverPolicyFor(uid)
	if err != nil {
		return err
	}
	if err := checkResolverSupport(ctx); err != nil {
		return err
	}
	return policy.install(ctx, resolverRulesDirectory, 0, func(ctx context.Context) error {
		return verifyResolverAuthorization(ctx, uid, parent)
	})
}
