package desktop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jeemi/internal/configfile"
)

func TestAppAssemblyDoesNotReadConfigurationBeforeSingleInstance(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "settings.json")
	if err := os.WriteFile(path, []byte("broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	app, err := NewApp(root)
	if err != nil || app.service != nil || app.hostReady == nil {
		t.Fatal("application service initialized before the startup gate", err)
	}
	contents, _ := os.ReadFile(path)
	if string(contents) != "broken" {
		t.Fatal("assembly changed a configuration file")
	}
}

func TestNativeStartupMessagesUseSharedLabelsWithoutDocumentValues(t *testing.T) {
	contents, err := os.ReadFile("../../frontend/src/i18n/startup-labels.json")
	if err != nil {
		t.Fatal(err)
	}
	var languages map[string]startupDialogLabels
	if err := json.Unmarshal(contents, &languages); err != nil {
		t.Fatal(err)
	}
	for language, labels := range languages {
		for _, reason := range []string{"invalid", "read", "delete"} {
			message := startupFailureMessage(labels, &configfile.Failure{Path: "example/settings.json", Reason: reason, Offset: 7})
			if labels.Confirm == "" || labels.Cancel == "" || !strings.Contains(message, "example/settings.json") || strings.Contains(message, "{{") {
				t.Fatal(language, reason, message)
			}
		}
	}
}
