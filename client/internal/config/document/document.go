package document

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	defaultMaxBytes   = 4 << 20
	defaultMaxNodes   = 100_000
	defaultMaxDepth   = 64
	defaultMaxAliases = 128
)

// Limits bounds YAML work before configuration data reaches the composer.
// It protects both imported subscription documents and local field fragments.
type Limits struct {
	MaxBytes   int
	MaxNodes   int
	MaxDepth   int
	MaxAliases int
}

var DefaultLimits = Limits{
	MaxBytes:   defaultMaxBytes,
	MaxNodes:   defaultMaxNodes,
	MaxDepth:   defaultMaxDepth,
	MaxAliases: defaultMaxAliases,
}

// Parse reads exactly one YAML document whose root is a mapping.
func Parse(contents []byte) (*yaml.Node, error) {
	document, err := parse(contents, DefaultLimits)
	if err != nil {
		return nil, err
	}
	if Root(document).Kind != yaml.MappingNode {
		return nil, fmt.Errorf("configuration root must be a mapping")
	}
	return document, nil
}

// ParseValue reads one YAML value used by a local configuration field.
func ParseValue(contents string) (*yaml.Node, error) {
	limits := DefaultLimits
	limits.MaxBytes = 256 << 10
	document, err := parse([]byte(contents), limits)
	if err != nil {
		return nil, err
	}
	return Clone(Root(document)), nil
}

func parse(contents []byte, limits Limits) (*yaml.Node, error) {
	if len(bytes.TrimSpace(contents)) == 0 {
		return nil, fmt.Errorf("YAML document is empty")
	}
	if len(contents) > limits.MaxBytes {
		return nil, fmt.Errorf("YAML document exceeds the %d byte limit", limits.MaxBytes)
	}

	decoder := yaml.NewDecoder(bytes.NewReader(contents))
	var document yaml.Node
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("parse YAML: %w", err)
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("multiple YAML documents are not supported")
		}
		return nil, fmt.Errorf("parse trailing YAML: %w", err)
	}
	if document.Kind != yaml.DocumentNode || len(document.Content) != 1 {
		return nil, fmt.Errorf("YAML must contain exactly one document")
	}

	state := validationState{limits: limits, active: make(map[*yaml.Node]bool)}
	if err := state.visit(Root(&document), "", 1); err != nil {
		return nil, err
	}
	return &document, nil
}

type validationState struct {
	limits  Limits
	nodes   int
	aliases int
	active  map[*yaml.Node]bool
}

func (s *validationState) visit(node *yaml.Node, path string, depth int) error {
	if node == nil {
		return fmt.Errorf("invalid empty YAML node at %s", displayPath(path))
	}
	if depth > s.limits.MaxDepth {
		return fmt.Errorf("YAML nesting exceeds the %d level limit at %s", s.limits.MaxDepth, displayPath(path))
	}
	s.nodes++
	if s.nodes > s.limits.MaxNodes {
		return fmt.Errorf("YAML node count exceeds the %d node limit", s.limits.MaxNodes)
	}
	if !isSupportedTag(node.Tag) {
		return fmt.Errorf("unsupported YAML tag at %s", displayPath(path))
	}

	if node.Kind == yaml.AliasNode {
		s.aliases++
		if s.aliases > s.limits.MaxAliases {
			return fmt.Errorf("YAML alias count exceeds the %d alias limit", s.limits.MaxAliases)
		}
		if node.Alias == nil || s.active[node.Alias] {
			return fmt.Errorf("cyclic or invalid YAML alias at %s", displayPath(path))
		}
		s.active[node.Alias] = true
		err := s.visit(node.Alias, path, depth+1)
		delete(s.active, node.Alias)
		return err
	}

	switch node.Kind {
	case yaml.MappingNode:
		if len(node.Content)%2 != 0 {
			return fmt.Errorf("invalid YAML mapping at %s", displayPath(path))
		}
		seen := make(map[string]struct{}, len(node.Content)/2)
		for index := 0; index < len(node.Content); index += 2 {
			key := node.Content[index]
			if key.Kind != yaml.ScalarNode || key.Value == "" {
				return fmt.Errorf("mapping keys must be non-empty scalars at %s", displayPath(path))
			}
			if key.Value == "<<" {
				return fmt.Errorf("YAML merge keys are not supported at %s", displayPath(path))
			}
			identity := key.Tag + "\x00" + key.Value
			if _, exists := seen[identity]; exists {
				return fmt.Errorf("duplicate YAML key at %s", displayPath(joinPointer(path, key.Value)))
			}
			seen[identity] = struct{}{}
			if err := s.visit(node.Content[index+1], joinPointer(path, key.Value), depth+1); err != nil {
				return err
			}
		}
	case yaml.SequenceNode:
		for index, child := range node.Content {
			if err := s.visit(child, joinPointer(path, strconv.Itoa(index)), depth+1); err != nil {
				return err
			}
		}
	case yaml.ScalarNode:
		// Scalars have no descendants.
	default:
		return fmt.Errorf("unsupported YAML node at %s", displayPath(path))
	}
	return nil
}

func isSupportedTag(tag string) bool {
	if tag == "" {
		return true
	}
	name := tag
	name = strings.TrimPrefix(name, "tag:yaml.org,2002:")
	name = strings.TrimPrefix(name, "!!")
	switch name {
	case "map", "seq", "str", "int", "bool", "null", "float", "timestamp", "binary":
		return true
	default:
		return false
	}
}

// Root returns the value below a YAML document node.
func Root(document *yaml.Node) *yaml.Node {
	if document == nil || document.Kind != yaml.DocumentNode || len(document.Content) != 1 {
		return nil
	}
	return document.Content[0]
}

// New creates an empty mapping document.
func New() *yaml.Node {
	root := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	return &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{root}}
}

// Clone makes a detached copy while preserving anchors and aliases.
func Clone(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}
	copies := make(map[*yaml.Node]*yaml.Node)
	var clone func(*yaml.Node) *yaml.Node
	clone = func(current *yaml.Node) *yaml.Node {
		if current == nil {
			return nil
		}
		if existing, ok := copies[current]; ok {
			return existing
		}
		result := *current
		result.Content = nil
		result.Alias = nil
		copies[current] = &result
		for _, child := range current.Content {
			result.Content = append(result.Content, clone(child))
		}
		result.Alias = clone(current.Alias)
		return &result
	}
	return clone(node)
}

// Encode returns deterministic two-space-indented YAML.
func Encode(document *yaml.Node) ([]byte, error) {
	if Root(document) == nil {
		return nil, fmt.Errorf("invalid YAML document")
	}
	var output bytes.Buffer
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	if err := encoder.Encode(document); err != nil {
		return nil, fmt.Errorf("encode YAML: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("close YAML encoder: %w", err)
	}
	return output.Bytes(), nil
}

// EncodeValue returns a YAML fragment suitable for the field editor.
func EncodeValue(node *yaml.Node) (string, error) {
	if node == nil {
		return "", fmt.Errorf("invalid empty YAML value")
	}
	document := &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{Clone(node)}}
	encoded, err := Encode(document)
	if err != nil {
		return "", err
	}
	value := strings.TrimSuffix(string(encoded), "\n")
	value = strings.TrimSuffix(value, "\n...")
	return value, nil
}

func displayPath(path string) string {
	if path == "" {
		return "/"
	}
	return path
}

func joinPointer(base, segment string) string {
	escaped := strings.ReplaceAll(strings.ReplaceAll(segment, "~", "~0"), "/", "~1")
	return base + "/" + escaped
}
