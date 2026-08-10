// Package redactor implements the privacy redaction engine. It redacts HTTP
// headers (case-insensitive), URL query parameters, nested JSON object fields
// (dotted paths such as "credit_card.number"), and structured log attributes.
package redactor

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
)

// Redactor is an immutable, concurrency-safe redaction engine. The zero value
// redacts nothing.
type Redactor struct {
	headers     map[string]struct{}
	params      map[string]struct{}
	jsonPaths   map[string]struct{}
	replacement string
}

// New constructs a Redactor from the supplied redaction rules. Headers are
// matched case-insensitively; query parameters and JSON field paths are
// matched exactly.
func New(headers, params, jsonFields []string, replacement string) *Redactor {
	if strings.TrimSpace(replacement) == "" {
		replacement = "[REDACTED]"
	}
	r := &Redactor{
		headers:     buildSet(headers, true),
		params:      buildSet(params, false),
		jsonPaths:   buildSet(jsonFields, false),
		replacement: replacement,
	}
	return r
}

// Replacement returns the configured replacement value.
func (r *Redactor) Replacement() string {
	if r == nil {
		return "[REDACTED]"
	}
	return r.replacement
}

// Header reports whether the named HTTP header should be redacted.
func (r *Redactor) Header(name string) bool {
	if r == nil || len(r.headers) == 0 {
		return false
	}
	_, ok := r.headers[strings.ToLower(strings.TrimSpace(name))]
	return ok
}

// QueryParameter reports whether the named URL query parameter should be redacted.
func (r *Redactor) QueryParameter(name string) bool {
	if r == nil || len(r.params) == 0 {
		return false
	}
	_, ok := r.params[name]
	return ok
}

// RedactHeaders returns a copy of headers with redacted values replaced. The
// returned map preserves the original header casing. The original map is not
// modified.
func (r *Redactor) RedactHeaders(headers http.Header) http.Header {
	if r == nil || len(headers) == 0 {
		return cloneHeader(headers)
	}
	out := cloneHeader(headers)
	for name, values := range out {
		if !r.Header(name) {
			continue
		}
		for i := range values {
			values[i] = r.replacement
		}
	}
	return out
}

// RedactQuery returns a query string with redacted parameter values replaced.
// The original url.Values is not modified. Parameter names are preserved.
func (r *Redactor) RedactQuery(values url.Values) string {
	if r == nil {
		return values.Encode()
	}
	clone := make(url.Values, len(values))
	for name, vs := range values {
		if r.QueryParameter(name) {
			redacted := make([]string, len(vs))
			for i := range redacted {
				redacted[i] = r.replacement
			}
			clone[name] = redacted
			continue
		}
		clone[name] = append([]string(nil), vs...)
	}
	return clone.Encode()
}

// RedactJSON redacts any nesting of JSON-decoded values. The dotted field
// paths (e.g. "credit_card.number") select leaf values to replace. Unknown
// fields are left untouched. The input is not mutated; a new value is returned.
func (r *Redactor) RedactJSON(data []byte) ([]byte, error) {
	if r == nil || len(data) == 0 {
		return data, nil
	}

	var decoded any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, err
	}

	redacted := r.redactValue(decoded, "")
	return json.Marshal(redacted)
}

func (r *Redactor) redactValue(value any, prefix string) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, child := range v {
			path := key
			if prefix != "" {
				path = prefix + "." + key
			}
			if r.shouldRedactPath(path) {
				out[key] = r.replacement
			} else {
				out[key] = r.redactValue(child, path)
			}
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = r.redactValue(item, prefix)
		}
		return out
	default:
		return value
	}
}

func (r *Redactor) shouldRedactPath(path string) bool {
	if r == nil || len(r.jsonPaths) == 0 {
		return false
	}
	_, ok := r.jsonPaths[path]
	return ok
}

// RedactAttributes redacts a flat set of structured log attributes. The dotted
// paths are matched against attribute names (e.g. "credit_card.number"). The
// original map is not modified.
func (r *Redactor) RedactAttributes(attrs map[string]any) map[string]any {
	if r == nil {
		return attrs
	}
	out := make(map[string]any, len(attrs))
	for name, value := range attrs {
		if r.shouldRedactPath(name) {
			out[name] = r.replacement
		} else {
			out[name] = value
		}
	}
	return out
}

func cloneHeader(headers http.Header) http.Header {
	if len(headers) == 0 {
		return http.Header{}
	}
	clone := make(http.Header, len(headers))
	for name, values := range headers {
		clone[name] = append([]string(nil), values...)
	}
	return clone
}

func buildSet(values []string, lower bool) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if lower {
			value = strings.ToLower(value)
		}
		if value != "" {
			set[value] = struct{}{}
		}
	}
	return set
}
