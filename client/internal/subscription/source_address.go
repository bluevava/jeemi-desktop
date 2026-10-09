package subscription

import (
	"fmt"
	"path/filepath"
)

// SourceAddress is read only and intended for an explicit clipboard action in
// Go. Sensitive URLs and managed paths must not be added to list DTOs.
func (s *Store) SourceAddress(id string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	metadata, err := s.loadMetadataUnlocked(id)
	if err != nil {
		return "", err
	}
	if metadata.SourceKind == SourceURL {
		return metadata.SourceURL, nil
	}
	revision, found := findRevision(metadata.Revisions, metadata.CurrentRevisionID)
	if !found {
		return "", fmt.Errorf("current subscription revision is missing")
	}
	if err := s.verifyRevisionUnlocked(id, revision); err != nil {
		return "", err
	}
	return filepath.Abs(filepath.Join(s.root, id, "revisions", revision.ID, revision.FileName))
}

func (m *Manager) SourceAddress(id string) (string, error) {
	return m.store.SourceAddress(id)
}
