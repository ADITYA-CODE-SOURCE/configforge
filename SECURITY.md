# Security

This project is currently a developer project and does not claim production readiness.

## Supported versions

Please refer to the `go.mod` file for the minimum Go version supported. Configuration format is versioned at `v1`.

## Reporting security issues

Do not report security issues in public issues. Instead, contact the maintainer privately via the GitHub repository's private issues (if available) or email.

## Known limitations

- The demonstration identity adapter reads `X-User-ID` and `X-Roles` headers and is **not** a real security boundary. It is provided as a demo only.
- The rate limiter is in-memory only; it does not persist across process restarts.
- Secrets should not be placed in example configurations, tests, logs, or generated output.

## Best practices for consumers

- Do not rely on the demo identity adapter for production authentication.
- Configure a Redis-backed rate limiter for production traffic.
- Use TLS and secure header handling at the reverse-proxy or load-balancer level.