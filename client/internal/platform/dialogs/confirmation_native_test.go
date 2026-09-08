//go:build windows || linux

package dialogs

import (
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"testing"
)

func TestConfirmationUsesSupportedNativeQuestionAndSafeDefault(t *testing.T) {
	options, accepted := confirmationOptions(Confirmation{Title: "重新加载", Message: "未保存的编辑会丢失", Confirm: "确认", Cancel: "取消"})
	if options.Type != wailsRuntime.QuestionDialog || options.DefaultButton != "No" || accepted != "Yes" || options.Title != "重新加载" || options.Message != "未保存的编辑会丢失" {
		t.Fatal("native Yes/No must use the English return identifier")
	}
}
