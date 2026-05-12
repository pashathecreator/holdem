.PHONY: build test integration lint run

build:
	GOCACHE=/tmp/go-build go build ./cmd/ingestor

test:
	GOCACHE=/tmp/go-build go test ./... -v

integration:
	GOCACHE=/tmp/go-build go test -tags=integration ./integration -v

lint:
	GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint golangci-lint run ./...

run:
	GOCACHE=/tmp/go-build go run ./cmd/ingestor
