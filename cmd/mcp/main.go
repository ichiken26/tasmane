package main

import (
	"context"
	"database/sql"
	_ "github.com/jackc/pgx/v5/stdlib"
	"log"
	"os"
	"tasmane/db"
	"tasmane/internal/application"
	"tasmane/internal/repository"
	mcpserver "tasmane/mcp"
)

func main() {
	databaseURL, userID := os.Getenv("DATABASE_URL"), os.Getenv("TASMANE_MCP_USER_ID")
	if databaseURL == "" || userID == "" {
		log.Fatal("DATABASE_URL and TASMANE_MCP_USER_ID are required")
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
	server := mcpserver.Server{UserID: userID, Services: mcpserver.Services{Tasks: application.Tasks{Repo: store}, Catalog: application.Catalog{Repo: store}, Relations: application.Relations{Repo: store}, Confirmations: application.Confirmations{Repo: store}, Google: application.GoogleJobs{Queue: store}, GoogleQueries: application.GoogleQueries{Repo: store}}}
	if err := server.Serve(context.Background(), os.Stdin, os.Stdout); err != nil {
		log.Fatal(err)
	}
}
