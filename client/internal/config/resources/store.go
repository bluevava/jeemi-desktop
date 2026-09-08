package resources

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"jeemi/internal/configfile"
	"jeemi/internal/platform/paths"
)

const maxLibraryBytes = 8 << 20

type StoreOptions struct {
	DataDirectory string
	Now           func() time.Time
	NewID         func() (string, error)
}

type Store struct {
	mu    sync.Mutex
	root  string
	file  string
	now   func() time.Time
	newID func() (string, error)
}

func NewStore(options StoreOptions) (*Store, error) {
	file, err := paths.LocalConfigResourcesFileFromRoot(options.DataDirectory)
	if err != nil {
		return nil, err
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	newID := options.NewID
	if newID == nil {
		newID = randomID
	}
	return &Store{root: filepath.Dir(file), file: file, now: now, newID: newID}, nil
}

func (s *Store) State() (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadUnlocked()
}

func (s *Store) SaveStrategyGroup(input StrategyGroup) (State, error) {
	return s.SaveStrategyGroupValidated(input, nil)
}

func (s *Store) SaveStrategyGroupValidated(input StrategyGroup, validate func(State) error) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.loadUnlocked()
	if err != nil {
		return State{}, err
	}
	now := s.now().UTC()
	index := -1
	if input.ID == "" {
		// The library owns identity and timestamps.  A new resource must not be
		// able to smuggle client-provided persistence metadata into the store.
		input.CreatedAt = ""
		input.UpdatedAt = ""
		input.ID, err = s.newID()
		if err != nil || !resourceIDPattern.MatchString(input.ID) {
			return State{}, fmt.Errorf("create strategy group id")
		}
	} else {
		if !resourceIDPattern.MatchString(input.ID) {
			return State{}, fmt.Errorf("strategy group id is invalid")
		}
		for candidate := range state.StrategyGroups {
			if state.StrategyGroups[candidate].ID == input.ID {
				index = candidate
				input.CreatedAt = state.StrategyGroups[candidate].CreatedAt
				if input.Kind != state.StrategyGroups[candidate].Kind {
					return State{}, fmt.Errorf("strategy group kind cannot be changed")
				}
				break
			}
		}
		if index < 0 {
			return State{}, fmt.Errorf("strategy group does not exist")
		}
	}
	input, err = normalizeStrategyGroup(input, now)
	if err != nil {
		return State{}, err
	}
	for _, item := range state.StrategyGroups {
		if item.ID != input.ID && item.Name == input.Name {
			return State{}, fmt.Errorf("strategy group name already exists")
		}
	}
	if index < 0 {
		state.StrategyGroups = append(state.StrategyGroups, input)
	} else {
		state.StrategyGroups[index] = input
	}
	if err := validateState(state.StrategyGroups, state.RuleSets); err != nil {
		return State{}, err
	}
	if validate != nil {
		if err := validate(state); err != nil {
			return State{}, err
		}
	}
	return s.saveUnlocked(state)
}

func (s *Store) DeleteStrategyGroup(id string) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !resourceIDPattern.MatchString(id) {
		return State{}, fmt.Errorf("strategy group id is invalid")
	}
	state, err := s.loadUnlocked()
	if err != nil {
		return State{}, err
	}
	next := make([]StrategyGroup, 0, len(state.StrategyGroups))
	found := false
	for _, item := range state.StrategyGroups {
		if item.ID == id {
			found = true
			continue
		}
		next = append(next, item)
	}
	if !found {
		return State{}, fmt.Errorf("strategy group does not exist")
	}
	state.StrategyGroups = next
	return s.saveUnlocked(state)
}

func (s *Store) SaveRuleSet(input RuleSet) (State, error) {
	return s.SaveRuleSetValidated(input, nil)
}

func (s *Store) SaveRuleSetValidated(input RuleSet, validate func(State) error) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.loadUnlocked()
	if err != nil {
		return State{}, err
	}
	return s.saveRuleSetUnlocked(input, state, validate)
}

