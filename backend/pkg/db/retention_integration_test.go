package db

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestDeleteSamplesBeforeBatchesAndPreservesFreshRows(t *testing.T) {
	raw := os.Getenv("HAKAISHIELD_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("set HAKAISHIELD_TEST_DATABASE_URL for real Postgres retention test")
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(u.Host, "localhost") && !strings.Contains(u.Host, "127.0.0.1") && !strings.Contains(strings.ToLower(u.Host+u.Path), "test") && os.Getenv("HAKAISHIELD_ALLOW_REMOTE_TEST_DATABASE") != "1" {
		t.Skip("test database URL safety gate")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("hs_retention_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		_, _ = admin.Exec(cleanup, `DROP SCHEMA IF EXISTS `+schema+` CASCADE`)
	})
	cfg, err := pgxpool.ParseConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ConnConfig.RuntimeParams == nil {
		cfg.ConnConfig.RuntimeParams = map[string]string{}
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := InitSchema(ctx, pool); err != nil {
		t.Fatal(err)
	}
	old := DB
	DB = pool
	t.Cleanup(func() { DB = old })
	_, err = pool.Exec(ctx, `INSERT INTO training_samples(tenant_id,fired,feature_version,automated,source,created_at)
		SELECT 't',0,'v',false,'test',now()-interval '40 days' FROM generate_series(1,1002)
		UNION ALL SELECT 't',0,'v',false,'test',now() FROM generate_series(1,2)`)
	if err != nil {
		t.Fatal(err)
	}
	cutoff := time.Now().AddDate(0, 0, -30)
	for i, want := range []int64{1000, 2, 0} {
		got, err := DeleteSamplesBefore(ctx, cutoff)
		if err != nil || got != want {
			t.Fatalf("batch %d deleted=%d err=%v, want %d", i, got, err, want)
		}
	}
	var fresh int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM training_samples`).Scan(&fresh); err != nil || fresh != 2 {
		t.Fatalf("fresh rows=%d err=%v", fresh, err)
	}
}
