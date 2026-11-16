package coresnapshot

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// Synthetic, offline outbounds: no subscription credentials or live servers.
const testTransportProxies = `proxies:
  - name: XHTTP
    type: vless
    server: 192.0.2.1
    port: 443
    uuid: 00000000-0000-4000-8000-000000000001
    udp: true
    packet-encoding: ""
    encryption: ""
    tls: true
    servername: example.test
    client-fingerprint: edge
    network: xhttp
    alpn: [h2]
    fingerprint: abababababababababababababababababababababababababababababababab
    xhttp-opts:
      path: /standard-http-path
      host: example.test
      mode: stream-up
      no-grpc-header: false
      download-settings:
        path: /user/runtime/download
  - name: WS
    type: vmess
    server: 192.0.2.1
    port: 443
    uuid: 00000000-0000-4000-8000-000000000002
    alterId: 0
    cipher: auto
    network: ws
    ws-opts:
      path: /user/runtime/websocket?ed=2048
  - name: H2
    type: vless
    server: 192.0.2.1
    port: 443
    uuid: 00000000-0000-4000-8000-000000000003
    network: h2
    h2-opts:
      path: /http2
  - name: HTTP
    type: vmess
    server: 192.0.2.1
    port: 443
    uuid: 00000000-0000-4000-8000-000000000004
    alterId: 0
    cipher: auto
    network: http
    http-opts:
      path: [/user/runtime/http, /video]
  - name: Trojan-WS
    type: trojan
    server: 192.0.2.1
    port: 443
    password: fixture-password
    network: ws
    ws-opts:
      path: /trojan-websocket
rules: ['MATCH,DIRECT']
`

func TestSnapshotPreservesTransportRequestPaths(t *testing.T) {
	for _, inline := range []bool{false, true} {
		name := "proxies"
		var input map[string]any
		if err := yaml.Unmarshal([]byte(testController+testTransportProxies), &input); err != nil {
			t.Fatal(err)
		}
		if inline {
			name = "inline-provider"
			input["proxy-providers"] = map[string]any{"test": map[string]any{"type": "inline", "payload": input["proxies"]}}
			delete(input, "proxies")
		}
		t.Run(name, func(t *testing.T) {
			data, err := yaml.Marshal(input)
			if err != nil {
				t.Fatal(err)
			}
			result, err := rewriteRuntimePaths(data, "/user/runtime", "/protected/session")
			if err != nil {
				t.Fatal(err)
			}
			var got map[string]any
			if err := yaml.Unmarshal(result, &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(input, got) {
				t.Fatal("snapshot changed HTTP request paths or other outbound fields")
			}
		})
	}
}

func TestTransportPathsDoNotBypassFileOrStructureChecks(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows protected snapshot file checks")
	}
	for name, fields := range map[string]string{
		"rule-provider":        "rule-providers: {test: {type: file, path: /outside/rules.yaml}}",
		"proxy-provider":       "proxy-providers: {test: {type: file, path: /outside/proxies.yaml}}",
		"certificate":          "proxies: [{type: vless, xhttp-opts: {path: /stream}, certificate: /outside/cert.pem}]",
		"private-key":          "proxies: [{type: vless, xhttp-opts: {path: /stream}, private-key: /outside/key.pem}]",
		"download-certificate": "proxies: [{type: vless, xhttp-opts: {path: /stream, download-settings: {path: /down, certificate: /outside/cert.pem}}}]",
		"download-key":         "proxies: [{type: vless, xhttp-opts: {path: /stream, download-settings: {path: /down, private-key: ../key.pem}}}]",
		"unknown-option":       "proxies: [{type: vless, xhttp-opts: {path: /stream, unknown-file: /outside/file}}]",
		"unknown-path":         "proxies: [{type: vless, unknown-opts: {path: /outside/file}}]",
		"unsupported-protocol": "proxies: [{type: ss, xhttp-opts: {path: /outside/file}}]",
		"wrong-context":        "rule-providers: {test: {xhttp-opts: {path: /outside/file}}}",
		"mapping-path":         "proxies: [{type: vless, xhttp-opts: {path: {certificate: /outside/cert.pem}}}]",
		"mapping-in-http-list": "proxies: [{type: vmess, http-opts: {path: [{certificate: /outside/cert.pem}]}}]",
		"duplicate-path":       "proxies: [{type: vless, xhttp-opts: {path: /stream, path: /another}}]",
		"alias":                "proxies: [{type: vless, xhttp-opts: {path: &p /stream, host: *p}}]",
		"nul":                  `proxies: [{type: vless, xhttp-opts: {path: "/stream\0"}}]`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := rewriteRuntimePaths([]byte(testController+fields+"\n"), "/user/runtime", "/protected/session"); err == nil {
				t.Fatal("unsafe file or malformed YAML bypassed snapshot checks")
			}
		})
	}
}

