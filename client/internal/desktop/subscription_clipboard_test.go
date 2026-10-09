package desktop

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jeemi/internal/application"
	"jeemi/internal/subscription"
)

func TestCopySubscriptionSourceKeepsAddressesInsideGo(t *testing.T) {
	service, err := application.NewService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	app := &App{service: service}
	source := filepath.Join(t.TempDir(), "import.yaml")
	if err := os.WriteFile(source, []byte("proxies: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	state, err := service.ImportSubscriptionFile(source)
	if err != nil {
		t.Fatal(err)
	}
	var copied string
	if err := app.copySubscriptionSource(state.Subscriptions[0].ID, func(value string) error { copied = value; return nil }); err != nil {
		t.Fatal(err)
	}
	if copied == source || !filepath.IsAbs(copied) {
		t.Fatal("copied the original import path instead of its managed copy")
	}
	if content, err := os.ReadFile(copied); err != nil || string(content) != "proxies: []\n" {
		t.Fatal("copied file path is not the stored source")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("proxies: []\n"))
	}))
	defer server.Close()
	address := server.URL + "/sub?token=fixture-secret"
	state, err = service.ImportSubscriptionURL(subscription.ImportURLInput{Name: "Remote", SourceURL: address, IconKind: subscription.IconEmoji, Icon: "🐱"})
	if err != nil {
		t.Fatal(err)
	}
	var id string
	for _, item := range state.Subscriptions {
		if item.SourceKind == subscription.SourceURL {
			id = item.ID
		}
	}
	if err := app.copySubscriptionSource(id, func(value string) error { copied = value; return nil }); err != nil || copied != address {
		t.Fatal("clipboard did not receive the full saved URL")
	}
	err = app.copySubscriptionSource(id, func(value string) error { return errors.New(value) })
	if err == nil || strings.Contains(err.Error(), "fixture-secret") || strings.Contains(err.Error(), server.URL) {
		t.Fatal("clipboard failure exposed the source address")
	}
	err = app.copySubscriptionSource("invalid", func(string) error { t.Error("invalid subscription changed clipboard"); return nil })
	if err == nil {
		t.Fatal("unknown subscription succeeded")
	}
}
