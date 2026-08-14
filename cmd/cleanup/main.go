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

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	database, err := sql.Open("pgx", databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()
	ctx := context.Background()
	if err := db.Migrate(ctx, database); err != nil {
		log.Fatal(err)
	}
	count, err := (application.Cleanup{DB: database}).HardDeleteExpiredTasks(ctx, time.Now().UTC())
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("hard-deleted %d expired soft-deleted tasks", count)
}
