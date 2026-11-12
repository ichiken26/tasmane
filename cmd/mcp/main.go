package main

import (
	"context"
	// distroless でもユーザーTimezoneを解決できるようIANA tzdataを同梱
	"log"
	"os"
	"tasmane/db"
	"tasmane/internal/application"
	"tasmane/internal/repository"
	mcpserver "tasmane/mcp"
	_ "time/tzdata"
)

// MCP サーバプロセスの起動
func main() {
	// 必須環境変数の検証
	databaseURL, userID, impactSecret := os.Getenv("DATABASE_URL"), os.Getenv("TASMANE_MCP_USER_ID"), os.Getenv("TASMANE_IMPACT_TOKEN_SECRET")
	if databaseURL == "" || userID == "" || len(impactSecret) < 32 {
		log.Fatal("DATABASE_URL, TASMANE_MCP_USER_ID, and a 32+ byte TASMANE_IMPACT_TOKEN_SECRET are required")
	}
	// PostgreSQL 接続の確立
	// 接続とマイグレーション
	database, err := db.OpenMigrated(context.Background(), databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()
	store := repository.NewPostgres(database, nil)
	server := mcpserver.Server{UserID: userID, Services: mcpserver.Services{Tasks: application.Tasks{Repo: store}, Catalog: application.Catalog{Repo: store}, Relations: application.Relations{Repo: store}, Confirmations: application.Confirmations{Repo: store}, Google: application.GoogleJobs{Queue: store}, GoogleQueries: application.GoogleQueries{Repo: store}, ImpactTokens: application.ImpactTokens{Secret: []byte(impactSecret)}}}
	// stdio MCP サーバの待受
	if err := server.Serve(context.Background(), os.Stdin, os.Stdout); err != nil {
		log.Fatal(err)
	}
}