func (s *Store) saveRuleSetUnlocked(input RuleSet, state State, validate func(State) error) (State, error) {
	var err error
	now := s.now().UTC()
	index := -1
	if input.ID == "" {
		input.CreatedAt = ""
		input.UpdatedAt = ""
		input.ID, err = s.newID()
		if err != nil || !resourceIDPattern.MatchString(input.ID) {
			return State{}, fmt.Errorf("create rule set id")
		}
	} else {
		if !resourceIDPattern.MatchString(input.ID) {
			return State{}, fmt.Errorf("rule set id is invalid")
		}
		for candidate := range state.RuleSets {
			if state.RuleSets[candidate].ID == input.ID {
				index = candidate
				input.CreatedAt = state.RuleSets[candidate].CreatedAt
				break
			}
		}
		if index < 0 {
			return State{}, fmt.Errorf("rule set does not exist")
		}
	}
	input, err = normalizeRuleSet(input, now)
	if err != nil {
		return State{}, err
	}
	// Enforce this on candidate saves only. Old invalid definitions must
	// remain loadable so the editor can repair them.
	if input.Behavior == "classical" && input.Format == "mrs" {
		return State{}, fmt.Errorf("classical rule sets require yaml or text format")
	}
	for _, item := range state.RuleSets {
		if item.ID != input.ID && item.Name == input.Name {
			return State{}, fmt.Errorf("rule set name already exists")
		}
	}
	if index < 0 {
		state.RuleSets = append(state.RuleSets, input)
	} else {
		state.RuleSets[index] = input
	}
	if err := validateState(state.StrategyGroups, state.RuleSets); err != nil {
		return State{}, err
	}
	if validate != nil {
		if err := validate(state); err != nil {
			return State{}, err
		}
	}
	return s.saveUnlocked(state)
}

func (s *Store) DeleteRuleSet(id string) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !resourceIDPattern.MatchString(id) {
		return State{}, fmt.Errorf("rule set id is invalid")
	}
	state, err := s.loadUnlocked()
	if err != nil {
		return State{}, err
	}
	next := make([]RuleSet, 0, len(state.RuleSets))
	found := false
	for _, item := range state.RuleSets {
		if item.ID == id {
			found = true
			continue
		}
		next = append(next, item)
	}
	if !found {
		return State{}, fmt.Errorf("rule set does not exist")
	}
	for _, group := range state.StrategyGroups {
		for _, reference := range group.RuleSetReferences {
			if reference.RuleSetID == id {
				return State{}, fmt.Errorf("rule set is still referenced by a strategy group")
			}
		}
	}
	state.RuleSets = next
	return s.saveUnlocked(state)
}

func (s *Store) loadUnlocked() (result State, resultErr error) {
	contents, err := os.ReadFile(s.file)
	if errors.Is(err, os.ErrNotExist) {
		return State{Directory: s.root, StrategyGroups: []StrategyGroup{}, RuleSets: []RuleSet{}}, nil
	}
	if err != nil {
		return State{}, configfile.Unreadable(s.file, fmt.Errorf("read local resource library: %w", err))
	}
	defer func() {
		if resultErr != nil {
			resultErr = configfile.Invalid(s.file, contents, resultErr)
		}
	}()
	if len(contents) > maxLibraryBytes {
		return State{}, fmt.Errorf("local resource library exceeds size limit")
	}
	var document libraryDocument
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		return State{}, fmt.Errorf("parse local resource library: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return State{}, fmt.Errorf("parse local resource library: trailing data")
	}
	if document.Version != documentVersion || document.Revision < 1 {
		return State{}, fmt.Errorf("local resource library version is unsupported")
	}
	if _, err := time.Parse(time.RFC3339Nano, document.UpdatedAt); err != nil {
		return State{}, fmt.Errorf("local resource library update time is invalid")
	}
	if document.StrategyGroups == nil {
		document.StrategyGroups = []StrategyGroup{}
	}
	if document.RuleSets == nil {
		document.RuleSets = []RuleSet{}
	}
	state := State{
		Directory:      s.root,
		Revision:       document.Revision,
		UpdatedAt:      document.UpdatedAt,
		StrategyGroups: document.StrategyGroups,
		RuleSets:       document.RuleSets,
	}
	if err := validateState(state.StrategyGroups, state.RuleSets); err != nil {
		return State{}, err
	}
	return state, nil
}

func (s *Store) saveUnlocked(state State) (State, error) {
	state.Revision++
	state.UpdatedAt = s.now().UTC().Format(time.RFC3339Nano)
	sort.Slice(state.StrategyGroups, func(i, j int) bool { return state.StrategyGroups[i].Name < state.StrategyGroups[j].Name })
	sort.Slice(state.RuleSets, func(i, j int) bool { return state.RuleSets[i].Name < state.RuleSets[j].Name })
	document := libraryDocument{
		Version:        documentVersion,
		Revision:       state.Revision,
		UpdatedAt:      state.UpdatedAt,
		StrategyGroups: state.StrategyGroups,
		RuleSets:       state.RuleSets,
	}
	contents, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return State{}, err
	}
	if len(contents)+1 > maxLibraryBytes {
		return State{}, fmt.Errorf("local resource library is too large")
	}
	if err := os.MkdirAll(s.root, 0o700); err != nil {
		return State{}, fmt.Errorf("create local resource directory: %w", err)
	}
	temporary, err := os.CreateTemp(s.root, ".library-*.tmp")
	if err != nil {
		return State{}, err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return State{}, err
	}
	if _, err := temporary.Write(append(contents, '\n')); err != nil {
		temporary.Close()
		return State{}, err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return State{}, err
	}
	if err := temporary.Close(); err != nil {
		return State{}, err
	}
	if err := os.Rename(temporaryName, s.file); err != nil {
		return State{}, fmt.Errorf("activate local resource library: %w", err)
	}
	state.Directory = s.root
	return state, nil
}

