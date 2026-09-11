package application

import (
	"context"
	"regexp"
	"time"

	"jeemi/internal/iconcache"
)

var iconRequestID = regexp.MustCompile(`^[A-Za-z0-9-]{1,64}$`)

func (s *Service) SelectorIcon(id, address string) (string, error) {
	if !iconRequestID.MatchString(id) || s.selectorIcons == nil {
		return "", iconcache.ErrUnavailable
	}
	s.mu.RLock()
	parent, stopping := s.ctx, s.shuttingDown
	s.mu.RUnlock()
	if stopping {
		return "", iconcache.ErrUnavailable
	}
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, time.Minute)
	defer cancel()
	s.iconMu.Lock()
	if s.iconsStopped || len(s.iconCancels) >= 256 || s.iconCancels[id] != nil {
		s.iconMu.Unlock()
		return "", iconcache.ErrUnavailable
	}
	s.iconCancels[id] = cancel
	s.iconMu.Unlock()
	defer func() { s.iconMu.Lock(); delete(s.iconCancels, id); s.iconMu.Unlock() }()
	return s.selectorIcons.Load(ctx, address)
}

func (s *Service) CancelSelectorIcon(id string) {
	s.iconMu.Lock()
	defer s.iconMu.Unlock()
	if cancel := s.iconCancels[id]; cancel != nil {
		cancel()
	}
}

func (s *Service) cancelSelectorIcons() {
	s.iconMu.Lock()
	defer s.iconMu.Unlock()
	s.iconsStopped = true
	for _, cancel := range s.iconCancels {
		cancel()
	}
}
