package chainproxy

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"gopkg.in/yaml.v3"
	"jeemi/internal/config/document"
	"jeemi/internal/subscriptionformat"
	"net"
	"strconv"
	"strings"
	"unicode"
)

type Imported struct {
	Nodes  []Node
	Report subscriptionformat.Report
}

// Parse accepts the subscription formats, a standalone proxy object or a YAML
// list of objects. Only nodes and their DNS dependencies leave this boundary.
func Parse(raw []byte, filter string) (Imported, error) {
	if len(raw) == 0 || len(raw) > subscriptionformat.MaxBytes {
		return Imported{}, failure("input_limit")
	}
	input := raw
	trimmed := bytes.TrimSpace(bytes.TrimPrefix(raw, []byte{0xef, 0xbb, 0xbf}))
	normalized, err := subscriptionformat.Normalize(input)
	if err != nil && (bytes.HasPrefix(trimmed, []byte("- ")) || bytes.HasPrefix(trimmed, []byte("["))) {
		// Indenting a list under proxies retains the normal document size and
		// complexity limits, unlike the smaller single-field editor parser.
		input = []byte("proxies:\n  " + strings.ReplaceAll(string(trimmed), "\n", "\n  "))
		normalized, err = subscriptionformat.Normalize(input)
	}
	if err != nil {
		return Imported{}, err
	}
	ast, err := document.Parse(normalized.Contents)
	if err != nil {
		return Imported{}, failure("invalid_nodes")
	}
	root := document.Root(ast)
	nodes := value(root, "proxies")
	if nodes == nil && value(root, "type") != nil {
		nodes = sequence()
		nodes.Content = append(nodes.Content, root)
	}
	if nodes == nil || nodes.Kind != yaml.SequenceNode || len(nodes.Content) == 0 {
		return Imported{}, failure("no_nodes")
	}
	result := Imported{Nodes: []Node{}, Report: normalized.Report}
	result.Report.ProxyCount = len(nodes.Content)
	matcher := ParseNameFilter(filter)
	seen := map[string]bool{}
	landingCount := 0
	for _, source := range nodes.Content {
		node := detached(source)
		kind := scalar(value(node, "type"))
		if kind == "direct" || kind == "dns" || kind == "rematch" {
			result.Report.ProxyCount--
			result.Report.SkippedNodes++
			result.Report.Diagnostics = append(result.Report.Diagnostics, subscriptionformat.Diagnostic{Code: "unsupported_protocol"})
			continue // Local outbounds cannot serve as a remote landing proxy.
		}
		if err := validateNode(node); err != nil {
			return Imported{}, err
		}
		landingCount++
		name := scalar(value(node, "name"))
		if !matcher.Match(name) {
			continue
		}
		if seen[name] {
			return Imported{}, failure("duplicate_node_name")
		}
		seen[name] = true
		// Both fields belong to Jeemi's generated link, never the imported B.
		if value(node, "dialer-proxy") != nil {
			result.Report.Diagnostics = append(result.Report.Diagnostics, subscriptionformat.Diagnostic{Code: "chain_dialer_replaced"})
		}
		remove(node, "dialer-proxy")
		dns, hosts := nodeDNS(root, node)
		digest := sha256.Sum256([]byte(name))
		result.Nodes = append(result.Nodes, Node{ID: hex.EncodeToString(digest[:16]), Name: name, Type: kind, YAML: nodeYAML(node), DNSYAML: dns, HostsYAML: hosts, RequiredCore: normalized.RequiredCore})
	}
	if landingCount == 0 {
		return Imported{}, failure("no_nodes")
	}
	return result, nil
}

func validateNode(node *yaml.Node) error {
	if node == nil || node.Kind != yaml.MappingNode {
		return failure("invalid_nodes")
	}
	name, kind, server := scalar(value(node, "name")), scalar(value(node, "type")), scalar(value(node, "server"))
	if !validText(name, 256) || name == "" || !validText(kind, 40) || kind == "" {
		return failure("invalid_nodes")
	}
	if builtin(name) || builtin(strings.ToUpper(kind)) || strings.HasPrefix(name, "__jeemi_") {
		return failure("invalid_nodes")
	}
	if server == "" {
		if kind != "tailscale" && kind != "zerotier" && (kind != "wireguard" || value(node, "peers") == nil) {
			return failure("invalid_nodes")
		}
	} else if !validText(server, 1024) || strings.ContainsAny(server, " \t\r\n/@?#") {
		return failure("invalid_nodes")
	}
	if port := value(node, "port"); port != nil {
		n, err := strconv.Atoi(scalar(port))
		if err != nil || n < 1 || n > 65535 {
			return failure("invalid_nodes")
		}
	} else if kind != "ssh" && kind != "wireguard" && kind != "openvpn" && kind != "tailscale" && kind != "zerotier" && !(kind == "mieru" && scalar(value(node, "port-range")) != "") {
		return failure("invalid_nodes")
	}
	return nil
}

