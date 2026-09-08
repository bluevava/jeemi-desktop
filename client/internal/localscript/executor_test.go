package localscript

import (
	"strings"
	"testing"
	"time"
)

func TestExecuteTransformsConfigurationWithLexicalMain(t *testing.T) {
	input := []byte("proxies:\n  - name: Original\n    type: ss\nrules:\n  - MATCH,DIRECT\n")
	source := `const main = (config) => {
  config.proxies[0].name = "Updated";
  config.rules.unshift("DOMAIN,example.com,DIRECT");
  return config;
};`

	output, err := Execute(source, input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	text := string(output)
	if !strings.Contains(text, "name: Updated") || !strings.Contains(text, "- DOMAIN,example.com,DIRECT") {
		t.Fatalf("Execute() output did not contain the script changes:\n%s", text)
	}
}

func TestExecuteRejectsMissingMainAndInvalidResult(t *testing.T) {
	configuration := []byte("mode: rule\n")
	for _, testCase := range []struct {
		name   string
		source string
	}{
		{name: "missing main", source: `const helper = (config) => config;`},
		{name: "undefined result", source: `function main(config) { config.mode = "direct"; }`},
		{name: "array root", source: `function main() { return []; }`},
		{name: "promise result", source: `async function main(config) { return config; }`},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := Execute(testCase.source, configuration); err == nil {
				t.Fatal("Execute() accepted an invalid script contract")
			}
		})
	}
}

func TestExecuteInterruptsRunawayScript(t *testing.T) {
	started := time.Now()
	_, err := Execute(`function main() { while (true) {} }`, []byte("mode: rule\n"))
	if err == nil || !strings.Contains(err.Error(), errExecutionTimeout.Error()) {
		t.Fatalf("Execute() error = %v, want timeout", err)
	}
	if elapsed := time.Since(started); elapsed > executionTimeout+2*time.Second {
		t.Fatalf("Execute() timeout took too long: %s", elapsed)
	}
}

func TestExecuteUsesDeterministicClockAndRandomSources(t *testing.T) {
	source := `function main(config) {
  config.scriptValues = [Date.now(), Math.random()];
  return config;
}`
	first, err := Execute(source, []byte("mode: rule\n"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := Execute(source, []byte("mode: rule\n"))
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) || !strings.Contains(string(first), "- 0.5") {
		t.Fatalf("script output is not deterministic:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

func TestExecuteKeepsInternalSerializationWhenScriptReplacesGlobalJSON(t *testing.T) {
	output, err := Execute(`
JSON = null;
function main(config) {
  config.mode = "direct";
  return config;
}`, []byte("mode: rule\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(output), "mode: direct") {
		t.Fatalf("script output was not serialized with the captured intrinsic:\n%s", output)
	}
}
