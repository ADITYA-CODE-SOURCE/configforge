// Package feature contains the public types used for feature-flag evaluation.
package feature

// EvaluationContext is the input used by the feature-flag engine to make a
// deterministic decision for a request.
type EvaluationContext struct {
	UserID  string
	Country string
	Roles   []string
}

// Decision reports the enabled state of a feature for a given EvaluationContext.
type Decision struct {
	// Enabled reports whether the feature is on for this context.
	Enabled bool
	// Reason describes why the decision was made.
	Reason string
	// Rule names the rule that produced the decision.
	Rule string
}
