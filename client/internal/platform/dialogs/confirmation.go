package dialogs

import (
	"context"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type Confirmation struct {
	Title   string
	Message string
	Confirm string
	Cancel  string
}

func Confirm(ctx context.Context, labels Confirmation) (bool, error) {
	options, accepted := confirmationOptions(labels)
	answer, err := wailsRuntime.MessageDialog(ctx, options)
	return err == nil && answer == accepted, err
}
