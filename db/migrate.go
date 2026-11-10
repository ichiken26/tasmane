package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed migrations/*.sql
var migrations embed.FS

// 同時起動インスタンス間で適用系列を直列化するトランザクションスコープロックキー
const migrationLockID int64 = 0x7461736d616e65

// 未適用の埋め込みマイグレーションの逐次適用
func Migrate(ctx context.Context, database *sql.DB) error {
	if database == nil {
		return fmt.Errorf("database is required")
	}
	// マイグレーション履歴表の確保
	if _, err := database.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return fmt.Errorf("create migration history: %w", err)
	}
	entries, err := fs.ReadDir(migrations, "migrations")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".sql" {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	// 各マイグレーションの適用
	for _, name := range names {
		if err := apply(ctx, database, name); err != nil {
			return err
		}
	}
	return nil
}

// 単一マイグレーションのトランザクション適用
func apply(ctx context.Context, database *sql.DB, name string) error {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %s: %w", name, err)
	}
	defer tx.Rollback()
	// アドバイザリロックの取得
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, migrationLockID); err != nil {
		return fmt.Errorf("lock migration %s: %w", name, err)
	}
	var applied bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`, name).Scan(&applied); err != nil {
		return fmt.Errorf("check migration %s: %w", name, err)
	}
	// 適用済み判定と早期コミット
	if applied {
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", name, err)
		}
		return nil
	}
	body, err := migrations.ReadFile("migrations/" + name)
	if err != nil {
		return fmt.Errorf("read migration %s: %w", name, err)
	}
	statement := strings.TrimSpace(string(body))
	statement = strings.TrimSpace(strings.TrimPrefix(statement, "BEGIN;"))
	statement = strings.TrimSpace(strings.TrimSuffix(statement, "COMMIT;"))
	// SQL 本体の実行と履歴記録
	if _, err := tx.ExecContext(ctx, statement); err != nil {
		return fmt.Errorf("apply migration %s: %w", name, err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, name); err != nil {
		return fmt.Errorf("record migration %s: %w", name, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %s: %w", name, err)
	}
	return nil
}
