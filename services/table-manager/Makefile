PROTO_SRC = proto/table_manager/v1/table_manager.proto
PROTO_OUT = pkg/gen/go
SWAGGER_OUT = docs
GOOGLEAPIS = ../engine/third_party/googleapis
OPENAPI_INCLUDES = ../engine/third_party/openapiv2

.PHONY: proto test build run lint

proto:
	mkdir -p $(PROTO_OUT)
	mkdir -p $(SWAGGER_OUT)
	PATH="$(PATH):$(HOME)/go/bin" protoc \
		-I proto \
		-I $(GOOGLEAPIS) \
		-I $(OPENAPI_INCLUDES) \
		--go_out=$(PROTO_OUT) \
		--go_opt=paths=source_relative \
		--go-grpc_out=$(PROTO_OUT) \
		--go-grpc_opt=paths=source_relative,require_unimplemented_servers=false \
		--grpc-gateway_out=$(PROTO_OUT) \
		--grpc-gateway_opt=paths=source_relative \
		--openapiv2_out=$(SWAGGER_OUT) \
		--openapiv2_opt=logtostderr=true \
		$(PROTO_SRC)

test:
	GOCACHE=/tmp/go-build go test ./... -v

build:
	GOCACHE=/tmp/go-build go build ./cmd/server

run:
	GOCACHE=/tmp/go-build go run ./cmd/server

lint:
	GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint golangci-lint run ./...
