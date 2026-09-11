//go:build windows

package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/windows"
	"gopkg.in/yaml.v3"
)

// The child is the package's test binary, not mihomo. Simulated downloads write
// only to this test's private temporary directory and never modify networking.
func TestServiceRuleCacheSurvivesStopAndUnexpectedExit(t *testing.T) {
	for _, unexpected := range []bool{false, true} {
		name := "stop"
		if unexpected {
			name = "unexpected-exit"
		}
		t.Run(name, func(t *testing.T) {
			target, _, cores, workspace, configuration := selectedCoreFixture(t, true)
			stop := func() {
				t.Helper()
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := cores.stop(ctx); err != nil {
					t.Fatal(err)
				}
			}
			t.Cleanup(stop)
			modified := time.Now().Add(-48 * time.Hour).Truncate(time.Second)
			for run := range 2 {
				writeTestWorkspace(t, workspace, configuration)
				for _, file := range []string{"bootstrap.yaml", "config.yaml"} {
					file = filepath.Join(workspace, "generations", "one", file)
					input, err := os.ReadFile(file)
					if err != nil {
						t.Fatal(err)
					}
					input = append(input, []byte("rule-providers:\n  Test:\n    type: http\n    url: https://example.test/rules\n    behavior: domain\n    interval: 600\n")...)
					if err := os.WriteFile(file, input, 0600); err != nil {
						t.Fatal(err)
					}
				}
				binary, err := openCallerCore(context.Background(), windows.CurrentProcess(), target)
				if err != nil {
					t.Fatal(err)
				}
				if err := cores.prepareRuleCache(windows.CurrentProcess(), target, workspace, configuration); err != nil {
					binary.Close()
					t.Fatal(err)
				}
				if err := cores.start(binary, workspace, configuration); err != nil {
					t.Fatal(err)
				}
				assertTestCoreImage(t, workspace, target.ExecutablePath)
				input, err := os.ReadFile(filepath.Join(workspace, filepath.FromSlash(configuration)))
				if err != nil {
					t.Fatal(err)
				}
				var config struct {
					Providers map[string]struct{ Path string } `yaml:"rule-providers"`
				}
				if err := yaml.Unmarshal(input, &config); err != nil {
					t.Fatal(err)
				}
				provider := filepath.Join(workspace, filepath.FromSlash(config.Providers["Test"].Path))
				if run == 0 {
					if _, err := os.Stat(provider); !os.IsNotExist(err) {
						t.Fatal("first run fabricated a cache")
					}
					if err := os.WriteFile(provider, []byte("payload: [cached.example]\n"), 0600); err != nil {
						t.Fatal(err)
					}
					if err := os.Chtimes(provider, modified, modified); err != nil {
						t.Fatal(err)
					}
				} else {
					data, err := os.ReadFile(provider)
					if err != nil || string(data) != "payload: [cached.example]\n" {
						t.Fatalf("next service session lost the downloaded rules: %q, %v", data, err)
					}
					info, err := os.Stat(provider)
					if err != nil || !info.ModTime().Equal(modified) {
						t.Fatal("next service session reset rule expiry")
					}
				}
				if unexpected {
					if err := os.WriteFile(filepath.Join(workspace, "test-exit"), nil, 0600); err != nil {
						t.Fatal(err)
					}
					select {
					case <-cores.done:
					case <-time.After(5 * time.Second):
						t.Fatal("test child did not exit")
					}
				}
				stop()
				if cores.cacheToken != 0 || cores.ruleCache != nil {
					t.Fatal("session cache retained the caller's token after cleanup")
				}
				if _, err := os.Stat(workspace); !os.IsNotExist(err) {
					t.Fatal("private session was not removed")
				}
			}
		})
	}
}
