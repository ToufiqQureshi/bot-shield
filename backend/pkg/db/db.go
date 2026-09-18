package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var DB *pgxpool.Pool

func Init(databaseURL string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("unable to connect to database: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("unable to ping database: %w", err)
	}

	DB = pool
	return initSchema(ctx)
}

func initSchema(ctx context.Context) error {
	schema := `
	CREATE TABLE IF NOT EXISTS tenants (
		id VARCHAR(255) PRIMARY KEY,
		host VARCHAR(255) UNIQUE NOT NULL,
		target VARCHAR(255) NOT NULL,
		mode VARCHAR(50) NOT NULL,
		evidence_token VARCHAR(255)
	);
	`
	_, err := DB.Exec(ctx, schema)
	return err
}

func GetTenant(ctx context.Context, host string) (id, target, mode, evidenceToken string, err error) {
	if DB == nil {
		return "", "", "", "", fmt.Errorf("database not initialized")
	}

	query := `SELECT id, target, mode, evidence_token FROM tenants WHERE host = $1 LIMIT 1`
	err = DB.QueryRow(ctx, query, host).Scan(&id, &target, &mode, &evidenceToken)
	return
}
