FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o bin/table-manager ./cmd/server
RUN go install github.com/grpc-ecosystem/grpc-health-probe@v0.4.34

FROM alpine:3.19

WORKDIR /app

COPY --from=builder /go/bin/grpc-health-probe /usr/local/bin/grpc_health_probe
COPY --from=builder /app/bin/table-manager ./bin/table-manager
COPY config ./config
COPY docs ./docs

EXPOSE 9190 8180 9191

CMD ["./bin/table-manager", "-config", "/app/config/config.yaml"]
