# Configuration Options

Options are declared in manifests under `manifests/` and are generated deterministically by ConfigForge.

## feature-flags

Controls declarative feature flags and targeting conditions.

| Option | Type | Default | Env | Description |
| --- | --- | --- | --- | --- |
| `rollout_percentage` | `integer` | `0` | `—` | Percentage of matching users that receive a feature. |

## privacy

Controls sensitive data redaction rules.

| Option | Type | Default | Env | Description |
| --- | --- | --- | --- | --- |
| `redact_headers` | `string_array` | `authorization, cookie, x-api-key` | `CONFIGFORGE_REDACT_HEADERS` | HTTP headers to redact case-insensitively. |
| `redact_query_parameters` | `string_array` | `password, token, api_key` | `CONFIGFORGE_REDACT_QUERY_PARAMETERS` | URL query parameters to redact. |

## route-security

Controls HTTP route authentication, authorization, and rate limits.

| Option | Type | Default | Env | Description |
| --- | --- | --- | --- | --- |
| `requests` | `integer` | `100` | `CONFIGFORGE_DEFAULT_RATE_LIMIT_REQUESTS` | Maximum requests allowed during the configured window. |
| `window` | `duration` | `1m` | `CONFIGFORGE_DEFAULT_RATE_LIMIT_WINDOW` | Rate-limit evaluation window. |

