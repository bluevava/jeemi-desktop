// Package dnstool owns ephemeral, authenticated mihomo inbounds used only by
// DNS diagnostics. They never enter subscriptions or resolved snapshots.
package dnstool

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"strings"

	"jeemi/internal/config/document"

	"gopkg.in/yaml.v3"
)

const Prefix = "__jeemi_dns_"
const Group = Prefix + "egress"

type Session struct {
	ProxyAddress  string
	DirectAddress string
	Username      string
	Password      string
	MatchTarget   string
	ProviderNames []string
}

// Prepare reserves distinct loopback ports, then releases them for the core.
// The actual generation is validated by the selected official core as usual.
func Prepare(configuration []byte, secret string) ([]byte, Session, error) {
	if len(secret) < 32 {
		return nil, Session{}, fmt.Errorf("invalid DNS tool session")
	}
	first, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, Session{}, fmt.Errorf("reserve DNS tool endpoint")
	}
	defer first.Close()
	second, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, Session{}, fmt.Errorf("reserve DNS tool endpoint")
	}
	defer second.Close()
	// Never send the controller bearer secret to a SOCKS endpoint. A
	// domain-separated credential limits a loopback-port collision to this
	// diagnostic channel, without disclosing API authentication.
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte("Jeemi DNS diagnostic SOCKS authentication v1"))
	session := Session{ProxyAddress: first.Addr().String(), DirectAddress: second.Addr().String(), Username: "jeemi-dns", Password: hex.EncodeToString(mac.Sum(nil))}
	contents, err := Apply(configuration, &session)
	return contents, session, err
}

func Apply(configuration []byte, session *Session) ([]byte, error) {
	parsed, err := document.Parse(configuration)
	if err != nil {
		return nil, err
	}
	root := document.Root(parsed)
	Strip(root)
	session.MatchTarget = MatchTarget(root)
	session.ProviderNames = nil
	if providers, found, _ := document.Find(root, "/proxy-providers"); found && providers.Kind == yaml.MappingNode {
		for index := 0; index+1 < len(providers.Content); index += 2 {
			session.ProviderNames = append(session.ProviderNames, providers.Content[index].Value)
		}
	}
	group := map[string]any{"name": Group, "type": "select", "hidden": true, "proxies": []string{"REJECT"}, "include-all": true}
	if err := appendObject(root, "/proxy-groups", group); err != nil {
		return nil, err
	}
	for _, endpoint := range []struct{ name, address, target string }{
		{Prefix + "proxy", session.ProxyAddress, Group}, {Prefix + "direct", session.DirectAddress, "DIRECT"},
	} {
		host, port, err := net.SplitHostPort(endpoint.address)
		if err != nil || host != "127.0.0.1" || len(session.Password) < 32 {
			return nil, fmt.Errorf("invalid DNS tool endpoint")
		}
		portNumber, err := strconv.Atoi(port)
		if err != nil || portNumber < 1 || portNumber > 65535 {
			return nil, fmt.Errorf("invalid DNS tool port")
		}
		listener := map[string]any{"name": endpoint.name, "type": "socks", "listen": host, "port": portNumber, "udp": false, "proxy": endpoint.target, "users": []map[string]string{{"username": session.Username, "password": session.Password}}}
		if err := appendObject(root, "/listeners", listener); err != nil {
			return nil, err
		}
	}
	return document.Encode(parsed)
}

func appendObject(root *yaml.Node, path string, value any) error {
	sequence, found, err := document.Find(root, path)
	if err != nil {
		return err
	}
	if !found {
		sequence = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	}
	if sequence.Kind != yaml.SequenceNode {
		return fmt.Errorf("invalid DNS tool configuration collection")
	}
	var node yaml.Node
	if err := node.Encode(value); err != nil {
		return err
	}
	sequence.Content = append(sequence.Content, &node)
	if !found {
		// SetMappingPath clones the node. Insert the populated sequence so
		// the first listener/group is retained in the actual configuration.
		return document.SetMappingPath(root, path, sequence)
	}
	return nil
}

func MatchTarget(root *yaml.Node) string {
	rules, found, _ := document.Find(root, "/rules")
	if !found || rules.Kind != yaml.SequenceNode {
		return ""
	}
	for _, rule := range rules.Content {
		parts := strings.Split(rule.Value, ",")
		if len(parts) >= 2 && (strings.EqualFold(strings.TrimSpace(parts[0]), "MATCH") || strings.EqualFold(strings.TrimSpace(parts[0]), "FINAL")) {
			return strings.TrimSpace(parts[1])
		}
	}
	return ""
}

// Strip also covers restart snapshots and the read-only configuration view.
// The reserved prefix prevents these internal objects and their credentials
// from being persisted or exposed as user-owned configuration.
func Strip(root *yaml.Node) {
	for _, path := range []string{"/listeners", "/proxy-groups"} {
		sequence, found, _ := document.Find(root, path)
		if !found || sequence.Kind != yaml.SequenceNode {
			continue
		}
		kept := sequence.Content[:0]
		removed := false
		for _, node := range sequence.Content {
			name, found, _ := document.Find(node, "/name")
			if found && strings.HasPrefix(name.Value, Prefix) {
				removed = true
				continue
			}
			kept = append(kept, node)
		}
		sequence.Content = kept
		if removed && len(kept) == 0 {
			_ = document.DeleteMappingPath(root, path)
		}
	}
}
