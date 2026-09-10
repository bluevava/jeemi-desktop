package subscriptionformat

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"jeemi/internal/config/document"
)

var uriStart = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*://`)

// Normalize always derives from the supplied bytes, not a stored format hint.
// Native documents retain every field and comment; external formats use a
// typed intermediate representation before generating a minimal mihomo input.
func Normalize(raw []byte) (Result, error) {
	if len(raw) == 0 || len(raw) > MaxBytes {
		return Result{}, failure("size_limit", 0, "")
	}
	contents := bytes.TrimPrefix(raw, []byte{0xef, 0xbb, 0xbf})
	if !utf8.Valid(contents) || bytes.IndexByte(contents, 0) >= 0 {
		return Result{}, failure("invalid_encoding", 0, "")
	}
	text := strings.TrimSpace(string(contents))
	format, encoding := "mihomo", "plain"
	var parsed parsedDocument
	var err error
	switch {
	case looksSurge(text):
		format = "surge"
		parsed, err = parseSurge(text)
	case looksURIList(text):
		format = "uri"
		parsed, err = parseURIList(text)
	default:
		if decoded, ok := decodeSubscription(text); ok {
			format, encoding = "uri", "base64"
			parsed, err = parseURIList(decoded)
		} else {
			// Flow-style YAML can also start with '{'. JSON storage containers
			// additionally validate JSON syntax at the persistence boundary.
			ast, parseErr := document.Parse(contents)
			if parseErr != nil {
				// A malformed first node must not hide later valid URI lines.
				// Only use this fallback after native YAML has failed, so URI
				// text inside a valid YAML block scalar remains untouched.
				if containsURILine(text) {
					format = "uri"
					parsed, err = parseURIList(text)
					break
				}
				return Result{}, failure("invalid_native", 0, "")
			}
			root := document.Root(ast)
			native, foreign := false, false
			for i := 0; i < len(root.Content); i += 2 {
				switch root.Content[i].Value {
				case "proxies", "proxy-groups", "proxy-providers", "rules", "rule-providers", "mixed-port", "mode", "port", "socks-port":
					native = true
				case "outbounds", "inbounds", "entryDns", "brandCode", "error":
					foreign = true
				case "code", "message", "status":
					foreign = true
				}
			}
			if foreign && !native {
				return Result{}, failure("unsupported_format", 0, "")
			}
			return finish(raw, contents, Report{Format: format, Encoding: encoding, Diagnostics: []Diagnostic{}}, ""), nil
		}
	}
	if err != nil {
		return Result{}, err
	}
	if len(parsed.Nodes) == 0 {
		return Result{}, failure("no_supported_nodes", 0, "")
	}
	if err = applyCertificates(&parsed); err != nil {
		return Result{}, err
	}
	output, required, err := renderMihomo(&parsed)
	if err != nil {
		return Result{}, err
	}
	if _, err = document.Parse(output); err != nil {
		return Result{}, failure("output_limit", 0, "")
	}
	diagnostics := parsed.Diagnostics
	if diagnostics == nil {
		diagnostics = []Diagnostic{}
	}
	return finish(raw, output, Report{Format: format, Encoding: encoding, ProxyCount: parsed.ProxyCount, SkippedNodes: parsed.Skipped, Diagnostics: diagnostics}, required), nil
}

func finish(raw, output []byte, report Report, required string) Result {
	h := sha256.New()
	fmt.Fprintf(h, "subscription-normalization:%d\x00", Version)
	h.Write(raw)
	h.Write([]byte{0})
	h.Write(output)
	report.Fingerprint = hex.EncodeToString(h.Sum(nil))
	report.RequiredCore = required
	return Result{Contents: bytes.Clone(output), Report: report, RequiredCore: required}
}

func looksSurge(text string) bool {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		// Only the document's first statement identifies INI. A native YAML
		// block scalar may legitimately contain a literal [Proxy] line.
		line = strings.ToLower(stripSurgeComment(line))
		return oneOf(line, "[proxy]", "[general]", "[host]", "[proxy group]", "[rule]")
	}
	return false
}

func looksURIList(text string) bool {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		return uriStart.MatchString(line)
	}
	return false
}

func decodeBase64(value string) ([]byte, bool) {
	for _, encoding := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if b, err := encoding.Strict().DecodeString(value); err == nil {
			return b, true
		}
	}
	return nil, false
}

func containsURILine(text string) bool {
	for _, line := range strings.Split(text, "\n") {
		if uriStart.MatchString(strings.TrimSpace(line)) {
			return true
		}
	}
	return false
}

func decodeSubscription(text string) (string, bool) {
	value := strings.NewReplacer("\r", "", "\n", "", " ", "", "\t", "").Replace(text)
	b, ok := decodeBase64(value)
	if !ok || len(b) > MaxBytes || !utf8.Valid(b) || bytes.IndexByte(b, 0) >= 0 {
		return "", false
	}
	decoded := strings.TrimSpace(string(bytes.TrimPrefix(b, []byte{0xef, 0xbb, 0xbf})))
	return decoded, containsURILine(decoded)
}

// Advanced VLESS fields are verified against the official v1.19.30 adapter.
// An unknown imported binary is not evidence that it implements those fields.
func (r Result) CheckCore(version string) error {
	if r.RequiredCore == "" {
		return nil
	}
	if !versionAtLeast(version, r.RequiredCore) {
		return failure("core_version_required", 0, r.RequiredCore)
	}
	return nil
}

func versionAtLeast(value, minimum string) bool {
	parse := func(v string) ([]int, bool) {
		parts := strings.Split(strings.TrimPrefix(v, "v"), ".")
		if len(parts) != 3 {
			return nil, false
		}
		n := make([]int, 3)
		for i, p := range parts {
			v, e := strconv.Atoi(p)
			if e != nil || v < 0 {
				return nil, false
			}
			n[i] = v
		}
		return n, true
	}
	a, ok := parse(value)
	b, _ := parse(minimum)
	if !ok {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return true
}
