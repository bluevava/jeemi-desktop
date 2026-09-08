package proxyoverride

import (
	"strings"
	"testing"
)

func TestApplyUpdatesOnlyCompatibleProxyTypes(t *testing.T) {
	input := []byte(`proxies:
  - name: vless-node
    type: vless
    server: vless.example
    port: 443
    uuid: test-uuid
  - name: ss-node
    type: ss
    server: ss.example
    port: 443
    cipher: aes-128-gcm
    password: secret
`)
	result, err := Apply(input, []Override{{
		Path:            "/proxies/*/client-fingerprint",
		TargetPath:      "/client-fingerprint",
		ValueYAML:       "chrome",
		ApplicableTypes: []string{"vmess", "vless", "trojan", "anytls"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	output := string(result)
	if strings.Count(output, "client-fingerprint: chrome") != 1 {
		t.Fatalf("override count is wrong:\n%s", output)
	}
	if !strings.Contains(output, "name: vless-node") || !strings.Contains(output, "uuid: test-uuid") ||
		!strings.Contains(output, "name: ss-node") || !strings.Contains(output, "password: secret") {
		t.Fatalf("proxy identity or protocol fields changed:\n%s", output)
	}
}

func TestApplyUpdatesNestedFieldsWithoutSyntheticProxyNodes(t *testing.T) {
	input := []byte("proxies:\n  - name: node\n    type: vmess\n    server: example.test\n    port: 443\n    uuid: id\n")
	result, err := Apply(input, []Override{{
		Path:       "/proxies/*/smux/enabled",
		TargetPath: "/smux/enabled",
		ValueYAML:  "true",
	}})
	if err != nil {
		t.Fatal(err)
	}
	output := string(result)
	if !strings.Contains(output, "smux:\n      enabled: true") || strings.Contains(output, "__jeemi") {
		t.Fatalf("nested override was not materialized cleanly:\n%s", output)
	}
}
