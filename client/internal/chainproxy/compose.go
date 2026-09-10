package chainproxy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"jeemi/internal/config/document"
	"jeemi/internal/subscriptionformat"
	"strings"
	"time"
	"unicode/utf8"
)

// Apply runs after the local handler and the final MATCH override, before
// homepage DNS ownership, reserved fields, validation and snapshot publication.
func Apply(contents []byte, groups []Group, associationRevision int, coreVersion string) ([]byte, Composition, error) {
	report := Composition{Diagnostics: []Diagnostic{}}
	if len(groups) == 0 && associationRevision == 0 {
		return contents, report, nil
	}
	fingerprint, _ := json.Marshal(struct {
		Version, Parser, Revision int
		Groups                    []Group
	}{Version, subscriptionformat.Version, associationRevision, groups})
	digest := sha256.Sum256(fingerprint)
	report.Fingerprint = hex.EncodeToString(digest[:])
	if len(groups) == 0 {
		return contents, report, nil
	}
	ast, err := document.Parse(contents)
	if err != nil {
		return nil, report, failure("invalid_configuration")
	}
	root := document.Root(ast)
	proxies, selectors := value(root, "proxies"), value(root, "proxy-groups")
	if proxies == nil {
		proxies = sequence()
	}
	if proxies.Kind != yaml.SequenceNode {
		return nil, report, failure("invalid_configuration")
	}
	if selectors == nil {
		selectors = sequence()
	}
	if selectors.Kind != yaml.SequenceNode {
		return nil, report, failure("invalid_configuration")
	}
	byName := map[string]*yaml.Node{}
	occupied := map[string]bool{}
	order := []string{}
	for _, node := range proxies.Content {
		name := scalar(value(node, "name"))
		if name == "" || occupied[name] {
			return nil, report, failure("invalid_configuration")
		}
		occupied[name] = true
		byName[name] = node
		order = append(order, name)
	}
	members := []groupMembership{}
	deadline := time.Now().Add(5 * time.Second)
	for _, selector := range selectors.Content {
		name := scalar(value(selector, "name"))
		if name == "" || occupied[name] {
			return nil, report, failure("invalid_configuration")
		}
		occupied[name] = true
		m, err := membership(selector, order, deadline)
		if err != nil {
			return nil, report, err
		}
		members = append(members, m)
		if time.Now().After(deadline) {
			return nil, report, failure("input_limit")
		}
	}
	match := finalMatch(root)
	additions := map[string][]string{}
	generated := map[string]string{}
	usedNodes := map[string]Node{}
	for _, group := range groups {
		selectorFilter, nodeFilter := ParseNameFilter(group.SelectorFilter), ParseNameFilter(group.NodeFilter)
		nodes := group.LandingNodes()
		selected := 0
		for _, member := range members {
			selector := scalar(value(member.node, "name"))
			if selectorFilter.Empty() && selector != match || !selectorFilter.Empty() && !selectorFilter.Match(selector) {
				continue
			}
			selected++
			if member.unresolved {
				report.Diagnostics = append(report.Diagnostics, Diagnostic{GroupID: group.ID, Selector: selector, Code: "provider_skipped"})
			}
			count, skipped := 0, 0
			for _, name := range member.names {
				middle := byName[name]
				if middle == nil || builtin(name) || strings.HasPrefix(name, "__jeemi_") || !nodeFilter.Match(name) {
					continue
				}
				// Existing multi-hop nodes are not eligible intermediaries in this
				// two-hop feature; this prevents cycles through containing groups.
				if dialer := scalar(value(middle, "dialer-proxy")); dialer != "" && dialer != "DIRECT" {
					skipped++
					continue
				}
				ok, err := member.accepts(name, scalar(value(middle, "type")))
				if err != nil {
					return nil, report, err
				}
				if !ok {
					continue
				}
				for _, landing := range nodes {
					if time.Now().After(deadline) {
						return nil, report, failure("input_limit")
					}
					identity := group.ID + "/" + landing.ID + "/" + name
					generatedName := chainName(landing.Name, name, identity)
					accepted, err := member.accepts(generatedName, landing.Type)
					if err != nil {
						return nil, report, err
					}
					if !accepted {
						skipped++
						continue
					}
					if generated[identity] == "" {
						if err := CheckCore([]Node{landing}, coreVersion); err != nil {
							return nil, report, err
						}
						if occupied[generatedName] {
							return nil, report, failure("name_conflict")
						}
						if len(generated) >= MaxGeneratedNodes {
							return nil, report, failure("generation_limit")
						}
						parsed, err := document.Parse([]byte(landing.YAML))
						if err != nil {
							return nil, report, failure("invalid_nodes")
						}
						copy := detached(document.Root(parsed))
						put(copy, "name", textNode(generatedName))
						put(copy, "dialer-proxy", textNode(name))
						proxies.Content = append(proxies.Content, copy)
						occupied[generatedName] = true
						generated[identity] = generatedName
						usedNodes[group.ID+"/"+landing.ID] = landing
					}
					additions[selector] = append(additions[selector], generatedName)
					count++
				}
				if time.Now().After(deadline) {
					return nil, report, failure("input_limit")
				}
			}
			code := "generated"
			if count == 0 {
				code = "no_nodes_matched"
			}
			report.Diagnostics = append(report.Diagnostics, Diagnostic{GroupID: group.ID, Selector: selector, Code: code, Count: count})
			if skipped > 0 {
				report.Diagnostics = append(report.Diagnostics, Diagnostic{GroupID: group.ID, Selector: selector, Code: "nodes_skipped", Count: skipped})
			}
		}
		if selected == 0 {
			code := "no_selectors_matched"
			if selectorFilter.Empty() {
				code = "match_not_selector"
			}
			report.Diagnostics = append(report.Diagnostics, Diagnostic{GroupID: group.ID, Code: code})
		}
	}
	report.Generated = len(generated)
	if report.Generated == 0 {
		return contents, report, nil
	}
	for _, member := range members {
		// Without freezing the original include-all membership, inserting B
		// globally would also expose it in unrelated selectors.
		freezeIncludeAll(member)
		names := additions[scalar(value(member.node, "name"))]
		if len(names) == 0 {
			continue
		}
		list := value(member.node, "proxies")
		if list == nil {
			list = sequence()
			put(member.node, "proxies", list)
		}
		seen := map[string]bool{}
		for _, entry := range list.Content {
			seen[entry.Value] = true
		}
		for _, name := range names {
			if !seen[name] {
				list.Content = append(list.Content, textNode(name))
				seen[name] = true
			}
		}
	}
	put(root, "proxies", proxies)
	if err := mergeNodeDNS(root, groups, usedNodes); err != nil {
		return nil, report, err
	}
	output, err := document.Encode(ast)
	if err != nil {
		return nil, report, failure("invalid_configuration")
	}
	if _, err := document.Parse(output); err != nil {
		return nil, report, failure("generation_limit")
	}
	return output, report, nil
}

func finalMatch(root *yaml.Node) string {
	rules := value(root, "rules")
	if rules == nil || rules.Kind != yaml.SequenceNode {
		return ""
	}
	for _, entry := range rules.Content {
		parts := strings.Split(entry.Value, ",")
		if len(parts) >= 2 && (strings.EqualFold(strings.TrimSpace(parts[0]), "MATCH") || strings.EqualFold(strings.TrimSpace(parts[0]), "FINAL")) {
			return strings.TrimSpace(parts[1])
		}
	}
	return ""
}

func chainName(landing, middle, identity string) string {
	clip := func(s string) string {
		for len(s) > 96 {
			_, size := utf8.DecodeLastRuneInString(s)
			s = s[:len(s)-size]
		}
		return s
	}
	digest := sha256.Sum256([]byte(identity))
	return fmt.Sprintf("%s ⇐ %s · %x", clip(landing), clip(middle), digest[:6])
}
