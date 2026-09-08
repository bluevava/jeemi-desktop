package subscriptionformat

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"math/big"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

const testUUID = "11111111-1111-4111-8111-111111111111"
const simpleURI = "anytls://example-password@edge.example.invalid:443?sni=tls.example.invalid#Example"

func configuration(t *testing.T, r Result) map[string]any {
	t.Helper()
	var root map[string]any
	if err := yaml.Unmarshal(r.Contents, &root); err != nil {
		t.Fatal(err)
	}
	return root
}
func proxy(t *testing.T, r Result, index int) map[string]any {
	t.Helper()
	return configuration(t, r)["proxies"].([]any)[index].(map[string]any)
}
func normalize(t *testing.T, raw string) Result {
	t.Helper()
	r, err := Normalize([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestURIEnvelopeAndImmutableInput(t *testing.T) {
	text := "host://*.entry.example.invalid?server=https%3A%2F%2Fdns.example.invalid%2Fdns-query\r\nanytls://pw@node.entry.example.invalid:443?sni=tls.example.invalid#%F0%9F%87%AD%F0%9F%87%B0%20HK\r\n"
	for _, encoding := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		raw := append([]byte{0xef, 0xbb, 0xbf}, []byte(encoding.EncodeToString([]byte(text)))...)
		before := bytes.Clone(raw)
		r, err := Normalize(raw)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(raw, before) || r.Report.Encoding != "base64" || r.Report.Format != "uri" {
			t.Fatal("raw source or detected envelope changed")
		}
		root := configuration(t, r)
		dns := root["dns"].(map[string]any)
		if _, found := dns["nameserver"]; found {
			t.Fatal("private resolver became general DNS")
		}
		if !reflect.DeepEqual(dns["proxy-server-nameserver"], []any{"system"}) {
			t.Fatal("node policy requires a nonempty default")
		}
		if !reflect.DeepEqual(dns["nameserver-policy"], dns["proxy-server-nameserver-policy"]) {
			t.Fatal("entry policies differ")
		}
		if proxy(t, r, 0)["name"] != "🇭🇰 HK" {
			t.Fatal("name decoding lost Unicode")
		}
		if !reflect.DeepEqual(root["rules"], []any{"MATCH,Proxy"}) || len(root["proxy-groups"].([]any)) != 1 {
			t.Fatal("unexpected base routing")
		}
	}
}

func TestTLSAliasesAreIndependentAndEquivalent(t *testing.T) {
	pin := strings.Repeat("ab", 32)
	r := normalize(t, simpleURI+"&unused") // Fragment is a name, not another query field.
	if proxy(t, r, 0)["name"] != "Example&unused" {
		t.Fatal("fragment was parsed as query")
	}
	u, _ := url.Parse(simpleURI)
	q := u.Query()
	q.Set("fingerprint", pin)
	q.Set("pcs", strings.ToUpper(pin))
	q.Set("server-cert-fingerprint-sha256", "sha256:"+pin)
	q.Set("fp", "chrome")
	q.Set("sni", "TLS.Example.Invalid.")
	q.Set("peer", "tls.example.invalid")
	q.Set("insecure", "0")
	q.Set("skip-cert-verify", "false")
	u.RawQuery = q.Encode()
	n := proxy(t, normalize(t, u.String()), 0)
	if n["fingerprint"] != pin || n["client-fingerprint"] != "chrome" || n["skip-cert-verify"] != false || n["sni"] != "tls.example.invalid" {
		t.Fatal("PIN, ClientHello, or explicit false lost")
	}
}

func TestProtocolFields(t *testing.T) {
	ss := url.URL{Scheme: "ss", User: url.UserPassword("2022-blake3-aes-256-gcm", "identity:user/+key"), Host: "[2001:db8::1]:443", Fragment: "SS"}
	for _, tc := range []struct {
		name, uri string
		want      map[string]any
	}{
		{"ss2022", ss.String(), map[string]any{"type": "ss", "password": "identity:user/+key", "server": "2001:db8::1"}},
		{"ss_user_base64", "ss://" + base64.RawURLEncoding.EncodeToString([]byte("aes-128-gcm:p:a+ss")) + "@ss.example.invalid:8443#SS", map[string]any{"cipher": "aes-128-gcm", "password": "p:a+ss"}},
		{"ss_legacy", "ss://" + base64.StdEncoding.EncodeToString([]byte("aes-128-gcm:pw@ss.example.invalid:8443")) + "#SS", map[string]any{"password": "pw", "port": 8443}},
		{"socks", "socks5://user:p%2Bass@localhost:1080#Socks", map[string]any{"username": "user", "password": "p+ass"}},
		{"anytls", strings.Split(simpleURI, "#")[0] + "&idle_session_check_interval=30s&idle_session_timeout=1m&min_idle_session=0", map[string]any{"idle-session-check-interval": 30, "idle-session-timeout": 60, "min-idle-session": 0}},
		{"tuic", "tuic://" + testUUID + ":pw@tuic.example.invalid:443?sni=tls.example.invalid&congestion_control=bbr&heartbeat=10s&zero_rtt_handshake=1&udp_relay_mode=native", map[string]any{"type": "tuic", "heartbeat-interval": 10000, "reduce-rtt": true, "congestion-controller": "bbr"}},
		{"vless_ws", "vless://" + testUUID + "@ws.example.invalid:443?security=tls&type=ws&path=%2Fws&host=cdn.example.invalid&sni=tls.example.invalid&fp=chrome&encryption=none&packetEncoding=xudp", map[string]any{"type": "vless", "network": "ws", "servername": "tls.example.invalid", "packet-encoding": "xudp"}},
		{"vless_grpc", "vless://" + testUUID + "@grpc.example.invalid:443?security=tls&type=grpc&serviceName=example", map[string]any{"network": "grpc"}},
		{"snell", "snell://pw@snell.example.invalid:443?version=4&obfs=http&obfs_host=example.invalid&udp=1", map[string]any{"psk": "pw", "version": 4, "udp": true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n := proxy(t, normalize(t, tc.uri), 0)
			for key, want := range tc.want {
				if !reflect.DeepEqual(n[key], want) {
					t.Fatalf("wrong field %s", key)
				}
			}
		})
	}
	r := normalize(t, "vless://"+testUUID+"@ws.example.invalid:443?security=tls&type=ws&path=%2Fa%3Fb%3D1&host=cdn.example.invalid")
	opts := proxy(t, r, 0)["ws-opts"].(map[string]any)
	if opts["path"] != "/a?b=1" || opts["headers"].(map[string]any)["Host"] != "cdn.example.invalid" {
		t.Fatal("nested transport fields lost")
	}
}

func TestUnsupportedNodesAreReportedNotDowngraded(t *testing.T) {
	for _, extra := range []string{"snell://pw@node.example.invalid:443?version=5&userkey=independent-secret", "snell://pw@node.example.invalid:443?version=6", "anytls://pw@node.example.invalid:443?pinSHA256=unknown-public-key", "vmess://unsupported"} {
		r := normalize(t, simpleURI+"\n"+extra)
		if r.Report.ProxyCount != 1 || r.Report.SkippedNodes != 1 || len(r.Report.Diagnostics) != 1 {
			t.Fatal("unsupported node was not reported")
		}
		if _, err := Normalize([]byte(extra)); err == nil {
			t.Fatal("unsupported-only subscription accepted")
		}
	}
}

func TestInvalidSourcesDoNotLeakSecrets(t *testing.T) {
	pin := strings.Repeat("ab", 32)
	for _, raw := range []string{
		"anytls://secret@node.example.invalid:443?fingerprint=" + pin + "&pcs=" + strings.Repeat("cd", 32),
		"anytls://secret@node.example.invalid:443?allowInsecure=1&insecure=false",
		"anytls://secret@node.example.invalid:443?fingerprint=secret",
		"anytls://secret@node.example.invalid:443?idle_session_timeout=1.5s",
		"anytls://secret@node.example.invalid:443?min_idle_session=-1",
		"anytls://secret@node.example.invalid:0", "anytls://secret@node.example.invalid:443?alpn=%0Asecret",
		"host://*.entry.example.invalid?server=secret\n" + simpleURI,
		"host://*.entry.example.invalid?server=https%3A%2F%2Fdns.example.invalid%2Fquery&secret=1\n" + simpleURI,
		"cert://+.tls.example.invalid#secret\n" + simpleURI,
		"[Proxy]\nsecret = anytls, node.example.invalid, 443, password=\"unterminated",
		"<html>secret</html>", `{"outbounds":[],"password":"secret"}`, `{"error":"secret"}`, "a: 1\n---\nb: 2",
		base64.StdEncoding.EncodeToString([]byte("just secret text")),
	} {
		_, err := Normalize([]byte(raw))
		if err == nil {
			t.Fatal("invalid source accepted")
		}
		if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "node.example") {
			t.Fatal("sensitive diagnostic")
		}
		var parsed *Error
		if !errors.As(err, &parsed) {
			t.Fatal("unstructured parser failure")
		}
	}
}

func TestNativePreservesUnknownFieldsAndComments(t *testing.T) {
	raw := "# comment\nproxies: []\nproxy-groups: []\nx-vendor:\n  nested: [one, two]\nrules: [MATCH,DIRECT]\n"
	r := normalize(t, raw)
	if string(r.Contents) != raw || r.Report.Format != "mihomo" {
		t.Fatal("native configuration was rebuilt")
	}
	if normalize(t, raw).Report.Fingerprint != r.Report.Fingerprint {
		t.Fatal("normalization is not deterministic")
	}
	if normalize(t, raw+"# updated\n").Report.Fingerprint == r.Report.Fingerprint {
		t.Fatal("source change ignored")
	}
}

func TestNamesDoNotMergeDistinctCredentials(t *testing.T) {
	first := "anytls://one@node.example.invalid:443#Proxy"
	r := normalize(t, first+"\n"+first+"\nanytls://two@node.example.invalid:443#Proxy")
	if r.Report.ProxyCount != 2 || proxy(t, r, 0)["name"] == proxy(t, r, 1)["name"] || proxy(t, r, 0)["password"] == proxy(t, r, 1)["password"] {
		t.Fatal("identity or collision handling failed")
	}
}

func TestSurgeDNSAndQuotedCredentials(t *testing.T) {
	raw := `#!MANAGED-CONFIG https://account.example.invalid/api/subscribe/example-token?type=surge interval=86400 strict=false
[General]
dns-server = 192.0.2.53, system
encrypted-dns-server = https://entry-dns.example.invalid/query, https://public-dns.example.invalid/query
[Host]
*.entry.example.invalid = server:https://entry-dns.example.invalid/query
[Proxy]
Example = anytls, node.entry.example.invalid, 443, password=" p,a=ss ", sni=tls.example.invalid
[Proxy Group]
Original = select, Example
[Rule]
FINAL,Original
`
	r := normalize(t, raw)
	root := configuration(t, r)
	dns := root["dns"].(map[string]any)
	if proxy(t, r, 0)["password"] != " p,a=ss " {
		t.Fatal("quoted credential changed")
	}
	if !reflect.DeepEqual(dns["nameserver"], []any{"https://public-dns.example.invalid/query"}) || !reflect.DeepEqual(dns["proxy-server-nameserver"], dns["nameserver"]) {
		t.Fatal("private DNS leaked into default resolvers")
	}
	if !reflect.DeepEqual(dns["default-nameserver"], []any{"192.0.2.53"}) {
		t.Fatal("bootstrap DNS not retained")
	}
	policy := dns["proxy-server-nameserver-policy"].(map[string]any)
	if _, ok := policy[".entry.example.invalid"]; !ok {
		t.Fatal("Surge wildcard changed meaning")
	}
	// Without the producer contract, preserve the explicit global DNS list.
	generic := normalize(t, strings.Replace(raw, "/api/subscribe/example-token?type=surge", "/other.conf", 1))
	if len(configuration(t, generic)["dns"].(map[string]any)["nameserver"].([]any)) != 2 {
		t.Fatal("generic DNS classified as private by URL position")
	}
}

func TestDNSMetadataOrderAndPatternScopes(t *testing.T) {
	r := normalize(t, "host://*.entry.example.invalid?server=https%3A%2F%2Fdns.example.invalid%2FA%3Fx%3DCase\nhost://*.entry.example.invalid?server=udp%3A%2F%2F192.0.2.53%3A5353\n"+simpleURI)
	policy := configuration(t, r)["dns"].(map[string]any)["nameserver-policy"].(map[string]any)
	if !reflect.DeepEqual(policy["*.entry.example.invalid"], []any{"https://dns.example.invalid/A?x=Case", "udp://192.0.2.53:5353"}) {
		t.Fatal("DNS endpoint or order changed")
	}
	for _, tc := range []struct {
		p, h string
		want bool
	}{{"*.example.invalid", "a.example.invalid", true}, {"*.example.invalid", "b.a.example.invalid", false}, {".example.invalid", "b.a.example.invalid", true}, {".example.invalid", "example.invalid", false}, {"+.example.invalid", "example.invalid", true}} {
		if matchesDomain(tc.p, tc.h) != tc.want {
			t.Fatal("wrong domain scope")
		}
	}
	if got, err := normalizeResolver("h3://dns.example.invalid/A?q=Case"); err != nil || got != "https://dns.example.invalid/A?q=Case#h3=true" {
		t.Fatal("DoH3 conversion failed")
	}
}

func TestLeafCertificateMetadataAndPinConflict(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), DNSNames: []string{"*.tls.example.invalid"}, NotBefore: time.Unix(1, 0), NotAfter: time.Unix(2000000000, 0)}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	material := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	header := "cert://+.tls.example.invalid#" + base64.RawURLEncoding.EncodeToString(material) + "\n"
	node := "anytls://pw@node.entry.example.invalid:443?sni=node.tls.example.invalid"
	r := normalize(t, header+node)
	digest := sha256.Sum256(der)
	if proxy(t, r, 0)["fingerprint"] != hex.EncodeToString(digest[:]) {
		t.Fatal("leaf pin was not derived")
	}
	if _, ok := proxy(t, r, 0)["certificate"]; ok {
		t.Fatal("server certificate became mTLS identity")
	}
	if _, err := Normalize([]byte(header + node + "&fingerprint=" + strings.Repeat("ab", 32))); err == nil {
		t.Fatal("conflicting certificate accepted")
	}
}

