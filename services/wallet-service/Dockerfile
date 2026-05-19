FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .
RUN go build -o bin/wallet-service ./cmd/server

FROM alpine:3.19

WORKDIR /app

COPY --from=builder /app/bin/wallet-service ./bin/wallet-service
COPY config ./config

EXPOSE 8380 9391

CMD ["./bin/wallet-service", "-config", "/app/config/config.yaml"]

