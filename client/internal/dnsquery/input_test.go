package dnsquery

import (
	"strings"
	"testing"
)

func TestDomainExtractsOnlyTheHostname(t *testing.T) {
	for _, test := range []struct{ input, want string }{
		{" Example.COM. ", "example.com"},
		{"http://Example.COM/", "example.com"},
		{"https://Example.COM/path/to/page/?x=1#section", "example.com"},
		{" HTTPS://Example.COM:8443/path/ ", "example.com"},
		{"https://user:password@Example.COM/private", "example.com"},
		{"https://例子.测试/路径?参数=值", "xn--fsqu00a.xn--0zwm56d"},
		{"example.com/", "example.com"},
		{"example.com/path?next=https://other.example/path", "example.com"},
		{"example.com?x=1#section", "example.com"},
		{"example.com#section", "example.com"},
		{"https://example.com/" + strings.Repeat("x", 1024), "example.com"},
	} {
		t.Run(test.input[:min(len(test.input), 80)], func(t *testing.T) {
			got, err := Domain(test.input)
			if err != nil || got != test.want {
				t.Fatalf("Domain() = %q, %v; want %q", got, err, test.want)
			}
		})
	}
}

func TestDomainRejectsMissingHostsIPsAndUnsupportedSchemes(t *testing.T) {
	for _, input := range []string{
		"", " ", "/", "https://", "https:///path", "http:/example.com",
		"ftp://example.com/path", "file:///example.com", "mailto:user@example.com",
		"https://1.2.3.4/path", "https://[2001:db8::1]/", "１２７.０.０.１",
		"https://*.example.com/", "https://a..com/", "https://a b.com/",
		"https://example.com:notaport/", "https://example.com/\nprivate",
		strings.Repeat("a", 64) + ".com", strings.Repeat("a", 8193),
	} {
		if _, err := Domain(input); err == nil || err.Error() != "invalid_domain" {
			t.Errorf("accepted invalid input or exposed details (length %d): %v", len(input), err)
		}
	}
}

func TestDoHHostValidationStillRejectsURLSyntax(t *testing.T) {
	for _, input := range []string{
		"https://dns.example%2Fother/dns-query", "https://dns.example%3Fkey/dns-query",
		"https://user:pass@dns.example/dns-query", "https://dns.example/#fragment",
	} {
		if _, _, err := Server(input); err == nil {
			t.Fatal("accepted invalid DoH server")
		}
	}
}
