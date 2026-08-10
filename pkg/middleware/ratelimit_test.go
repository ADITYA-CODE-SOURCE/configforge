package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ADITYA-CODE-SOURCE/configforge/pkg/config"
	"github.com/ADITYA-CODE-SOURCE/configforge/pkg/engine"
)

func TestMemoryStorageRateLimitUnderLimit(t *testing.T) {
	storage := NewMemoryStorage(0)
	defer storage.Close()

	var allowed int32
	for i := 0; i < 10; i++ {
		res, err := storage.Hit(context.Background(), "k", 5, time.Minute, time.Now())
		if err != nil {
			t.Fatalf("Hit error: %v", err)
		}
		if res.Allowed {
			atomic.AddInt32(&allowed, 1)
		}
	}
	if allowed != 5 {
		t.Fatalf("allowed = %d, want 5", allowed)
	}
}

func TestMemoryStorageResetAcrossWindows(t *testing.T) {
	storage := NewMemoryStorage(0)
	defer storage.Close()

	now := time.Now()
	bucketStart := now.Truncate(time.Minute)

	if res, _ := storage.Hit(context.Background(), "k", 1, time.Minute, now); !res.Allowed {
		t.Fatal("first hit should be allowed")
	}
	if res, _ := storage.Hit(context.Background(), "k", 1, time.Minute, now); res.Allowed {
		t.Fatal("second hit in same window should be denied")
	}
	next := bucketStart.Add(time.Minute + time.Second)
	if res, _ := storage.Hit(context.Background(), "k", 1, time.Minute, next); !res.Allowed {
		t.Fatal("hit in next window should be allowed")
	}
}

func TestRateLimitConcurrent(t *testing.T) {
	storage := NewMemoryStorage(0)
	defer storage.Close()

	const limit = 100
	const workers = 50
	const perWorker = 4

	var allowed int64
	var wg sync.WaitGroup
	start := make(chan struct{})
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < perWorker; i++ {
				res, _ := storage.Hit(context.Background(), "concurrent-key", limit, time.Minute, time.Now())
				if res.Allowed {
					atomic.AddInt64(&allowed, 1)
				}
			}
		}()
	}
	close(start)
	wg.Wait()

	if allowed != limit {
		t.Fatalf("allowed = %d, want exactly %d (limit)", allowed, limit)
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	cfg := engineTestConfig()
	e := mustCompileEngine(t, cfg)
	storage := NewMemoryStorage(0)
	defer storage.Close()

	rl := RateLimit(e, storage, nil)

	called := 0
	handler := rl(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
	}))

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/payments/create", nil)
		req.Header.Set(UserIDHeader, "user-1")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("request %d: status = %d, want 200", i, rr.Code)
		}
	}
	if called != 5 {
		t.Fatalf("handler called %d times, want 5", called)
	}
}

func TestRateLimitMiddlewareDeniesAt429(t *testing.T) {
	cfg := engineTestConfig()
	cfg.Security.Routes[0].RateLimit = &config.RateLimitPolicy{Requests: 2, Window: config.MustDuration("1m")}
	e := mustCompileEngine(t, cfg)
	storage := NewMemoryStorage(0)
	defer storage.Close()

	rl := RateLimit(e, storage, nil)
	handler := rl(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	denied := 0
	for i := 0; i < 4; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/payments/create", nil)
		req.Header.Set(UserIDHeader, "user-2")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code == http.StatusTooManyRequests {
			denied++
			if rr.Header().Get("Retry-After") == "" {
				t.Fatal("missing Retry-After header")
			}
		}
	}
	if denied != 2 {
		t.Fatalf("denied = %d, want 2", denied)
	}
}

func TestRequestIDMiddleware(t *testing.T) {
	var captured string
	h := RequestID(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = RequestIDFromContext(r.Context())
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if captured == "" {
		t.Fatal("request id not in context")
	}
	if rr.Header().Get(RequestIDHeader) != captured {
		t.Fatalf("response header = %q, want %q", rr.Header().Get(RequestIDHeader), captured)
	}
}

func TestSecurityMiddleware(t *testing.T) {
	e := mustCompileEngine(t, engineTestConfig())
	sec := Security(e, nil)

	// Authenticated customer is allowed.
	req := httptest.NewRequest(http.MethodPost, "/api/payments/create", nil)
	req.Header.Set(UserIDHeader, "u1")
	req.Header.Set(RolesHeader, "customer")
	rr := httptest.NewRecorder()
	allowed := false
	sec(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		allowed = true
		id, ok := IdentityFromContext(r.Context())
		if !ok || !id.Authenticated {
			t.Fatal("identity not propagated")
		}
	})).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || !allowed {
		t.Fatalf("authed status = %d allowed=%v, want 200 true", rr.Code, allowed)
	}

	// Unauthenticated protected route -> 401.
	req = httptest.NewRequest(http.MethodPost, "/api/payments/create", nil)
	rr = httptest.NewRecorder()
	sec(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("unauthenticated handler should not run")
	})).ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("unauth status = %d, want 401", rr.Code)
	}

	// Wrong role -> 403.
	req = httptest.NewRequest(http.MethodPost, "/api/payments/create", nil)
	req.Header.Set(UserIDHeader, "u2")
	req.Header.Set(RolesHeader, "viewer")
	rr = httptest.NewRecorder()
	sec(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("wrong-role handler should not run")
	})).ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("wrong-role status = %d, want 403", rr.Code)
	}
}

func TestRedactionMiddleware(t *testing.T) {
	e := mustCompileEngine(t, engineTestConfig())
	var sawHeader, sawQuery string
	h := Redaction(e)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawHeader = r.Header.Get("Authorization")
		sawQuery = r.URL.RawQuery
	}))
	req := httptest.NewRequest(http.MethodGet, "/health?token=secret123&user=aditya", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if sawHeader != "[REDACTED]" {
		t.Fatalf("header = %q, want [REDACTED]", sawHeader)
	}
	if sawQuery == "" {
		t.Fatal("query not passed")
	}
	if sawQuery != "token=%5BREDACTED%5D&user=aditya" {
		t.Fatalf("query = %q, want token redacted", sawQuery)
	}
}

func engineTestConfig() config.Config {
	return config.Config{
		Version: config.SupportedVersion,
		Privacy: config.PrivacyConfig{
			RedactHeaders:         []string{"authorization"},
			RedactQueryParameters: []string{"token"},
			Replacement:           "[REDACTED]",
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
		Logging:          config.LoggingConfig{Level: "info"},
		DefaultRateLimit: config.RateLimitPolicy{Requests: 100, Window: config.MustDuration("1m")},
	}
}

func mustCompileEngine(t *testing.T, cfg config.Config) *engine.Engine {
	t.Helper()
	e, err := engine.Compile(cfg)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	return e
}
