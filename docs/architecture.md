# Architecture

ConfigForge separates configuration loading, validation, compilation, and runtime evaluation.

## Layers

1. **Configuration (`pkg/config`)** — strongly typed structs, strict YAML parsing, unknown-field rejection, duplicate-key detection, defaults, typed environment overrides, and field-level validation with YAML line/column errors.
2. **Internal helpers (`internal/...`)** — `matcher` implements route path/method matching and wildcard overlap detection; `redactor` implements header, query, nested-JSON, and structured-attribute redaction. Internal packages are only imported within this module.
3. **Engine (`pkg/engine`)** — `engine.Compile` validates configuration and produces an immutable `*Engine`. The engine holds defensive copies of all configuration so later mutations to the source `Config` do not affect runtime. The engine is safe for concurrent use.
4. **Public types (`pkg/feature`, `pkg/policy`)** — `feature.EvaluationContext`, `feature.Decision`, and `policy.Decision` are the stable public types returned by the engine, keeping the public API small.
5. **Middleware (`pkg/middleware`)** — reusable `net/http` middleware for request IDs, redaction, security, rate limiting, and decision logging. The rate limiter is backed by a `RateLimitStorage` interface with an in-memory implementation.

## Immutability

After `Compile`, the `Engine` never mutates its configuration. Defensive copies are taken during compilation and in `Engine.Config()`, so callers cannot tamper with runtime policies.

## Evaluation Order

Feature evaluation rules are applied deterministically:

1. Disabled features are never enabled.
2. Explicitly targeted users are enabled.
3. Country conditions are checked.
4. Role conditions are checked.
5. Percentage rollout uses stable FNV-1a hashing of `(feature name, user id)`; no randomness is used, so the same user always receives the same decision.

Route evaluation matches routes in declaration order (first match wins), then checks authentication and role authorization. Protected routes default to deny; unmatched routes are denied.

## Wildcard Matching

`/api/payments/*` matches `/api/payments` and any path beginning with `/api/payments/`. Only a single trailing `/*` is allowed; ambiguous wildcard overlaps are reported at compile time.