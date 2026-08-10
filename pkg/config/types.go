// Package config loads and validates ConfigForge YAML configuration files.
package config

import (
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	// SupportedVersion is the only configuration version currently accepted.
	SupportedVersion = "v1"

	// DefaultRedactionReplacement is used when sensitive values are redacted.
	DefaultRedactionReplacement = "[REDACTED]"
)

// Config is the root ConfigForge configuration document.
type Config struct {
	Version          string                 `json:"version" yaml:"version"`
	Features         map[string]FeatureFlag `json:"features,omitempty" yaml:"features"`
	Security         SecurityConfig         `json:"security,omitempty" yaml:"security"`
	Privacy          PrivacyConfig          `json:"privacy,omitempty" yaml:"privacy"`
	Logging          LoggingConfig          `json:"logging,omitempty" yaml:"logging"`
	DefaultRateLimit RateLimitPolicy        `json:"-" yaml:"-"`
}

// FeatureFlag declares a named feature flag and its targeting conditions.
type FeatureFlag struct {
	Enabled           bool              `json:"enabled" yaml:"enabled"`
	RolloutPercentage int               `json:"rollout_percentage,omitempty" yaml:"rollout_percentage"`
	Conditions        FeatureConditions `json:"conditions,omitempty" yaml:"conditions"`
}

// FeatureConditions restrict feature access to users, countries, or roles.
type FeatureConditions struct {
	Countries []string `json:"countries,omitempty" yaml:"countries"`
	Users     []string `json:"users,omitempty" yaml:"users"`
	Roles     []string `json:"roles,omitempty" yaml:"roles"`
}

// SecurityConfig contains HTTP route policies.
type SecurityConfig struct {
	Routes []RoutePolicy `json:"routes,omitempty" yaml:"routes"`
}

// RoutePolicy describes authentication, authorization, and rate limiting for a route.
type RoutePolicy struct {
	Name                  string           `json:"name" yaml:"name"`
	Path                  string           `json:"path" yaml:"path"`
	Methods               []string         `json:"methods" yaml:"methods"`
	RequireAuthentication bool             `json:"require_authentication" yaml:"require_authentication"`
	AllowedRoles          []string         `json:"allowed_roles,omitempty" yaml:"allowed_roles"`
	RateLimit             *RateLimitPolicy `json:"rate_limit,omitempty" yaml:"rate_limit"`
}

// RateLimitPolicy describes a fixed request budget over a positive duration.
type RateLimitPolicy struct {
	Requests int      `json:"requests" yaml:"requests"`
	Window   Duration `json:"window" yaml:"window"`
}

// PrivacyConfig declares values that must be redacted from requests and logs.
type PrivacyConfig struct {
	RedactHeaders         []string `json:"redact_headers,omitempty" yaml:"redact_headers"`
	RedactQueryParameters []string `json:"redact_query_parameters,omitempty" yaml:"redact_query_parameters"`
	RedactJSONFields      []string `json:"redact_json_fields,omitempty" yaml:"redact_json_fields"`
	Replacement           string   `json:"replacement,omitempty" yaml:"replacement"`
}

// LoggingConfig controls ConfigForge decision logging.
type LoggingConfig struct {
	Level            string `json:"level" yaml:"level"`
	IncludeRequestID bool   `json:"include_request_id" yaml:"include_request_id"`
}

// Duration wraps time.Duration so YAML values can use Go duration strings such as "1m".
type Duration struct {
	time.Duration
}

// ParseDuration parses a ConfigForge duration string.
func ParseDuration(value string) (Duration, error) {
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return Duration{}, err
	}
	return Duration{Duration: parsed}, nil
}

// MustDuration is a convenience helper for tests and defaults that panics on
// invalid input. Production callers should use ParseDuration and handle the
// error.
func MustDuration(value string) Duration {
	d, err := ParseDuration(value)
	if err != nil {
		panic(err)
	}
	return d
}

// UnmarshalYAML decodes a Go duration string from YAML.
func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var raw string
	if err := value.Decode(&raw); err != nil {
		return err
	}

	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", raw, err)
	}
	d.Duration = parsed
	return nil
}

// MarshalYAML encodes the duration using Go's duration string format.
func (d Duration) MarshalYAML() (any, error) {
	return d.String(), nil
}

// IsZero reports whether the duration is unset.
func (d Duration) IsZero() bool {
	return d.Duration == 0
}
