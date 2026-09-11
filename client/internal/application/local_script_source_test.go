package application

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"jeemi/internal/configtransfer"
	"jeemi/internal/localscript"
	"jeemi/internal/subscription"
)

type scriptFetchFunc func(context.Context, string) (string, error)

func (f scriptFetchFunc) Fetch(ctx context.Context, address string) (string, error) {
	return f(ctx, address)
}

func TestURLScriptImportUsesCacheUntilExplicitRefresh(t *testing.T) {
	s, _, state := fallbackService(t)
	calls := 0
	s.scriptFetcher = scriptFetchFunc(func(_ context.Context, address string) (string, error) {
		calls++
		if address != "https://example.test/script?token=private" {
			t.Error("unexpected source")
		}
		return "function main(c) { c['tcp-concurrent'] = true; return c; }", nil
	})
	script, err := s.SaveLocalScript(localscript.SaveInput{Name: "URL", Contents: " https://example.test/script?token=private \n"})
	if err != nil || calls != 1 || script.SourceType != "url" || !strings.HasPrefix(script.Contents, "function main") {
		t.Fatal("URL was not imported", err)
	}
	if _, err := s.SetSubscriptionLocalScript(state.SelectedSubscriptionID, script.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.LocalScriptState(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.LocalScript(script.ID); err != nil {
		t.Fatal(err)
	}
	input := localscript.SaveInput{ID: script.ID, Name: "Renamed", Contents: script.SourceURL, ExpectedRevision: script.Revision}
	if _, err := s.TestLocalScript(localscript.TestInput{SaveInput: input, SubscriptionID: state.SelectedSubscriptionID}); err != nil {
		t.Fatal(err)
	}
	updated, err := s.SaveLocalScript(input)
	if err != nil || calls != 1 || updated.SourceURL != script.SourceURL {
		t.Fatal("ordinary use re-downloaded a saved script", err)
	}
	if _, err := s.RefreshLocalScript(script.ID); err != nil || calls != 2 {
		t.Fatal("explicit refresh did not download", err)
	}
	if _, err := s.SaveLocalScript(input); err == nil {
		t.Fatal("old editor overwrote the refreshed script")
	}
}

func TestURLScriptFailuresPreserveAllReferencesAndResolved(t *testing.T) {
	s, subscriptions, state := fallbackService(t)
	s.scriptFetcher = scriptFetchFunc(func(context.Context, string) (string, error) { return "function main(c) { return c; }", nil })
	script, err := s.SaveLocalScript(localscript.SaveInput{Name: "Remote", SourceURL: "https://example.test/code"})
	if err != nil {
		t.Fatal(err)
	}
	first := state.SelectedSubscriptionID
	if _, err := s.SetSubscriptionLocalScript(first, script.ID); err != nil {
		t.Fatal(err)
	}
	subscriptions.contents = "special: true\n" + fallbackFixture
	secondState, err := s.ImportSubscriptionURL(subscription.ImportURLInput{Name: "Inactive", SourceURL: "https://example.test/second"})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range secondState.Subscriptions {
		if item.ID != first {
			if _, err := s.SetSubscriptionLocalScript(item.ID, script.ID); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := s.SelectSubscription(first); err != nil {
		t.Fatal(err)
	}
	before, _ := s.RuntimeConfigurationText()
	for _, body := range []string{"", "<html>login</html>", "function main(c) { if(c.special) c['proxy-groups'] = {}; return c; }"} {
		s.scriptFetcher = scriptFetchFunc(func(context.Context, string) (string, error) {
			if body == "" {
				return "", errors.New("download failed")
			}
			return body, nil
		})
		if _, err := s.RefreshLocalScript(script.ID); err == nil {
			t.Fatal("invalid update was accepted")
		}
		if _, err := s.SaveLocalScript(localscript.SaveInput{ID: script.ID, Name: script.Name, Contents: "https://example.test/changed-url"}); err == nil {
			t.Fatal("invalid changed URL was accepted")
		}
		after, err := s.LocalScript(script.ID)
		view, viewErr := s.RuntimeConfigurationText()
		if err != nil || viewErr != nil || !reflect.DeepEqual(after, script) || view.Contents != before.Contents {
			t.Fatal("failed download/validation changed local state", err, viewErr)
		}
	}
	s.scriptFetcher = scriptFetchFunc(func(context.Context, string) (string, error) {
		return "function main(c) { c['tcp-concurrent'] = true; return c; }", nil
	})
	if _, err := s.RefreshLocalScript(script.ID); err != nil {
		t.Fatal(err)
	}
	view, _ := s.RuntimeConfigurationText()
	if !strings.Contains(view.Contents, "tcp-concurrent: true") {
		t.Fatal("updated cached source was not composed")
	}
}

func TestScriptDownloadDoesNotOverwriteConcurrentEdit(t *testing.T) {
	s, _, _ := fallbackService(t)
	s.scriptFetcher = scriptFetchFunc(func(context.Context, string) (string, error) { return "function main(c) { return c; }", nil })
	script, err := s.SaveLocalScript(localscript.SaveInput{Name: "Remote", SourceURL: "https://example.test/script"})
	if err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	s.scriptFetcher = scriptFetchFunc(func(context.Context, string) (string, error) {
		close(entered)
		<-release
		return "function main(c) { c.mode='direct'; return c; }", nil
	})
	done := make(chan error, 1)
	go func() { _, err := s.RefreshLocalScript(script.ID); done <- err }()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("download did not start")
	}
	newer, editErr := s.SaveLocalScript(localscript.SaveInput{ID: script.ID, Name: "Local edit", Contents: "function main(c) { return c; }", ExpectedRevision: script.Revision})
	close(release)
	if editErr != nil {
		t.Fatal(editErr)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("stale download replaced local edit")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("download did not complete")
	}
	current, err := s.LocalScript(script.ID)
	if err != nil || !reflect.DeepEqual(current, newer) {
		t.Fatal("newer edit changed", err)
	}
}

func TestURLScriptPackagePreservesSourceWithoutDownloading(t *testing.T) {
	s, _, _ := fallbackService(t)
	s.scriptFetcher = scriptFetchFunc(func(context.Context, string) (string, error) { return "function main(c) { return c; }", nil })
	script, err := s.SaveLocalScript(localscript.SaveInput{Name: "Remote", SourceURL: "https://example.test/script?token=private"})
	if err != nil {
		t.Fatal(err)
	}
	data, _, err := s.ExportLocalPackage(configtransfer.ScriptKind, script.ID)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := s.SaveLocalScript(localscript.SaveInput{Name: "Text", Contents: "function main(c) { return c; }"})
	if err != nil {
		t.Fatal(err)
	}
	s.scriptFetcher = scriptFetchFunc(func(context.Context, string) (string, error) {
		t.Error("package import must use cached code")
		return "", errors.New("unexpected download")
	})
	preview, err := s.PreviewLocalPackage(configtransfer.ScriptKind, plain.ID, data)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ResolveLocalPackage(preview.Token, true); err != nil {
		t.Fatal(err)
	}
	imported, err := s.LocalScript(plain.ID)
	if err != nil || imported.SourceURL != script.SourceURL || imported.Contents != script.Contents {
		t.Fatal("package lost remote source", err)
	}
}

func TestNewURLScriptPreviewAndCancellationDoNotPersist(t *testing.T) {
	s, _, state := fallbackService(t)
	before, err := s.LocalScriptState()
	if err != nil {
		t.Fatal(err)
	}
	s.scriptFetcher = scriptFetchFunc(func(context.Context, string) (string, error) {
		return "function main(c) { return c; }", nil
	})
	input := localscript.SaveInput{Name: "Preview", Contents: "https://example.test/new-script"}
	if _, err := s.TestLocalScript(localscript.TestInput{SaveInput: input, SubscriptionID: state.SelectedSubscriptionID}); err != nil {
		t.Fatal(err)
	}
	after, err := s.LocalScriptState()
	if err != nil || !reflect.DeepEqual(after, before) {
		t.Fatal("preview persisted a script", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.mu.Lock()
	s.ctx = ctx
	s.mu.Unlock()
	s.scriptFetcher = scriptFetchFunc(func(context.Context, string) (string, error) {
		cancel()
		return "function main(c) { return c; }", nil
	})
	if _, err := s.SaveLocalScript(input); !errors.Is(err, context.Canceled) {
		t.Fatal("canceled download was accepted", err)
	}
	after, err = s.LocalScriptState()
	if err != nil || !reflect.DeepEqual(after, before) {
		t.Fatal("canceled download persisted a script", err)
	}
}
