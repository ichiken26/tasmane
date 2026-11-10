package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"tasmane/db"
	"tasmane/internal/application"
)

// 期限切れソフト削除タスクの物理削除プロセス
func main() {
	// 必須環境変数の検証
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	// PostgreSQL 接続の確立
	database, err := sql.Open("pgx", databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()
	ctx := context.Background()
	// 起動時マイグレーションの適用
	if err := db.Migrate(ctx, database); err != nil {
		log.Fatal(err)
	}
	// 期限切れタスクの物理削除
	count, err := (application.Cleanup{DB: database}).HardDeleteExpiredTasks(ctx, time.Now().UTC())
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("hard-deleted %d expired soft-deleted tasks", count)
}
