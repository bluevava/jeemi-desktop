//go:build !windows && !linux

package dialogs

import wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"

func confirmationOptions(labels Confirmation) (wailsRuntime.MessageDialogOptions, string) {
	return wailsRuntime.MessageDialogOptions{
		Type: wailsRuntime.QuestionDialog, Title: labels.Title, Message: labels.Message,
		Buttons:       []string{labels.Confirm, labels.Cancel},
		DefaultButton: labels.Cancel, CancelButton: labels.Cancel,
	}, labels.Confirm
}
