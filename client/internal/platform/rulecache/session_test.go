package rulecache

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

const testConfiguration = `external-controller: 127.0.0.1:9090
secret: private-session-secret-must-not-be-cached
rule-providers:
  Google:
    type: http
    url: https://example.test/google.yaml?token=private
    path: ./rules/google.yaml
    behavior: domain
    format: yaml
    interval: 600
    proxy: DIRECT
    header:
      Authorization: [Bearer-private-test-value]
  Local:
    type: file
    behavior: domain
    path: ./rules/local.yaml
  Inline:
    type: inline
    behavior: domain
    payload: [example.test]
rules: ["RULE-SET,Google,DIRECT", "MATCH,DIRECT"]
`

const configurationName = "generations/one/config.yaml"

func sessionFixture(t *testing.T, store Store, input string) (*Session, string) {
	t.Helper()
	workspace := t.TempDir()
	configuration := filepath.Join(workspace, filepath.FromSlash(configurationName))
	if err := os.MkdirAll(filepath.Dir(configuration), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configuration, []byte(input), 0600); err != nil {
		t.Fatal(err)
	}
	session := NewSession(workspace, store)
	if err := session.Prepare(configurationName); err != nil {
		t.Fatal(err)
	}
	return session, configuration
}

func providerPath(t *testing.T, configuration string) string {
	t.Helper()
	contents, err := os.ReadFile(configuration)
	if err != nil {
		t.Fatal(err)
	}
	var value struct {
		Providers map[string]struct {
			Path string `yaml:"path"`
		} `yaml:"rule-providers"`
	}
	if err := yaml.Unmarshal(contents, &value); err != nil {
		t.Fatal(err)
	}
	return value.Providers["Google"].Path
}

