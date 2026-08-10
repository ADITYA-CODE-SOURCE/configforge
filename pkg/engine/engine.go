// Package engine compiles validated ConfigForge configuration into immutable
// runtime policies and evaluates feature flags and route policies.
package engine

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/ADITYA-CODE-SOURCE/configforge/internal/matcher"
	"github.com/ADITYA-CODE-SOURCE/configforge/pkg/config"
	"github.com/ADITYA-CODE-SOURCE/configforge/pkg/feature"
	"github.com/ADITYA-CODE-SOURCE/configforge/pkg/policy"
)

// Engine is an immutable, compiled view of a ConfigForge configuration. After
// compilation an Engine value is safe for concurrent use and never mutates the
// underlying configuration.
type Engine struct {
	cfg              config.Config
	features         map[string]feature.Def
	redactHeaders    map[string]struct{}
	redactParams     map[string]struct{}
	redactJSONFields []string
	replacement      string
}

// Compile validates and compiles the configuration into an immutable Engine.
// The returned Engine holds its own copies of all configuration data so later
// mutations to the supplied Config do not affect the Engine.
func Compile(cfg config.Config) (*Engine, error) {
	if err := config.Validate(cfg, "", nil); err != nil {
		return nil, fmt.Errorf("compile configforge configuration: %w", err)
	}

	overlaps := matcher.DetectOverlaps(cfg.Security.Routes)
	if len(overlaps) > 0 {
		var msgs []string
		for _, o := range overlaps {
			msgs = append(msgs, fmt.Sprintf("route %q: %s", o.First.Name, o.Reason))
		}
		return nil, fmt.Errorf("ambiguous route overlap: %s", strings.Join(msgs, "; "))
	}

	e := &Engine{
		cfg:              copyConfig(cfg),
		features:         make(map[string]feature.Def, len(cfg.Features)),
		redactHeaders:    buildSet(cfg.Privacy.RedactHeaders, true),
		redactParams:     buildSet(cfg.Privacy.RedactQueryParameters, false),
		redactJSONFields: append([]string(nil), cfg.Privacy.RedactJSONFields...),
	}
	if cfg.Privacy.Replacement != "" {
		e.replacement = cfg.Privacy.Replacement
	} else {
		e.replacement = config.DefaultRedactionReplacement
	}

	for name, flag := range cfg.Features {
		e.features[name] = feature.NewDef(
			flag.Enabled,
			flag.RolloutPercentage,
			flag.Conditions.Countries,
			flag.Conditions.Users,
			flag.Conditions.Roles,
		)
	}

	return e, nil
}

// EvaluateFeature returns a deterministic decision for the named feature and
// the supplied evaluation context. An unknown feature name is reported as
// disabled with a clear reason.
func (e *Engine) EvaluateFeature(name string, ctx feature.EvaluationContext) feature.Decision {
	def, ok := e.features[name]
	if !ok {
		return feature.Decision{Enabled: false, Reason: "feature " + name + " is not configured", Rule: "unknown"}
	}
	return feature.Evaluate(name, def, ctx)
}

// EvaluateRequest returns a policy decision for an HTTP request described by
// its method, path, authentication state, and roles.
//
// Matching and authorization rules:
//
//   - Routes are matched in declaration order; the first matching route wins.
//   - If no route matches, the request is allowed only when no protected route
//     could have applied; the default is deny with "no matching policy".
//   - A route that does not require authentication allows anonymous access.
//   - A route that requires authentication denies unauthenticated requests.
//   - If roles are configured on the route, the request must hold at least one
//     of them; otherwise it is denied.
func (e *Engine) EvaluateRequest(req Request) policy.Decision {
	route, ok := matcher.Find(e.cfg.Security.Routes, req.Method, req.Path)
	if !ok {
		return policy.Decision{Allowed: false, Reason: "no matching policy", MatchedPolicy: ""}
	}

	if route.RequireAuthentication && !req.Authenticated {
		return policy.Decision{Allowed: false, Reason: "authentication required", MatchedPolicy: route.Name}
	}

	if len(route.AllowedRoles) > 0 && !rolesIntersect(route.AllowedRoles, req.Roles) {
		return policy.Decision{Allowed: false, Reason: "role not allowed", MatchedPolicy: route.Name}
	}

	return policy.Decision{Allowed: true, Reason: "request allowed by route policy", MatchedPolicy: route.Name}
}

// Request describes an HTTP request for route-policy evaluation.
type Request struct {
	Method        string
	Path          string
	Authenticated bool
	Roles         []string
	ClientIP      string
	UserID        string
}

// RateLimitFor returns the rate-limit policy that applies to a matched route,
// or the default policy when the route has no explicit rate limit. ok is false
// when no route matches the request.
func (e *Engine) RateLimitFor(method, path string) (config.RateLimitPolicy, string, bool) {
	route, ok := matcher.Find(e.cfg.Security.Routes, method, path)
	if !ok {
		return config.RateLimitPolicy{}, "", false
	}
	if route.RateLimit != nil {
		return *route.RateLimit, route.Name, true
	}
	return e.cfg.DefaultRateLimit, route.Name, true
}

// FeatureNames returns the configured feature names in sorted order.
func (e *Engine) FeatureNames() []string {
	names := make([]string, 0, len(e.features))
	for name := range e.features {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// AllFeatureNames returns the configured feature names in sorted order.
func (e *Engine) AllFeatureNames() []string {
	return e.FeatureNames()
}

// Config returns a defensive copy of the compiled configuration. Callers must
// not mutate the returned value to preserve runtime immutability guarantees.
func (e *Engine) Config() config.Config {
	return copyConfig(e.cfg)
}

// ErrUnknownFeature is returned by helpers when a feature name is not known.
var ErrUnknownFeature = errors.New("unknown feature")

func rolesIntersect(allowed, provided []string) bool {
	if len(allowed) == 0 {
		return true
	}
	if len(provided) == 0 {
		return false
	}
	set := make(map[string]struct{}, len(provided))
	for _, role := range provided {
		role = strings.TrimSpace(role)
		if role != "" {
			set[role] = struct{}{}
		}
	}
	for _, role := range allowed {
		if _, ok := set[role]; ok {
			return true
		}
	}
	return false
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

func copyConfig(cfg config.Config) config.Config {
	cfg.Features = make(map[string]config.FeatureFlag, len(cfg.Features))
	for name, flag := range cfg.Features {
		flag.Conditions.Countries = append([]string(nil), flag.Conditions.Countries...)
		flag.Conditions.Users = append([]string(nil), flag.Conditions.Users...)
		flag.Conditions.Roles = append([]string(nil), flag.Conditions.Roles...)
		cfg.Features[name] = flag
	}

	routes := make([]config.RoutePolicy, len(cfg.Security.Routes))
	for i, route := range cfg.Security.Routes {
		route.Methods = append([]string(nil), route.Methods...)
		route.AllowedRoles = append([]string(nil), route.AllowedRoles...)
		if route.RateLimit != nil {
			rl := *route.RateLimit
			route.RateLimit = &rl
		}
		routes[i] = route
	}
	cfg.Security.Routes = routes

	cfg.Privacy.RedactHeaders = append([]string(nil), cfg.Privacy.RedactHeaders...)
	cfg.Privacy.RedactQueryParameters = append([]string(nil), cfg.Privacy.RedactQueryParameters...)
	cfg.Privacy.RedactJSONFields = append([]string(nil), cfg.Privacy.RedactJSONFields...)
	return cfg
}
