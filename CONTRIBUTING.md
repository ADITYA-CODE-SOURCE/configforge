# Contributing

Contributions are welcome. This project is built phase by phase as an open-source developer project.

## Getting started

1. Install Go 1.24 or newer.
2. Run `go mod download` to fetch dependencies.
3. Run `go test ./...` to verify all tests pass.
4. Run `gofmt -l .` — fix any formatting issues before committing.

## Development workflow

- Keep changes small and focused.
- Add tests for new functionality.
- Run `go vet ./...` and ensure no issues.
- If adding CLI commands, update `cmd/configforge/main.go` explain function accordingly.
- If adding or changing manifests, run `configforge generate --manifests ./manifests` to regenerate artifacts, then verify with `git diff --exit-code`.
- Lint: `golangci-lint run` (requires separate installation) or manually run the linters referenced in `.golangci.yml`.

## Adding new manifest options

1. Add the option to a manifest YAML file under `manifests/`.
2. Ensure the option has a `type`, `default`, and `description`.
3. Re-run generation and confirm golden files update correctly.

## Submitting changes

1. Fork the repository.
2. Create a feature branch (`git checkout -b feature/foo`).
3. Commit with a clear message.
4. Push and open a Pull Request against `main`.

See the GitHub Issues for planned work and known limitations.