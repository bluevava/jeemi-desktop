package inspect

import (
	"strings"
	"testing"
)

func TestValidateReferencesKeepsLogicalRulesAndProviderMembership(t *testing.T) {
	configuration := []byte(`proxy-providers:
  remote: {type: http, url: https://example.test/proxies.yaml}
proxy-groups:
  - {name: Outbound, type: select, proxies: [DIRECT], use: [remote]}
rule-providers:
  local: {type: inline, behavior: domain, payload: [example.org]}
sub-rules:
  nested: ['RULE-SET,local,Outbound', 'IP-CIDR,10.0.0.0/8,DIRECT,no-resolve']
rules:
  - 'AND,((NETWORK,TCP),(DST-PORT,443)),Outbound'
  - 'SUB-RULE,(NETWORK,UDP),nested'
  - 'MATCH,Outbound'
`)
	if err := ValidateReferences(configuration); err != nil {
		t.Fatal("valid nested rules or remote provider reference rejected", err)
	}
	for _, entry := range []struct{ before, after, diagnostic string }{
		{"proxies: [DIRECT]", "proxies: [missing]", "proxy-groups[0].proxies"},
		{"use: [remote]", "use: [missing]", "proxy-groups[0].use"},
		{"RULE-SET,local,Outbound", "RULE-SET,missing,Outbound", "missing rule provider"},
		{"MATCH,Outbound", "MATCH,missing", "missing proxy or selector"},
		{"SUB-RULE,(NETWORK,UDP),nested", "SUB-RULE,(NETWORK,UDP),missing", "missing sub-rule"},
	} {
		t.Run(entry.diagnostic, func(t *testing.T) {
			candidate := strings.ReplaceAll(string(configuration), entry.before, entry.after)
			err := ValidateReferences([]byte(candidate))
			if err == nil || !strings.Contains(err.Error(), entry.diagnostic) {
				t.Fatalf("missing reference diagnostic: %v", err)
			}
		})
	}
}
