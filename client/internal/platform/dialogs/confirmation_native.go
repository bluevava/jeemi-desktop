//go:build windows || linux

package dialogs

import wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"

func confirmationOptions(labels Confirmation) (wailsRuntime.MessageDialogOptions, string) {
	// Wails 2 on Windows and Linux ignores custom Buttons and returns English
	// identifiers for OS-localized Yes/No. Only Windows honors DefaultButton.
	return wailsRuntime.MessageDialogOptions{
		Type: wailsRuntime.QuestionDialog, Title: labels.Title, Message: labels.Message,
		DefaultButton: "No", CancelButton: "No",
	}, "Yes"
}
