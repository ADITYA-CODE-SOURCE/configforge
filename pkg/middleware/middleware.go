// Package middleware provides reusable net/http middleware that enforces
// ConfigForge policies for authentication, authorization, rate limiting,
// request-id generation, and sensitive-data redaction.
package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/ADITYA-CODE-SOURCE/configforge/internal/redactor"
	"github.com/ADITYA-CODE-SOURCE/configforge/pkg/config"
	"github.com/ADITYA-CODE-SOURCE/configforge/pkg/engine"
)

// ctxKey is an unexported context key type to avoid collisions.
type ctxKey int

const (
	keyRequestID ctxKey = iota
	keyIdentity
)

const (
	// RequestIDHeader is the name of the request/response id header.
	RequestIDHeader = "X-Request-ID"

	// UserIDHeader and RolesHeader are used by the demonstration identity
	// adapter. Real applications should provide their own IdentityFunc.
	UserIDHeader = "X-User-ID"
	RolesHeader  = "X-Roles"
)

// Identity describes the authenticated principal for a request, extracted by
// an IdentityFunc. It is intentionally simple and not a complete identity
// provider.
type Identity struct {
	Authenticated bool
	UserID        string
	Roles         []string
	ClientIP      string
}

// IdentityFunc extracts an Identity from an HTTP request. Applications provide
// their own implementation; DemoIdentity provides a header-based example.
type IdentityFunc func(*http.Request) Identity

// DemoIdentity is a demonstration identity adapter that treats the presence of
// the X-User-ID header as authenticated and reads roles from X-Roles
// (comma-separated). It must not be used as a real security boundary.
func DemoIdentity(r *http.Request) Identity {
	userID := strings.TrimSpace(r.Header.Get(UserIDHeader))
	roles := splitCSV(r.Header.Get(RolesHeader))
	return Identity{
		Authenticated: userID != "",
		UserID:        userID,
		Roles:         roles,
		ClientIP:      clientIP(r),
	}
}

func splitCSV(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func clientIP(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// RequestID generates a request id, attaches it to the request context and the
// X-Request-ID response header, and records it for downstream handlers.
func RequestID(_ *engine.Engine) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := strings.TrimSpace(r.Header.Get(RequestIDHeader))
			if id == "" {
				id = newRequestID()
			}
			w.Header().Set(RequestIDHeader, id)
			ctx := context.WithValue(r.Context(), keyRequestID, id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequestIDFromContext returns the request id stored in the context, or "".
func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(keyRequestID).(string); ok {
		return v
	}
	return ""
}

// IdentityFromContext returns the identity stored by the Security middleware.
func IdentityFromContext(ctx context.Context) (Identity, bool) {
	v, ok := ctx.Value(keyIdentity).(Identity)
	return v, ok
}

func newRequestID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return "cf-" + hex.EncodeToString(b)
}

// Redaction redacts sensitive HTTP headers and query parameters before the
// request reaches the next handler. The original request is not mutated; a
// cloned, redacted request is passed downstream.
func Redaction(e *engine.Engine) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		red := buildRedactor(e)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			redacted := r.Clone(r.Context())
			redacted.Header = red.RedactHeaders(r.Header)
			redacted.URL.RawQuery = red.RedactQuery(r.URL.Query())
			next.ServeHTTP(w, redacted)
		})
	}
}

// Security enforces route authentication and authorization policies. It uses
// the supplied IdentityFunc to extract the principal; if nil, DemoIdentity is
// used. Denied requests stop the chain. Authentication failures yield 401;
// authorization failures and unmatched routes yield 403.
func Security(e *engine.Engine, identity IdentityFunc) func(http.Handler) http.Handler {
	if identity == nil {
		identity = DemoIdentity
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := identity(r)
			decision := e.EvaluateRequest(engine.Request{
				Method:        r.Method,
				Path:          r.URL.Path,
				Authenticated: id.Authenticated,
				Roles:         id.Roles,
				ClientIP:      id.ClientIP,
				UserID:        id.UserID,
			})
			ctx := context.WithValue(r.Context(), keyIdentity, id)
			if decision.Allowed {
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			status := http.StatusForbidden
			if decision.Reason == "authentication required" {
				status = http.StatusUnauthorized
			}
			http.Error(w, decision.Reason, status)
		})
	}
}

