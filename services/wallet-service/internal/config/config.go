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
	HTTP     HTTPConfig
	Metrics  MetricsConfig
	Postgres PostgresConfig
	Auth     AuthConfig
	Internal InternalConfig
	Kafka    KafkaConfig
	Chain    ChainConfig
}

type HTTPConfig struct{ Addr string }
type MetricsConfig struct{ Addr string }

type PostgresConfig struct {
	DSN             string
	MaxConns        int32
	MinConns        int32
	MaxConnIdleTime time.Duration
}

type AuthConfig struct {
	Issuer  string
	JWKSURL string
}

type InternalConfig struct {
	Token string
}

type KafkaConfig struct {
	Brokers          []string
	UserCreatedTopic string
	ConsumerGroup    string
}

type ChainConfig struct {
	Enabled             bool
	Network             string
	RPCURL              string
	Confirmations       int64
	StartBlock          uint64
	ScanInterval        time.Duration
	ReceiptPollInterval time.Duration
	HotWalletAddress    string
	HotWalletPrivateKey string
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
	scanInterval, err := time.ParseDuration(viper.GetString("chain.scan_interval"))
	if err != nil {
		return nil, fmt.Errorf("config: parse chain.scan_interval: %w", err)
	}
	receiptPollInterval, err := time.ParseDuration(viper.GetString("chain.receipt_poll_interval"))
	if err != nil {
		return nil, fmt.Errorf("config: parse chain.receipt_poll_interval: %w", err)
	}

	password, err := readSecret("/run/secrets/postgres_password")
	if err != nil {
		return nil, fmt.Errorf("config: read postgres password: %w", err)
	}

	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable",
		viper.GetString("postgres.user"),
		password,
		viper.GetString("postgres.host"),
		viper.GetInt("postgres.port"),
		viper.GetString("postgres.dbname"),
	)

	return &Config{
		HTTP:    HTTPConfig{Addr: viper.GetString("http.addr")},
		Metrics: MetricsConfig{Addr: viper.GetString("metrics.addr")},
		Postgres: PostgresConfig{
			DSN:             dsn,
			MaxConns:        int32(viper.GetInt("postgres.max_conns")),
			MinConns:        int32(viper.GetInt("postgres.min_conns")),
			MaxConnIdleTime: idleTime,
		},
		Auth: AuthConfig{
			Issuer:  viper.GetString("auth.issuer"),
			JWKSURL: viper.GetString("auth.jwks_url"),
		},
		Internal: InternalConfig{
			Token: viper.GetString("internal.token"),
		},
		Kafka: KafkaConfig{
			Brokers:          viper.GetStringSlice("kafka.brokers"),
			UserCreatedTopic: viper.GetString("kafka.user_created_topic"),
			ConsumerGroup:    viper.GetString("kafka.consumer_group"),
		},
		Chain: ChainConfig{
			Enabled:             viper.GetBool("chain.enabled"),
			Network:             viper.GetString("chain.network"),
			RPCURL:              viper.GetString("chain.rpc_url"),
			Confirmations:       viper.GetInt64("chain.confirmations"),
			StartBlock:          viper.GetUint64("chain.start_block"),
			ScanInterval:        scanInterval,
			ReceiptPollInterval: receiptPollInterval,
			HotWalletAddress:    viper.GetString("chain.hot_wallet_address"),
			HotWalletPrivateKey: viper.GetString("chain.hot_wallet_private_key"),
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
