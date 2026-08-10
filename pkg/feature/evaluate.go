package feature

import (
	"hash/fnv"
	"strings"
)

// Evaluate applies the configured feature flag against the evaluation context
// and returns a deterministic decision. The flag is the compiled representation
// produced by the engine; userID and country are taken from the context.
//
// Evaluation rules, applied in order:
//
//  1. A disabled feature is never enabled.
//  2. An explicitly targeted user is always enabled.
//  3. If countries are configured and the context country is not among them,
//     the feature is disabled.
//  4. If roles are configured and the context roles do not include any of
//     them, the feature is disabled.
//  5. Percentage rollout uses a stable hash of (feature name, user id); the
//     same user always receives the same decision and no randomness is used.
//  6. A feature with no conditions and zero rollout percentage is enabled only
//     when Enabled is true.
func Evaluate(name string, flag Def, ctx EvaluationContext) Decision {
	if !flag.Enabled {
		return Decision{Enabled: false, Reason: "feature is disabled", Rule: "disabled"}
	}

	if contains(flag.Users, ctx.UserID) {
		return Decision{Enabled: true, Reason: "user is explicitly targeted", Rule: "explicit-user"}
	}

	if len(flag.Countries) > 0 && !contains(flag.Countries, ctx.Country) {
		return Decision{Enabled: false, Reason: "country is not targeted", Rule: "country"}
	}

	if len(flag.Roles) > 0 && !rolesIntersect(flag.Roles, ctx.Roles) {
		return Decision{Enabled: false, Reason: "role is not targeted", Rule: "role"}
	}

	if flag.RolloutPercentage <= 0 {
		return Decision{Enabled: true, Reason: "feature is enabled for all matching contexts", Rule: "enabled"}
	}
	if flag.RolloutPercentage >= 100 {
		return Decision{Enabled: true, Reason: "rollout percentage is 100", Rule: "rollout"}
	}

	if ctx.UserID == "" {
		return Decision{Enabled: false, Reason: "user id required for percentage rollout", Rule: "rollout"}
	}

	bucket := rolloutBucket(name, ctx.UserID)
	if bucket < uint32(flag.RolloutPercentage) {
		return Decision{Enabled: true, Reason: "user is within rollout percentage", Rule: "rollout"}
	}
	return Decision{Enabled: false, Reason: "user is outside rollout percentage", Rule: "rollout"}
}

// Def is the compiled, immutable representation of a feature flag. It is
// populated by the engine from the validated configuration. The fields are
// exported so the engine can construct it, but a Def value should be treated
// as immutable after construction.
type Def struct {
	Enabled           bool
	RolloutPercentage int
	Countries         []string
	Users             []string
	Roles             []string
}

// NewDef constructs an immutable compiled feature-flag definition from the
// values provided by the engine.
func NewDef(enabled bool, rolloutPercentage int, countries, users, roles []string) Def {
	return Def{
		Enabled:           enabled,
		RolloutPercentage: rolloutPercentage,
		Countries:         cloneStrings(countries),
		Users:             cloneStrings(users),
		Roles:             cloneStrings(roles),
	}
}

func cloneStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, len(values))
	copy(out, values)
	return out
}

func contains(values []string, target string) bool {
	target = strings.TrimSpace(target)
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func rolesIntersect(targeted, provided []string) bool {
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
	for _, role := range targeted {
		if _, ok := set[role]; ok {
			return true
		}
	}
	return false
}

// rolloutBucket returns a stable bucket in [0, 100) derived from the
// feature name and user id. It uses FNV-1a hashing and is deterministic:
// the same (name, userID) always produces the same bucket, with no
// randomness involved.
func rolloutBucket(name, userID string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(name))
	h.Write([]byte{0})
	h.Write([]byte(userID))
	return h.Sum32() % 100
}