func validateState(groups []StrategyGroup, ruleSets []RuleSet) error {
	seen := make(map[string]struct{}, len(groups)+len(ruleSets))
	for index := range groups {
		item := groups[index]
		if !resourceIDPattern.MatchString(item.ID) {
			return fmt.Errorf("stored strategy group id is invalid")
		}
		if _, duplicate := seen["group-id:"+item.ID]; duplicate {
			return fmt.Errorf("stored strategy group id is duplicated")
		}
		seen["group-id:"+item.ID] = struct{}{}
		displayName := strategyGroupDisplayName(item)
		if _, duplicate := seen["group-name:"+displayName]; duplicate {
			return fmt.Errorf("stored strategy group name is duplicated")
		}
		seen["group-name:"+displayName] = struct{}{}
		updatedAt, updatedErr := time.Parse(time.RFC3339Nano, item.UpdatedAt)
		if updatedErr != nil {
			return fmt.Errorf("stored strategy group %q update time is invalid", item.Name)
		}
		if _, createdErr := time.Parse(time.RFC3339Nano, item.CreatedAt); createdErr != nil {
			return fmt.Errorf("stored strategy group %q creation time is invalid", item.Name)
		}
		normalized, err := normalizeStrategyGroup(item, updatedAt)
		if err != nil {
			return fmt.Errorf("stored strategy group %q is invalid: %w", item.Name, err)
		}
		groups[index] = normalized
	}
	for index := range ruleSets {
		item := ruleSets[index]
		if !resourceIDPattern.MatchString(item.ID) {
			return fmt.Errorf("stored rule set id is invalid")
		}
		if _, duplicate := seen["rule-id:"+item.ID]; duplicate {
			return fmt.Errorf("stored rule set id is duplicated")
		}
		seen["rule-id:"+item.ID] = struct{}{}
		if _, duplicate := seen["rule-name:"+item.Name]; duplicate {
			return fmt.Errorf("stored rule set name is duplicated")
		}
		seen["rule-name:"+item.Name] = struct{}{}
		updatedAt, updatedErr := time.Parse(time.RFC3339Nano, item.UpdatedAt)
		if updatedErr != nil {
			return fmt.Errorf("stored rule set %q update time is invalid", item.Name)
		}
		if _, createdErr := time.Parse(time.RFC3339Nano, item.CreatedAt); createdErr != nil {
			return fmt.Errorf("stored rule set %q creation time is invalid", item.Name)
		}
		normalized, err := normalizeRuleSet(item, updatedAt)
		if err != nil {
			return fmt.Errorf("stored rule set %q is invalid: %w", item.Name, err)
		}
		ruleSets[index] = normalized
	}
	state := State{StrategyGroups: groups, RuleSets: ruleSets}
	ruleSetsByID := make(map[string]RuleSet, len(ruleSets))
	for _, ruleSet := range ruleSets {
		ruleSetsByID[ruleSet.ID] = ruleSet
	}
	ruleSetOwners := make(map[string]string, len(ruleSets))
	groupNames := make(map[string]string, len(groups))
	for _, group := range groups {
		groupNames[group.ID] = strategyGroupDisplayName(group)
	}
	for _, group := range groups {
		if err := validateStrategyGroupReferences(group, state); err != nil {
			return fmt.Errorf("stored strategy group %q has invalid references: %w", group.Name, err)
		}
		for _, reference := range group.RuleSetReferences {
			if owner, exists := ruleSetOwners[reference.RuleSetID]; exists && owner != group.ID {
				return fmt.Errorf(
					"rule set %q can only be referenced by one strategy group; it already belongs to %q",
					ruleSetsByID[reference.RuleSetID].Name,
					groupNames[owner],
				)
			}
			ruleSetOwners[reference.RuleSetID] = group.ID
		}
	}
	return nil
}

func randomID() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}
