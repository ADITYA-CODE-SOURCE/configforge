// Package schema generates the JSON Schema (Draft 2020-12) for ConfigForge
// user configuration from the typed configuration model. Generation is
// deterministic: the same inputs always produce byte-identical output.
package schema

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/ADITYA-CODE-SOURCE/configforge/pkg/config"
)

// Draft is the JSON Schema draft used by ConfigForge.
const Draft = "https://json-schema.org/draft/2020-12/schema"

// Generate produces the JSON Schema document for ConfigForge configuration as
// deterministically ordered JSON. Manifests are accepted to keep the API
// forward-compatible; the schema is currently derived from the typed model,
// with manifest descriptions enriched where available.
func Generate() ([]byte, error) {
	builder := newBuilder()
	if err := buildRoot(builder.docRoot, builder); err != nil {
		return nil, err
	}
	return builder.bytes()
}

type builder struct {
	docRoot map[string]any
}

func newBuilder() *builder {
	return &builder{docRoot: map[string]any{}}
}

func (b *builder) bytes() ([]byte, error) {
	return json.MarshalIndent(b.docRoot, "", "  ")
}

// Build is exported for tests that want access to the schema as a Go value.
func Build() (map[string]any, error) {
	builder := newBuilder()
	if err := buildRoot(builder.docRoot, builder); err != nil {
		return nil, err
	}
	return builder.docRoot, nil
}

func buildRoot(root map[string]any, b *builder) error {
	root["$schema"] = Draft
	root["$id"] = "https://github.com/ADITYA-CODE-SOURCE/configforge/schemas/config.schema.json"
	root["title"] = "ConfigForge Configuration"
	root["description"] = "Declarative feature and policy configuration for ConfigForge."
	root["type"] = "object"
	root["additionalProperties"] = false

	required := []string{"version"}
	properties := map[string]any{
		"version": map[string]any{
			"type":        "string",
			"description": "Configuration schema version. Only v1 is supported.",
			"enum":        []string{config.SupportedVersion},
		},
	}

	featuresSchema := map[string]any{
		"type":                 "object",
		"description":          "Feature flag declarations keyed by unique feature name.",
		"additionalProperties": map[string]any{"$ref": "#/$defs/featureFlag"},
	}
	properties["features"] = featuresSchema
	properties["security"] = buildSecurity()
	properties["privacy"] = buildPrivacy()
	properties["logging"] = buildLogging()
	root["properties"] = properties
	root["required"] = required
	root["$defs"] = buildDefs()
	return nil
}

func buildSecurity() map[string]any {
	return map[string]any{
		"type":                 "object",
		"description":          "HTTP route security policies.",
		"additionalProperties": false,
		"properties": map[string]any{
			"routes": map[string]any{
				"type":        "array",
				"items":       map[string]any{"$ref": "#/$defs/routePolicy"},
				"description": "Route policies evaluated in declaration order.",
			},
		},
	}
}

func buildPrivacy() map[string]any {
	return map[string]any{
		"type":                 "object",
		"description":          "Privacy and redaction rules.",
		"additionalProperties": false,
		"properties": map[string]any{
			"redact_headers":          stringArrayProp("HTTP headers to redact case-insensitively."),
			"redact_query_parameters": stringArrayProp("URL query parameters to redact."),
			"redact_json_fields":      stringArrayProp("Nested JSON fields to redact using dotted paths, e.g. credit_card.number."),
			"replacement": map[string]any{
				"type":        "string",
				"description": "Value used to replace redacted data. Defaults to [REDACTED].",
				"default":     config.DefaultRedactionReplacement,
			},
		},
	}
}

func buildLogging() map[string]any {
	return map[string]any{
		"type":                 "object",
		"description":          "Logging configuration.",
		"additionalProperties": false,
		"properties": map[string]any{
			"level": map[string]any{
				"type":        "string",
				"description": "Log severity.",
				"enum":        []string{"debug", "info", "warn", "error"},
				"default":     "info",
			},
			"include_request_id": map[string]any{
				"type":        "boolean",
				"description": "Whether to attach a request id to each request.",
				"default":     true,
			},
		},
	}
}

