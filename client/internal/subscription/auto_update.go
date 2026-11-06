package subscription

import "fmt"

const DefaultAutoUpdateIntervalMinutes = 60

// Zero is the omitted optional value, not a separate disabled setting.
func normalizeAutoUpdateInterval(kind SourceKind, minutes int) (int, error) {
	if kind == SourceFile {
		if minutes != 0 {
			return 0, fmt.Errorf("file subscriptions do not support automatic updates")
		}
		return 0, nil
	}
	if minutes == 0 {
		minutes = DefaultAutoUpdateIntervalMinutes
	}
	switch minutes {
	case 10, 30, 60, 180, 720:
		return minutes, nil
	default:
		return 0, fmt.Errorf("subscription update interval must be 10, 30, 60, 180 or 720 minutes")
	}
}
