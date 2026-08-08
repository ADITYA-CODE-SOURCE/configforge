package config

import "time"

// Defaults returns the built-in ConfigForge defaults.
func Defaults() Config {
	return Config{
		Version:  "",
		Features: map[string]FeatureFlag{},
		Security: SecurityConfig{
			Routes: []RoutePolicy{},
		},
		Privacy: PrivacyConfig{
			RedactHeaders:         []string{"authorization", "cookie", "x-api-key"},
			RedactQueryParameters: []string{"password", "token", "api_key"},
			RedactJSONFields:      []string{"password", "access_token"},
			Replacement:           DefaultRedactionReplacement,
		},
		Logging: LoggingConfig{
			Level:            "info",
			IncludeRequestID: true,
		},
		DefaultRateLimit: RateLimitPolicy{
			Requests: 100,
			Window:   Duration{Duration: time.Minute},
		},
	}
}