func writeProvider(t *testing.T, workspace, relative, content string, modified time.Time) {
	t.Helper()
	name := filepath.Join(workspace, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(name, modified, modified); err != nil {
		t.Fatal(err)
	}
}

func TestCachedProvidersSurviveNewSessionAndKeepExpiry(t *testing.T) {
	store := Store{Root: t.TempDir(), Path: "cache/mihomo/rule-providers"}
	first, configuration := sessionFixture(t, store, testConfiguration)
	location := providerPath(t, configuration)
	if !strings.HasPrefix(location, sessionDirectory+"/") {
		t.Fatal("HTTP provider does not use an isolated cache file")
	}
	modified := time.Now().Add(-48 * time.Hour).Truncate(time.Second)
	writeProvider(t, first.workspace, location, "payload: [cached.example]\n", modified)
	if err := first.Save(context.Background()); err != nil {
		t.Fatal(err)
	}
	second, secondConfiguration := sessionFixture(t, store, testConfiguration)
	if providerPath(t, secondConfiguration) != location {
		t.Fatal("new session changed the resource identity")
	}
	cache := filepath.Join(second.workspace, filepath.FromSlash(location))
	content, err := os.ReadFile(cache)
	if err != nil || string(content) != "payload: [cached.example]\n" {
		t.Fatalf("cache not restored: %q, %v", content, err)
	}
	info, _ := os.Stat(cache)
	if !info.ModTime().Equal(modified) {
		t.Fatalf("expiry was reset: got %v, want %v", info.ModTime(), modified)
	}
	contents, _ := os.ReadFile(secondConfiguration)
	if !bytes.Contains(contents, []byte("interval: 600")) || !bytes.Contains(contents, []byte("proxy: DIRECT")) || !bytes.Contains(contents, []byte("Authorization:")) {
		t.Fatal("download or update preferences changed")
	}
	// A hot reload cannot replace the newer live cache with the stored copy.
	newTime := modified.Add(24 * time.Hour)
	writeProvider(t, second.workspace, location, "payload: [updated.example]\n", newTime)
	if err := os.WriteFile(secondConfiguration, []byte(strings.Replace(testConfiguration, "interval: 600", "interval: 3600", 1)), 0600); err != nil {
		t.Fatal(err)
	}
	if err := second.Prepare(configurationName); err != nil {
		t.Fatal(err)
	}
	if err := second.Save(context.Background()); err != nil {
		t.Fatal(err)
	}
	third, _ := sessionFixture(t, store, testConfiguration)
	content, err = os.ReadFile(filepath.Join(third.workspace, filepath.FromSlash(location)))
	if err != nil || string(content) != "payload: [updated.example]\n" {
		t.Fatalf("latest core update lost: %q, %v", content, err)
	}
}

func TestSourceIdentityAndNonHTTPProviders(t *testing.T) {
	_, baseline, err := rewrite([]byte(testConfiguration))
	if err != nil || len(baseline) != 1 {
		t.Fatalf("unexpected HTTP entries: %v", err)
	}
	for _, change := range [][2]string{
		{"google.yaml?token=private", "other.yaml?token=private"},
		{"Bearer-private-test-value", "Bearer-another-value"},
		{"behavior: domain", "behavior: ipcidr"},
		{"format: yaml", "format: text"},
	} {
		_, entries, err := rewrite([]byte(strings.Replace(testConfiguration, change[0], change[1], 1)))
		if err != nil || entries[0].key == baseline[0].key {
			t.Fatalf("changed source reused a cache: %s, %v", change[0], err)
		}
	}
	for _, change := range [][2]string{{"interval: 600", "interval: 3600"}, {"./rules/google.yaml", "./another-path.yaml"}} {
		_, entries, err := rewrite([]byte(strings.Replace(testConfiguration, change[0], change[1], 1)))
		if err != nil || entries[0].key != baseline[0].key {
			t.Fatalf("non-content edit discarded cache: %s, %v", change[0], err)
		}
	}
	result, _, _ := rewrite([]byte(testConfiguration))
	if !bytes.Contains(result, []byte("path: ./rules/local.yaml")) || !bytes.Contains(result, []byte("type: inline")) {
		t.Fatal("non-HTTP providers changed")
	}
	_, repeat, err := rewrite(result)
	if err != nil || repeat[0].key != baseline[0].key {
		t.Fatal("rewriting a staged configuration is not idempotent")
	}
}

func TestSessionNeverPersistsConfigurationOrUnknownFiles(t *testing.T) {
	for _, input := range []string{configurationName, "Generations/one/config.yaml", "./GENERATIONS/one/config.yaml"} {
		t.Run(input, func(t *testing.T) {
			assertPrivateFilesNotCached(t, input)
		})
	}
}

func assertPrivateFilesNotCached(t *testing.T, input string) {
	t.Helper()
	store := Store{Root: t.TempDir(), Path: "cache"}
	session, configuration := sessionFixture(t, store, strings.Replace(testConfiguration, "./rules/google.yaml", input, 1))
	location := providerPath(t, configuration)
	if _, err := os.Stat(filepath.Join(session.workspace, filepath.FromSlash(location))); !os.IsNotExist(err) {
		t.Fatal("session config was imported as a rule cache")
	}
	writeProvider(t, session.workspace, sessionDirectory+"/unregistered-file", "secret must stay private", time.Now())
	if err := session.Save(context.Background()); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(store.Root)
	if len(entries) != 0 {
		t.Fatal("unregistered session data was persisted")
	}
}

func TestMissingOrUnusableCacheLeavesColdDownloadAvailable(t *testing.T) {
	store := Store{Root: t.TempDir(), Path: "cache"}
	first, configuration := sessionFixture(t, store, testConfiguration)
	location := providerPath(t, configuration)
	key := filepath.Base(location)
	if err := os.Mkdir(filepath.Join(store.Root, "cache"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(store.Root, "cache", key), nil, 0600); err != nil {
		t.Fatal(err)
	}
	second, _ := sessionFixture(t, store, testConfiguration)
	for _, session := range []*Session{first, second} {
		if _, err := os.Stat(filepath.Join(session.workspace, filepath.FromSlash(location))); !os.IsNotExist(err) {
			t.Fatal("missing or empty cache must not produce a placeholder rule file")
		}
	}
}

func TestAtomicSaveFailureRetainsPreviousCache(t *testing.T) {
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := atomicCopy(root, "entry", strings.NewReader("valid"), 5, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := atomicCopy(root, "entry", strings.NewReader("short"), 20, time.Now()); err == nil {
		t.Fatal("truncated cache was accepted")
	}
	file, _ := root.Open("entry")
	defer file.Close()
	var content [5]byte
	if _, err := file.Read(content[:]); err != nil || string(content[:]) != "valid" {
		t.Fatal("failed write replaced the good cache")
	}
}

func TestUnsafeConfigurationAndCancelledSave(t *testing.T) {
	for _, input := range []string{
		"rule-providers: {one: &a {type: http, url: x}, two: *a}",
		"rule-providers: {one: {type: http, type: file}}",
		"rule-providers: []",
		strings.Repeat("x", maxConfigurationBytes+1),
	} {
		if _, _, err := rewrite([]byte(input)); err == nil {
			t.Fatal("unsafe configuration accepted")
		}
	}
	session, _ := sessionFixture(t, Store{Root: t.TempDir(), Path: "cache"}, testConfiguration)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := session.Save(ctx); err != context.Canceled {
		t.Fatalf("cancellation lost: %v", err)
	}
}

func TestCacheIdentityAccessAndExistingProviderTimestamp(t *testing.T) {
	accesses := 0
	store := Store{Root: t.TempDir(), Path: "cache", Access: func(action func() error) error { accesses++; return action() }}
	session, configuration := sessionFixture(t, store, testConfiguration)
	modified := time.Unix(1600000000, 0)
	location := providerPath(t, configuration)
	writeProvider(t, session.workspace, location, "payload: [test.example]\n", modified)
	if err := session.Save(context.Background()); err != nil {
		t.Fatal(err)
	}
	if accesses < 2 {
		t.Fatal("persistent reads and writes did not use the owner identity")
	}
	_, secondConfiguration := sessionFixture(t, store, testConfiguration)
	if strings.Contains(providerPath(t, secondConfiguration), "private") {
		t.Fatal("credentials leaked into cache names")
	}
	// Explicit existing provider input is also usable, without resetting its age.
	root := t.TempDir()
	config := filepath.Join(root, filepath.FromSlash(configurationName))
	if err := os.MkdirAll(filepath.Dir(config), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, []byte(testConfiguration), 0600); err != nil {
		t.Fatal(err)
	}
	writeProvider(t, root, "rules/google.yaml", "payload: [input.example]\n", modified)
	emptyStore := Store{Root: t.TempDir(), Path: "cache"}
	if err := NewSession(root, emptyStore).Prepare(configurationName); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(root, filepath.FromSlash(providerPath(t, config))))
	if err != nil || !info.ModTime().Equal(modified) {
		t.Fatalf("existing rule timestamp lost: %v", err)
	}
}

func TestCorrectedCacheCanReplaceFutureTimestamp(t *testing.T) {
	store := Store{Root: t.TempDir(), Path: "cache"}
	session, configuration := sessionFixture(t, store, testConfiguration)
	location := providerPath(t, configuration)
	writeProvider(t, session.workspace, location, "outdated", time.Now().Add(24*time.Hour))
	if err := session.Save(context.Background()); err != nil {
		t.Fatal(err)
	}
	writeProvider(t, session.workspace, location, "repaired", time.Now())
	if err := session.Save(context.Background()); err != nil {
		t.Fatal(err)
	}
	restarted, _ := sessionFixture(t, store, testConfiguration)
	content, err := os.ReadFile(filepath.Join(restarted.workspace, filepath.FromSlash(location)))
	if err != nil || string(content) != "repaired" {
		t.Fatalf("old future timestamp blocked the active core's update: %q, %v", content, err)
	}
}

func TestCacheRejectsLinksAndEscapingPaths(t *testing.T) {
	directory := t.TempDir()
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	for _, name := range []string{"../outside", "nested/../../outside", "/absolute", "C:/outside", "file:stream", "nested\\outside"} {
		if file, err := openRegular(root, name); err == nil {
			file.Close()
			t.Fatalf("invalid cache path accepted: %q", name)
		}
	}
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "private"), []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(directory, "cache")); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	if file, err := openRegular(root, "cache/private"); err == nil {
		file.Close()
		t.Fatal("linked cache directory accepted")
	}
	if err := makeDirectories(root, "cache/nested"); err == nil {
		t.Fatal("persistent cache followed a linked directory")
	}
}
