package ruleprovideroverride

import (
	"strings"
	"testing"

	"jeemi/internal/config/document"
)

func TestApplyRemovesProviderAndKnownReferences(t *testing.T) {
	source := []byte(`rule-providers:
  ads:
    type: http
  private:
    type: http
rules:
  - RULE-SET,ads,REJECT
  - AND,((RULE-SET,ads),(DOMAIN,example.com)),REJECT
  - RULE-SET,private,DIRECT
  - MATCH,Proxy
sub-rules:
  nested:
    - RULE-SET,ads,REJECT
    - DOMAIN,kept.example,DIRECT
dns:
  nameserver-policy:
    rule-set:ads,private: [1.1.1.1]
    rule-set:private: [8.8.8.8]
  proxy-server-nameserver-policy:
    rule-set:ads,private: [9.9.9.9]
  fake-ip-filter:
    - RULE-SET,ads,real-ip
    - rule-set:ads,private
    - DOMAIN,kept.example,real-ip
sniffer:
  force-domain: ["rule-set:ads,private", geosite:cn]
  skip-src-address: ["rule-set:ads"]
tun:
  route-address-set: [ads, private]
listeners:
  - name: secondary
    type: tun
    route-exclude-address-set: [ads, private]
future-option: kept
`)

	result, err := Apply(source, []string{"ads"})
	if err != nil {
		t.Fatal(err)
	}
	text := string(result)
	for _, forbidden := range []string{"RULE-SET,ads", "rule-set:ads", "route-address-set: [ads", "route-exclude-address-set: [ads"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("disabled reference %q remains:\n%s", forbidden, text)
		}
	}
	for _, expected := range []string{"private:", "RULE-SET,private,DIRECT", "DOMAIN,kept.example,DIRECT", "rule-set:private:", "future-option: kept"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected %q to remain:\n%s", expected, text)
		}
	}
	configuration, err := document.Parse(result)
	if err != nil {
		t.Fatalf("result is not valid YAML: %v", err)
	}
	providers, found, err := document.Find(document.Root(configuration), "/rule-providers/ads")
	if err != nil || found || providers != nil {
		t.Fatalf("disabled provider remains: found=%v value=%+v err=%v", found, providers, err)
	}
	for path, expected := range map[string][]string{
		"/dns/fake-ip-filter":                    {"rule-set:private", "DOMAIN,kept.example,real-ip"},
		"/sniffer/force-domain":                  {"rule-set:private", "geosite:cn"},
		"/tun/route-address-set":                 {"private"},
		"/listeners/0/route-exclude-address-set": {"private"},
	} {
		value, found, err := document.Find(document.Root(configuration), path)
		if err != nil || !found || len(value.Content) != len(expected) {
			t.Fatalf("unexpected filtered sequence %s: value=%+v found=%v err=%v", path, value, found, err)
		}
		for index, want := range expected {
			if value.Content[index].Value != want {
				t.Fatalf("%s[%d] = %q, want %q", path, index, value.Content[index].Value, want)
			}
		}
	}
	policy, found, err := document.Find(document.Root(configuration), "/dns/proxy-server-nameserver-policy/rule-set:private")
	if err != nil || !found || policy == nil {
		t.Fatalf("proxy server nameserver policy was not retained under its filtered key: found=%v value=%+v err=%v", found, policy, err)
	}
}

func TestApplyIsNoopWhenNothingIsDisabled(t *testing.T) {
	source := []byte("rule-providers:\n  kept: {type: inline}\n")
	result, err := Apply(source, nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(result) != string(source) {
		t.Fatalf("no-op changed source:\n%s", result)
	}
}

func TestApplyRejectsMalformedKnownReferenceCollections(t *testing.T) {
	if _, err := Apply([]byte("rules: not-a-list\nrule-providers: {}\n"), []string{"ads"}); err == nil {
		t.Fatal("Apply() accepted malformed rules")
	}
}