func TestAdvancedVLESSRequiresVerifiedCore(t *testing.T) {
	r := normalize(t, "vless://"+testUUID+"@node.example.invalid:443?security=reality&type=xhttp&sni=public.example.invalid&pbk="+base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))+"&sid=abcd&path=%2Fv&mode=packet-up")
	if r.CheckCore("v1.19.30") != nil || r.CheckCore("v1.19.20") == nil || r.CheckCore("unknown-example") == nil {
		t.Fatal("advanced core capability was guessed")
	}
	if proxy(t, r, 0)["xhttp-opts"].(map[string]any)["path"] != "/v" {
		t.Fatal("XHTTP lost")
	}
}

func FuzzNormalize(f *testing.F) {
	f.Add([]byte(simpleURI))
	f.Add([]byte("[Proxy]\nx = socks5, 127.0.0.1, 1080, a, b"))
	f.Add([]byte("proxies: []"))
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 32768 {
			return
		}
		before := bytes.Clone(raw)
		result, err := Normalize(raw)
		if !bytes.Equal(raw, before) {
			t.Fatal("input mutated")
		}
		if err == nil {
			var doc map[string]any
			if yaml.Unmarshal(result.Contents, &doc) != nil {
				t.Fatal("invalid output")
			}
		}
	})
}

