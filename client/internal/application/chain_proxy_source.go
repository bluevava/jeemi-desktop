package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"jeemi/internal/chainproxy"
	"strings"
	"time"
)

func (s *Service) beginChainNetwork() (context.Context, func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.chainOperationCancel != nil {
		return nil, nil, chainError("busy")
	}
	parent := s.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
	s.chainOperationCancel = cancel
	return ctx, func() { cancel(); s.mu.Lock(); s.chainOperationCancel = nil; s.mu.Unlock() }, nil
}

func (s *Service) CancelChainProxyRefresh() {
	s.mu.RLock()
	cancel := s.chainOperationCancel
	s.mu.RUnlock()
	if cancel != nil {
		cancel()
	}
}

func (s *Service) fetchChainSource(ctx context.Context, input chainproxy.SourceInput) (chainproxy.Source, error) {
	address, err := chainproxy.NormalizeURL(input.URL)
	if err != nil {
		return chainproxy.Source{}, err
	}
	fetched, err := s.chainFetcher.Fetch(ctx, address)
	if err != nil {
		if ctx.Err() != nil {
			return chainproxy.Source{}, chainError("cancelled")
		}
		return chainproxy.Source{}, chainError("fetch_failed")
	}
	parsed, err := chainproxy.Parse(fetched.Contents, input.Filter)
	if err != nil {
		return chainproxy.Source{}, err
	}
	id := input.ID
	if id == "" {
		id, err = chainproxy.NewID()
		if err != nil {
			return chainproxy.Source{}, err
		}
	}
	for i := range parsed.Nodes {
		// A source/name identity survives response ordering and credential
		// changes. Separate sources can contain equally named nodes safely.
		digest := sha256.Sum256([]byte(id + "\x00" + parsed.Nodes[i].Name))
		parsed.Nodes[i].ID = hex.EncodeToString(digest[:16])
	}
	return chainproxy.Source{ID: id, URL: address, Filter: strings.TrimSpace(input.Filter), UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano), Nodes: parsed.Nodes, Report: parsed.Report}, nil
}

func (s *Service) SaveChainProxySource(input chainproxy.SourceInput) (chainproxy.MutationResult, error) {
	library, err := s.chainProxies.Load()
	if err != nil {
		return chainproxy.MutationResult{}, err
	}
	if library.Revision != input.Revision {
		return chainproxy.MutationResult{}, chainError("revision_conflict")
	}
	group, err := chainproxy.FindGroup(&library, input.GroupID)
	if err != nil {
		return chainproxy.MutationResult{}, err
	}
	if group.Kind != "subscription" {
		return chainproxy.MutationResult{}, chainError("invalid_group")
	}
	ctx, done, err := s.beginChainNetwork()
	if err != nil {
		return chainproxy.MutationResult{}, err
	}
	defer done()
	source, err := s.fetchChainSource(ctx, input)
	if err != nil {
		return chainproxy.MutationResult{}, err
	}
	return s.commitFetchedChainSource(ctx, input.Revision, input.GroupID, input.ID, source)
}

func (s *Service) commitFetchedChainSource(ctx context.Context, revision int, groupID, previousID string, source chainproxy.Source) (chainproxy.MutationResult, error) {
	result, err := s.commitChainChange(ctx, revision, groupID, func(library *chainproxy.Library) error {
		group, err := chainproxy.FindGroup(library, groupID)
		if err != nil {
			return err
		}
		if group.Kind != "subscription" {
			return chainError("invalid_group")
		}
		if previousID == "" {
			group.Sources = append(group.Sources, source)
			return nil
		}
		for i := range group.Sources {
			if group.Sources[i].ID == previousID {
				group.Sources[i] = source
				return nil
			}
		}
		return chainError("source_missing")
	})
	result.Report = &source.Report
	return result, err
}

// Empty groupID refreshes all URL sources. Each successful source has its own
// transaction; a failed source retains its previous nodes and timestamp.
func (s *Service) RefreshChainProxySources(groupID string, revision int) (chainproxy.MutationResult, error) {
	library, err := s.chainProxies.Load()
	if err != nil {
		return chainproxy.MutationResult{}, err
	}
	if library.Revision != revision {
		return chainproxy.MutationResult{}, chainError("revision_conflict")
	}
	if groupID != "" {
		if _, err := chainproxy.FindGroup(&library, groupID); err != nil {
			return chainproxy.MutationResult{}, err
		}
	}
	ctx, done, err := s.beginChainNetwork()
	if err != nil {
		return chainproxy.MutationResult{}, err
	}
	defer done()
	result := chainproxy.MutationResult{State: chainproxy.List(library), Failures: []chainproxy.RefreshFailure{}}
	for _, group := range library.Groups {
		if groupID != "" && group.ID != groupID {
			continue
		}
		for _, source := range group.Sources {
			if ctx.Err() != nil {
				result.Failures = append(result.Failures, chainproxy.RefreshFailure{GroupID: group.ID, SourceID: source.ID, Code: "cancelled"})
				return result, nil
			}
			fetched, fetchErr := s.fetchChainSource(ctx, chainproxy.SourceInput{ID: source.ID, URL: source.URL, Filter: source.Filter})
			code := "fetch_failed"
			if fetchErr == nil {
				var committed chainproxy.MutationResult
				committed, fetchErr = s.commitFetchedChainSource(ctx, result.State.Revision, group.ID, source.ID, fetched)
				if fetchErr == nil {
					result.State = committed.State
					continue
				}
				code = "candidate_invalid"
			}
			var known *chainproxy.Error
			if errors.As(fetchErr, &known) {
				code = known.Code
			}
			if strings.Contains(fetchErr.Error(), "chain_proxy:revision_conflict") {
				code = "revision_conflict"
			}
			result.Failures = append(result.Failures, chainproxy.RefreshFailure{GroupID: group.ID, SourceID: source.ID, Code: code})
			if code == "revision_conflict" {
				latest, loadErr := s.ChainProxyState()
				if loadErr == nil {
					result.State = latest
				}
				return result, nil
			}
		}
	}
	return result, nil
}
