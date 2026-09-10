package application

import (
	"context"
	"errors"
	"jeemi/internal/chainproxy"
	"jeemi/internal/config/fallbackoverride"
	"jeemi/internal/localscript"
	mihomoruntime "jeemi/internal/mihomo/runtime"
	"jeemi/internal/subscription"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const chainSource = `proxies:
  - {name: HK GM, type: ss, server: 192.0.2.1, port: 443, cipher: aes-128-gcm, password: fixture}
  - {name: JP GM, type: ss, server: 192.0.2.2, port: 443, cipher: aes-128-gcm, password: fixture}
proxy-groups:
  - {name: Main, type: select, proxies: [HK GM, JP GM]}
rules: ['MATCH,Main']
`
const chainLanding = "anytls://fixture-password@landing.example.invalid:443?type=tcp&fp=firefox&security=tls&sni=tls.example.invalid&udp=1&tfo=0#Landing"

func chainService(t *testing.T) (*Service, string, subscription.State) {
	t.Helper()
	root := t.TempDir()
	s, err := NewService(root)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "subscription.yaml")
	if err := os.WriteFile(file, []byte(chainSource), 0600); err != nil {
		t.Fatal(err)
	}
	state, err := s.ImportSubscriptionFile(file)
	if err != nil {
		t.Fatal(err)
	}
	return s, root, state
}

func manualChain(t *testing.T, s *Service) (chainproxy.State, string) {
	t.Helper()
	state, err := s.ChainProxyState()
	if err != nil {
		t.Fatal(err)
	}
	created, err := s.SaveChainProxyGroup(chainproxy.SaveGroupInput{Revision: state.Revision, Name: "Landing", Kind: "manual"})
	if err != nil {
		t.Fatal(err)
	}
	id := created.State.Groups[len(created.State.Groups)-1].ID
	imported, err := s.ImportChainProxyNodes(chainproxy.ImportInput{Revision: created.State.Revision, GroupID: id, Contents: chainLanding})
	if err != nil {
		t.Fatal(err)
	}
	return imported.State, id
}