func TestTransportPathsKeepManagedFileRewrites(t *testing.T) {
	input := testController + `proxies:
  - type: vless
    certificate: /user/runtime/cert.pem
    xhttp-opts:
      path: /user/runtime/upload
      download-settings:
        path: /user/runtime/download
        private-key: /user/runtime/key.pem
rule-providers:
  test: {type: file, path: /user/runtime/rules.yaml}
proxy-providers:
  test: {type: file, path: /user/runtime/proxies.yaml}
`
	result, err := rewriteRuntimePaths([]byte(input), "/user/runtime", "/protected/session")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"/user/runtime/upload", "/user/runtime/download", "/protected/session/cert.pem", "/protected/session/key.pem", "/protected/session/rules.yaml", "/protected/session/proxies.yaml"} {
		if !strings.Contains(string(result), want) {
			t.Errorf("snapshot did not preserve the meaning of %s", want)
		}
	}
}

func TestTransportPathsSurviveInitialAndReloadSnapshots(t *testing.T) {
	source, target := t.TempDir(), t.TempDir()
	digests := map[string]string{}
	for _, stage := range []struct {
		configuration string
		initial       bool
	}{
		{"generations/initial/bootstrap.yaml", true},
		{"generations/reload/config.yaml", false},
	} {
		data := []byte(testController + testTransportProxies)
		file := filepath.Join(source, filepath.FromSlash(stage.configuration))
		if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, data, 0600); err != nil {
			t.Fatal(err)
		}
		var archive bytes.Buffer
		if err := WriteSnapshot(source, stage.configuration, stage.initial, &archive); err != nil {
			t.Fatal(err)
		}
		if err := Receive(target, source, &archive, digests); err != nil {
			t.Fatalf("%s: %v", stage.configuration, err)
		}
		result, err := os.ReadFile(filepath.Join(target, filepath.FromSlash(stage.configuration)))
		if err != nil {
			t.Fatal(err)
		}
		var want, got any
		if err := yaml.Unmarshal(data, &want); err != nil {
			t.Fatal(err)
		}
		if err := yaml.Unmarshal(result, &got); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(want, got) {
			t.Fatal("snapshot changed transport configuration")
		}
	}
}

// Opt-in core validation only; never starts proxy listeners or connects to nodes.
func TestMihomoAcceptsSnapshotTransportPaths(t *testing.T) {
	core := os.Getenv("JEEMI_TEST_MIHOMO")
	if core == "" {
		t.Skip("set JEEMI_TEST_MIHOMO to an installed official mihomo executable")
	}
	directory := t.TempDir()
	data, err := rewriteRuntimePaths([]byte(testController+testTransportProxies), "/user/runtime", directory)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(directory, "config.yaml")
	if err := os.WriteFile(file, data, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if output, err := exec.CommandContext(ctx, core, "-t", "-d", directory, "-f", file).CombinedOutput(); err != nil {
		t.Fatalf("official core rejected transport fixture: %v\n%s", err, output)
	}
}
