package matcher

import (
	"testing"

	"github.com/ADITYA-CODE-SOURCE/configforge/pkg/config"
)

func mustRoute(name, path string, methods ...string) config.RoutePolicy {
	return config.RoutePolicy{Name: name, Path: path, Methods: methods}
}

func TestPathMatching(t *testing.T) {
	for _, tc := range []struct {
		name         string
		route        string
		routeMethods []string
		method       string
		request      string
		match        bool
	}{
		{"exact", "/health", []string{"GET"}, "GET", "/health", true},
		{"exact-mismatch", "/health", []string{"GET"}, "GET", "/healthz", false},
		{"wildcard-exact-prefix", "/api/payments/*", []string{"POST"}, "POST", "/api/payments", true},
		{"wildcard-child", "/api/payments/*", []string{"POST"}, "POST", "/api/payments/create", true},
		{"wildcard-nested-child", "/api/payments/*", []string{"POST"}, "POST", "/api/payments/v1/create", true},
		{"wildcard-wrong-prefix", "/api/payments/*", []string{"POST"}, "POST", "/api/products", false},
		{"method-mismatch", "/health", []string{"GET"}, "POST", "/health", false},
		{"method-case-insensitive", "/health", []string{"GET"}, "get", "/health", true},
		{"trailing-slash", "/api", []string{"GET"}, "GET", "/api/", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			route := config.RoutePolicy{Name: "r", Path: tc.route, Methods: tc.routeMethods}
			if got := Match(route, tc.method, tc.request); got != tc.match {
				t.Fatalf("Match = %v, want %v", got, tc.match)
			}
		})
	}
}

func TestFindFirstWins(t *testing.T) {
	routes := []config.RoutePolicy{
		mustRoute("exact", "/api/products", "GET"),
		mustRoute("wildcard", "/api/*", "GET"),
	}
	route, ok := Find(routes, "GET", "/api/products")
	if !ok || route.Name != "exact" {
		t.Fatalf("expected exact route to win, got %+v ok=%v", route, ok)
	}
	route, ok = Find(routes, "GET", "/api/orders")
	if !ok || route.Name != "wildcard" {
		t.Fatalf("expected wildcard route, got %+v ok=%v", route, ok)
	}
}

func TestDetectOverlaps(t *testing.T) {
	routes := []config.RoutePolicy{
		mustRoute("api-all", "/api/*", "GET"),
		mustRoute("api-payments", "/api/payments/*", "GET"),
	}
	overlaps := DetectOverlaps(routes)
	if len(overlaps) != 1 {
		t.Fatalf("overlaps = %d, want 1", len(overlaps))
	}
	if overlaps[0].First.Name != "api-all" {
		t.Fatalf("first overlap = %q, want api-all", overlaps[0].First.Name)
	}

	// Non-overlapping routes should not report an overlap.
	routes = []config.RoutePolicy{
		mustRoute("payments", "/api/payments/*", "GET"),
		mustRoute("products", "/api/products/*", "GET"),
	}
	if overlaps := DetectOverlaps(routes); len(overlaps) != 0 {
		t.Fatalf("expected no overlaps, got %d", len(overlaps))
	}

	// Exact route under a wildcard does not produce a shadow/false overlap.
	routes = []config.RoutePolicy{
		mustRoute("api-all", "/api/*", "GET"),
		mustRoute("exact-products", "/api/products", "GET"),
	}
	if overlaps := DetectOverlaps(routes); len(overlaps) != 0 {
		t.Fatalf("exact under wildcard should not overlap, got %d", len(overlaps))
	}
}
