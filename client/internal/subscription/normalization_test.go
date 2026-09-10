package subscription

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"jeemi/internal/subscriptionformat"
)

func TestFetchDetectsConvertedBodiesDespiteMIMEAndExtension(t *testing.T) {
	for _, body := range []string{
		"[Proxy]\nNode = socks5, 192.0.2.1, 1080, user, pass\n",
		base64.StdEncoding.EncodeToString([]byte("anytls://pass@node.example.test:443#Node")),
		"broken first node\nunknown://fixture\ntrojan://fixture@node.example.invalid:443?tfo=0#Good",
	} {
		t.Run(strings.Split(body, "\n")[0][:6], func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Subscription-Userinfo", "download=2; total=10")
				_, _ = w.Write([]byte(body))
			}))
			defer server.Close()
			fetched, err := NewHTTPFetcher().Fetch(context.Background(), server.URL+"/config.json")
			if err != nil || string(fetched.Contents) != body || fetched.Format != FormatText || fetched.RemoteProfile.DownloadBytes != 2 {
				t.Fatal("body detection or raw preservation failed", err)
			}
		})
	}
}

func TestURLRefreshKeepsValidURINodesAndRejectsZeroNodes(t *testing.T) {
	store, _ := newTestStore(t)
	mixed := []byte("broken first node\nunknown://fixture\ntrojan://fixture@node.example.invalid:443?tfo=0#Good")
	f := &queuedFetcher{results: []FetchedContent{
		{Format: FormatText, Contents: []byte("anytls://pass@node.example.test:443#Old")},
		{Format: FormatText, Contents: mixed, RemoteProfile: RemoteProfile{HasTraffic: true, TotalBytes: 20}},
		{Format: FormatText, Contents: []byte("unknown://fixture\ntrojan://fixture@node.example.invalid:invalid")},
	}}
	m, err := NewManager(ManagerOptions{Store: store, Fetcher: f})
	if err != nil {
		t.Fatal(err)
	}
	initial, err := m.ImportURL(context.Background(), ImportURLInput{Name: "Fixture", SourceURL: "https://example.test/sub"})
	if err != nil {
		t.Fatal(err)
	}
	id := initial.Subscriptions[0].ID
	if _, err := m.Refresh(context.Background(), id); err != nil {
		t.Fatal("mixed-node refresh rejected valid nodes", err)
	}
	before, _ := m.State()
	text, err := m.Text(id)
	digest := sha256.Sum256(mixed)
	if err != nil || text.Contents != string(mixed) || before.Subscriptions[0].CurrentRevisionID != hex.EncodeToString(digest[:]) {
		t.Fatal("refresh did not preserve the full original bytes and digest", err)
	}
	parsed, err := subscriptionformat.Normalize([]byte(text.Contents))
	if err != nil || parsed.Report.ProxyCount != 1 || parsed.Report.SkippedNodes != 2 {
		t.Fatal("stored mixed revision cannot be projected", err)
	}
	if _, err := m.Refresh(context.Background(), id); err == nil {
		t.Fatal("refresh accepted zero usable nodes")
	}
	after, err := m.State()
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("zero-node refresh replaced the valid revision or metadata", err)
	}
}

func TestSurgeFilePreservesBytesAndDigestAndCandidateFailure(t *testing.T) {
	store, _ := newTestStore(t)
	raw := []byte("\ufeff[Proxy]\r\nNode = socks5, 192.0.2.1, 1080, user, pass\r\n")
	path := filepath.Join(t.TempDir(), "profile.conf")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	validated := 0
	m, err := NewManager(ManagerOptions{Store: store, Fetcher: &queuedFetcher{}, ValidateImport: func(_ context.Context, b []byte) error {
		validated++
		if string(b) != string(raw) {
			t.Fatal("validator did not receive raw bytes")
		}
		return errors.New("candidate rejected")
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.ImportFile(path); err == nil {
		t.Fatal("candidate rejection ignored")
	}
	state, _ := m.State()
	if validated != 1 || len(state.Subscriptions) != 0 {
		t.Fatal("failed import persisted source")
	}
	m.validateImport = nil
	state, err = m.ImportFile(path)
	if err != nil {
		t.Fatal(err)
	}
	summary := state.Subscriptions[0]
	text, err := m.Text(summary.ID)
	digest := sha256.Sum256(raw)
	if err != nil || text.Contents != string(raw) || summary.Format != FormatText || summary.CurrentRevisionID != hex.EncodeToString(digest[:]) {
		t.Fatal("raw Surge revision is not byte-identical", err)
	}
}

func TestMalformedMetadataCannotReplaceSuccessfulURLRevision(t *testing.T) {
	store, _ := newTestStore(t)
	f := &queuedFetcher{results: []FetchedContent{
		{Format: FormatText, Contents: []byte("anytls://pass@node.example.test:443#Node"), RemoteProfile: RemoteProfile{HasTraffic: true, TotalBytes: 10}},
		{Format: FormatText, Contents: []byte("host://*.entry.test?server=invalid\nanytls://pass@node.example.test:443#Node"), RemoteProfile: RemoteProfile{HasTraffic: true, TotalBytes: 20}},
	}}
	m, err := NewManager(ManagerOptions{Store: store, Fetcher: f})
	if err != nil {
		t.Fatal(err)
	}
	before, err := m.ImportURL(context.Background(), ImportURLInput{Name: "Fixture", SourceURL: "https://example.test/sub"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Refresh(context.Background(), before.Subscriptions[0].ID); err == nil {
		t.Fatal("invalid DNS metadata accepted")
	}
	after, err := m.State()
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("failed refresh changed success state", err)
	}
}

func TestNativeJSONContainerRemainsStrictButURIFormatHintsDoNotBlockConversion(t *testing.T) {
	if err := validateContents(FormatJSON, []byte("{mode: rule}")); err == nil {
		t.Fatal("JSON file accepted YAML-only syntax")
	}
	if err := validateContents(FormatYAML, []byte("{mode: rule}")); err != nil {
		t.Fatal("flow YAML rejected", err)
	}
	if err := validateContents(FormatJSON, []byte("anytls://pass@node.example.test:443#Node")); err != nil {
		t.Fatal("stale format hint blocked converted source", err)
	}
}
