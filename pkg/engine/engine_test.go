package engine

import (
	"strings"
	"testing"

	"github.com/ADITYA-CODE-SOURCE/configforge/pkg/config"
	"github.com/ADITYA-CODE-SOURCE/configforge/pkg/feature"
)

func validEngineConfig() config.Config {
	return config.Config{
		Version: config.SupportedVersion,
		Features: map[string]config.FeatureFlag{
			"new_checkout": {
				Enabled:           true,
				RolloutPercentage: 25,
				Conditions: config.FeatureConditions{
					Countries: []string{"IN", "US"},
					Users:     []string{"user-101", "user-205"},
				},
			},
			"disabled_feature": {Enabled: false},
			"all_users":        {Enabled: true},
			"country_only": {
				Enabled:    true,
				Conditions: config.FeatureConditions{Countries: []string{"IN"}},
			},
			"role_only": {
				Enabled:    true,
				Conditions: config.FeatureConditions{Roles: []string{"admin"}},
			},
		},
		Security: config.SecurityConfig{
			Routes: []config.RoutePolicy{
				{
					Name:                  "create-payment",
					Path:                  "/api/payments/*",
					Methods:               []string{"POST"},
					RequireAuthentication: true,
					AllowedRoles:          []string{"admin", "customer"},
					RateLimit:             &config.RateLimitPolicy{Requests: 100, Window: config.MustDuration("1m")},
				},
				{
					Name:                  "public-health",
					Path:                  "/health",
					Methods:               []string{"GET"},
					RequireAuthentication: false,
				},
			},
		},
		Privacy: config.PrivacyConfig{
			RedactHeaders:         []string{"authorization", "cookie"},
			RedactQueryParameters: []string{"token"},
			RedactJSONFields:      []string{"password", "credit_card.number"},
			Replacement:           "[REDACTED]",
		},
		Logging:          config.LoggingConfig{Level: "info", IncludeRequestID: true},
		DefaultRateLimit: config.RateLimitPolicy{Requests: 50, Window: config.MustDuration("1m")},
	}
}

func TestCompileAndFeatureEvaluation(t *testing.T) {
	e := mustCompile(t, validEngineConfig())

	for _, tc := range []struct {
		name    string
		ctx     feature.EvaluationContext
		enabled bool
	}{
		{"new_checkout/explicit-user", feature.EvaluationContext{UserID: "user-101"}, true},
		{"new_checkout/non-targeted-country", feature.EvaluationContext{UserID: "random-99999", Country: "GB"}, false},
		{"new_checkout/no-user-rollout", feature.EvaluationContext{UserID: "", Country: "IN"}, false},
		{"disabled_feature", feature.EvaluationContext{UserID: "user-101"}, false},
		{"all_users", feature.EvaluationContext{UserID: "anyone"}, true},
		{"country_only/match", feature.EvaluationContext{Country: "IN"}, true},
		{"country_only/no-match", feature.EvaluationContext{Country: "US"}, false},
		{"role_only/match", feature.EvaluationContext{Roles: []string{"admin"}}, true},
		{"role_only/no-match", feature.EvaluationContext{Roles: []string{"customer"}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dec := e.EvaluateFeature(featureNameFor(tc.name), tc.ctx)
			if dec.Enabled != tc.enabled {
				t.Fatalf("%s: Enabled = %v, want %v (reason=%s)", tc.name, dec.Enabled, tc.enabled, dec.Reason)
			}
		})
	}
}

func featureNameFor(testName string) string {
	if idx := strings.Index(testName, "/"); idx >= 0 {
		return testName[:idx]
	}
	return testName
}

func TestRolloutIsDeterministic(t *testing.T) {
	e := mustCompile(t, validEngineConfig())
	ctx := feature.EvaluationContext{UserID: "user-9999", Country: "IN"}
	first := e.EvaluateFeature("new_checkout", ctx)
	for i := 0; i < 20; i++ {
		got := e.EvaluateFeature("new_checkout", ctx)
		if got != first {
			t.Fatalf("non-deterministic decision: got %+v, want %+v", got, first)
		}
	}
}

