// Package runtimecontrol owns the control-plane fields that Jeemi injects
// after subscription/local composition and proxy-runtime preference overrides.
package runtimecontrol

import (
	"bytes"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"unicode"

	"jeemi/internal/config/dnstool"
	"jeemi/internal/config/document"

	"gopkg.in/yaml.v3"
)

var managedPaths = []string{
	"/external-controller",
	"/secret",
	"/external-controller-cors",
	"/external-controller-unix",
	"/external-controller-pipe",
	"/external-controller-tls",
	"/external-controller-routing-mark",
	"/external-doh-server",
}

var defaultAllowedOrigins = []string{
	"http://wails.localhost",
	"wails://wails",
	"http://localhost:34115",
	"http://127.0.0.1:34115",
}

type Options struct {
	Address        string
	Secret         string
	AllowedOrigins []string
}

func ManagedPaths() []string {
	return append([]string{}, managedPaths...)
}

func DefaultAllowedOrigins() []string {
	return append([]string{}, defaultAllowedOrigins...)
}

// StripSessionFields removes every control-plane field owned by a concrete
// mihomo process session. The result is safe to keep as a long-lived resolved
// snapshot: a later start still injects a fresh loopback endpoint, secret and
// exact CORS policy through Apply.
func StripSessionFields(configurationYAML []byte) ([]byte, error) {
	configuration, err := document.Parse(configurationYAML)
	if err != nil {
		return nil, fmt.Errorf("parse resolved runtime configuration: %w", err)
	}
	root := document.Root(configuration)
	dnstool.Strip(root)
	for _, path := range managedPaths {
		if err := document.DeleteMappingPath(root, path); err != nil {
			return nil, fmt.Errorf("remove managed control field %s: %w", path, err)
		}
	}
	encoded, err := document.Encode(configuration)
	if err != nil {
		return nil, fmt.Errorf("encode resolved runtime configuration: %w", err)
	}
	return encoded, nil
}

// FormatForDisplay re-encodes a configuration for the read-only UI view while
// preserving its YAML values. yaml.v3 otherwise keeps an imported double-quote
// style and writes supplementary-plane runes as \U escapes, which makes node
// names and other emoji-bearing fields unnecessarily hard to read.
func FormatForDisplay(configurationYAML []byte) ([]byte, error) {
	configuration, err := document.Parse(configurationYAML)
	if err != nil {
		return nil, fmt.Errorf("parse runtime configuration view: %w", err)
	}
	prefix := "__JEEMI_DISPLAY_UNICODE_"
	for bytes.Contains(configurationYAML, []byte(prefix)) {
		prefix += "_"
	}
	maskDisplayUnicode(document.Root(configuration), prefix, make(map[*yaml.Node]struct{}))
	encoded, err := document.Encode(configuration)
	if err != nil {
		return nil, fmt.Errorf("encode runtime configuration view: %w", err)
	}
	return restoreDisplayUnicode(encoded, prefix), nil
}

func maskDisplayUnicode(node *yaml.Node, prefix string, visited map[*yaml.Node]struct{}) {
	if node == nil {
		return
	}
	if _, exists := visited[node]; exists {
		return
	}
	visited[node] = struct{}{}
	if node.Kind == yaml.ScalarNode {
		var value strings.Builder
		value.Grow(len(node.Value))
		changed := false
		for _, decoded := range node.Value {
			if decoded <= 0xffff || !unicode.IsPrint(decoded) {
				value.WriteRune(decoded)
				continue
			}
			value.WriteString(fmt.Sprintf("%s%X__", prefix, decoded))
			changed = true
		}
		if changed {
			node.Value = value.String()
		}
	}
	for _, child := range node.Content {
		maskDisplayUnicode(child, prefix, visited)
	}
	maskDisplayUnicode(node.Alias, prefix, visited)
}

func restoreDisplayUnicode(encoded []byte, prefix string) []byte {
	text := string(encoded)
	var output strings.Builder
	output.Grow(len(encoded))
	offset := 0
	for {
		relativeStart := strings.Index(text[offset:], prefix)
		if relativeStart < 0 {
			output.WriteString(text[offset:])
			break
		}
		start := offset + relativeStart
		digitsStart := start + len(prefix)
		relativeEnd := strings.Index(text[digitsStart:], "__")
		if relativeEnd < 0 {
			output.WriteString(text[offset:])
			break
		}
		end := digitsStart + relativeEnd
		value, err := strconv.ParseUint(text[digitsStart:end], 16, 32)
		if err != nil || value <= 0x7f || value > unicode.MaxRune {
			output.WriteString(text[offset:digitsStart])
			offset = digitsStart
			continue
		}
		output.WriteString(text[offset:start])
		output.WriteRune(rune(value))
		offset = end + 2
	}
	return []byte(output.String())
}

// Apply removes every alternate control entry point and writes a loopback
// HTTP controller protected by a per-session secret and exact CORS origins.
func Apply(configurationYAML []byte, options Options) ([]byte, error) {
	if err := validateOptions(&options); err != nil {
		return nil, err
	}
	configuration, err := document.Parse(configurationYAML)
	if err != nil {
		return nil, fmt.Errorf("parse final runtime configuration: %w", err)
	}
	root := document.Root(configuration)
	for _, path := range managedPaths {
		if err := document.DeleteMappingPath(root, path); err != nil {
			return nil, fmt.Errorf("remove managed control field %s: %w", path, err)
		}
	}
	if err := document.SetMappingPath(root, "/external-controller", scalar(options.Address)); err != nil {
		return nil, err
	}
	if err := document.SetMappingPath(root, "/secret", scalar(options.Secret)); err != nil {
		return nil, err
	}
	cors := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{
		scalar("allow-origins"), stringSequence(options.AllowedOrigins),
		scalar("allow-private-network"), {Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"},
	}}
	if err := document.SetMappingPath(root, "/external-controller-cors", cors); err != nil {
		return nil, err
	}
	encoded, err := document.Encode(configuration)
	if err != nil {
		return nil, fmt.Errorf("encode controlled runtime configuration: %w", err)
	}
	return encoded, nil
}

func validateOptions(options *Options) error {
	host, port, err := net.SplitHostPort(strings.TrimSpace(options.Address))
	if err != nil || port == "" {
		return fmt.Errorf("external controller address is invalid")
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("external controller must bind a loopback address")
	}
	if len(options.Secret) < 32 {
		return fmt.Errorf("external controller secret is too short")
	}
	if len(options.AllowedOrigins) == 0 {
		options.AllowedOrigins = DefaultAllowedOrigins()
	}
	seen := make(map[string]struct{}, len(options.AllowedOrigins))
	for _, origin := range options.AllowedOrigins {
		if origin == "*" {
			return fmt.Errorf("wildcard external controller origin is forbidden")
		}
		parsed, err := url.Parse(origin)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.Path != "" {
			return fmt.Errorf("external controller origin is invalid")
		}
		if _, duplicate := seen[origin]; duplicate {
			return fmt.Errorf("external controller origins contain duplicates")
		}
		seen[origin] = struct{}{}
	}
	return nil
}

func scalar(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

func stringSequence(values []string) *yaml.Node {
	node := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for _, value := range values {
		node.Content = append(node.Content, scalar(value))
	}
	return node
}
