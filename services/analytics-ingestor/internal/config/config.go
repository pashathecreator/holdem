package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Kafka      KafkaConfig
	ClickHouse ClickHouseConfig
}

type KafkaConfig struct {
	Brokers           []string
	Topics            []string
	SchemaRegistryURL string
	ConsumerGroup     string
}

type ClickHouseConfig struct {
	Addr          string
	Database      string
	Username      string
	Password      string
	BatchSize     int
	FlushInterval time.Duration
}

func LoadFromEnv() (*Config, error) {
	batchSize, err := intFromEnv("CLICKHOUSE_BATCH_SIZE", 500)
	if err != nil {
		return nil, fmt.Errorf("parse CLICKHOUSE_BATCH_SIZE: %w", err)
	}

	flushInterval, err := durationFromEnv("CLICKHOUSE_FLUSH_INTERVAL", time.Second)
	if err != nil {
		return nil, fmt.Errorf("parse CLICKHOUSE_FLUSH_INTERVAL: %w", err)
	}

	return &Config{
		Kafka: KafkaConfig{
			Brokers:           strings.Split(getEnv("KAFKA_BROKERS", "kafka:9092"), ","),
			Topics:            strings.Split(getEnv("KAFKA_TOPICS", "hand.started,hand.acted,hand.ended"), ","),
			SchemaRegistryURL: getEnv("SCHEMA_REGISTRY_URL", "http://schema-registry:8081"),
			ConsumerGroup:     getEnv("KAFKA_CONSUMER_GROUP", "analytics-ingestor-v1"),
		},
		ClickHouse: ClickHouseConfig{
			Addr:          getEnv("CLICKHOUSE_ADDR", "clickhouse:9000"),
			Database:      getEnv("CLICKHOUSE_DATABASE", "analytics"),
			Username:      getEnv("CLICKHOUSE_USERNAME", "default"),
			Password:      getEnv("CLICKHOUSE_PASSWORD", ""),
			BatchSize:     batchSize,
			FlushInterval: flushInterval,
		},
	}, nil
}

func getEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func intFromEnv(key string, fallback int) (int, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}

	return parsed, nil
}

func durationFromEnv(key string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, err
	}

	return parsed, nil
}
