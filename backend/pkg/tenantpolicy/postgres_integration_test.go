package tenantpolicy

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ToufiqQureshi/hakaishield/pkg/db"
	"github.com/ToufiqQureshi/hakaishield/pkg/policy"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresRevisionIsolationAndRollback(t *testing.T) {
	raw := os.Getenv("HAKAISHIELD_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("set HAKAISHIELD_TEST_DATABASE_URL for integration test")
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.ToLower(u.Host+u.Path), "test") && !strings.Contains(u.Host, "localhost") && !strings.Contains(u.Host, "127.0.0.1") && os.Getenv("HAKAISHIELD_ALLOW_REMOTE_TEST_DATABASE") != "1" {
		t.Skip("test database URL safety gate")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("hs_policy_test_%d", time.Now().UnixNano())
	if _, err = admin.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		admin.Exec(cleanup, `DROP SCHEMA IF EXISTS `+schema+` CASCADE`)
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
	if err = db.InitSchema(ctx, pool); err != nil {
		t.Fatalf("actual startup migration: %v", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO tenants (id,host,target,mode,owner_user_id) VALUES ('ta','a.example.com','http://127.0.0.1:1','enforce','alice'),('tb','b.example.com','http://127.0.0.1:1','enforce','bob')`)
	if err != nil {
		t.Fatal(err)
	}
	s := NewStore(pool)
	doc := Document{Mode: "shadow", RouteClasses: map[string]string{"/account/signin": policy.ClassLogin}, Rules: []policy.Rule{{ID: "rule-a", Name: "block login", Enabled: true, Action: policy.ActionBlock, Conditions: []policy.Condition{{Field: policy.FieldRequestClass, Operator: policy.OpEquals, Value: policy.ClassLogin}}}}}
	first, err := s.Save(ctx, "alice", "ta", "alice", 0, doc)
	if err != nil || first.Version != 1 {
		t.Fatalf("first revision=%+v err=%v", first, err)
	}
	if _, err = s.Save(ctx, "bob", "ta", "bob", 1, doc); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner write=%v", err)
	}
	if _, err = s.Latest(ctx, "bob", "ta"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner read=%v", err)
	}
	if _, err = s.Save(ctx, "alice", "ta", "alice", 0, doc); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale version write=%v", err)
	}
	doc.Mode = "enforce"
	second, err := s.Save(ctx, "alice", "ta", "alice", 1, doc)
	if err != nil || second.Version != 2 {
		t.Fatalf("second revision=%+v err=%v", second, err)
	}
	loaded, err := s.LoadForTenant(ctx, "ta")
	if err != nil || loaded.OwnerUserID != "alice" || loaded.Version != 2 || loaded.Document.Mode != "enforce" || loaded.Document.RouteClasses["/account/signin"] != policy.ClassLogin {
		t.Fatalf("proxy load=%+v err=%v", loaded, err)
	}
	rolled, err := s.Rollback(ctx, "alice", "ta", "alice", 2, 2)
	if err != nil || rolled.Version != 3 || rolled.Document.Mode != "shadow" {
		t.Fatalf("rollback=%+v err=%v", rolled, err)
	}
	history, err := s.History(ctx, "alice", "ta")
	if err != nil || len(history) != 3 || history[0].Version != 3 {
		t.Fatalf("history=%+v err=%v", history, err)
	}
	historical, err := s.GetVersion(ctx, "alice", "ta", 2)
	if err != nil || historical.Document.Mode != "enforce" {
		t.Fatalf("historical revision=%+v err=%v", historical, err)
	}
	if _, err = s.GetVersion(ctx, "bob", "ta", 2); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner historical read=%v", err)
	}
	other, err := s.History(ctx, "alice", "tb")
	if err != nil || len(other) != 0 {
		t.Fatalf("cross-tenant history=%+v err=%v", other, err)
	}
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { _, err := s.Save(ctx, "alice", "ta", "alice", 3, doc); results <- err }()
	}
	a, b := <-results, <-results
	if (a != nil || !errors.Is(b, ErrConflict)) && (b != nil || !errors.Is(a, ErrConflict)) {
		t.Fatalf("concurrent optimistic writes = %v, %v", a, b)
	}
}
