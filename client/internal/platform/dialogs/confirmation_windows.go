package dialogs

import wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"

func confirmationOptions(labels Confirmation) (wailsRuntime.MessageDialogOptions, string) {
	// Wails 2 on Windows ignores custom Buttons and WarningDialog has only OK.
	// QuestionDialog uses OS-localized Yes/No and returns the English identifier.
	return wailsRuntime.MessageDialogOptions{
		Type: wailsRuntime.QuestionDialog, Title: labels.Title, Message: labels.Message,
		DefaultButton: "No", CancelButton: "No",
	}, "Yes"
}
