.PHONY: build test test-race lint generate verify-generated coverage run-example docker-up docker-down

build:
	go build ./cmd/configforge

test:
	go test ./...

test-race:
	go test -race ./...

lint:
	golangci-lint run

generate:
	go run ./cmd/configforge generate --manifests ./manifests

verify-generated:
	go test ./...
	git diff --exit-code

coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out

run-example:
	go run ./examples/basic-api --config examples/configs/default.yaml

docker-up:
	docker compose up --build

docker-down:
	docker compose down
