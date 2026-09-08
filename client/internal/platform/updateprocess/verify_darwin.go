package updateprocess

import (
	"context"
	"errors"
	"os/exec"
)

func VerifyBundle(ctx context.Context, bundle string) error {
	if exec.CommandContext(ctx, "/usr/bin/codesign", "--verify", "--deep", "--strict", bundle).Run() != nil {
		return errors.New("invalid application signature")
	}
	return nil
}
