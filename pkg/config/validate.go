package config

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
)

var validHTTPMethods = map[string]struct{}{
	http.MethodConnect: {},
	http.MethodDelete:  {},
	http.MethodGet:     {},
	http.MethodHead:    {},
	http.MethodOptions: {},
	http.MethodPatch:   {},
	http.MethodPost:    {},
	http.MethodPut:     {},
	http.MethodTrace:   {},
}

var validLogLevels = map[string]struct{}{
	"debug": {},
	"info":  {},
	"warn":  {},
	"error": {},
}

// Validate checks a fully loaded ConfigForge configuration.
func Validate(cfg Config, filename string, positions map[string]Position) error {
	var errs []FieldError

	if strings.TrimSpace(cfg.Version) == "" {
		errs = appendFieldError(errs, filename, positions, "version", "is required")
	} else if cfg.Version != SupportedVersion {
		errs = appendFieldError(errs, filename, positions, "version", fmt.Sprintf("must be %q", SupportedVersion))
	}

	errs = validateFeatures(cfg, filename, positions, errs)
	errs = validateSecurity(cfg, filename, positions, errs)
	errs = validatePrivacy(cfg, filename, positions, errs)
	errs = validateLogging(cfg, filename, positions, errs)
	errs = validateDefaultRateLimit(cfg, filename, positions, errs)

	if len(errs) > 0 {
		return &ValidationError{Errors: errs}
	}
	return nil
}

func validateFeatures(cfg Config, filename string, positions map[string]Position, errs []FieldError) []FieldError {
	for name, flag := range cfg.Features {
		featurePath := "features." + name
		if strings.TrimSpace(name) == "" {
			errs = appendFieldError(errs, filename, positions, featurePath, "name must be non-empty")
		}
		if flag.RolloutPercentage < 0 || flag.RolloutPercentage > 100 {
			errs = appendFieldError(errs, filename, positions, featurePath+".rollout_percentage", "must be between 0 and 100")
		}

		errs = validateStringList(filename, positions, featurePath+".conditions.countries", flag.Conditions.Countries, false, errs)
		errs = validateStringList(filename, positions, featurePath+".conditions.users", flag.Conditions.Users, false, errs)
		errs = validateStringList(filename, positions, featurePath+".conditions.roles", flag.Conditions.Roles, false, errs)
	}
	return errs
}

func validateSecurity(cfg Config, filename string, positions map[string]Position, errs []FieldError) []FieldError {
	routeNames := map[string]int{}
	routeMatchers := map[string]string{}
	for i, route := range cfg.Security.Routes {
		routePath := fmt.Sprintf("security.routes[%d]", i)
		trimmedName := strings.TrimSpace(route.Name)
		if trimmedName == "" {
			errs = appendFieldError(errs, filename, positions, routePath+".name", "must be non-empty")
		} else if first, ok := routeNames[trimmedName]; ok {
			errs = appendFieldError(errs, filename, positions, routePath+".name", fmt.Sprintf("duplicates security.routes[%d].name", first))
		} else {
			routeNames[trimmedName] = i
		}

		if !strings.HasPrefix(route.Path, "/") {
			errs = appendFieldError(errs, filename, positions, routePath+".path", "must start with /")
		}
		if strings.Count(route.Path, "*") > 1 || (strings.Contains(route.Path, "*") && !strings.HasSuffix(route.Path, "/*")) {
			errs = appendFieldError(errs, filename, positions, routePath+".path", "wildcards are only supported as a trailing /* suffix")
		}

		if len(route.Methods) == 0 {
			errs = appendFieldError(errs, filename, positions, routePath+".methods", "must contain at least one HTTP method")
		}
		methodSet := map[string]struct{}{}
		for j, method := range route.Methods {
			methodPath := fmt.Sprintf("%s.methods[%d]", routePath, j)
			normalized := strings.ToUpper(strings.TrimSpace(method))
			if normalized == "" {
				errs = appendFieldError(errs, filename, positions, methodPath, "must be non-empty")
				continue
			}
			if _, ok := validHTTPMethods[normalized]; !ok {
				errs = appendFieldError(errs, filename, positions, methodPath, "must be a valid HTTP method")
			}
			if _, ok := methodSet[normalized]; ok {
				errs = appendFieldError(errs, filename, positions, methodPath, "duplicates another method in the same route")
			}
			methodSet[normalized] = struct{}{}

			matchKey := normalized + " " + route.Path
			if firstRoute, ok := routeMatchers[matchKey]; ok {
				errs = appendFieldError(errs, filename, positions, routePath+".path", fmt.Sprintf("conflicts with route %q for %s", firstRoute, matchKey))
			} else if trimmedName != "" {
				routeMatchers[matchKey] = trimmedName
			}
		}

		errs = validateStringList(filename, positions, routePath+".allowed_roles", route.AllowedRoles, false, errs)
		if route.RequireAuthentication && len(route.AllowedRoles) == 0 {
			errs = appendFieldError(errs, filename, positions, routePath+".allowed_roles", "must contain at least one role when authentication is required")
		}

		if route.RateLimit != nil {
			errs = validateRateLimit(*route.RateLimit, filename, positions, routePath+".rate_limit", errs)
		}
	}
	return errs
}

