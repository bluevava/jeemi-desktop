package compose

import "fmt"

type Strategy string

const (
	StrategyReplace              Strategy = "replace"
	StrategyMerge                Strategy = "merge"
	StrategyPrepend              Strategy = "prepend"
	StrategyAppend               Strategy = "append"
	StrategyMergeByName          Strategy = "merge_by_name"
	StrategyAppendBeforeTerminal Strategy = "append_before_terminal"
)

type ConflictPolicy string

const (
	ConflictError    ConflictPolicy = "error"
	ConflictUseLocal ConflictPolicy = "use_local"
)

// Rule describes one enabled local field. Paths use JSON Pointer syntax and
// never appear in the generated mihomo YAML.
type Rule struct {
	Path           string         `json:"path"`
	Strategy       Strategy       `json:"strategy"`
	ConflictPolicy ConflictPolicy `json:"conflictPolicy"`
}

type Result struct {
	YAML string `json:"yaml"`
}

type Conflict struct {
	Path string
	Name string
}

func (e *Conflict) Error() string {
	return fmt.Sprintf("configuration conflict at %s for named item %q", e.Path, e.Name)
}
