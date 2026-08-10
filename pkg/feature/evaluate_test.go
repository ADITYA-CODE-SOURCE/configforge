package feature

import "testing"

func TestEvaluateRules(t *testing.T) {
	for _, tc := range []struct {
		name    string
		def     Def
		ctx     EvaluationContext
		enabled bool
		rule    string
	}{
		{"disabled", Def{Enabled: false}, EvaluationContext{UserID: "u"}, false, "disabled"},
		{"no-conditions-enabled", Def{Enabled: true}, EvaluationContext{}, true, "enabled"},
		{"explicit-user", Def{Enabled: true, Users: []string{"u1"}}, EvaluationContext{UserID: "u1"}, true, "explicit-user"},
		{"country-no-match", Def{Enabled: true, Countries: []string{"IN"}}, EvaluationContext{UserID: "u", Country: "US"}, false, "country"},
		{"role-no-match", Def{Enabled: true, Roles: []string{"admin"}}, EvaluationContext{UserID: "u", Roles: []string{"viewer"}}, false, "role"},
		{"rollout-100", Def{Enabled: true, RolloutPercentage: 100}, EvaluationContext{UserID: "u"}, true, "rollout"},
		{"rollout-no-user", Def{Enabled: true, RolloutPercentage: 50}, EvaluationContext{}, false, "rollout"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dec := Evaluate("feat", tc.def, tc.ctx)
			if dec.Enabled != tc.enabled {
				t.Fatalf("Enabled = %v, want %v (%s)", dec.Enabled, tc.enabled, dec.Reason)
			}
			if dec.Rule != tc.rule {
				t.Fatalf("Rule = %q, want %q", dec.Rule, tc.rule)
			}
		})
	}
}

func TestRolloutStable(t *testing.T) {
	// A user inside the bucket stays inside; a user outside stays outside.
	// Pick a feature/user and verify the decision is stable and consistent
	// across repeated evaluations.
	def := Def{Enabled: true, RolloutPercentage: 50}
	ctx := EvaluationContext{UserID: "stable-user", Country: "US"}
	first := Evaluate("f", def, ctx)
	for i := 0; i < 50; i++ {
		if got := Evaluate("f", def, ctx); got != first {
			t.Fatalf("non-stable: %+v != %+v", got, first)
		}
	}
}

func TestNewDefCopies(t *testing.T) {
	src := []string{"a", "b"}
	def := NewDef(true, 10, src, src, src)
	src[0] = "mutated"
	if def.Countries[0] == "mutated" {
		t.Fatal("NewDef did not copy input slice")
	}
}