func validatePrivacy(cfg Config, filename string, positions map[string]Position, errs []FieldError) []FieldError {
	errs = validateStringList(filename, positions, "privacy.redact_headers", cfg.Privacy.RedactHeaders, true, errs)
	errs = validateStringList(filename, positions, "privacy.redact_query_parameters", cfg.Privacy.RedactQueryParameters, true, errs)
	errs = validateStringList(filename, positions, "privacy.redact_json_fields", cfg.Privacy.RedactJSONFields, false, errs)
	if strings.TrimSpace(cfg.Privacy.Replacement) == "" {
		errs = appendFieldError(errs, filename, positions, "privacy.replacement", "must be non-empty")
	}
	return errs
}

func validateLogging(cfg Config, filename string, positions map[string]Position, errs []FieldError) []FieldError {
	if err := validateLogLevel(cfg.Logging.Level); err != nil {
		errs = appendFieldError(errs, filename, positions, "logging.level", err.Error())
	}
	return errs
}

func validateDefaultRateLimit(cfg Config, filename string, positions map[string]Position, errs []FieldError) []FieldError {
	if cfg.DefaultRateLimit.Requests <= 0 {
		errs = appendFieldError(errs, filename, positions, "default_rate_limit.requests", "must be greater than zero")
	}
	if cfg.DefaultRateLimit.Window.Duration <= 0 {
		errs = appendFieldError(errs, filename, positions, "default_rate_limit.window", "must be a positive Go duration")
	}
	return errs
}

func validateRateLimit(policy RateLimitPolicy, filename string, positions map[string]Position, path string, errs []FieldError) []FieldError {
	if policy.Requests <= 0 {
		errs = appendFieldError(errs, filename, positions, path+".requests", "must be greater than zero")
	}
	if policy.Window.Duration <= 0 {
		errs = appendFieldError(errs, filename, positions, path+".window", "must be a positive Go duration")
	}
	return errs
}

func validateStringList(filename string, positions map[string]Position, path string, values []string, caseInsensitive bool, errs []FieldError) []FieldError {
	seen := map[string]int{}
	for i, value := range values {
		itemPath := fmt.Sprintf("%s[%d]", path, i)
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			errs = appendFieldError(errs, filename, positions, itemPath, "must be non-empty")
			continue
		}
		key := trimmed
		if caseInsensitive {
			key = strings.ToLower(trimmed)
		}
		if first, ok := seen[key]; ok {
			errs = appendFieldError(errs, filename, positions, itemPath, fmt.Sprintf("duplicates %s[%d]", path, first))
		}
		seen[key] = i
	}
	return errs
}

func validateLogLevel(level string) error {
	if _, ok := validLogLevels[strings.ToLower(strings.TrimSpace(level))]; !ok {
		return fmt.Errorf("must be one of debug, info, warn, error")
	}
	return nil
}

func normalize(cfg *Config) {
	cfg.Logging.Level = strings.ToLower(strings.TrimSpace(cfg.Logging.Level))
	cfg.Privacy.Replacement = strings.TrimSpace(cfg.Privacy.Replacement)
	cfg.Privacy.RedactHeaders = normalizeOrKeep(cfg.Privacy.RedactHeaders, true)
	cfg.Privacy.RedactQueryParameters = normalizeOrKeep(cfg.Privacy.RedactQueryParameters, true)
	cfg.Privacy.RedactJSONFields = normalizeOrKeep(cfg.Privacy.RedactJSONFields, false)

	for name, flag := range cfg.Features {
		flag.Conditions.Countries = normalizeOrKeep(flag.Conditions.Countries, false)
		flag.Conditions.Users = normalizeOrKeep(flag.Conditions.Users, false)
		flag.Conditions.Roles = normalizeOrKeep(flag.Conditions.Roles, false)
		cfg.Features[name] = flag
	}

	for i := range cfg.Security.Routes {
		route := &cfg.Security.Routes[i]
		route.Name = strings.TrimSpace(route.Name)
		route.Path = strings.TrimSpace(route.Path)
		for j := range route.Methods {
			route.Methods[j] = strings.ToUpper(strings.TrimSpace(route.Methods[j]))
		}
		route.AllowedRoles = normalizeOrKeep(route.AllowedRoles, false)
	}
}

func normalizeOrKeep(values []string, lower bool) []string {
	if len(values) == 0 {
		return values
	}

	result := make([]string, len(values))
	copy(result, values)
	for i, value := range result {
		value = strings.TrimSpace(value)
		if lower {
			value = strings.ToLower(value)
		}
		result[i] = value
	}
	return result
}

func normalizeUniqueList(values []string, path string, caseInsensitive bool) ([]string, error) {
	normalized := normalizeOrKeep(values, caseInsensitive)
	seen := map[string]struct{}{}
	for i, value := range normalized {
		if value == "" {
			return nil, fmt.Errorf("%s[%d] must be non-empty", path, i)
		}
		key := value
		if caseInsensitive {
			key = strings.ToLower(value)
		}
		if _, ok := seen[key]; ok {
			return nil, fmt.Errorf("%s[%d] duplicates another value", path, i)
		}
		seen[key] = struct{}{}
	}
	sort.Strings(normalized)
	return normalized, nil
}
