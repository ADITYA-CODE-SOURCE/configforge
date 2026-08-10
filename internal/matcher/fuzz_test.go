package matcher

import "testing"

func FuzzPathMatch(f *testing.F) {
	f.Add("/api/*", "GET", "/api/payments")
	f.Add("/health", "GET", "/health")
	f.Add("/api/*", "POST", "/products")
	f.Fuzz(func(t *testing.T, routePath, method, requestPath string) {
		route := mustRoute("r", routePath, method)
		Match(route, method, requestPath) // must not panic
	})
}
