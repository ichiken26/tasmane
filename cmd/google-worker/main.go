package main

import (
	"context"
	"encoding/base64"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	// distroless でもユーザーTimezoneを解決できるようIANA tzdataを同梱
	"tasmane/db"
	"tasmane/internal/application"
	googleintegration "tasmane/internal/integration/google"
	"tasmane/internal/repository"
	_ "time/tzdata"
)

// Google Tasks 同期ワーカの起動
func main() {
	// 必須環境変数と暗号鍵の検証
	databaseURL := os.Getenv("DATABASE_URL")
	clientID := os.Getenv("GOOGLE_OAUTH_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET")
	key, err := encryptionKey(os.Getenv("TASMANE_GOOGLE_TOKEN_ENCRYPTION_KEY"))
	if databaseURL == "" || clientID == "" || clientSecret == "" || err != nil {
		log.Fatal("DATABASE_URL, GOOGLE_OAUTH_CLIENT_ID, GOOGLE_OAUTH_CLIENT_SECRET, and a 32-byte TASMANE_GOOGLE_TOKEN_ENCRYPTION_KEY are required")
	}
	// PostgreSQL 接続の確立
	database, err := db.OpenMigrated(context.Background(), databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()
	cipher, err := googleintegration.NewTokenCipher(key)
	if err != nil {
		log.Fatal(err)
	}
	store := repository.NewPostgres(database, nil)
	client := &googleintegration.Client{Store: store, Tasks: application.Tasks{Repo: store}, Cipher: cipher, ClientID: clientID, ClientSecret: clientSecret}
	consumer := application.GoogleConsumer{Client: client, Store: store, Queue: store, MaxAttempts: 5}
	workerID, err := os.Hostname()
	if err != nil || workerID == "" {
		workerID = "google-worker"
	}
	// SIGINT/SIGTERM 待ちのジョブ消費ループ
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	for ctx.Err() == nil {
		// 同期ジョブのクレームと消費
		job, ok, err := store.Claim(ctx, workerID, time.Now().UTC(), 2*time.Minute)
		if err != nil {
			log.Printf("claim google sync job: %v", err)
			wait(ctx, time.Second)
			continue
		}
		if !ok {
			wait(ctx, time.Second)
			continue
		}
		if err := consumer.Consume(ctx, job); err != nil {
			log.Printf("consume google sync job %s: %v", job.ID, err)
			continue
		}
		if err := store.Ack(ctx, job.ID, workerID); err != nil {
			log.Printf("ack google sync job %s: %v", job.ID, err)
		}
	}
}

// Google トークン暗号化鍵の正規化
func encryptionKey(value string) ([]byte, error) {
	if len(value) == 32 {
		return []byte(value), nil
	}
	decoded, err := base64.RawStdEncoding.DecodeString(value)
	if err == nil && len(decoded) == 32 {
		return decoded, nil
	}
	decoded, err = base64.StdEncoding.DecodeString(value)
	if err == nil && len(decoded) == 32 {
		return decoded, nil
	}
	return nil, errors.New("invalid encryption key")
}

// キャンセル可能な待機
func wait(ctx context.Context, duration time.Duration) {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}
