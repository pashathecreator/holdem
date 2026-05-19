FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .
RUN go build -o bin/auth-service ./cmd/server

FROM alpine:3.19

WORKDIR /app

COPY --from=builder /app/bin/auth-service ./bin/auth-service
COPY config ./config

EXPOSE 8280 9291

CMD ["./bin/auth-service", "-config", "/app/config/config.yaml"]
