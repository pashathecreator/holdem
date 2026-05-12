package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	password, err := readSecret("/run/secrets/postgres_password")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read postgres password: %v\n", err)
		os.Exit(1)
	}

	host := getEnv("POSTGRES_HOST", "postgres")
	port := getEnv("POSTGRES_PORT", "5432")
	user := getEnv("POSTGRES_USER", "postgres")
	dbname := getEnv("POSTGRES_DB", "holdem")

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		user, password, host, port, dbname,
	)

	m, err := migrate.New("file://migrations", dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create migrator: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		sourceErr, dbErr := m.Close()
		if sourceErr != nil {
			fmt.Fprintf(os.Stderr, "failed to close migrator source: %v\n", sourceErr)
		}
		if dbErr != nil {
			fmt.Fprintf(os.Stderr, "failed to close migrator database: %v\n", dbErr)
		}
	}()

	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			fmt.Println("no migrations to apply")
			return
		}
		fmt.Fprintf(os.Stderr, "failed to apply migrations: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("migrations applied successfully")
}

func readSecret(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read secret %s: %w", path, err)
	}
	return strings.TrimSpace(string(data)), nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
