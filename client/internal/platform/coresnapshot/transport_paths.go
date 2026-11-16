package coresnapshot

import "gopkg.in/yaml.v3"

// Mihomo's HTTP transport paths are request data, not filesystem paths. Identify
// only their string values in outbounds; exempting an entire transport mapping
// would also exempt the certificate/private-key in XHTTP download-settings.
// Field shapes follow the official mihomo VLESS/VMess/Trojan transport options.
func transportRequestPaths(root *yaml.Node) map[*yaml.Node]bool {
	paths := map[*yaml.Node]bool{}
	markString := func(node *yaml.Node) {
		if node != nil && node.Kind == yaml.ScalarNode && node.Tag == "!!str" {
			paths[node] = true
		}
	}
	markPath := func(options *yaml.Node, list bool) {
		if options == nil {
			return
		}
		node := mappingValue(options, "path")
		if list {
			if node != nil && node.Kind == yaml.SequenceNode {
				for _, child := range node.Content {
					markString(child)
				}
			}
		} else {
			markString(node)
		}
	}
	visitProxies := func(proxies *yaml.Node) {
		if proxies == nil || proxies.Kind != yaml.SequenceNode {
			return
		}
		for _, proxy := range proxies.Content {
			kind := scalarValue(proxy, "type")
			switch kind {
			case "vmess", "vless":
				markPath(mappingValue(proxy, "http-opts"), true)
				markPath(mappingValue(proxy, "h2-opts"), false)
				markPath(mappingValue(proxy, "ws-opts"), false)
			case "trojan":
				markPath(mappingValue(proxy, "ws-opts"), false)
			}
			if kind == "vless" {
				if options := mappingValue(proxy, "xhttp-opts"); options != nil {
					markPath(options, false)
					markPath(mappingValue(options, "download-settings"), false)
				}
			}
		}
	}
	visitProxies(mappingValue(root, "proxies"))
	if providers := mappingValue(root, "proxy-providers"); providers != nil && providers.Kind == yaml.MappingNode {
		for i := 1; i < len(providers.Content); i += 2 {
			visitProxies(mappingValue(providers.Content[i], "payload"))
		}
	}
	return paths
}
