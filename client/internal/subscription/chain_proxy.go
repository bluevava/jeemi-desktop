package subscription

import (
	"fmt"
	"slices"
	"time"
)

func validateChainGroups(ids []string, revision int) error {
	if revision < 0 || len(ids) > 128 {
		return fmt.Errorf("invalid chain proxy association")
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if !localConfigIDPattern.MatchString(id) || seen[id] {
			return fmt.Errorf("invalid chain proxy association")
		}
		seen[id] = true
	}
	return nil
}

func (m *Manager) SetChainProxyGroups(id string, ids []string, expectedRevision int) (Detail, error) {
	s := m.store
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := validateChainGroups(ids, expectedRevision); err != nil {
		return Detail{}, err
	}
	metadata, err := s.loadMetadataUnlocked(id)
	if err != nil {
		return Detail{}, err
	}
	if metadata.ChainProxyRevision != expectedRevision {
		return Detail{}, fmt.Errorf("chain_proxy:revision_conflict")
	}
	if slices.Equal(metadata.ChainProxyGroupIDs, ids) {
		return detailFromMetadata(metadata), nil
	}
	metadata.ChainProxyGroupIDs = append([]string{}, ids...)
	metadata.ChainProxyRevision++
	metadata.UpdatedAt = s.now().UTC().Format(time.RFC3339Nano)
	if err := s.writeMetadataUnlocked(metadata); err != nil {
		return Detail{}, err
	}
	return detailFromMetadata(metadata), nil
}
