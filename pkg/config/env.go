package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	envLogLevel                 = "CONFIGFORGE_LOG_LEVEL"
	envDefaultRateLimitRequests = "CONFIGFORGE_DEFAULT_RATE_LIMIT_REQUESTS"
	envDefaultRateLimitWindow   = "CONFIGFORGE_DEFAULT_RATE_LIMIT_WINDOW"
	envRedactHeaders            = "CONFIGFORGE_REDACT_HEADERS"
	envRedactQueryParameters    = "CONFIGFORGE_REDACT_QUERY_PARAMETERS"
)

func applyEnv(cfg *Config) error {
	if value, ok := os.LookupEnv(envLogLevel); ok {
		cfg.Logging.Level = strings.TrimSpace(value)
	}

	if value, ok := os.LookupEnv(envDefaultRateLimitRequests); ok {
		parsed, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return fmt.Errorf("%s must be an integer: %w", envDefaultRateLimitRequests, err)
		}
		cfg.DefaultRateLimit.Requests = parsed
	}

	if value, ok := os.LookupEnv(envDefaultRateLimitWindow); ok {
		parsed, err := ParseDuration(strings.TrimSpace(value))
		if err != nil {
			return fmt.Errorf("%s must be a Go duration: %w", envDefaultRateLimitWindow, err)
		}
		cfg.DefaultRateLimit.Window = parsed
	}

	if value, ok := os.LookupEnv(envRedactHeaders); ok {
		parsed, err := parseCSVEnv(envRedactHeaders, value)
		if err != nil {
			return err
		}
		cfg.Privacy.RedactHeaders = parsed
	}

	if value, ok := os.LookupEnv(envRedactQueryParameters); ok {
		parsed, err := parseCSVEnv(envRedactQueryParameters, value)
		if err != nil {
			return err
		}
		cfg.Privacy.RedactQueryParameters = parsed
	}

	if err := validateEnvDerivedConfig(*cfg); err != nil {
		return err
	}

	return nil
}

func parseCSVEnv(name, value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, fmt.Errorf("%s must contain at least one non-empty value", name)
	}

	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for i, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			return nil, fmt.Errorf("%s contains an empty value at position %d", name, i+1)
		}
		result = append(result, trimmed)
	}
	return result, nil
}

func validateEnvDerivedConfig(cfg Config) error {
	if err := validateLogLevel(cfg.Logging.Level); err != nil {
		return fmt.Errorf("%s %w", envLogLevel, err)
	}
	if cfg.DefaultRateLimit.Requests <= 0 {
		return fmt.Errorf("%s must be greater than zero", envDefaultRateLimitRequests)
	}
	if cfg.DefaultRateLimit.Window.Duration <= 0 {
		return fmt.Errorf("%s must be a positive Go duration", envDefaultRateLimitWindow)
	}
	if _, err := normalizeUniqueList(cfg.Privacy.RedactHeaders, "privacy.redact_headers", true); err != nil {
		return fmt.Errorf("%s %w", envRedactHeaders, err)
	}
	if _, err := normalizeUniqueList(cfg.Privacy.RedactQueryParameters, "privacy.redact_query_parameters", true); err != nil {
		return fmt.Errorf("%s %w", envRedactQueryParameters, err)
	}
	return nil
}