func TestNativeBlockScalarsDoNotTriggerINIAndErrorJSONIsRejected(t *testing.T) {
	raw := "proxies: []\nvendor-note: |\n  [Proxy]\n  a = b\nerror: vendor-extension\n"
	result := normalize(t, raw)
	if result.Report.Format != "mihomo" || string(result.Contents) != raw {
		t.Fatal("native unknown fields were reinterpreted")
	}
	if _, err := Normalize([]byte(`{"code":403,"message":"sensitive-token"}`)); err == nil || strings.Contains(err.Error(), "sensitive-token") {
		t.Fatal("error response accepted or leaked", err)
	}
	flow := "{mode: rule, proxies: [], vendor: {flag: true}}"
	if got := normalize(t, flow); string(got.Contents) != flow {
		t.Fatal("flow-style native YAML was rejected or changed")
	}
}

func TestSurgeCommentsAndBuiltins(t *testing.T) {
	raw := "[General] # comment\ndns-server = 192.0.2.53\n[Proxy]\nDirect = direct\nNode = socks5, 192.0.2.1, 1080, user, \"p # ; , = x\" ; comment\n"
	result := normalize(t, raw)
	if result.Report.SkippedNodes != 1 || proxy(t, result, 0)["password"] != "p # ; , = x" {
		t.Fatal("comments changed quoted credentials or unsupported builtin was treated as a node")
	}
}

