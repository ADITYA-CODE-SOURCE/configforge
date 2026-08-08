# ConfigForge

ConfigForge is a declarative feature and policy engine for Go applications. It lets developers describe feature flags, API access rules, rate limits, and privacy/redaction policies with YAML configuration instead of hard-coding them.

This repository is being built phase by phase as an open-source developer project. It should not yet be treated as production ready.

## Phase 1 Status

- Go module and repository structure initialized.
- Strongly typed configuration model added.
- Strict YAML loading rejects unknown fields.
- Built-in defaults and typed environment overrides are implemented.
- Runtime validation reports field paths and YAML line/column information where available.
- Initial `configforge validate --config <file>` CLI command added.

## Quick Start

```bash
go test ./...
go run ./cmd/configforge validate --config examples/configs/default.yaml
```

## Environment Overrides

- `CONFIGFORGE_LOG_LEVEL`
- `CONFIGFORGE_DEFAULT_RATE_LIMIT_REQUESTS`
- `CONFIGFORGE_DEFAULT_RATE_LIMIT_WINDOW`
- `CONFIGFORGE_REDACT_HEADERS`
- `CONFIGFORGE_REDACT_QUERY_PARAMETERS`

Configuration file values take precedence over environment values. Environment values take precedence over built-in defaults.
