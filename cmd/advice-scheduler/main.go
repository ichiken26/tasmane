package main

import (
	"context"
	"database/sql"
	_ "github.com/jackc/pgx/v5/stdlib"
	// distroless でもユーザーTimezoneを解決できるようIANA tzdataを同梱
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"tasmane/db"
	"tasmane/internal/application"
	"tasmane/internal/integration/gemini"
	"tasmane/internal/repository"
	"time"
	_ "time/tzdata"
)

// 助言生成スケジューラの起動
func main() {
	// 必須環境変数の検証
	databaseURL, key, model := os.Getenv("DATABASE_URL"), os.Getenv("GEMINI_API_KEY"), os.Getenv("GEMINI_MODEL")
	if databaseURL == "" || key == "" || model == "" {
		log.Fatal("DATABASE_URL, GEMINI_API_KEY, and GEMINI_MODEL are required")
	}
	// PostgreSQL 接続の確立
	database, err := sql.Open("pgx", databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()
	// 起動時マイグレーションの適用
	if err := db.Migrate(context.Background(), database); err != nil {
		log.Fatal(err)
	}
	store := repository.NewPostgres(database, nil)
	scheduler := application.AdviceScheduler{Users: store, Tasks: application.Tasks{Repo: store}, Catalog: application.Catalog{Repo: store}, Advices: application.Advices{Provider: gemini.Client{APIKey: key, Model: model, HTTP: &http.Client{Timeout: 45 * time.Second}}, Repo: store}}
	// SIGINT/SIGTERM 待ちの分間隔実行
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		if err := scheduler.RunOnce(ctx); err != nil {
			log.Printf("advice scheduler: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
