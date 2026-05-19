package config

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	HTTP      HTTPConfig
	Metrics   MetricsConfig
	Telemetry TelemetryConfig
	Postgres  PostgresConfig
	JWT       JWTConfig
	Kafka     KafkaConfig
	Admin     AdminConfig
}

type HTTPConfig struct{ Addr string }
type MetricsConfig struct{ Addr string }
type TelemetryConfig struct{ OTLPEndpoint string }

type PostgresConfig struct {
	DSN             string
	MaxConns        int32
	MinConns        int32
	MaxConnIdleTime time.Duration
}

type JWTConfig struct {
	Issuer         string
	KeyID          string
	PrivateKeyPath string
	AccessTTL      time.Duration
	RefreshTTL     time.Duration
}

type KafkaConfig struct {
	Brokers          []string
	UserCreatedTopic string
}

type AdminConfig struct {
	Emails []string
}

func Load() (*Config, error) {
	configPath := flag.String("config", "config/config.yaml", "path to config file")
	flag.Parse()

	viper.SetConfigFile(*configPath)
	viper.SetConfigType("yaml")

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("config: read config file: %w", err)
	}

	idleTime, err := time.ParseDuration(viper.GetString("postgres.max_conn_idle_time"))
	if err != nil {
		return nil, fmt.Errorf("config: parse max_conn_idle_time: %w", err)
	}
	accessTTL, err := time.ParseDuration(viper.GetString("jwt.access_ttl"))
	if err != nil {
		return nil, fmt.Errorf("config: parse access_ttl: %w", err)
	}
	refreshTTL, err := time.ParseDuration(viper.GetString("jwt.refresh_ttl"))
	if err != nil {
		return nil, fmt.Errorf("config: parse refresh_ttl: %w", err)
	}

	password, err := readSecret("/run/secrets/postgres_password")
	if err != nil {
		return nil, fmt.Errorf("config: read postgres password: %w", err)
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		viper.GetString("postgres.user"),
		password,
		viper.GetString("postgres.host"),
		viper.GetInt("postgres.port"),
		viper.GetString("postgres.dbname"),
	)

	return &Config{
		HTTP:      HTTPConfig{Addr: viper.GetString("http.addr")},
		Metrics:   MetricsConfig{Addr: viper.GetString("metrics.addr")},
		Telemetry: TelemetryConfig{OTLPEndpoint: viper.GetString("telemetry.otlp_endpoint")},
		Postgres:  PostgresConfig{DSN: dsn, MaxConns: int32(viper.GetInt("postgres.max_conns")), MinConns: int32(viper.GetInt("postgres.min_conns")), MaxConnIdleTime: idleTime},
		JWT: JWTConfig{
			Issuer:         viper.GetString("jwt.issuer"),
			KeyID:          viper.GetString("jwt.key_id"),
			PrivateKeyPath: viper.GetString("jwt.private_key_path"),
			AccessTTL:      accessTTL,
			RefreshTTL:     refreshTTL,
		},
		Kafka: KafkaConfig{
			Brokers:          viper.GetStringSlice("kafka.brokers"),
			UserCreatedTopic: viper.GetString("kafka.user_created_topic"),
		},
		Admin: AdminConfig{
			Emails: normalizeEmails(viper.GetStringSlice("auth.admin_emails")),
		},
	}, nil
}

func normalizeEmails(items []string) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		email := strings.ToLower(strings.TrimSpace(item))
		if email != "" {
			result = append(result, email)
		}
	}
	return result
}

func readSecret(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read secret %s: %w", path, err)
	}
	return strings.TrimSpace(string(data)), nil
}
