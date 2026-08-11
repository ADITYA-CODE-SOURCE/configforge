package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestCLIValidateValid(t *testing.T) {
	result := runCLI(t, "validate", "--config", filepath.Join("..", "..", "examples", "configs", "default.yaml"))
	if result.exitCode != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%s", result.exitCode, result.stderr)
	}
	if !strings.Contains(result.stdout, "is valid") {
		t.Fatalf("stdout = %q", result.stdout)
	}
}

func TestCLIValidateInvalid(t *testing.T) {
	result := runCLI(t, "validate", "--config", filepath.Join("..", "..", "testdata", "invalid", "bad-rollout.yaml"))
	if result.exitCode == 0 {
		t.Fatal("exit = 0, want non-zero for invalid config")
	}
	if !strings.Contains(result.stderr, "rollout_percentage") {
		t.Fatalf("stderr = %q", result.stderr)
	}
}

func TestCLISchema(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "schema.json")
	result := runCLI(t, "schema", "--output", output)
	if result.exitCode != 0 {
		t.Fatalf("exit = %d; stderr=%s", result.exitCode, result.stderr)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	if !strings.Contains(string(data), "draft/2020-12") {
		t.Fatal("schema missing draft 2020-12")
	}
}

func TestCLIGenerate(t *testing.T) {
	dir := t.TempDir()
	result := runCLI(t, "generate", "--manifests", filepath.Join("..", "..", "manifests"), "--output", dir)
	if result.exitCode != 0 {
		t.Fatalf("exit = %d; stderr=%s", result.exitCode, result.stderr)
	}
	for _, want := range []string{
		filepath.Join("pkg", "config", "generated.go"),
		filepath.Join("docs", "configuration_options.md"),
		filepath.Join("schemas", "config.schema.json"),
	} {
		path := filepath.Join(dir, want)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Fatalf("missing generated file: %s", want)
		}
	}
}

func TestCLIExplain(t *testing.T) {
	result := runCLI(t, "explain", "version", "--config", filepath.Join("..", "..", "examples", "configs", "default.yaml"))
	if result.exitCode != 0 {
		t.Fatalf("exit = %d; stderr=%s", result.exitCode, result.stderr)
	}
	if !strings.Contains(result.stdout, "v1") {
		t.Fatalf("stdout = %q", result.stdout)
	}
}

func TestCLIExplainFeature(t *testing.T) {
	result := runCLI(t, "explain", "features.new_checkout", "--config", filepath.Join("..", "..", "examples", "configs", "default.yaml"))
	if result.exitCode != 0 {
		t.Fatalf("exit = %d; stderr=%s", result.exitCode, result.stderr)
	}
	if !strings.Contains(result.stdout, "enabled=true") {
		t.Fatalf("stdout = %q", result.stdout)
	}
}

func TestCLIExplainPrivacy(t *testing.T) {
	result := runCLI(t, "explain", "privacy.redact_headers", "--config", filepath.Join("..", "..", "examples", "configs", "default.yaml"))
	if result.exitCode != 0 {
		t.Fatalf("exit = %d; stderr=%s", result.exitCode, result.stderr)
	}
	if !strings.Contains(result.stdout, "authorization") {
		t.Fatalf("stdout = %q", result.stdout)
	}
}

func TestCLIMissingConfigFlag(t *testing.T) {
	result := runCLI(t, "validate")
	if result.exitCode == 0 {
		t.Fatal("exit = 0, want non-zero")
	}
	if !strings.Contains(result.stderr, "--config is required") {
		t.Fatalf("stderr = %q", result.stderr)
	}
}

func TestCLIExplainUnknownPath(t *testing.T) {
	result := runCLI(t, "explain", "nonexistent.path", "--config", filepath.Join("..", "..", "examples", "configs", "default.yaml"))
	if result.exitCode != 0 {
		t.Fatalf("exit = %d; stderr=%s", result.exitCode, result.stderr)
	}
	if !strings.Contains(result.stdout, "unknown") {
		t.Fatalf("stdout = %q", result.stdout)
	}
}

