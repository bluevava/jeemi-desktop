package localscript

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/dop251/goja"
	"gopkg.in/yaml.v3"

	"jeemi/internal/config/document"
)

const executionTimeout = 2 * time.Second

var (
	errExecutionTimeout = errors.New("local script execution timed out")
	deterministicTime   = time.Unix(0, 0).UTC()
)

// ValidateSource executes only the script declaration and confirms that it
// exposes the FlClash-compatible main(config) function. No host APIs are
// installed in the runtime.
func ValidateSource(source string) error {
	runtime := newRuntime()
	stop := interruptAfter(runtime, executionTimeout)
	defer stop()
	if _, err := runtime.RunString(source); err != nil {
		return scriptError("compile local script", err)
	}
	mainValue, err := runtime.RunString("main")
	if err != nil {
		return fmt.Errorf("local script must define main(config)")
	}
	if _, callable := goja.AssertFunction(mainValue); !callable {
		return fmt.Errorf("local script must define main(config)")
	}
	return nil
}

// Execute converts a bounded YAML mapping to a JSON-compatible JavaScript
// object, calls main(config), and parses the returned object back through the
// same bounded YAML document validator. The runtime has no filesystem,
// network, process, environment, or Wails bindings.
func Execute(source string, configuration []byte) ([]byte, error) {
	documentNode, err := document.Parse(configuration)
	if err != nil {
		return nil, fmt.Errorf("parse script input configuration: %w", err)
	}
	input, err := yamlToJSONValue(document.Root(documentNode))
	if err != nil {
		return nil, err
	}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("encode script input configuration: %w", err)
	}
	if len(inputJSON) > document.DefaultLimits.MaxBytes {
		return nil, fmt.Errorf("local script input exceeds the %d byte limit", document.DefaultLimits.MaxBytes)
	}

	runtime := newRuntime()
	jsonObject := runtime.Get("JSON").ToObject(runtime)
	parse, callable := goja.AssertFunction(jsonObject.Get("parse"))
	if !callable {
		return nil, fmt.Errorf("local script JSON parser is unavailable")
	}
	stringify, callable := goja.AssertFunction(jsonObject.Get("stringify"))
	if !callable {
		return nil, fmt.Errorf("local script JSON serializer is unavailable")
	}
	stop := interruptAfter(runtime, executionTimeout)
	defer stop()
	parsed, err := parse(jsonObject, runtime.ToValue(string(inputJSON)))
	if err != nil {
		return nil, scriptError("prepare local script input", err)
	}
	if _, err := runtime.RunString(source); err != nil {
		return nil, scriptError("compile local script", err)
	}
	mainValue, err := runtime.RunString("main")
	if err != nil {
		return nil, fmt.Errorf("local script must define main(config)")
	}
	main, callable := goja.AssertFunction(mainValue)
	if !callable {
		return nil, fmt.Errorf("local script must define main(config)")
	}
	result, err := main(goja.Undefined(), parsed)
	if err != nil {
		return nil, scriptError("run local script", err)
	}
	if goja.IsUndefined(result) || goja.IsNull(result) ||
		result.ExportType() == reflect.TypeOf((*goja.Promise)(nil)) {
		return nil, fmt.Errorf("local script main(config) must return a configuration object")
	}
	encoded, err := stringify(jsonObject, result)
	if err != nil {
		return nil, scriptError("serialize local script result", err)
	}
	if goja.IsUndefined(encoded) {
		return nil, fmt.Errorf("local script result is not JSON-compatible")
	}
	resultJSON := []byte(encoded.String())
	if len(resultJSON) > document.DefaultLimits.MaxBytes {
		return nil, fmt.Errorf("local script result exceeds the %d byte limit", document.DefaultLimits.MaxBytes)
	}
	resultDocument, err := document.Parse(resultJSON)
	if err != nil {
		return nil, fmt.Errorf("local script result must be a valid configuration object: %w", err)
	}
	normalizeYAMLStyle(document.Root(resultDocument))
	output, err := document.Encode(resultDocument)
	if err != nil {
		return nil, fmt.Errorf("encode local script result: %w", err)
	}
	if len(output) > document.DefaultLimits.MaxBytes {
		return nil, fmt.Errorf("local script result exceeds the %d byte limit", document.DefaultLimits.MaxBytes)
	}
	return output, nil
}