func buildDefs() map[string]any {
	return map[string]any{
		"featureFlag": buildFeatureFlag(),
		"routePolicy": buildRoutePolicy(),
		"rateLimit":   buildRateLimit(),
		"conditions":  buildConditions(),
	}
}

func buildFeatureFlag() map[string]any {
	return map[string]any{
		"type":                 "object",
		"description":          "A named feature flag with targeting conditions and rollout.",
		"additionalProperties": false,
		"properties": map[string]any{
			"enabled": map[string]any{
				"type":        "boolean",
				"description": "Whether the feature is enabled at all.",
				"default":     false,
			},
			"rollout_percentage": map[string]any{
				"type":        "integer",
				"description": "Percentage of users that receive the feature, using stable hashing.",
				"minimum":     0,
				"maximum":     100,
				"default":     0,
			},
			"conditions": map[string]any{"$ref": "#/$defs/conditions"},
		},
	}
}

func buildConditions() map[string]any {
	return map[string]any{
		"type":                 "object",
		"description":          "Feature targeting conditions.",
		"additionalProperties": false,
		"properties": map[string]any{
			"countries": stringArrayProp("Countries that receive the feature (ISO codes)."),
			"users":     stringArrayProp("Explicitly targeted user IDs."),
			"roles":     stringArrayProp("Roles that receive the feature."),
		},
	}
}

func buildRoutePolicy() map[string]any {
	return map[string]any{
		"type":                 "object",
		"description":          "Authentication, authorization, and rate-limit policy for an HTTP route.",
		"additionalProperties": false,
		"required":             []string{"name", "path", "methods"},
		"properties": map[string]any{
			"name": map[string]any{
				"type":        "string",
				"description": "Unique route name.",
				"minLength":   1,
			},
			"path": map[string]any{
				"type":        "string",
				"description": "Route path. Supports a single trailing /* wildcard.",
				"pattern":     "^/.*$",
			},
			"methods": map[string]any{
				"type":        "array",
				"description": "HTTP methods allowed by this route.",
				"items": map[string]any{
					"type": "string",
					"enum": []string{"GET", "PUT", "POST", "DELETE", "PATCH", "HEAD", "OPTIONS", "CONNECT", "TRACE"},
				},
				"minItems": 1,
			},
			"require_authentication": map[string]any{
				"type":        "boolean",
				"description": "Whether authentication is required.",
				"default":     false,
			},
			"allowed_roles": stringArrayProp("Roles allowed to access this route when authenticated."),
			"rate_limit":    map[string]any{"$ref": "#/$defs/rateLimit"},
		},
	}
}

func buildRateLimit() map[string]any {
	return map[string]any{
		"type":                 "object",
		"description":          "Fixed-window rate limit policy.",
		"additionalProperties": false,
		"required":             []string{"requests", "window"},
		"properties": map[string]any{
			"requests": map[string]any{
				"type":        "integer",
				"description": "Maximum requests during the window.",
				"minimum":     1,
				"default":     100,
			},
			"window": map[string]any{
				"type":        "string",
				"description": "Go duration string for the window, e.g. 1m.",
			},
		},
	}
}

func stringArrayProp(desc string) map[string]any {
	return map[string]any{
		"type":        "array",
		"description": desc,
		"items":       map[string]any{"type": "string", "minLength": 1},
	}
}

// StableMap returns a sorted, deterministic representation of a map[string]any
// for tests. It recurses into nested maps and slices.
func StableMap(v any) string {
	var b strings.Builder
	writeStable(&b, v)
	return b.String()
}

func writeStable(b *strings.Builder, v any) {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		b.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				b.WriteByte(',')
			}
			fmt.Fprintf(b, "%q:", k)
			writeStable(b, t[k])
		}
		b.WriteByte('}')
	case []any:
		b.WriteByte('[')
		for i, item := range t {
			if i > 0 {
				b.WriteByte(',')
			}
			writeStable(b, item)
		}
		b.WriteByte(']')
	case []string:
		b.WriteByte('[')
		for i, item := range t {
			if i > 0 {
				b.WriteByte(',')
			}
			fmt.Fprintf(b, "%q", item)
		}
		b.WriteByte(']')
	default:
		fmt.Fprintf(b, "%v", v)
	}
}
