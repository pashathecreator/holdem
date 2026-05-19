.PHONY: test build run lint

test:
	GOCACHE=/tmp/go-build go test ./... -v

build:
	GOCACHE=/tmp/go-build go build ./cmd/server

run:
	GOCACHE=/tmp/go-build go run ./cmd/server

lint:
	GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint golangci-lint run ./...

