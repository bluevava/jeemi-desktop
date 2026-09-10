package chainproxy

import (
	"encoding/base64"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
	"jeemi/internal/subscriptionformat"
)

func TestAnyTLSURIImportMatchesSubscriptionParser(t *testing.T) {
	const uri = "anytls://fixture-password@192.0.2.10:443?type=tcp&fp=firefox&security=tls&sni=tls.example.invalid&udp=1&tfo=0#HK Example 04"
	for name, raw := range map[string]string{
		"text":              uri,
		"subscription_body": base64.StdEncoding.EncodeToString([]byte(uri)),
	} {
		t.Run(name, func(t *testing.T) {
			imported, err := Parse([]byte(raw), "hk & !excluded")
			if err != nil {
				t.Fatal(err)
			}
			if len(imported.Nodes) != 1 || imported.Nodes[0].Name != "HK Example 04" || imported.Nodes[0].Type != "anytls" {
				t.Fatal("AnyTLS landing node was not imported")
			}
			var landing map[string]any
			if err := yaml.Unmarshal([]byte(imported.Nodes[0].YAML), &landing); err != nil {
				t.Fatal(err)
			}
			normalized, err := subscriptionformat.Normalize([]byte(raw))
			if err != nil {
				t.Fatal(err)
			}
			var subscription map[string]any
			if err := yaml.Unmarshal(normalized.Contents, &subscription); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(landing, subscription["proxies"].([]any)[0]) || landing["tfo"] != false || landing["client-fingerprint"] != "firefox" {
				t.Fatal("chain import differs from ordinary subscription or loses TLS/TFO fields")
			}
			edited, err := Parse([]byte(EditableNode(imported.Nodes[0])), "")
			if err != nil || len(edited.Nodes) != 1 || edited.Nodes[0].YAML != imported.Nodes[0].YAML {
				t.Fatal("editing the imported AnyTLS node loses options", err)
			}
		})
	}
}

func TestNativeURIChainImportRetainsMultiPeerDNS(t *testing.T) {
	peers := `[{"server":"one.example.invalid","port":51820,"public-key":"fixture","allowed-ips":["0.0.0.0/0"]},{"server":"two.example.invalid","port":51821,"public-key":"fixture","allowed-ips":["::/0"]}]`
	raw := "host://one.example.invalid?server=192.0.2.53\nhost://two.example.invalid?server=192.0.2.54\nunknown://fixture\ndirect://?name=Local\nwireguard://?name=WG&private-key=fixture&peers=" + url.QueryEscape(peers)
	imported, err := Parse([]byte(raw), "")
	if err != nil || len(imported.Nodes) != 1 || imported.Report.ProxyCount != 1 || imported.Report.SkippedNodes != 2 {
		t.Fatal("chain import diverged from the shared URI parser", err)
	}
	for _, domain := range []string{"one.example.invalid", "two.example.invalid"} {
		if !strings.Contains(imported.Nodes[0].DNSYAML, domain) {
			t.Fatal("peer custom DNS was lost")
		}
	}
	roundtrip, err := Parse([]byte(EditableNode(imported.Nodes[0])), "")
	if err != nil || !reflect.DeepEqual(roundtrip.Nodes, imported.Nodes) {
		t.Fatal("editing the native node lost peer fields or DNS", err)
	}
	for _, raw := range []string{
		"tailscale://?hostname=fixture&name=TS",
		"zerotier://?network=0123456789abcdef&name=ZT",
		"mieru://fixture:fixture@node.example.invalid?port-range=1000-2000&transport=TCP#Mieru",
	} {
		if r, err := Parse([]byte(raw), ""); err != nil || len(r.Nodes) != 1 {
			t.Fatal("protocol-specific endpoint requirements were ignored", err)
		}
	}
	if _, err := Parse([]byte("direct://?name=Local\ndns://?name=Resolver"), ""); err == nil {
		t.Fatal("local outbounds were accepted as remote landing proxies")
	}
}
