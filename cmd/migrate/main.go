package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"tasmane/db"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is required")
		os.Exit(2)
	}
	database, err := sql.Open("pgx", databaseURL)
	if err == nil {
		err = db.Migrate(context.Background(), database)
	}
	if database != nil {
		_ = database.Close()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
