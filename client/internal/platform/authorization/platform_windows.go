//go:build windows

package authorization

import (
	"context"
	"jeemi/internal/platform/coreauth"
	"jeemi/internal/platform/winauth"
)

func Inspect(ctx context.Context, _ *coreauth.Target, _ string) (Status, error) {
	s, err := winauth.Status(ctx)
	return Status{Platform: "windows", Kind: "service", Ready: s.Ready, Present: s.Present, Code: s.Code, Action: s.Action, Steps: []Step{{"service", s.Ready}}}, err
}
func Setup(ctx context.Context, _ *coreauth.Target) error { return winauth.Install(ctx) }
func Remove(ctx context.Context, _ string) error          { return winauth.Remove(ctx) }
func OpenSettings(context.Context) error                  { return coreauth.ErrCancelled }
