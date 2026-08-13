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
	databaseURL, userID, impactSecret := os.Getenv("DATABASE_URL"), os.Getenv("TASMANE_MCP_USER_ID"), os.Getenv("TASMANE_IMPACT_TOKEN_SECRET")
	if databaseURL == "" || userID == "" || len(impactSecret) < 32 {
		log.Fatal("DATABASE_URL, TASMANE_MCP_USER_ID, and a 32+ byte TASMANE_IMPACT_TOKEN_SECRET are required")
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
	server := mcpserver.Server{UserID: userID, Services: mcpserver.Services{Tasks: application.Tasks{Repo: store}, Catalog: application.Catalog{Repo: store}, Relations: application.Relations{Repo: store}, Confirmations: application.Confirmations{Repo: store}, Google: application.GoogleJobs{Queue: store}, GoogleQueries: application.GoogleQueries{Repo: store}, ImpactTokens: application.ImpactTokens{Secret: []byte(impactSecret)}}}
	if err := server.Serve(context.Background(), os.Stdin, os.Stdout); err != nil {
		log.Fatal(err)
	}
}
