package application

import (
	"context"
	"jeemi/internal/chainproxy"
	"jeemi/internal/subscription"
	"slices"
	"strings"
	"time"
)

func (s *Service) ChainProxyState() (chainproxy.State, error) {
	library, err := s.chainProxies.Load()
	if err != nil {
		return chainproxy.State{}, err
	}
	return chainproxy.List(library), nil
}

func (s *Service) SaveChainProxyGroup(input chainproxy.SaveGroupInput) (chainproxy.MutationResult, error) {
	ctx, cancel := s.operationContext(45 * time.Second)
	defer cancel()
	return s.commitChainChange(ctx, input.Revision, input.ID, func(library *chainproxy.Library) error {
		group := chainproxy.Group{ID: input.ID, Name: strings.TrimSpace(input.Name), Kind: input.Kind, SelectorFilter: strings.TrimSpace(input.SelectorFilter), NodeFilter: strings.TrimSpace(input.NodeFilter), Nodes: []chainproxy.Node{}, Sources: []chainproxy.Source{}}
		if input.ID == "" {
			id, err := chainproxy.NewID()
			if err != nil {
				return err
			}
			group.ID = id
			library.Groups = append(library.Groups, group)
		} else {
			previous, err := chainproxy.FindGroup(library, input.ID)
			if err != nil {
				return err
			}
			if previous.Kind != group.Kind {
				return chainError("invalid_group")
			}
			group.Nodes, group.Sources = previous.Nodes, previous.Sources
			*previous = group
		}
		return nil
	})
}

func (s *Service) ImportChainProxyNodes(input chainproxy.ImportInput) (chainproxy.MutationResult, error) {
	parsed, err := chainproxy.Parse([]byte(input.Contents), "")
	if err != nil {
		return chainproxy.MutationResult{}, err
	}
	if len(parsed.Nodes) == 0 || input.NodeID != "" && len(parsed.Nodes) != 1 {
		return chainproxy.MutationResult{}, chainError("single_node_required")
	}
	ctx, cancel := s.operationContext(45 * time.Second)
	defer cancel()
	result, err := s.commitChainChange(ctx, input.Revision, input.GroupID, func(library *chainproxy.Library) error {
		group, err := chainproxy.FindGroup(library, input.GroupID)
		if err != nil {
			return err
		}
		if group.Kind != "manual" {
			return chainError("invalid_group")
		}
		if input.NodeID != "" {
			found := false
			for i := range group.Nodes {
				if group.Nodes[i].ID == input.NodeID {
					parsed.Nodes[0].ID = input.NodeID
					group.Nodes[i] = parsed.Nodes[0]
					found = true
					break
				}
			}
			if !found {
				return chainError("node_missing")
			}
		} else {
			for _, node := range parsed.Nodes {
				node.ID, err = chainproxy.NewID()
				if err != nil {
					return err
				}
				group.Nodes = append(group.Nodes, node)
			}
		}
		names := map[string]bool{}
		for _, node := range group.Nodes {
			if names[node.Name] {
				return chainError("duplicate_node_name")
			}
			names[node.Name] = true
		}
		return nil
	})
	result.Report = &parsed.Report
	return result, err
}

// kind identifies a managed object, never a path. Referenced groups cannot be
// deleted; node/source changes validate all their current subscriptions first.
func (s *Service) DeleteChainProxyItem(groupID, kind, id string, revision int) (chainproxy.MutationResult, error) {
	ctx, cancel := s.operationContext(45 * time.Second)
	defer cancel()
	return s.commitChainChange(ctx, revision, groupID, func(library *chainproxy.Library) error {
		group, err := chainproxy.FindGroup(library, groupID)
		if err != nil {
			return err
		}
		switch kind {
		case "group":
			state, err := s.subscriptions.State()
			if err != nil {
				return err
			}
			for _, item := range state.Subscriptions {
				if slices.Contains(item.ChainProxyGroupIDs, groupID) {
					return chainError("group_in_use")
				}
			}
			library.Groups = slices.DeleteFunc(library.Groups, func(g chainproxy.Group) bool { return g.ID == groupID })
		case "source":
			length := len(group.Sources)
			group.Sources = slices.DeleteFunc(group.Sources, func(source chainproxy.Source) bool { return source.ID == id })
			if length == len(group.Sources) {
				return chainError("source_missing")
			}
		case "node":
			if group.Kind != "manual" {
				return chainError("invalid_group")
			}
			length := len(group.Nodes)
			group.Nodes = slices.DeleteFunc(group.Nodes, func(node chainproxy.Node) bool { return node.ID == id })
			if length == len(group.Nodes) {
				return chainError("node_missing")
			}
		default:
			return chainError("invalid_group")
		}
		return nil
	})
}

