//go:build integration

package integration_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	tcredpanda "github.com/testcontainers/testcontainers-go/modules/redpanda"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/pashathecreator/holdem/services/engine/internal/application"
	deliverygrpc "github.com/pashathecreator/holdem/services/engine/internal/delivery/grpc"
	deliverykafka "github.com/pashathecreator/holdem/services/engine/internal/delivery/kafka"
	"github.com/pashathecreator/holdem/services/engine/internal/domain"
	"github.com/pashathecreator/holdem/services/engine/internal/infrastructure/evaluator"
	repopostgres "github.com/pashathecreator/holdem/services/engine/internal/infrastructure/repository/postgres"
	enginev1 "github.com/pashathecreator/holdem/services/engine/pkg/gen/go/engine/v1"
	kafka "github.com/segmentio/kafka-go"
)

const (
	testPostgresImage = "postgres:16-alpine"
	testRedpandaImage = "docker.redpanda.com/redpandadata/redpanda:v25.2.4"
	testDBName        = "holdem"
	testDBUser        = "postgres"
	testDBPassword    = "secret"
)

type integrationHarness struct {
	ctx       context.Context
	repo      *repopostgres.GameStateRepo
	client    enginev1.GameEngineClient
	publisher *deliverykafka.Publisher
	pool      *pgxpool.Pool
	server    *grpc.Server
	conn      *grpc.ClientConn
	listener  net.Listener
	broker    string
}

func newIntegrationHarness(t *testing.T, shuffleFunc domain.ShuffleFunc) *integrationHarness {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	postgresContainer, err := tcpostgres.Run(
		ctx,
		testPostgresImage,
		tcpostgres.WithDatabase(testDBName),
		tcpostgres.WithUsername(testDBUser),
		tcpostgres.WithPassword(testDBPassword),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() {
		_ = testcontainers.TerminateContainer(postgresContainer)
	})

	redpandaContainer, err := tcredpanda.Run(ctx, testRedpandaImage)
	if err != nil {
		t.Fatalf("start redpanda container: %v", err)
	}
	t.Cleanup(func() {
		_ = testcontainers.TerminateContainer(redpandaContainer)
	})

	dsn, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("postgres connection string: %v", err)
	}
	broker, err := redpandaContainer.KafkaSeedBroker(ctx)
	if err != nil {
		t.Fatalf("redpanda broker: %v", err)
	}
	schemaRegistryURL, err := redpandaContainer.SchemaRegistryAddress(ctx)
	if err != nil {
		t.Fatalf("redpanda schema registry: %v", err)
	}

	if err := createTopics(ctx, broker, []string{"hand.started", "hand.acted", "hand.ended"}); err != nil {
		t.Fatalf("create topics: %v", err)
	}
	if err := runMigrations(ctx, dsn); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("open pgx pool: %v", err)
	}
	t.Cleanup(pool.Close)

	repo := repopostgres.NewGameStateRepo(pool)
	publisher, err := deliverykafka.NewPublisher([]string{broker}, schemaRegistryURL)
	if err != nil {
		t.Fatalf("new kafka publisher: %v", err)
	}
	t.Cleanup(func() {
		_ = publisher.Close()
	})

	eval := evaluator.New()
	pubsub := deliverygrpc.NewPubSub()
	finishHand := application.NewFinishHand(repo, publisher, eval, domain.RakeConfig{
		Percent:      0.05,
		Cap:          150,
		NoFlopNoDrop: true,
	})
	startHand := application.NewStartHand(repo, publisher, shuffleFunc)
	applyAction := application.NewApplyAction(repo, publisher, finishHand)

	server := grpc.NewServer()
	enginev1.RegisterGameEngineServer(
		server,
		deliverygrpc.NewServer(startHand, applyAction, finishHand, repo, pubsub),
	)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen grpc: %v", err)
	}
	t.Cleanup(func() {
		_ = listener.Close()
	})

	go func() {
		_ = server.Serve(listener)
	}()
	t.Cleanup(server.GracefulStop)

	conn, err := grpc.DialContext(
		ctx,
		listener.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial grpc: %v", err)
	}
	t.Cleanup(func() {
		_ = conn.Close()
	})

	return &integrationHarness{
		ctx:       ctx,
		repo:      repo,
		client:    enginev1.NewGameEngineClient(conn),
		publisher: publisher,
		pool:      pool,
		server:    server,
		conn:      conn,
		listener:  listener,
		broker:    broker,
	}
}

func (h *integrationHarness) mustReadTopicMessages(t *testing.T, topic string, minCount int) int {
	t.Helper()

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     []string{h.broker},
		Topic:       topic,
		StartOffset: kafka.FirstOffset,
		MaxWait:     200 * time.Millisecond,
		MinBytes:    1,
		MaxBytes:    10e6,
	})
	defer func() {
		_ = reader.Close()
	}()

	ctx, cancel := context.WithTimeout(h.ctx, 10*time.Second)
	defer cancel()

	count := 0
	for count < minCount {
		if _, err := reader.ReadMessage(ctx); err != nil {
			t.Fatalf("read topic %s message %d: %v", topic, count+1, err)
		}
		count++
	}

	return count
}

func createTopics(ctx context.Context, broker string, topics []string) error {
	conn, err := kafka.DialContext(ctx, "tcp", broker)
	if err != nil {
		return fmt.Errorf("dial kafka broker: %w", err)
	}
	defer conn.Close()

	controller, err := conn.Controller()
	if err != nil {
		return fmt.Errorf("get kafka controller: %w", err)
	}

	controllerAddr := net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port))
	controllerConn, err := kafka.DialContext(ctx, "tcp", controllerAddr)
	if err != nil {
		return fmt.Errorf("dial kafka controller: %w", err)
	}
	defer controllerConn.Close()

	configs := make([]kafka.TopicConfig, len(topics))
	for i, topic := range topics {
		configs[i] = kafka.TopicConfig{
			Topic:             topic,
			NumPartitions:     1,
			ReplicationFactor: 1,
		}
	}

	if err := controllerConn.CreateTopics(configs...); err != nil {
		return fmt.Errorf("create kafka topics: %w", err)
	}

	return nil
}

func runMigrations(ctx context.Context, dsn string) error {
	migrationsPath, err := filepath.Abs(filepath.Join("..", "migrations"))
	if err != nil {
		return fmt.Errorf("resolve migrations path: %w", err)
	}

	m, err := migrate.New("file://"+migrationsPath, dsn)
	if err != nil {
		return fmt.Errorf("new migrator: %w", err)
	}
	defer func() {
		_, _ = m.Close()
	}()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}

	return nil
}
