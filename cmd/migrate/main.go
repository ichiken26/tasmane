package main

import (
	"context"
	"fmt"
	"os"
	"tasmane/db"
)

// 埋め込みマイグレーションの適用プロセス
func main() {
	// 必須環境変数の検証
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is required")
		os.Exit(2)
	}
	// PostgreSQL 接続とマイグレーション適用
	database, err := db.OpenMigrated(context.Background(), databaseURL)
	if err == nil {
		err = database.Close()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
