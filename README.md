# ConfigForge

ConfigForge is a declarative feature and policy engine for Go applications. It lets developers describe feature flags, API access rules, rate limits, and privacy/redaction policies with YAML configuration instead of hard-coding them.

This repository is being built phase by phase as an open-source developer project. It should not yet be treated as production ready.

> **Note:** This project is independent of OpenTelemetry and does not derive from `otelc` or any other project.

## Status

### Phase 1 — Configuration Foundation (complete)

- Go module and repository structure initialized.
- Strongly typed configuration model added.
- Strict YAML loading rejects unknown fields and duplicate YAML keys.
- Built-in defaults and typed environment overrides are implemented.
- Runtime validation reports field paths and YAML line/column information where available.
- `configforge validate --config <file>` CLI command added.

### Phase 2 — Core Engine (complete)

- Immutable compilation via `engine.Compile`.
- Deterministic feature-flag evaluation with stable percentage rollout hashing.
- Explicit user, country, and role targeting.
- Route-policy evaluation with documented wildcard matching (`/api/payments/*`).
- Authentication and role authorization decisions.
- Privacy redaction engine: HTTP headers (case-insensitive), URL query parameters, nested JSON fields (`credit_card.number`), and structured log attributes.
- Concurrency-safe in-memory fixed-window rate limiter behind a `RateLimitStorage` interface, with periodic cleanup and HTTP `429` responses.
- HTTP middleware: `RequestID`, `Redaction`, `Security`, `RateLimit`, `DecisionLog`.
- Race-safe, table, and fuzz tests across matcher, redactor, feature, engine, and middleware.

## Quick Start

```bash
# Run all tests (race detector included)
go test -race ./...

# Validate a configuration file
go run ./cmd/configforge validate --config examples/configs/default.yaml
```

## Library Integration

```go
cfg, err := config.LoadFile("config.yaml")
if err != nil {
    log.Fatal(err)
}

runtime, err := engine.Compile(*cfg)
if err != nil {
    log.Fatal(err)
}

decision := runtime.EvaluateFeature(
    "new_checkout",
    feature.EvaluationContext{UserID: "user-101", Country: "IN"},
)
fmt.Println(decision.Enabled, decision.Reason)

route := runtime.EvaluateRequest(engine.Request{
    Method: "POST", Path: "/api/payments/create",
    Authenticated: true, Roles: []string{"customer"},
})
fmt.Println(route.Allowed, route.MatchedPolicy, route.Reason)
```

## HTTP Middleware

```go
store := middleware.NewMemoryStorage(time.Minute)
defer store.Close()

handler := http.HandlerFunc(apiHandler)
handler = middleware.RequestID(runtime)(handler)
handler = middleware.Redaction(runtime)(handler)
handler = middleware.RateLimit(runtime, store, nil)(handler)
handler = middleware.Security(runtime, nil)(handler)
http.ListenAndServe(":8080", handler)
```

The demonstration identity adapter reads `X-User-ID` and `X-Roles` headers and is **not** a real security boundary. Provide your own `middleware.IdentityFunc` for production.

## Route Wildcard Semantics

A wildcard route `/api/payments/*` matches:

- exactly `/api/payments`
- any path beginning with `/api/payments/` (including nested segments like `/api/payments/v1/create`)

Only one wildcard per path is allowed, and it must be a trailing `/*`. Ambiguous wildcard-vs-wildcard overlaps are reported at compile time.

## Configuration Precedence

```text
built-in defaults → environment variables → YAML configuration
```

Supported environment variables:

- `CONFIGFORGE_LOG_LEVEL`
- `CONFIGFORGE_DEFAULT_RATE_LIMIT_REQUESTS`
- `CONFIGFORGE_DEFAULT_RATE_LIMIT_WINDOW`
- `CONFIGFORGE_REDACT_HEADERS`
- `CONFIGFORGE_REDACT_QUERY_PARAMETERS`

Explicit configuration file values take the highest precedence.

## Redaction Default

The default replacement value is `[REDACTED]`. Header matching is case-insensitive; query parameters and JSON field paths are matched exactly (with dotted paths for nested fields).

## Testing

```bash
go test ./...
go test -race ./...
go vet ./...
```

## Project Limitations

- Phase 3 (manifest-driven generation, JSON Schema, generated docs) is not yet implemented.
- Phase 4 (full CLI commands, example API, Docker) is not yet implemented.
- Phase 5 (GitHub Actions CI, lint config, coverage artifact) is not yet implemented.
- `golangci-lint` is referenced by the Makefile/CI plan but is not yet installed in all environments.
- The rate limiter is in-memory only; a Redis backend can be added via the `RateLimitStorage` interface.
- The demonstration identity adapter is not a security boundary.

## Roadmap

- Phase 3: manifest parsing, JSON Schema generation, deterministic code/documentation generation, golden tests.
- Phase 4: full CLI (`generate`, `schema`, `explain`, `check-feature`, `check-route`), runnable example API, Docker/Compose.
- Phase 5: GitHub Actions CI, lint configuration, fuzz harnesses, coverage reporting.

## Contributing

See `CONTRIBUTING.md`. Keep changes small, formatted with `gofmt`, and aligned with the public API boundaries under `pkg/` and implementation details under `internal/`.