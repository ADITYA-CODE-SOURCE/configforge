// Package policy contains the public route-policy decision types returned by
// the route-policy engine.
package policy

// Decision reports whether an HTTP request is allowed by the configured
// route policies.
type Decision struct {
	// Allowed reports whether the request satisfies the matched policy.
	Allowed bool
	// Reason describes why the decision was made.
	Reason string
	// MatchedPolicy names the route policy that produced the decision.
	MatchedPolicy string
}
