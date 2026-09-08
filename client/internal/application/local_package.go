package application

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"jeemi/internal/config/resources"
	"jeemi/internal/configtransfer"
	"jeemi/internal/localscript"
	"jeemi/internal/profile"
)

type LocalPackagePreview struct {
	Cancelled             bool     `json:"cancelled"`
	Token                 string   `json:"token"`
	Kind                  string   `json:"kind"`
	TargetName            string   `json:"targetName"`
	ImportedName          string   `json:"importedName"`
	OverwrittenGroups     []string `json:"overwrittenGroups"`
	OverwrittenRuleSets   []string `json:"overwrittenRuleSets"`
	AddedGroups           []string `json:"addedGroups"`
	AddedRuleSets         []string `json:"addedRuleSets"`
	AffectedConfigs       []string `json:"affectedConfigs"`
	AffectedSubscriptions []string `json:"affectedSubscriptions"`
}

type pendingLocalPackage struct {
	token     string
	expires   time.Time
	kind      string
	targetID  string
	revision  int
	config    profile.SaveLocalConfigInput
	resources resources.ImportCandidate
	script    localscript.SaveInput
}

// ExportLocalPackage returns only to the Go native-dialog boundary, never Wails.
func (s *Service) ExportLocalPackage(kind, id string) (data []byte, filename string, resultErr error) {
	defer func() {
		if resultErr != nil {
			resultErr = fmt.Errorf("%s", safeConfigurationError(resultErr))
		}
	}()
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()
	var p configtransfer.Package
	var name string
	switch kind {
	case configtransfer.ConfigKind:
		local, err := s.localConfigs.Get(id)
		if err != nil {
			return nil, "", err
		}
		state, err := s.configResources.State()
		if err != nil {
			return nil, "", err
		}
		p, err = configtransfer.FromConfig(local, state)
		if err != nil {
			return nil, "", err
		}
		name = local.Name
	case configtransfer.ScriptKind:
		script, err := s.localScripts.Get(id)
		if err != nil {
			return nil, "", err
		}
		p, name = configtransfer.FromScript(script), script.Name
	default:
		return nil, "", fmt.Errorf("unsupported local package type")
	}
	contents, err := configtransfer.Encode(p)
	return contents, configtransfer.SuggestedFilename(name), err
}

func (s *Service) PreviewLocalPackage(kind, id string, contents []byte) (result LocalPackagePreview, resultErr error) {
	defer func() {
		if resultErr != nil {
			resultErr = fmt.Errorf("%s", safeConfigurationError(resultErr))
		}
	}()
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()
	s.pendingPackage = nil
	p, err := configtransfer.Decode(contents, kind)
	if err != nil {
		return LocalPackagePreview{}, err
	}
	pending := &pendingLocalPackage{kind: kind, targetID: id}
	preview := LocalPackagePreview{Kind: kind, OverwrittenGroups: []string{}, OverwrittenRuleSets: []string{}, AddedGroups: []string{}, AddedRuleSets: []string{}, AffectedConfigs: []string{}, AffectedSubscriptions: []string{}}
	switch kind {
	case configtransfer.ConfigKind:
		current, err := s.localConfigs.Get(id)
		if err != nil {
			return LocalPackagePreview{}, err
		}
		pending.revision, preview.TargetName, preview.ImportedName = current.Revision, current.Name, p.Config.Name
		candidate, err := s.configResources.PrepareImport(p.Config.ResourcePlan, p.Config.StrategyGroups, p.Config.RuleSets)
		if err != nil {
			return LocalPackagePreview{}, err
		}
		pending.resources = candidate
		pending.config = profile.SaveLocalConfigInput{ID: id, Name: p.Config.Name, Description: p.Config.Description, Fields: p.Config.Fields, ResourcePlan: candidate.Plan}
		preview.OverwrittenGroups, preview.OverwrittenRuleSets = candidate.OverwrittenGroups, candidate.OverwrittenRuleSets
		preview.AddedGroups, preview.AddedRuleSets = candidate.AddedGroups, candidate.AddedRuleSets
	case configtransfer.ScriptKind:
		current, err := s.localScripts.Get(id)
		if err != nil {
			return LocalPackagePreview{}, err
		}
		pending.revision, preview.TargetName, preview.ImportedName = current.Revision, current.Name, p.Script.Name
		pending.script = localscript.SaveInput{ID: id, Name: p.Script.Name, Description: p.Script.Description, Contents: p.Script.Contents}
	}
	configs, subscriptions, err := s.validateLocalPackage(pending)
	if err != nil {
		return LocalPackagePreview{}, err
	}
	preview.AffectedConfigs, preview.AffectedSubscriptions = configs, subscriptions
	token := make([]byte, 24)
	if _, err := rand.Read(token); err != nil {
		return LocalPackagePreview{}, fmt.Errorf("create package confirmation")
	}
	pending.token, pending.expires = hex.EncodeToString(token), time.Now().Add(10*time.Minute)
	s.pendingPackage = pending
	preview.Token = pending.token
	return preview, nil
}

func (s *Service) ResolveLocalPackage(token string, confirm bool) (resultErr error) {
	defer func() {
		if resultErr != nil {
			resultErr = fmt.Errorf("%s", safeConfigurationError(resultErr))
		}
	}()
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()
	pending := s.pendingPackage
	if pending == nil || pending.token != token {
		return fmt.Errorf("package confirmation expired; select the file again")
	}
	s.pendingPackage = nil
	if !confirm {
		return nil
	}
	if time.Now().After(pending.expires) {
		return fmt.Errorf("package confirmation expired; select the file again")
	}
	if pending.kind == configtransfer.ConfigKind {
		current, err := s.localConfigs.Get(pending.targetID)
		if err != nil {
			return err
		}
		state, err := s.configResources.State()
		if err != nil {
			return err
		}
		if current.Revision != pending.revision || state.Revision != pending.resources.State.Revision {
			return fmt.Errorf("configuration or resources changed; select the package again")
		}
	} else {
		current, err := s.localScripts.Get(pending.targetID)
		if err != nil {
			return err
		}
		if current.Revision != pending.revision {
			return fmt.Errorf("script changed; select the package again")
		}
	}
	// Settings, subscription source and associations can change while the dialog
	// is open. Validate the retained bytes again using the current complete chain.
	if _, _, err := s.validateLocalPackage(pending); err != nil {
		return err
	}
	if pending.kind == configtransfer.ConfigKind {
		err := s.configResources.CommitImport(pending.resources.State, func(state resources.State, commitLibrary func() error) error {
			_, err := s.localConfigs.SaveWithResourceCommit(pending.config, state, commitLibrary)
			return err
		})
		if err != nil {
			return err
		}
		_ = s.reconcileSelectedWithTimeoutLocked(reconcileTriggerLocalConfig, nil, reconcileAutomatic)
	} else {
		if _, err := s.localScripts.Save(pending.script); err != nil {
			return err
		}
		_ = s.reconcileSelectedWithTimeoutLocked(reconcileTriggerLocalScript, nil, reconcileAutomatic)
	}
	return nil
}
