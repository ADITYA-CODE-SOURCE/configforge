package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestSchemaIsValidJSON ensures the generated schema is parseable JSON with
// the expected Draft 2020-12 marker and core structural properties.
func TestSchemaIsValidJSON(t *testing.T) {
	schemaData, err := os.ReadFile(filepath.Join("..", "..", "schemas", "config.schema.json"))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}

	var schema map[string]any
	if err := json.Unmarshal(schemaData, &schema); err != nil {
		t.Fatalf("schema is not valid JSON: %v", err)
	}

	if schema["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
		t.Fatalf("$schema = %v, want draft 2020-12", schema["$schema"])
	}
	if schema["additionalProperties"] != false {
		t.Fatal("root additionalProperties must be false")
	}
	if schema["type"] != "object" {
		t.Fatalf("type = %v, want object", schema["type"])
	}

	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("missing properties map")
	}
	for _, key := range []string{"version", "features", "security", "privacy", "logging"} {
		if _, ok := props[key]; !ok {
			t.Fatalf("missing property %q", key)
		}
	}

	required, ok := schema["required"].([]any)
	if !ok || len(required) != 1 || required[0] != "version" {
		t.Fatalf("required = %v, want [version]", required)
	}

	defs, ok := schema["$defs"].(map[string]any)
	if !ok {
		t.Fatal("missing $defs")
	}
	for _, def := range []string{"featureFlag", "routePolicy", "rateLimit", "conditions"} {
		if _, ok := defs[def]; !ok {
			t.Fatalf("missing $def %q", def)
		}
	}

	featureFlag, _ := defs["featureFlag"].(map[string]any)
	if featureFlag["additionalProperties"] != false {
		t.Fatal("featureFlag additionalProperties must be false")
	}

	versionProp, _ := props["version"].(map[string]any)
	enum, _ := versionProp["enum"].([]any)
	if len(enum) != 1 || enum[0] != "v1" {
		t.Fatalf("version enum = %v, want [v1]", enum)
	}
}

// TestSchemaValidConfigAccepted loads valid configs and verifies they pass
// the runtime loader (which matches the schema's additionalProperties=false
// and enum constraints).
func TestSchemaValidConfigAccepted(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "valid", "main.yaml"))
	if err != nil {
		t.Fatalf("read valid: %v", err)
	}
	if _, err := Load(data, WithFilename("valid.yaml")); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
}

// TestSchemaInvalidConfigsRejected loads invalid configs and verifies they
// fail validation, mirroring what the JSON Schema would reject.
func TestSchemaInvalidConfigsRejected(t *testing.T) {
	invalidDir := filepath.Join("..", "..", "testdata", "invalid")
	entries, err := os.ReadDir(invalidDir)
	if err != nil {
		t.Fatalf("read invalid dir: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		path := filepath.Join(invalidDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", entry.Name(), err)
		}
		if _, err := Load(data, WithFilename(entry.Name())); err == nil {
			t.Fatalf("invalid config %s was accepted", entry.Name())
		}
	}
}
