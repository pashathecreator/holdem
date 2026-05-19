package main

import (
	"flag"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/pashathecreator/holdem/services/table-manager/internal/config"
)

func main() {
	flag.CommandLine = flag.NewFlagSet("migrator", flag.ExitOnError)

	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	m, err := migrate.New("file://migrations", cfg.Postgres.DSN)
	if err != nil {
		panic(fmt.Errorf("create migrator: %w", err))
	}
	defer func() {
		_, _ = m.Close()
	}()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		panic(fmt.Errorf("apply migrations: %w", err))
	}
}