func newRuntime() *goja.Runtime {
	runtime := goja.New()
	runtime.SetTimeSource(func() time.Time { return deterministicTime })
	runtime.SetRandSource(func() float64 { return 0.5 })
	installConsole(runtime)
	return runtime
}

func normalizeYAMLStyle(node *yaml.Node) {
	if node == nil {
		return
	}
	node.Style = 0
	for _, child := range node.Content {
		normalizeYAMLStyle(child)
	}
}

func interruptAfter(runtime *goja.Runtime, timeout time.Duration) func() {
	timer := time.AfterFunc(timeout, func() { runtime.Interrupt(errExecutionTimeout) })
	return func() {
		timer.Stop()
		runtime.ClearInterrupt()
	}
}

func scriptError(action string, err error) error {
	var interrupted *goja.InterruptedError
	if errors.As(err, &interrupted) {
		return fmt.Errorf("%s: %w", action, errExecutionTimeout)
	}
	return fmt.Errorf("%s: %w", action, err)
}

func installConsole(runtime *goja.Runtime) {
	console := runtime.NewObject()
	noop := func(goja.FunctionCall) goja.Value { return goja.Undefined() }
	_ = console.Set("log", noop)
	_ = console.Set("info", noop)
	_ = console.Set("warn", noop)
	_ = console.Set("error", noop)
	_ = runtime.Set("console", console)
}

func yamlToJSONValue(node *yaml.Node) (any, error) {
	if node == nil {
		return nil, fmt.Errorf("local script input contains an empty YAML node")
	}
	if node.Kind == yaml.AliasNode {
		return yamlToJSONValue(node.Alias)
	}
	switch node.Kind {
	case yaml.MappingNode:
		result := make(map[string]any, len(node.Content)/2)
		for index := 0; index < len(node.Content); index += 2 {
			key := node.Content[index]
			if key.Kind != yaml.ScalarNode || (key.Tag != "" && key.Tag != "!!str") {
				return nil, fmt.Errorf("local script input only supports string mapping keys")
			}
			value, err := yamlToJSONValue(node.Content[index+1])
			if err != nil {
				return nil, err
			}
			result[key.Value] = value
		}
		return result, nil
	case yaml.SequenceNode:
		result := make([]any, 0, len(node.Content))
		for _, child := range node.Content {
			value, err := yamlToJSONValue(child)
			if err != nil {
				return nil, err
			}
			result = append(result, value)
		}
		return result, nil
	case yaml.ScalarNode:
		return yamlScalarToJSONValue(node)
	default:
		return nil, fmt.Errorf("local script input contains an unsupported YAML node")
	}
}

func yamlScalarToJSONValue(node *yaml.Node) (any, error) {
	tag := strings.TrimPrefix(node.Tag, "tag:yaml.org,2002:")
	tag = strings.TrimPrefix(tag, "!!")
	switch tag {
	case "", "str", "timestamp", "binary":
		return node.Value, nil
	case "null":
		return nil, nil
	case "bool":
		value, err := strconv.ParseBool(node.Value)
		if err != nil {
			return nil, fmt.Errorf("local script input contains an invalid boolean")
		}
		return value, nil
	case "int":
		var signed int64
		if err := node.Decode(&signed); err == nil {
			if signed > (1<<53)-1 || signed < -((1<<53)-1) {
				return nil, fmt.Errorf("local script input integer exceeds JavaScript's safe range")
			}
			return signed, nil
		}
		var unsigned uint64
		if err := node.Decode(&unsigned); err != nil || unsigned > (1<<53)-1 {
			return nil, fmt.Errorf("local script input integer exceeds JavaScript's safe range")
		}
		return unsigned, nil
	case "float":
		var value float64
		if err := node.Decode(&value); err != nil || math.IsInf(value, 0) || math.IsNaN(value) {
			return nil, fmt.Errorf("local script input contains a non-finite number")
		}
		return value, nil
	default:
		return nil, fmt.Errorf("local script input contains unsupported YAML tag %q", node.Tag)
	}
}
