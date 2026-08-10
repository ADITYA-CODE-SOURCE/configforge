// Package matcher contains internal route path and method matching logic.
//
// Route paths may use a single trailing "/*" wildcard. A wildcard route
// "/api/payments/*" matches:
//
//   - exactly "/api/payments"
//   - any path that begins with "/api/payments/"
//
// The wildcard matches any remaining path segments, including additional
// slashes. Only one wildcard is allowed per path and it must be the trailing
// "/*" suffix; this is enforced during validation.
package matcher

import (
	"strings"

	"github.com/ADITYA-CODE-SOURCE/configforge/pkg/config"
)

// Match reports whether the request method and path match the route policy.
//
// Method comparison is case-insensitive and the route's stored methods are
// already normalized to upper case during validation.
func Match(route config.RoutePolicy, method, path string) bool {
	if !methodMatches(route.Methods, method) {
		return false
	}
	return pathMatches(route.Path, path)
}

func methodMatches(methods []string, method string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(method))
	for _, allowed := range methods {
		if allowed == normalized {
			return true
		}
	}
	return false
}

func pathMatches(routePath, requestPath string) bool {
	routePath = strings.TrimSpace(routePath)
	requestPath = strings.TrimSpace(requestPath)

	if !strings.HasSuffix(routePath, "/*") {
		return routePath == requestPath
	}

	prefix := strings.TrimSuffix(routePath, "/*")
	if requestPath == prefix {
		return true
	}
	return strings.HasPrefix(requestPath, prefix+"/")
}

// Find returns the first route policy (in declaration order) that matches the
// given method and path, or false if none match. Declaration order is used so
// that earlier, more specific routes take precedence over later wildcard
// routes. Validation reports ambiguous overlapping routes before compilation.
func Find(routes []config.RoutePolicy, method, path string) (config.RoutePolicy, bool) {
	for _, route := range routes {
		if Match(route, method, path) {
			return route, true
		}
	}
	return config.RoutePolicy{}, false
}
