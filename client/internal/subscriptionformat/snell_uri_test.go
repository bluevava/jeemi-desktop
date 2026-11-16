package subscriptionformat

import (
	"encoding/base64"
	"net/url"
	"strings"
	"testing"
)

func TestSnell6URIKeepsVersionModeAndCredentials(t *testing.T) {
	for _, mode := range []string{"", "default", "unshaped", "unsafe-raw"} {
		uri := "snell://fixture-long-psk@node.example.invalid:443?version=6&udp=true&reuse=false"
		if mode != "" {
			uri += "&mode=" + mode
		}
		for _, source := range []string{uri, base64.StdEncoding.EncodeToString([]byte(uri))} {
			result := normalize(t, source)
			got := proxy(t, result, 0)
			if result.Report.SkippedNodes != 0 || got["version"] != 6 || got["psk"] != "fixture-long-psk" || got["udp"] != true || got["reuse"] != false {
				t.Fatal("Snell v6 fields were lost or downgraded")
			}
			if mode != "" && got["mode"] != mode {
				t.Fatal("Snell v6 mode was lost")
			}
		}
	}
	// The simpler URI without native-only options must also reach v6 support.
	result := normalize(t, "snell://fixture-long-psk@node.example.invalid:443?version=6")
	if proxy(t, result, 0)["version"] != 6 {
		t.Fatal("simple Snell URI was downgraded")
	}
}

func TestSnellURIStillRejectsInvalidV6Options(t *testing.T) {
	for _, query := range []string{
		"version=7", "version=5&mode=unshaped", "version=6&mode=unknown",
		"version=6&obfs=http", "version=6&obfs=tls",
		"version=6&obfs-opts=" + url.QueryEscape(`{"mode":"http"}`),
		"version=6&userkey=fixture-userkey",
	} {
		result := normalize(t, "snell://fixture-long-psk@node.example.invalid:443?"+query+"\n"+simpleURI)
		if result.Report.ProxyCount != 1 || result.Report.SkippedNodes != 1 {
			t.Fatal("invalid or unsupported URI fields were accepted", query)
		}
	}
	for _, password := range []string{"short", strings.Repeat("a", 256)} {
		result := normalize(t, "snell://"+password+"@node.example.invalid:443?version=6\n"+simpleURI)
		if result.Report.SkippedNodes != 1 {
			t.Fatal("invalid v6 PSK length was accepted")
		}
	}
}
