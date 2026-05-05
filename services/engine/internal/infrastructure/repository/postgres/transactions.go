package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
)

func rollback(ctx context.Context, tx pgx.Tx) {
	_ = tx.Rollback(ctx)
}