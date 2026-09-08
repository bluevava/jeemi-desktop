package dialogs

import (
	"context"
	"runtime"
	"strings"

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
	options.Message = nativeMessage(runtime.GOOS, options.Message)
	answer, err := wailsRuntime.MessageDialog(ctx, options)
	return err == nil && answer == accepted, err
}

func ShowError(ctx context.Context, title, message, closeLabel string) error {
	_, err := wailsRuntime.MessageDialog(ctx, wailsRuntime.MessageDialogOptions{
		Type: wailsRuntime.ErrorDialog, Title: title, Message: nativeMessage(runtime.GOOS, message),
		Buttons: []string{closeLabel}, DefaultButton: closeLabel, CancelButton: closeLabel,
	})
	return err
}

func nativeMessage(platform, message string) string {
	// Wails 2.14 passes Linux message text directly as a GTK printf format.
	// Keep percent signs in paths/text literal without changing other platforms.
	if platform == "linux" {
		return strings.ReplaceAll(message, "%", "%%")
	}
	return message
}
