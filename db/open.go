package db

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

// 接続プール（Neon は遠隔のため接続確立が高く、並行クエリ分の接続を温存する）
const (
	MaxOpenConns = 16
	MaxIdleConns = 16
)

// 接続設定（prepare の往復を挟まない Exec モードを既定にする。URL の明示指定を優先）
func Config(url string) (*pgx.ConnConfig, error) {
	config, err := pgx.ParseConfig(url)
	if err != nil {
		return nil, err
	}
	if !strings.Contains(url, "default_query_exec_mode=") {
		config.DefaultQueryExecMode = pgx.QueryExecModeExec
	}
	return config, nil
}

// プール設定済みの接続を開く（まだ接続はしない）
func Open(url string) (*sql.DB, error) {
	config, err := Config(url)
	if err != nil {
		return nil, err
	}
	database := stdlib.OpenDB(*config)
	database.SetMaxOpenConns(MaxOpenConns)
	database.SetMaxIdleConns(MaxIdleConns)
	// Neon の自動停止（既定5分）より先に手放し、切断済みの接続を掴まない
	database.SetConnMaxIdleTime(4 * time.Minute)
	database.SetConnMaxLifetime(30 * time.Minute)
	return database, nil
}

// 接続確認とマイグレーション適用まで済ませた接続（各プロセスの起動処理）
func OpenMigrated(ctx context.Context, url string) (*sql.DB, error) {
	database, err := Open(url)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err = database.PingContext(ctx); err == nil {
		err = Migrate(ctx, database)
	}
	if err != nil {
		database.Close()
		return nil, err
	}
	return database, nil
}
