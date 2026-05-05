package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	GRPC      GRPCConfig
	HTTP      HTTPConfig
	Metrics   MetricsConfig
	Telemetry TelemetryConfig
	Rake      RakeConfig
	Kafka     KafkaConfig
	Postgres  PostgresConfig
}

type GRPCConfig struct {
	Addr string
}

type HTTPConfig struct {
	Addr string
}

type MetricsConfig struct {
	Addr string
}

type TelemetryConfig struct {
	OTLPEndpoint string
}

type RakeConfig struct {
	Percent      float64
	Cap          int
	NoFlopNoDrop bool
}

type KafkaConfig struct {
	Brokers           []string
	SchemaRegistryURL string
}

type PostgresConfig struct {
	DSN             string
	MaxConns        int32
	MinConns        int32
	MaxConnIdleTime time.Duration
}

func Load(configPath string) (*Config, error) {
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("config: read config.yaml: %w", err)
	}

	idleTime, err := time.ParseDuration(viper.GetString("postgres.max_conn_idle_time"))
	if err != nil {
		return nil, fmt.Errorf("config: parse max_conn_idle_time: %w", err)
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
		GRPC: GRPCConfig{
			Addr: viper.GetString("grpc.addr"),
		},
		HTTP: HTTPConfig{
			Addr: viper.GetString("http.addr"),
		},
		Metrics: MetricsConfig{
			Addr: viper.GetString("metrics.addr"),
		},
		Telemetry: TelemetryConfig{
			OTLPEndpoint: viper.GetString("telemetry.otlp_endpoint"),
		},
		Rake: RakeConfig{
			Percent:      viper.GetFloat64("rake.percent"),
			Cap:          viper.GetInt("rake.cap"),
			NoFlopNoDrop: viper.GetBool("rake.no_flop_no_drop"),
		},
		Kafka: KafkaConfig{
			Brokers:           viper.GetStringSlice("kafka.brokers"),
			SchemaRegistryURL: viper.GetString("kafka.schema_registry_url"),
		},
		Postgres: PostgresConfig{
			DSN:             dsn,
			MaxConns:        int32(viper.GetInt("postgres.max_conns")),
			MinConns:        int32(viper.GetInt("postgres.min_conns")),
			MaxConnIdleTime: idleTime,
		},
	}, nil
}

func readSecret(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read secret %s: %w", path, err)
	}
	return strings.TrimSpace(string(data)), nil
}