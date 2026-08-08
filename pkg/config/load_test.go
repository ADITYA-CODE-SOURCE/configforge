package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadFileValid(t *testing.T) {
	clearConfigForgeEnv(t)

	cfg, err := LoadFile(filepath.Join("..", "..", "testdata", "valid", "main.yaml"))
	if err != nil {
		t.Fatalf("LoadFile returned error: %v", err)
	}

	if cfg.Version != SupportedVersion {
		t.Fatalf("Version = %q, want %q", cfg.Version, SupportedVersion)
	}
	if got := cfg.Features["new_checkout"].RolloutPercentage; got != 25 {
		t.Fatalf("rollout = %d, want 25", got)
	}
	if got := len(cfg.Security.Routes); got != 2 {
		t.Fatalf("routes = %d, want 2", got)
	}
	if got := cfg.Privacy.RedactHeaders[0]; got != "authorization" {
		t.Fatalf("first redacted header = %q, want authorization", got)
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	clearConfigForgeEnv(t)

	_, err := LoadFile(filepath.Join("..", "..", "testdata", "invalid", "unknown-field.yaml"))
	if err == nil {
		t.Fatal("LoadFile succeeded, want error")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("errors.Is(err, ErrInvalidConfig) = false for %v", err)
	}
	if !strings.Contains(err.Error(), "field unknown not found") {
		t.Fatalf("error %q does not mention unknown field", err.Error())
	}
}

func TestLoadRequiresVersion(t *testing.T) {
	clearConfigForgeEnv(t)

	_, err := LoadFile(filepath.Join("..", "..", "testdata", "invalid", "missing-version.yaml"))
	if err == nil {
		t.Fatal("LoadFile succeeded, want error")
	}
	if !strings.Contains(err.Error(), "version is required") {
		t.Fatalf("error %q does not mention required version", err.Error())
	}
}

func TestLoadRejectsUnsupportedVersion(t *testing.T) {
	clearConfigForgeEnv(t)

	_, err := Load([]byte("version: v2\n"), WithFilename("config.yaml"))
	if err == nil {
		t.Fatal("Load succeeded, want error")
	}
	if !strings.Contains(err.Error(), `version must be "v1"`) {
		t.Fatalf("error %q does not mention supported version", err.Error())
	}
}

func TestValidationErrorsIncludePathAndPosition(t *testing.T) {
	clearConfigForgeEnv(t)

	_, err := LoadFile(filepath.Join("..", "..", "testdata", "invalid", "bad-rollout.yaml"))
	if err == nil {
		t.Fatal("LoadFile succeeded, want error")
	}
	if !strings.Contains(err.Error(), "features.checkout.rollout_percentage must be between 0 and 100") {
		t.Fatalf("error %q does not include field path", err.Error())
	}
	if !strings.Contains(err.Error(), "bad-rollout.yaml:5:") {
		t.Fatalf("error %q does not include line and column", err.Error())
	}
}

func TestValidationRejectsBadRoute(t *testing.T) {
	clearConfigForgeEnv(t)

	_, err := LoadFile(filepath.Join("..", "..", "testdata", "invalid", "bad-route.yaml"))
	if err == nil {
		t.Fatal("LoadFile succeeded, want error")
	}

	for _, want := range []string{
		"security.routes[0].path must start with /",
		"security.routes[0].methods[0] must be a valid HTTP method",
		"security.routes[0].allowed_roles[0] must be non-empty",
		"security.routes[0].rate_limit.requests must be greater than zero",
		"security.routes[0].rate_limit.window must be a positive Go duration",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not contain %q", err.Error(), want)
		}
	}
}

func TestValidationRejectsDuplicateValues(t *testing.T) {
	clearConfigForgeEnv(t)

	_, err := LoadFile(filepath.Join("..", "..", "testdata", "invalid", "duplicate-values.yaml"))
	if err == nil {
		t.Fatal("LoadFile succeeded, want error")
	}
	if !strings.Contains(err.Error(), "privacy.redact_headers[1] duplicates privacy.redact_headers[0]") {
		t.Fatalf("error %q does not mention duplicate redaction value", err.Error())
	}
}

func TestValidationRejectsDuplicateYAMLKeys(t *testing.T) {
	clearConfigForgeEnv(t)

	_, err := LoadFile(filepath.Join("..", "..", "testdata", "invalid", "duplicate-keys.yaml"))
	if err == nil {
		t.Fatal("LoadFile succeeded, want error")
	}
	if !strings.Contains(err.Error(), "features.checkout is duplicated") {
		t.Fatalf("error %q does not mention duplicate key", err.Error())
	}
}

func TestEnvironmentOverridesAndFilePrecedence(t *testing.T) {
	clearConfigForgeEnv(t)
	setenv(t, envLogLevel, "debug")
	setenv(t, envDefaultRateLimitRequests, "250")
	setenv(t, envDefaultRateLimitWindow, "30s")
	setenv(t, envRedactHeaders, "X-Secret, Authorization")
	setenv(t, envRedactQueryParameters, "session, token")

	cfg, err := Load([]byte(`
version: v1
logging:
  level: info
privacy:
  redact_headers:
    - cookie
`))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Logging.Level != "info" {
		t.Fatalf("logging.level = %q, want file value info", cfg.Logging.Level)
	}
	if cfg.DefaultRateLimit.Requests != 250 {
		t.Fatalf("default requests = %d, want env value 250", cfg.DefaultRateLimit.Requests)
	}
	if cfg.DefaultRateLimit.Window.Duration != 30*time.Second {
		t.Fatalf("default window = %s, want env value 30s", cfg.DefaultRateLimit.Window)
	}
	if got := strings.Join(cfg.Privacy.RedactHeaders, ","); got != "cookie" {
		t.Fatalf("redact headers = %q, want file override cookie", got)
	}
	if got := strings.Join(cfg.Privacy.RedactQueryParameters, ","); got != "session,token" {
		t.Fatalf("redact query parameters = %q, want env value", got)
	}
}

func TestEnvironmentRejectsInvalidValues(t *testing.T) {
	clearConfigForgeEnv(t)
	setenv(t, envDefaultRateLimitRequests, "zero")

	_, err := Load([]byte("version: v1\n"))
	if err == nil {
		t.Fatal("Load succeeded, want error")
	}
	if !strings.Contains(err.Error(), envDefaultRateLimitRequests) {
		t.Fatalf("error %q does not mention invalid env var", err.Error())
	}
}

func clearConfigForgeEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		envLogLevel,
		envDefaultRateLimitRequests,
		envDefaultRateLimitWindow,
		envRedactHeaders,
		envRedactQueryParameters,
	} {
		unsetenv(t, key)
	}
}

func setenv(t *testing.T, key, value string) {
	t.Helper()
	old, ok := os.LookupEnv(key)
	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("set env %s: %v", key, err)
	}
	t.Cleanup(func() {
		if ok {
			_ = os.Setenv(key, old)
			return
		}
		_ = os.Unsetenv(key)
	})
}

func unsetenv(t *testing.T, key string) {
	t.Helper()
	old, ok := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unset env %s: %v", key, err)
	}
	t.Cleanup(func() {
		if ok {
			_ = os.Setenv(key, old)
		}
	})
}
