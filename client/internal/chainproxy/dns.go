package chainproxy

import (
	"gopkg.in/yaml.v3"
	"jeemi/internal/config/document"
)

// Imported DNS is deliberately limited to used landing server names. Conflicts
// fail the candidate because mihomo cannot assign two policies to one hostname.
// Homepage optional DNS overrides run afterwards and retain final ownership.
func mergeNodeDNS(root *yaml.Node, groups []Group, used map[string]Node) error {
	mergedDNS, mergedHosts := mapping(), mapping()
	for _, group := range groups {
		for _, node := range group.LandingNodes() {
			if _, ok := used[group.ID+"/"+node.ID]; !ok {
				continue
			}
			for i, text := range []string{node.DNSYAML, node.HostsYAML} {
				if text == "" {
					continue
				}
				parsed, err := document.Parse([]byte(text))
				if err != nil {
					return failure("invalid_nodes")
				}
				destination := mergedDNS
				if i == 1 {
					destination = mergedHosts
				}
				if err := mergeMap(destination, document.Root(parsed)); err != nil {
					return err
				}
			}
		}
	}
	if len(mergedDNS.Content) > 0 {
		dns := value(root, "dns")
		if dns == nil {
			dns = mapping()
			put(root, "dns", dns)
		}
		if dns.Kind != yaml.MappingNode {
			return failure("dns_conflict")
		}
		policies := value(dns, "proxy-server-nameserver-policy")
		if policies == nil {
			policies = mapping()
			put(dns, "proxy-server-nameserver-policy", policies)
		}
		if err := mergeMap(policies, mergedDNS); err != nil {
			return err
		}
		if servers := value(dns, "proxy-server-nameserver"); servers == nil || len(servers.Content) == 0 {
			fallback := sequence()
			fallback.Content = append(fallback.Content, textNode("system"))
			put(dns, "proxy-server-nameserver", fallback)
		}
	}
	if len(mergedHosts.Content) > 0 {
		hosts := value(root, "hosts")
		if hosts == nil {
			hosts = mapping()
			put(root, "hosts", hosts)
		}
		if err := mergeMap(hosts, mergedHosts); err != nil {
			return err
		}
		dns := value(root, "dns")
		if dns == nil {
			dns = mapping()
			put(root, "dns", dns)
		}
		if dns.Kind != yaml.MappingNode {
			return failure("dns_conflict")
		}
		if scalar(value(dns, "use-hosts")) == "false" {
			return failure("dns_conflict")
		}
		put(dns, "use-hosts", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"})
	}
	return nil
}

func mergeMap(destination, source *yaml.Node) error {
	if destination.Kind != yaml.MappingNode || source.Kind != yaml.MappingNode {
		return failure("dns_conflict")
	}
	for i := 0; i < len(source.Content); i += 2 {
		key, item := source.Content[i].Value, source.Content[i+1]
		if previous := value(destination, key); previous != nil && nodeYAML(previous) != nodeYAML(item) {
			return failure("dns_conflict")
		}
		put(destination, key, detached(item))
	}
	return nil
}
