// Package builtinbrowser provides fixed browser process rules for each platform.
// It does not inspect installed applications or the user's URL associations.
package builtinbrowser

import "fmt"

func Rules(goos string) ([]string, error) {
	switch goos {
	case "windows":
		return []string{"PROCESS-NAME,msedge.exe"}, nil
	case "linux":
		return []string{"PROCESS-NAME,firefox", "PROCESS-NAME,firefox-bin"}, nil
	case "darwin":
		return []string{"PROCESS-NAME,Safari"}, nil
	default:
		return nil, fmt.Errorf("built-in browser rules are unavailable for this platform; configure process rules manually")
	}
}
