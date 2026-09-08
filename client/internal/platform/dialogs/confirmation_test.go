package dialogs

import "testing"

func TestNativeMessageKeepsPercentSignsLiteral(t *testing.T) {
	message := "Configuration /home/100%user/%s/settings.json"
	if got := nativeMessage("linux", message); got != "Configuration /home/100%%user/%%s/settings.json" {
		t.Fatal("Linux GTK message must escape format verbs", got)
	}
	for _, platform := range []string{"windows", "darwin"} {
		if got := nativeMessage(platform, message); got != message {
			t.Fatal(platform, "must retain the original message", got)
		}
	}
}
