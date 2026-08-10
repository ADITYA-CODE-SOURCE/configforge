package matcher

import (
	"fmt"
	"strings"

	"github.com/ADITYA-CODE-SOURCE/configforge/pkg/config"
)

// Overlap reports an ambiguous overlap between two route policies.
type Overlap struct {
	First  config.RoutePolicy
	Second config.RoutePolicy
	Reason string
}

// DetectOverlaps returns ambiguous overlapping route pairs. Two routes
// overlap ambiguously when one is a wildcard route whose prefix is also matched
// by another wildcard route with the same method, because both could match the
// same request and it becomes unclear which policy applies.
//
// Exact routes do not overlap with wildcard routes whose prefix could also
// match them, because exact routes always take precedence by declaration order
// during matching. Overlapping exact routes with the same method and path are
// already reported by the existing conflict check during validation.
//
// This function only reports genuinely ambiguous wildcard-vs-wildcard
// overlaps for routes sharing at least one HTTP method.
func DetectOverlaps(routes []config.RoutePolicy) []Overlap {
	var overlaps []Overlap
	for i := 0; i < len(routes); i++ {
		for j := i + 1; j < len(routes); j++ {
			a := routes[i]
			b := routes[j]
			if !shareMethod(a.Methods, b.Methods) {
				continue
			}
			if reason, ok := wildcardOverlap(a.Path, b.Path); ok {
				overlaps = append(overlaps, Overlap{
					First:  a,
					Second: b,
					Reason: reason,
				})
			}
		}
	}
	return overlaps
}

func shareMethod(a, b []string) bool {
	for _, x := range a {
		for _, y := range b {
			if x == y {
				return true
			}
		}
	}
	return false
}

func wildcardOverlap(p1, p2 string) (string, bool) {
	w1 := strings.HasSuffix(p1, "/*")
	w2 := strings.HasSuffix(p2, "/*")
	if !w1 || !w2 {
		return "", false
	}

	prefix1 := strings.TrimSuffix(p1, "/*")
	prefix2 := strings.TrimSuffix(p2, "/*")

	// Same wildcard prefix is already reported by the method+path conflict
	// check; skip it here to avoid duplicate errors.
	if prefix1 == prefix2 {
		return "", false
	}

	// A nested wildcard under another wildcard causes ambiguity, e.g.
	// "/api/*" and "/api/payments/*".
	if strings.HasPrefix(prefix2, prefix1+"/") {
		return fmt.Sprintf("wildcard route %q is shadowed by narrower %q", p1, p2), true
	}
	if strings.HasPrefix(prefix1, prefix2+"/") {
		return fmt.Sprintf("wildcard route %q is shadowed by narrower %q", p2, p1), true
	}
	return "", false
}
