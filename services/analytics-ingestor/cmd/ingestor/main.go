package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/pashathecreator/holdem/services/analytics-ingestor/internal/config"
	"github.com/pashathecreator/holdem/services/analytics-ingestor/internal/consumer/kafka"
	"github.com/pashathecreator/holdem/services/analytics-ingestor/internal/decoder/avro"
	"github.com/pashathecreator/holdem/services/analytics-ingestor/internal/storage/clickhouse"
)

func main() {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	store, err := clickhouse.New(ctx, cfg.ClickHouse)
	if err != nil {
		log.Fatalf("init clickhouse storage: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			log.Printf("close clickhouse storage: %v", err)
		}
	}()

	decoder := avro.NewDecoder(cfg.Kafka.SchemaRegistryURL)

	consumer, err := kafka.NewConsumer(cfg.Kafka, decoder, store)
	if err != nil {
		log.Fatalf("init kafka consumer: %v", err)
	}
	defer func() {
		if err := consumer.Close(); err != nil {
			log.Printf("close kafka consumer: %v", err)
		}
	}()

	if err := consumer.Run(ctx); err != nil && ctx.Err() == nil {
		log.Fatalf("run consumer: %v", err)
	}
}
