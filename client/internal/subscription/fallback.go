package subscription

import (
	"fmt"
	"time"

	"jeemi/internal/config/fallbackoverride"
)

// SetFallback uses an independent revision so a stale UI cannot overwrite a
// reset or a newer choice. The source revision and raw bytes stay unchanged.
func (s *Store) SetFallback(id string, input fallbackoverride.Selection, expectedRevision int, reset bool) (Detail, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	selection, err := fallbackoverride.Normalize(input)
	if err != nil {
		return Detail{}, err
	}
	metadata, err := s.loadMetadataUnlocked(id)
	if err != nil {
		return Detail{}, err
	}
	if metadata.FallbackOverrideRevision != expectedRevision {
		return Detail{}, fmt.Errorf("subscription fallback changed; refresh before saving")
	}
	previous := metadata.Fallback
	if previous == selection {
		return detailFromMetadata(metadata), nil
	}
	metadata.Fallback = selection
	metadata.FallbackOverrideRevision++
	metadata.FallbackResetTarget = ""
	if reset && previous.Mode == fallbackoverride.ModeSelector && selection.Mode == fallbackoverride.ModeNone {
		metadata.FallbackResetTarget = previous.Selector
	}
	metadata.UpdatedAt = s.now().UTC().Format(time.RFC3339Nano)
	if err := s.writeMetadataUnlocked(metadata); err != nil {
		return Detail{}, err
	}
	return detailFromMetadata(metadata), nil
}

func (m *Manager) SetFallback(id string, selection fallbackoverride.Selection, expectedRevision int, reset bool) (Detail, error) {
	return m.store.SetFallback(id, selection, expectedRevision, reset)
}
