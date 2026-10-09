package application

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"jeemi/internal/localscript"
	"jeemi/internal/subscription"
)

func TestSubscriptionDirectRetryStillValidatesBeforeCommit(t *testing.T) {
	for _, scenario := range []string{"valid", "invalid source", "handler conflict"} {
		t.Run(scenario, func(t *testing.T) {
			s, primary, state := fallbackService(t)
			id := state.SelectedSubscriptionID
			if scenario == "handler conflict" {
				script, err := s.SaveLocalScript(localscript.SaveInput{Name: "Conditional", Contents: "function main(config) { if (config.newSource) config['proxy-groups'] = {}; return config; }"})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := s.SetSubscriptionLocalScript(id, script.ID); err != nil {
					t.Fatal(err)
				}
			}
			before, err := s.Subscription(id)
			if err != nil {
				t.Fatal(err)
			}
			direct := &fallbackFetcher{contents: fallbackFixture + "newSource: true\n"}
			if scenario == "invalid source" {
				direct.contents = "proxy-groups: {}\n"
			}
			attempts := 0
			s.subscriptions, err = subscription.NewManager(subscription.ManagerOptions{
				DataDirectory: filepath.Dir(s.localConfigs.Directory()), Fetcher: primary,
				RefreshFallback: func() subscription.Fetcher {
					attempts++
					return direct
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			primary.err = errors.New("primary network unavailable")
			result, err := s.CheckSubscriptionRefresh(id)
			if attempts != 1 {
				t.Fatal("refresh did not make exactly one direct retry")
			}
			after, getErr := s.Subscription(id)
			if getErr != nil {
				t.Fatal(getErr)
			}
			if scenario == "valid" {
				if err != nil || result.State == nil || after.CurrentRevisionID == before.CurrentRevisionID {
					t.Fatalf("valid retried candidate was not committed: %v", err)
				}
				return
			}
			if !reflect.DeepEqual(before, after) {
				t.Fatal("unvalidated retry changed stored subscription")
			}
			if scenario == "invalid source" {
				if err == nil || result.Conflict != nil {
					t.Fatal("invalid retried source bypassed validation")
				}
				return
			}
			if err != nil || result.Conflict == nil {
				t.Fatalf("retried source lost handler conflict confirmation: %v", err)
			}
			candidate := direct.contents
			direct.contents = "mode: rule\n"
			if _, err := s.ResolveSubscriptionRefresh(result.Conflict.Token, true); err != nil {
				t.Fatal(err)
			}
			text, err := s.SubscriptionText(id)
			if err != nil || text.Contents != candidate || attempts != 1 {
				t.Fatal("confirmation refetched or replaced the checked DIRECT candidate")
			}
		})
	}
}

func TestSubscriptionRefreshHasNoDirectRouteWhenCoreIsStopped(t *testing.T) {
	s, _, _ := fallbackService(t)
	if s.subscriptionRefreshFallback() != nil {
		t.Fatal("stopped core offered a DIRECT fetcher")
	}
	if _, err := s.runtimeManager.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if s.subscriptionRefreshFallback() != nil {
		t.Fatal("refresh started the stopped core")
	}
}
