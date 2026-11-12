package db

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
)

// 遠隔の Neon へ毎回接続し直さないよう、接続を保持するプール設定で開く
func TestOpenKeepsWarmConnectionPool(t *testing.T) {
	database, err := Open("postgres://user:pass@127.0.0.1:1/none?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if got := database.Stats().MaxOpenConnections; got != MaxOpenConns {
		t.Fatalf("max open=%d", got)
	}
	if MaxIdleConns < 8 || MaxIdleConns > MaxOpenConns {
		t.Fatalf("idle pool must keep parallel query connections warm: %d", MaxIdleConns)
	}
}

// 接続できない URL は起動時に失敗させる
func TestOpenMigratedFailsFastWithoutDatabase(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if database, err := OpenMigrated(ctx, "postgres://user:pass@127.0.0.1:1/none?sslmode=disable"); err == nil {
		database.Close()
		t.Fatal("expected connection error")
	}
}

// 実DBでは接続・マイグレーションまで済んだ接続を返す
func TestOpenMigratedAppliesMigrations(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	database, err := OpenMigrated(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	var count int
	if err := database.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&count); err != nil || count == 0 {
		t.Fatalf("count=%d err=%v", count, err)
	}
}

// 遠隔DBでは prepare の往復を挟まず、常に1往復で実行する（URL で明示した場合はそれを優先）
func TestConfigUsesSingleRoundTripExecMode(t *testing.T) {
	config, err := Config("postgres://user:pass@127.0.0.1:1/none?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	if config.DefaultQueryExecMode != pgx.QueryExecModeExec {
		t.Fatalf("mode=%v", config.DefaultQueryExecMode)
	}
	explicit, err := Config("postgres://user:pass@127.0.0.1:1/none?sslmode=disable&default_query_exec_mode=cache_statement")
	if err != nil || explicit.DefaultQueryExecMode != pgx.QueryExecModeCacheStatement {
		t.Fatalf("explicit mode=%v err=%v", explicit.DefaultQueryExecMode, err)
	}
}
