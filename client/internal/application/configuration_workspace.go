package application

import (
	"context"
	"fmt"

	"jeemi/internal/chainproxy"
	"jeemi/internal/config/resources"
	"jeemi/internal/configworkspace"
	"jeemi/internal/localscript"
	"jeemi/internal/profile"
	"jeemi/internal/subscription"
)

// PrepareConfigurationWorkspace is the explicit startup migration boundary.
// Ordinary store constructors and readers never create or migrate a workspace.
func PrepareConfigurationWorkspace(ctx context.Context, dataRoot string) (string, error) {
	return configworkspace.Prepare(ctx, dataRoot, configworkspace.Options{Validate: validateConfigurationWorkspace})
}

func validateConfigurationWorkspace(ctx context.Context, root string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := CheckStartupConfiguration(root); err != nil {
		return err
	}
	library, err := resources.NewStore(resources.StoreOptions{DataDirectory: root})
	if err != nil {
		return err
	}
	resourceState, err := library.State()
	if err != nil {
		return err
	}
	configs, err := profile.NewLocalConfigStore(profile.LocalConfigStoreOptions{DataDirectory: root})
	if err != nil {
		return err
	}
	configState, err := configs.State()
	if err != nil {
		return err
	}
	configIDs := map[string]bool{}
	for _, summary := range configState.Configs {
		if err := ctx.Err(); err != nil {
			return err
		}
		config, err := configs.Get(summary.ID)
		if err != nil {
			return err
		}
		if _, err := resources.ValidatePlan(config.ResourcePlan, resourceState); err != nil {
			return err
		}
		configIDs[summary.ID] = true
	}
	scripts, err := localscript.NewStore(localscript.StoreOptions{DataDirectory: root})
	if err != nil {
		return err
	}
	scriptState, err := scripts.State()
	if err != nil {
		return err
	}
	scriptIDs := map[string]bool{}
	for _, script := range scriptState.Scripts {
		scriptIDs[script.ID] = true
	}
	chains, err := chainproxy.NewStore(root)
	if err != nil {
		return err
	}
	chainState, err := chains.Load()
	if err != nil {
		return err
	}
	chainIDs := map[string]bool{}
	for _, group := range chainState.Groups {
		chainIDs[group.ID] = true
	}
	subscriptions, err := subscription.NewStore(subscription.StoreOptions{DataDirectory: root})
	if err != nil {
		return err
	}
	subscriptionState, err := subscriptions.State()
	if err != nil {
		return err
	}
	for _, item := range subscriptionState.Subscriptions {
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, err := subscriptions.Text(item.ID); err != nil {
			return err
		}
		if item.LocalConfigID != "" && !configIDs[item.LocalConfigID] {
			return fmt.Errorf("subscription references a missing local configuration")
		}
		if item.LocalScriptID != "" && !scriptIDs[item.LocalScriptID] {
			return fmt.Errorf("subscription references a missing local script")
		}
		for _, id := range item.ChainProxyGroupIDs {
			if !chainIDs[id] {
				return fmt.Errorf("subscription references a missing chain group")
			}
		}
	}
	return ctx.Err()
}