func TestRolloutDistribution(t *testing.T) {
	cfg := validEngineConfig()
	cfg.Features["rollout50"] = config.FeatureFlag{Enabled: true, RolloutPercentage: 50}
	e := mustCompile(t, cfg)

	enabled := 0
	total := 1000
	for i := 0; i < total; i++ {
		ctx := feature.EvaluationContext{UserID: "user-" + itoa(i), Country: "IN"}
		if e.EvaluateFeature("rollout50", ctx).Enabled {
			enabled++
		}
	}
	// Expect roughly 50% with some tolerance. This proves hashing spreads
	// users across buckets; it is deterministic but not exact.
	if enabled < total/4 || enabled > total*3/4 {
		t.Fatalf("rollout distribution = %d/%d, outside tolerated range", enabled, total)
	}
}

func TestUnknownFeatureIsDisabled(t *testing.T) {
	e := mustCompile(t, validEngineConfig())
	dec := e.EvaluateFeature("nope", feature.EvaluationContext{UserID: "u"})
	if dec.Enabled {
		t.Fatalf("unknown feature should be disabled, got %+v", dec)
	}
	if dec.Rule != "unknown" {
		t.Fatalf("rule = %q, want unknown", dec.Rule)
	}
}

func TestRoutePolicyDecisions(t *testing.T) {
	e := mustCompile(t, validEngineConfig())

	for _, tc := range []struct {
		name    string
		req     Request
		allowed bool
		policy  string
	}{
		{"public-health", Request{Method: "GET", Path: "/health"}, true, "public-health"},
		{"payment-unauth", Request{Method: "POST", Path: "/api/payments/create"}, false, "create-payment"},
		{"payment-customer", Request{Method: "POST", Path: "/api/payments/create", Authenticated: true, Roles: []string{"customer"}}, true, "create-payment"},
		{"payment-wrong-role", Request{Method: "POST", Path: "/api/payments/create", Authenticated: true, Roles: []string{"viewer"}}, false, "create-payment"},
		{"payment-wrong-method", Request{Method: "GET", Path: "/api/payments/create", Authenticated: true, Roles: []string{"customer"}}, false, ""},
		{"no-match", Request{Method: "DELETE", Path: "/api/unknown"}, false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dec := e.EvaluateRequest(tc.req)
			if dec.Allowed != tc.allowed {
				t.Fatalf("allowed = %v, want %v (reason=%s)", dec.Allowed, tc.allowed, dec.Reason)
			}
			if dec.MatchedPolicy != tc.policy {
				t.Fatalf("policy = %q, want %q", dec.MatchedPolicy, tc.policy)
			}
		})
	}
}

func TestRateLimitFor(t *testing.T) {
	e := mustCompile(t, validEngineConfig())
	policy, name, ok := e.RateLimitFor("POST", "/api/payments/create")
	if !ok {
		t.Fatal("expected rate limit for payment route")
	}
	if name != "create-payment" {
		t.Fatalf("policy name = %q", name)
	}
	if policy.Requests != 100 {
		t.Fatalf("requests = %d, want 100", policy.Requests)
	}

	def, _, ok := e.RateLimitFor("GET", "/health")
	if !ok {
		t.Fatal("expected default rate limit fallback")
	}
	if def.Requests != 50 {
		t.Fatalf("default requests = %d, want 50", def.Requests)
	}
}

func TestCompileRejectsAmbiguousWildcardOverlap(t *testing.T) {
	cfg := validEngineConfig()
	cfg.Security.Routes = []config.RoutePolicy{
		{Name: "api-all", Path: "/api/*", Methods: []string{"GET"}, RequireAuthentication: false},
		{Name: "api-payments", Path: "/api/payments/*", Methods: []string{"GET"}, RequireAuthentication: false},
	}
	_, err := Compile(cfg)
	if err == nil {
		t.Fatal("Compile succeeded, want ambiguous overlap error")
	}
}

func TestEngineIsImmutable(t *testing.T) {
	cfg := validEngineConfig()
	e := mustCompile(t, cfg)

	// Mutate the original; the engine copy must be unaffected.
	cfg.Features["injected"] = config.FeatureFlag{Enabled: true}
	if _, ok := e.features["injected"]; ok {
		t.Fatal("engine shared feature map with caller")
	}
	out := e.Config()
	if _, ok := out.Features["injected"]; ok {
		t.Fatal("engine.Config leaked caller mutation")
	}

	// Mutate returned config; engine must be unaffected.
	out.Security.Routes[0].Name = "tampered"
	if e.cfg.Security.Routes[0].Name == "tampered" {
		t.Fatal("engine internal state mutated through Config()")
	}
}
