package schema

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestGenerateValidJSON(t *testing.T) {
	data, err := Generate()
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("generated schema is not valid JSON: %v", err)
	}

	if schema["$schema"].(string) != Draft {
		t.Fatalf("$schema = %q, want %q", schema["$schema"], Draft)
	}
	if schema["type"].(string) != "object" {
		t.Fatalf("type = %v, want object", schema["type"])
	}

	props := schema["properties"].(map[string]any)
	for _, key := range []string{"version", "features", "security", "privacy", "logging"} {
		if _, ok := props[key]; !ok {
			t.Fatalf("missing property %q", key)
		}
	}

	required := schema["required"].([]any)
	if len(required) != 1 || required[0] != "version" {
		t.Fatalf("required = %v, want [version]", required)
	}

	versionProp := props["version"].(map[string]any)
	enum := versionProp["enum"].([]any)
	if len(enum) != 1 || enum[0] != "v1" {
		t.Fatalf("version enum = %v, want [v1]", enum)
	}
}

func TestGenerateDeterministic(t *testing.T) {
	first, err := Generate()
	if err != nil {
		t.Fatalf("first Generate: %v", err)
	}
	second, err := Generate()
	if err != nil {
		t.Fatalf("second Generate: %v", err)
	}
	if string(first) != string(second) {
		t.Fatal("Generate is not deterministic")
	}
}

func TestGenerateRejectsUnknownProperties(t *testing.T) {
	data, err := Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if schema["additionalProperties"].(bool) != false {
		t.Fatal("root additionalProperties is not false")
	}

	defs := schema["$defs"].(map[string]any)
	featureFlag := defs["featureFlag"].(map[string]any)
	if featureFlag["additionalProperties"].(bool) != false {
		t.Fatal("featureFlag additionalProperties is not false")
	}

	routePolicy := defs["routePolicy"].(map[string]any)
	if routePolicy["additionalProperties"].(bool) != false {
		t.Fatal("routePolicy additionalProperties is not false")
	}
}

func TestGenerateIncludesDescriptions(t *testing.T) {
	data, err := Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if !strings.Contains(string(data), "Declarative feature and policy configuration") {
		t.Fatal("missing root description")
	}

	if !strings.Contains(string(data), "Maximum requests during the window") {
		t.Fatal("missing rate-limit description")
	}

	if !strings.Contains(string(data), "Percentage of users that receive the feature") {
		t.Fatal("missing rollout description")
	}
}