func (s *Service) ChainProxyNodeText(groupID, nodeID string) (string, error) {
	library, err := s.chainProxies.Load()
	if err != nil {
		return "", err
	}
	group, err := chainproxy.FindGroup(&library, groupID)
	if err != nil {
		return "", err
	}
	for _, node := range group.LandingNodes() {
		if node.ID == nodeID {
			return chainproxy.EditableNode(node), nil
		}
	}
	return "", chainError("node_missing")
}

func (s *Service) ChainProxySource(groupID, sourceID string) (chainproxy.SourceInput, error) {
	library, err := s.chainProxies.Load()
	if err != nil {
		return chainproxy.SourceInput{}, err
	}
	group, err := chainproxy.FindGroup(&library, groupID)
	if err != nil {
		return chainproxy.SourceInput{}, err
	}
	for _, source := range group.Sources {
		if source.ID == sourceID {
			return chainproxy.SourceInput{Revision: library.Revision, GroupID: groupID, ID: source.ID, URL: source.URL, Filter: source.Filter}, nil
		}
	}
	return chainproxy.SourceInput{}, chainError("source_missing")
}

func (s *Service) commitChainChange(ctx context.Context, revision int, groupID string, change func(*chainproxy.Library) error) (chainproxy.MutationResult, error) {
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()
	library, err := s.chainProxies.Load()
	if err != nil {
		return chainproxy.MutationResult{}, err
	}
	if library.Revision != revision {
		return chainproxy.MutationResult{}, chainError("revision_conflict")
	}
	if err := change(&library); err != nil {
		return chainproxy.MutationResult{}, err
	}
	if err := chainproxy.ValidateLibrary(library); err != nil {
		return chainproxy.MutationResult{}, err
	}
	state, err := s.subscriptions.State()
	if err != nil {
		return chainproxy.MutationResult{}, err
	}
	for _, item := range state.Subscriptions {
		if !slices.Contains(item.ChainProxyGroupIDs, groupID) {
			continue
		}
		if _, err := s.validateSubscriptionCandidate(ctx, item, compositionCandidate{chains: &library}); err != nil {
			return chainproxy.MutationResult{}, candidateValidationError(item, err)
		}
	}
	if err := ctx.Err(); err != nil {
		return chainproxy.MutationResult{}, chainError("cancelled")
	}
	saved, err := s.chainProxies.Commit(revision, library)
	if err != nil {
		return chainproxy.MutationResult{}, err
	}
	_ = s.reconcileSelectedWithTimeoutLocked(reconcileTriggerSubscription, nil, reconcileAutomatic)
	return chainproxy.MutationResult{State: chainproxy.List(saved), Failures: []chainproxy.RefreshFailure{}}, nil
}

func (s *Service) SetSubscriptionChainProxyGroups(id string, groupIDs []string, revision int, libraryRevision int) (subscription.State, error) {
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()
	library, err := s.chainProxies.Load()
	if err != nil {
		return subscription.State{}, err
	}
	if len(groupIDs) > 0 && library.Revision != libraryRevision {
		return subscription.State{}, chainError("revision_conflict")
	}
	if _, err := chainproxy.SelectedGroups(library, groupIDs); err != nil {
		return subscription.State{}, err
	}
	detail, err := s.subscriptions.Get(id)
	if err != nil {
		return subscription.State{}, err
	}
	if detail.ChainProxyRevision != revision {
		return subscription.State{}, chainError("revision_conflict")
	}
	if !slices.Equal(detail.ChainProxyGroupIDs, groupIDs) {
		candidate := detail.Summary
		candidate.ChainProxyGroupIDs = append([]string{}, groupIDs...)
		candidate.ChainProxyRevision++
		ctx, cancel := s.operationContext(45 * time.Second)
		defer cancel()
		if len(groupIDs) > 0 {
			if _, err := s.validateSubscriptionCandidate(ctx, candidate, compositionCandidate{chains: &library}); err != nil {
				return subscription.State{}, candidateValidationError(candidate, err)
			}
		}
		if _, err := s.subscriptions.SetChainProxyGroups(id, groupIDs, revision); err != nil {
			return subscription.State{}, err
		}
		_ = s.reconcileSelectedWithTimeoutLocked(reconcileTriggerSubscription, nil, reconcileAutomatic)
	}
	return s.SubscriptionState()
}

func chainError(code string) error { return &chainproxy.Error{Code: code} }