func validText(text string, maximum int) bool {
	if len(text) > maximum {
		return false
	}
	for _, c := range text {
		if unicode.IsControl(c) {
			return false
		}
	}
	return true
}

func nodeDNS(root, node *yaml.Node) (string, string) {
	servers := []string{scalar(value(node, "server"))}
	if peers := value(node, "peers"); peers != nil && peers.Kind == yaml.SequenceNode {
		for _, peer := range peers.Content {
			servers = append(servers, scalar(value(peer, "server")))
		}
	}
	policies, hosts := mapping(), mapping()
	for _, server := range servers {
		if server == "" || net.ParseIP(server) != nil {
			continue
		}
		resolvers, addresses := serverDNS(root, server)
		if resolvers != nil {
			put(policies, server, detached(resolvers))
		}
		if addresses != nil {
			put(hosts, server, detached(addresses))
		}
	}
	dnsResult, hostsResult := "", ""
	if len(policies.Content) > 0 {
		dnsResult = nodeYAML(policies)
	}
	if len(hosts.Content) > 0 {
		hostsResult = nodeYAML(hosts)
	}
	return dnsResult, hostsResult
}

func serverDNS(root *yaml.Node, server string) (*yaml.Node, *yaml.Node) {
	dns := value(root, "dns")
	var resolvers *yaml.Node
	policies := value(dns, "proxy-server-nameserver-policy")
	if policies != nil && policies.Kind == yaml.MappingNode {
		// Exact entries win over matching suffix entries.
		for i := 0; i < len(policies.Content); i += 2 {
			if domainMatches(policies.Content[i].Value, server) {
				resolvers = policies.Content[i+1]
				if policies.Content[i].Value == server {
					break
				}
			}
		}
	}
	if resolvers == nil {
		resolvers = value(dns, "proxy-server-nameserver")
	}
	if resolvers != nil && resolvers.Kind != yaml.ScalarNode && len(resolvers.Content) == 0 {
		resolvers = nil
	}
	var addresses *yaml.Node
	hosts := value(root, "hosts")
	if hosts != nil && hosts.Kind == yaml.MappingNode {
		for i := 0; i < len(hosts.Content); i += 2 {
			if domainMatches(hosts.Content[i].Value, server) {
				addresses = hosts.Content[i+1]
				if hosts.Content[i].Value == server {
					break
				}
			}
		}
	}
	return resolvers, addresses
}

func builtin(name string) bool {
	switch name {
	case "DIRECT", "REJECT", "REJECT-DROP", "PASS", "COMPATIBLE", "GLOBAL":
		return true
	}
	return false
}

type Error struct{ Code string }

func (e *Error) Error() string  { return "chain_proxy:" + e.Code }
func failure(code string) error { return &Error{Code: code} }

func CheckCore(nodes []Node, version string) error {
	if version == "" {
		return nil
	}
	for _, node := range nodes {
		if err := (subscriptionformat.Result{RequiredCore: node.RequiredCore}).CheckCore(version); err != nil {
			return failure("core_version_required")
		}
	}
	return nil
}

// Editing a node includes its resolver metadata so saving unchanged text does
// not accidentally discard custom DNS from a previous URI import.
func EditableNode(node Node) string {
	if node.DNSYAML == "" && node.HostsYAML == "" {
		return node.YAML
	}
	root := mapping()
	proxies := sequence()
	parsed, err := document.Parse([]byte(node.YAML))
	if err != nil {
		return node.YAML
	}
	proxies.Content = append(proxies.Content, document.Root(parsed))
	put(root, "proxies", proxies)
	if node.DNSYAML != "" {
		parsed, err := document.Parse([]byte(node.DNSYAML))
		if err == nil {
			dns := mapping()
			put(dns, "proxy-server-nameserver-policy", document.Root(parsed))
			put(root, "dns", dns)
		}
	}
	if node.HostsYAML != "" {
		parsed, err := document.Parse([]byte(node.HostsYAML))
		if err == nil {
			put(root, "hosts", document.Root(parsed))
		}
	}
	return nodeYAML(root)
}
