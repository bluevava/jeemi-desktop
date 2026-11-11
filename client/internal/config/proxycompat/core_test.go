package proxycompat_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"jeemi/internal/config/proxycompat"
)

// Optional contract check against an existing official core, without starting
// a proxy, downloading a core/provider, or touching the user's runtime files.
func TestMihomoAcceptsFilteredRuntime(t *testing.T) {
	core := os.Getenv("JEEMI_TEST_MIHOMO")
	if core == "" {
		t.Skip("set JEEMI_TEST_MIHOMO to an installed official mihomo executable")
	}
	fixtures := map[string]string{
		"mixed": `proxies:
  - {name: Snell6, type: snell, server: 192.0.2.1, port: 8390, version: 6, psk: fixture-psk, userkey: fixture-userkey, mode: unsafe-raw}
  - {name: Snell5, type: snell, server: 192.0.2.1, port: 8390, version: 5, psk: fixture-psk}
  - {name: AnyTLS, type: anytls, server: 192.0.2.2, port: 443, password: fixture-password}
proxy-groups:
  - {name: Proxy, type: select, proxies: [Snell6, Snell5, AnyTLS]}
  - {name: Auto, type: url-test, proxies: [Snell6], url: 'https://example.invalid/test'}
  - {name: Dynamic, type: select, include-all: true, filter: '^Snell6$'}
rules: ['DOMAIN,example.invalid,Snell6', 'MATCH,Proxy']
`,
		"inline-providers": `proxy-providers:
  mixed:
    type: inline
    payload:
      - {name: Unsupported, type: unsupported-fixture}
      - {name: AnyTLS, type: anytls, server: 192.0.2.2, port: 443, password: fixture-password}
  empty:
    type: inline
    payload: [{name: Snell6, type: snell, version: 6}]
proxy-groups:
  - {name: Proxy, type: select, use: [mixed, empty]}
  - {name: Empty, type: select, use: [empty]}
  - {name: Filtered, type: select, use: [mixed], filter: '^Unsupported$'}
rules: ['MATCH,Proxy']
`,
		"all-unsupported": "proxies: [{name: Snell6, type: snell, version: 6}]\n",
		"all-providers-unsupported": `proxy-providers:
  empty: {type: inline, payload: [{name: Bad, type: snell, version: 6}]}
proxy-groups:
  - {name: Proxy, type: select, include-all-providers: true}
rules: ['MATCH,Proxy']
`,
	}
	for name, source := range fixtures {
		t.Run(name, func(t *testing.T) {
			filtered, err := proxycompat.Apply([]byte(source))
			if err != nil {
				t.Fatal(err)
			}
			directory := t.TempDir()
			path := filepath.Join(directory, "config.yaml")
			if err := os.WriteFile(path, filtered, 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			if output, err := exec.CommandContext(ctx, core, "-t", "-d", directory, "-f", path).CombinedOutput(); err != nil {
				t.Fatalf("official core rejected filtered fixture: %v\n%s", err, output)
			}
		})
	}
}
