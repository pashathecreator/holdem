PROTO_SRC  = proto/engine/v1/engine.proto
PROTO_OUT  = pkg/gen/go
SWAGGER_OUT = docs
GOOGLEAPIS = third_party/googleapis
OPENAPI_INCLUDES = third_party/openapiv2

.PHONY: proto swagger test lint build run down vdown

proto:
	mkdir -p $(PROTO_OUT)
	protoc \
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
	go test ./... -v -race

lint:
	golangci-lint run ./...