// RateLimit enforces the rate-limit policy matched for the request route. The
// counter is keyed by route and authenticated user id, falling back to client
// IP. It requires a RateLimitStorage implementation; pass NewMemoryStorage for
// an in-memory store, or supply a Redis-backed implementation later.
func RateLimit(e *engine.Engine, storage RateLimitStorage, identity IdentityFunc) func(http.Handler) http.Handler {
	if identity == nil {
		identity = DemoIdentity
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			policy, routeName, ok := e.RateLimitFor(r.Method, r.URL.Path)
			if !ok {
				next.ServeHTTP(w, r)
				return
			}
			id := identity(r)
			subject := id.UserID
			if subject == "" {
				subject = id.ClientIP
			}
			key := rateLimitKey(routeName, subject)

			result, err := storage.Hit(r.Context(), key, policy.Requests, policy.Window.Duration, time.Now())
			if err != nil {
				http.Error(w, "rate limit check failed", http.StatusInternalServerError)
				return
			}
			setRateLimitHeaders(w, result, routeName, policy)
			if !result.Allowed {
				w.Header().Set("Retry-After", fmt.Sprintf("%d", int(result.RetryAfter.Seconds())+1))
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func rateLimitKey(route, subject string) string {
	return route + ":" + subject
}

func setRateLimitHeaders(w http.ResponseWriter, result RateLimitResult, routeName string, policy config.RateLimitPolicy) {
	h := w.Header()
	h.Set("X-RateLimit-Limit", fmt.Sprintf("%d", policy.Requests))
	h.Set("X-RateLimit-Remaining", fmt.Sprintf("%d", result.Remaining))
	if routeName != "" {
		h.Set("X-RateLimit-Policy", routeName)
	}
}

// DecisionLog logs security decisions at the supplied logger. It is safe to use
// with nil logger (defaults to log.Default()). Sensitive values provided by the
// identity adapter are never logged; only the route, decision, and reason are
// recorded.
func DecisionLog(e *engine.Engine, logger *log.Logger, identity IdentityFunc) func(http.Handler) http.Handler {
	if logger == nil {
		logger = log.Default()
	}
	if identity == nil {
		identity = DemoIdentity
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := identity(r)
			decision := e.EvaluateRequest(engine.Request{
				Method:        r.Method,
				Path:          r.URL.Path,
				Authenticated: id.Authenticated,
				Roles:         id.Roles,
				ClientIP:      id.ClientIP,
				UserID:        id.UserID,
			})
			logger.Printf(
				"configforge decision path=%s method=%s allowed=%t policy=%s reason=%s request_id=%s",
				r.URL.Path, r.Method, decision.Allowed, decision.MatchedPolicy, decision.Reason,
				RequestIDFromContext(r.Context()),
			)
			next.ServeHTTP(w, r)
		})
	}
}

// buildRedactor constructs a redactor from the compiled configuration.
func buildRedactor(e *engine.Engine) *redactor.Redactor {
	cfg := e.Config()
	return redactor.New(
		cfg.Privacy.RedactHeaders,
		cfg.Privacy.RedactQueryParameters,
		cfg.Privacy.RedactJSONFields,
		cfg.Privacy.Replacement,
	)
}

// RedactJSON is a convenience helper that redacts a JSON document using the
// engine's redaction rules. It is safe for concurrent use.
func RedactJSON(e *engine.Engine, data []byte) ([]byte, error) {
	red := buildRedactor(e)
	return red.RedactJSON(data)
}