func TestLegacySSPreservesLiteralEscapesInCredentials(t *testing.T) {
	password := "p%2F+@:字"
	raw := "ss://" + base64.StdEncoding.EncodeToString([]byte("aes-128-gcm:"+password+"@[2001:db8::1]:443")) + "#Node"
	result := normalize(t, raw)
	if proxy(t, result, 0)["password"] != password {
		t.Fatal("legacy base64 credentials were percent-decoded")
	}
	bad := "ss://" + base64.StdEncoding.EncodeToString([]byte("aes-128-gcm:\xff")) + "@node.example.test:443"
	if _, err := Normalize([]byte(bad)); err == nil {
		t.Fatal("invalid UTF-8 in SS credentials accepted")
	}
}

func TestTUICDoesNotInventUnsupportedUDPOrClientFingerprintFields(t *testing.T) {
	uri := "tuic://" + testUUID + ":pw@node.example.test:443?udp=true#TUIC"
	result := normalize(t, uri)
	if _, ok := proxy(t, result, 0)["udp"]; ok {
		t.Fatal("TUIC adapter does not implement an udp option")
	}
	for _, raw := range []string{strings.Replace(uri, "udp=true", "udp=false", 1), strings.Replace(uri, "udp=true", "fp=chrome", 1)} {
		mixed := normalize(t, simpleURI+"\n"+raw)
		if mixed.Report.ProxyCount != 1 || mixed.Report.SkippedNodes != 1 {
			t.Fatal("unsupported TUIC feature was silently discarded")
		}
	}
	surge := "[Proxy]\nGood = socks5, 192.0.2.1, 1080, a, b\nTUIC = tuic-v5, 192.0.2.2, 443, uuid=" + testUUID + ", password=pw, udp-relay=false\n"
	result = normalize(t, surge)
	if result.Report.SkippedNodes != 1 || result.Report.Diagnostics[0].Code != "unsupported_udp_setting" {
		t.Fatal("Surge UDP disable was not diagnosed")
	}
}