func TestCLICheckFeatureEnabled(t *testing.T) {
	result := runCLI(t, "check-feature",
		"--config", filepath.Join("..", "..", "examples", "configs", "default.yaml"),
		"--feature", "new_checkout",
		"--user", "user-101",
		"--country", "IN",
	)
	if result.exitCode != 0 {
		t.Fatalf("exit = %d; stderr=%s", result.exitCode, result.stderr)
	}
	if !strings.Contains(result.stdout, "enabled=true") {
		t.Fatalf("stdout = %q", result.stdout)
	}
}

func TestCLICheckFeatureDisabled(t *testing.T) {
	result := runCLI(t, "check-feature",
		"--config", filepath.Join("..", "..", "testdata", "valid", "main.yaml"),
		"--feature", "nonexistent",
		"--user", "unknown-user",
	)
	if result.exitCode != 2 {
		t.Fatalf("exit = %d, want 2 for denied feature; stdout=%s", result.exitCode, result.stdout)
	}
	if !strings.Contains(result.stdout, "enabled=false") {
		t.Fatalf("stdout = %q", result.stdout)
	}
}

func TestCLICheckRouteAllowed(t *testing.T) {
	result := runCLI(t, "check-route",
		"--config", filepath.Join("..", "..", "examples", "configs", "default.yaml"),
		"--path", "/api/payments/create",
		"--method", "POST",
		"--authenticated",
		"--role", "customer",
	)
	if result.exitCode != 0 {
		t.Fatalf("exit = %d; stderr=%s", result.exitCode, result.stderr)
	}
	if !strings.Contains(result.stdout, "allowed=true") {
		t.Fatalf("stdout = %q", result.stdout)
	}
	if !strings.Contains(result.stdout, "matched_policy=create-payment") {
		t.Fatalf("stdout = %q", result.stdout)
	}
}

func TestCLICheckRouteDenied(t *testing.T) {
	result := runCLI(t, "check-route",
		"--config", filepath.Join("..", "..", "examples", "configs", "default.yaml"),
		"--path", "/api/payments/create",
		"--method", "POST",
	)
	if result.exitCode != 2 {
		t.Fatalf("exit = %d, want 2 for denied route; stdout=%s", result.exitCode, result.stdout)
	}
	if !strings.Contains(result.stdout, "allowed=false") {
		t.Fatalf("stdout = %q", result.stdout)
	}
}

func TestCLICheckFeatureMissingFlag(t *testing.T) {
	result := runCLI(t, "check-feature",
		"--config", filepath.Join("..", "..", "examples", "configs", "default.yaml"),
	)
	if result.exitCode == 0 {
		t.Fatal("exit = 0, want non-zero")
	}
	if !strings.Contains(result.stderr, "--feature is required") {
		t.Fatalf("stderr = %q", result.stderr)
	}
}

func TestCLICheckRouteMissingFlag(t *testing.T) {
	result := runCLI(t, "check-route",
		"--config", filepath.Join("..", "..", "examples", "configs", "default.yaml"),
	)
	if result.exitCode == 0 {
		t.Fatal("exit = 0, want non-zero")
	}
	if !strings.Contains(result.stderr, "--path is required") {
		t.Fatalf("stderr = %q", result.stderr)
	}
}

type cliResult struct {
	stdout   string
	stderr   string
	exitCode int
}

func runCLI(t *testing.T, args ...string) cliResult {
	t.Helper()
	var stdout, stderr bytes.Buffer

	root := &cobra.Command{
		Use:           "configforge",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(validateCommand())
	root.AddCommand(generateCommand())
	root.AddCommand(schemaCommand())
	root.AddCommand(explainCommand())
	root.AddCommand(checkFeatureCommand())
	root.AddCommand(checkRouteCommand())

	root.SetOut(&stdout)
	root.SetErr(&stderr)

	root.SetArgs(args)
	err := root.Execute()
	exitCode := 0
	if err != nil {
		exitCode = 1
		if err.Error() == "denied" {
			exitCode = 2
		}
		_, _ = stderr.WriteString(err.Error() + "\n")
	}
	return cliResult{stdout: stdout.String(), stderr: stderr.String(), exitCode: exitCode}
}
