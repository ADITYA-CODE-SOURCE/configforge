package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ADITYA-CODE-SOURCE/configforge/pkg/config"
	"github.com/ADITYA-CODE-SOURCE/configforge/pkg/engine"
	"github.com/ADITYA-CODE-SOURCE/configforge/pkg/middleware"
)

func TestExampleAPIRoutes(t *testing.T) {
	cfg, err := config.LoadFile("../../examples/configs/default.yaml")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	runtime, err := engine.Compile(*cfg)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	store := middleware.NewMemoryStorage(time.Minute)
	defer store.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/api/products", productsHandler(runtime))
	mux.HandleFunc("/api/payments/create", paymentsHandler)
	mux.HandleFunc("/api/admin/reports", adminReportsHandler)

	handler := middleware.RequestID(runtime)(mux)
	handler = middleware.Redaction(runtime)(handler)
	handler = middleware.RateLimit(runtime, store, nil)(handler)
	handler = middleware.Security(runtime, nil)(handler)

	ts := httptest.NewServer(handler)
	defer ts.Close()

	for _, tc := range []struct {
		name       string
		method     string
		path       string
		headers    map[string]string
		body       string
		wantStatus int
	}{
		{"public-health", "GET", "/health", nil, "", 200},
		{"products-no-auth", "GET", "/api/products", nil, "", 200},
		{"payment-unauth", "POST", "/api/payments/create", nil, `{"amount":100}`, 401},
		{"payment-customer", "POST", "/api/payments/create", map[string]string{
			"X-User-ID": "u1", "X-Roles": "customer",
		}, `{"amount":100}`, 201},
		{"payment-wrong-role", "POST", "/api/payments/create", map[string]string{
			"X-User-ID": "u2", "X-Roles": "viewer",
		}, `{"amount":100}`, 403},
		{"admin-unauth", "GET", "/api/admin/reports", nil, "", 401},
		{"admin-customer", "GET", "/api/admin/reports", map[string]string{
			"X-User-ID": "u3", "X-Roles": "customer",
		}, "", 403},
		{"admin-admin", "GET", "/api/admin/reports", map[string]string{
			"X-User-ID": "admin1", "X-Roles": "admin",
		}, "", 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var body io.Reader
			if tc.body != "" {
				body = stringReader(tc.body)
			}
			req, err := http.NewRequest(tc.method, ts.URL+tc.path, body)
			if err != nil {
				t.Fatalf("request: %v", err)
			}
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("do: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tc.wantStatus {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tc.wantStatus)
			}

			if resp.StatusCode < 400 {
				if resp.Header.Get("X-Request-ID") == "" {
					t.Fatal("missing X-Request-ID header on successful response")
				}
			}
		})
	}
}

func TestExampleAPIRedaction(t *testing.T) {
	cfg, err := config.LoadFile("../../examples/configs/default.yaml")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	runtime, err := engine.Compile(*cfg)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	store := middleware.NewMemoryStorage(time.Minute)
	defer store.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/products", productsHandler(runtime))

	handler := middleware.RequestID(runtime)(mux)
	handler = middleware.Redaction(runtime)(handler)
	handler = middleware.RateLimit(runtime, store, nil)(handler)
	handler = middleware.Security(runtime, nil)(handler)

	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/api/products?token=secret123&country=IN", nil)
	req.Header.Set("Authorization", "Bearer super-secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()

	// The response should succeed (public route in default.yaml config).
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result["new_checkout_enabled"] == nil {
		t.Fatal("missing new_checkout in response")
	}
}

func TestExampleAPIRateLimit(t *testing.T) {
	cfg := testConfigLowLimit()
	runtime, err := engine.Compile(cfg)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	store := middleware.NewMemoryStorage(time.Minute)
	defer store.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/payments/create", paymentsHandler)

	handler := middleware.RequestID(runtime)(mux)
	handler = middleware.Redaction(runtime)(handler)
	handler = middleware.RateLimit(runtime, store, nil)(handler)
	handler = middleware.Security(runtime, nil)(handler)

	ts := httptest.NewServer(handler)
	defer ts.Close()

	allowed := 0
	denied := 0
	for i := 0; i < 5; i++ {
		req, _ := http.NewRequest("POST", ts.URL+"/api/payments/create", stringReader(`{"x":1}`))
		req.Header.Set("X-User-ID", "rate-test-user")
		req.Header.Set("X-Roles", "customer")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("do: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode == 201 {
			allowed++
		} else if resp.StatusCode == 429 {
			denied++
		}
	}
	if allowed != 2 {
		t.Fatalf("allowed = %d, want 2", allowed)
	}
	if denied != 3 {
		t.Fatalf("denied = %d, want 3", denied)
	}
}

func testConfigLowLimit() config.Config {
	cfg, _ := config.Load([]byte(`
version: v1
security:
  routes:
    - name: create-payment
      path: /api/payments/*
      methods:
        - POST
      require_authentication: true
      allowed_roles:
        - customer
      rate_limit:
        requests: 2
        window: 1m
`))
	return *cfg
}

func stringReader(s string) io.Reader {
	return strings.NewReader(s)
}
