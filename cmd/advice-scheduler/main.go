package main

import (
	"context"
	"database/sql"
	_ "github.com/jackc/pgx/v5/stdlib"
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
)

func main() {
	databaseURL, key, model := os.Getenv("DATABASE_URL"), os.Getenv("GEMINI_API_KEY"), os.Getenv("GEMINI_MODEL")
	if databaseURL == "" || key == "" || model == "" {
		log.Fatal("DATABASE_URL, GEMINI_API_KEY, and GEMINI_MODEL are required")
	}
	database, err := sql.Open("pgx", databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()
	if err := db.Migrate(context.Background(), database); err != nil {
		log.Fatal(err)
	}
	store := repository.NewPostgres(database, nil)
	scheduler := application.AdviceScheduler{Users: store, Tasks: application.Tasks{Repo: store}, Catalog: application.Catalog{Repo: store}, Advices: application.Advices{Provider: gemini.Client{APIKey: key, Model: model, HTTP: &http.Client{Timeout: 45 * time.Second}}, Repo: store}}
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