func TestChainAssociationUsesFinalScriptAndFallbackAndPersistsFingerprint(t *testing.T) {
	s, _, state := chainService(t)
	id := state.SelectedSubscriptionID
	library, groupID := manualChain(t, s)
	rawBefore, err := s.SubscriptionText(id)
	if err != nil {
		t.Fatal(err)
	}
	script, err := s.SaveLocalScript(localscript.SaveInput{Name: "Final selectors", Contents: `function main(config) { config['proxy-groups'][0].name='Final'; config.rules=['MATCH,Final']; return config; }`})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetSubscriptionLocalScript(id, script.ID); err != nil {
		t.Fatal(err)
	}
	state, err = s.SetSubscriptionChainProxyGroups(id, []string{groupID}, 0, library.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if state.Projection.ChainProxy.Generated != 2 || len(state.Projection.Selectors[0].Members) != 4 || state.Projection.Selectors[0].Name != "Final" {
		t.Fatalf("chain stage/order: %+v", state.Projection)
	}
	view, err := s.RuntimeConfigurationText()
	if err != nil {
		t.Fatal(err)
	}
	if view.ChainProxyFingerprint != state.Projection.ChainProxyFingerprint || !strings.Contains(view.Contents, "dialer-proxy: HK GM") || strings.Contains(view.Contents, "external-controller") {
		t.Fatal("resolved chain missing or unsafe")
	}
	if !strings.Contains(view.Contents, "tfo: false") || !strings.Contains(view.Contents, "client-fingerprint: firefox") || !strings.Contains(view.Contents, "sni: tls.example.invalid") {
		t.Fatal("saved AnyTLS URI options were lost during chain generation")
	}
	before := view.Fingerprint
	changed, err := s.SetSubscriptionFallback(id, fallbackoverride.Selection{Mode: "direct"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if changed.State.Projection.ChainProxy.Generated != 0 {
		t.Fatal("default MATCH did not honor final DIRECT")
	}
	view, err = s.RuntimeConfigurationText()
	if err != nil || view.Fingerprint == before {
		t.Fatal("fallback did not invalidate runtime input")
	}
	rawAfter, _ := s.SubscriptionText(id)
	if rawAfter.Contents != rawBefore.Contents || rawAfter.RevisionID != rawBefore.RevisionID {
		t.Fatal("chain wrote subscription source")
	}
	if _, err := s.DeleteChainProxyItem(groupID, "group", groupID, library.Revision); err == nil {
		t.Fatal("referenced group deleted")
	}
	if _, err := s.SetSubscriptionChainProxyGroups(id, nil, 1, library.Revision); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeleteChainProxyItem(groupID, "group", groupID, library.Revision); err != nil {
		t.Fatal(err)
	}
	if s.runtimeManager.Status().PID != 0 {
		t.Fatal("offline edit started core")
	}
}

type chainFetchFixture struct {
	mu     sync.Mutex
	bodies map[string]string
	calls  []string
	wait   chan struct{}
}

func (f *chainFetchFixture) Fetch(ctx context.Context, url string) (subscription.FetchedContent, error) {
	f.mu.Lock()
	f.calls = append(f.calls, url)
	body := f.bodies[url]
	wait := f.wait
	f.mu.Unlock()
	if wait != nil {
		close(wait)
		<-ctx.Done()
		return subscription.FetchedContent{}, ctx.Err()
	}
	if body == "" {
		return subscription.FetchedContent{}, errors.New("fixture failed")
	}
	return subscription.FetchedContent{Contents: []byte(body), Format: subscription.FormatYAML}, nil
}

func TestChainURLRefreshSyncsStableSourceAndPreservesFailedSource(t *testing.T) {
	s, root, state := chainService(t)
	fetcher := &chainFetchFixture{bodies: map[string]string{"https://source.example.invalid/one?token=fixture": chainLanding, "https://source.example.invalid/two": strings.Replace(chainLanding, "Landing", "Other", 1)}}
	s.chainFetcher = fetcher
	created, err := s.SaveChainProxyGroup(chainproxy.SaveGroupInput{Name: "URLs", Kind: "subscription"})
	if err != nil {
		t.Fatal(err)
	}
	groupID := created.State.Groups[0].ID
	current := created.State
	for _, address := range []string{"https://source.example.invalid/one?token=fixture", "https://source.example.invalid/two"} {
		result, err := s.SaveChainProxySource(chainproxy.SourceInput{Revision: current.Revision, GroupID: groupID, URL: address})
		if err != nil {
			t.Fatal(err)
		}
		current = result.State
	}
	oldID := current.Groups[0].Nodes[0].ID
	state, err = s.SetSubscriptionChainProxyGroups(state.SelectedSubscriptionID, []string{groupID}, 0, current.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if state.Projection.ChainProxy.Generated != 4 {
		t.Fatal("source associations lost")
	}
	fetcher.bodies["https://source.example.invalid/one?token=fixture"] = strings.Replace(chainLanding, "fixture-password", "new-password", 1) + "\n" + strings.Replace(chainLanding, "Landing", "New", 1)
	delete(fetcher.bodies, "https://source.example.invalid/two")
	result, err := s.RefreshChainProxySources(groupID, current.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Failures) != 1 || len(result.State.Groups[0].Nodes) != 3 || result.State.Groups[0].Nodes[0].ID != oldID {
		t.Fatalf("source refresh: %+v", result)
	}
	state, err = s.SubscriptionState()
	if err != nil || state.Projection.ChainProxy.Generated != 6 {
		t.Fatal("association did not follow source", err)
	}
	if len(fetcher.calls) != 4 {
		t.Fatal("fetched per node instead of per source")
	}
	// Ordinary reopen reads the cached group and node identities, without fetch.
	reopened, err := NewService(root)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := reopened.ChainProxyState()
	if err != nil || len(loaded.Groups[0].Nodes) != 3 {
		t.Fatal("reopen lost cache", err)
	}
	for _, source := range loaded.Groups[0].Sources {
		if strings.Contains(source.Label, "token") {
			t.Fatal("full URL leaked")
		}
	}
}

func TestChainSharedEditValidatesInactiveSubscriptionsAndKeepsLastGoodSnapshot(t *testing.T) {
	s, _, state := chainService(t)
	library, groupID := manualChain(t, s)
	if _, err := s.SetSubscriptionChainProxyGroups(state.SelectedSubscriptionID, []string{groupID}, 0, library.Revision); err != nil {
		t.Fatal(err)
	}
	conflictFile := filepath.Join(t.TempDir(), "other.yaml")
	conflictSource := chainSource + "dns:\n  proxy-server-nameserver-policy:\n    landing.example.invalid: [8.8.8.8]\n"
	if err := os.WriteFile(conflictFile, []byte(conflictSource), 0600); err != nil {
		t.Fatal(err)
	}
	other, err := s.ImportSubscriptionFile(conflictFile)
	if err != nil {
		t.Fatal(err)
	}
	var otherID string
	for _, item := range other.Subscriptions {
		if item.ID != state.SelectedSubscriptionID {
			otherID = item.ID
		}
	}
	if _, err := s.SetSubscriptionChainProxyGroups(otherID, []string{groupID}, 0, library.Revision); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SelectSubscription(state.SelectedSubscriptionID); err != nil {
		t.Fatal(err)
	}
	before, err := s.RuntimeConfigurationText()
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.ImportChainProxyNodes(chainproxy.ImportInput{Revision: library.Revision, GroupID: groupID, NodeID: library.Groups[0].Nodes[0].ID, Contents: "host://landing.example.invalid?server=1.1.1.1\n" + chainLanding})
	if err == nil || !strings.Contains(err.Error(), "chain_proxy:dns_conflict") {
		t.Fatal("inactive subscription did not reject DNS conflict", err)
	}
	after, err := s.RuntimeConfigurationText()
	if err != nil || after.Fingerprint != before.Fingerprint {
		t.Fatal("failed shared edit changed snapshot")
	}
	saved, _ := s.ChainProxyState()
	if saved.Revision != library.Revision {
		t.Fatal("failed edit committed library")
	}
	if _, err := s.ImportChainProxyNodes(chainproxy.ImportInput{Revision: library.Revision - 1, GroupID: groupID, Contents: chainLanding}); err == nil {
		t.Fatal("stale edit accepted")
	}
}

func TestChainRefreshCancellationRetainsStoredSource(t *testing.T) {
	s, _, _ := chainService(t)
	created, err := s.SaveChainProxyGroup(chainproxy.SaveGroupInput{Name: "URL", Kind: "subscription"})
	if err != nil {
		t.Fatal(err)
	}
	fetcher := &chainFetchFixture{bodies: map[string]string{}, wait: make(chan struct{})}
	s.chainFetcher = fetcher
	done := make(chan error, 1)
	go func() {
		_, err := s.SaveChainProxySource(chainproxy.SourceInput{Revision: created.State.Revision, GroupID: created.State.Groups[0].ID, URL: "https://source.example.invalid"})
		done <- err
	}()
	select {
	case <-fetcher.wait:
	case <-time.After(3 * time.Second):
		t.Fatal("fetch did not start")
	}
	s.CancelChainProxyRefresh()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled fetch succeeded")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("cancelled fetch did not return")
	}
	state, err := s.ChainProxyState()
	if err != nil || state.Revision != created.State.Revision || len(state.Groups[0].Nodes) != 0 {
		t.Fatal("cancelled fetch changed data")
	}
}

func TestChainChangeCannotUseRuntimeModeFastPath(t *testing.T) {
	a := mihomoruntime.Source{SubscriptionID: strings.Repeat("a", 32), ChainProxyFingerprint: strings.Repeat("b", 64)}
	b := a
	b.ChainProxyFingerprint = strings.Repeat("c", 64)
	if sameRuntimeSource(a, b) {
		t.Fatal("changed landing nodes reused active source")
	}
}
