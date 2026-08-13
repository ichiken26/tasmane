package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"tasmane/db"
	"tasmane/internal/application"
	googleintegration "tasmane/internal/integration/google"
	"tasmane/internal/repository"
	"tasmane/internal/transport/httpapi"
)

func main() {
	secret := os.Getenv("TASMANE_INTERNAL_BINDING_SECRET")
	impactSecret := os.Getenv("TASMANE_IMPACT_TOKEN_SECRET")
	tokenKeyEncoded := os.Getenv("TASMANE_GOOGLE_TOKEN_ENCRYPTION_KEY")
	databaseURL := os.Getenv("DATABASE_URL")
	if secret == "" || impactSecret == "" || len(impactSecret) < 32 || tokenKeyEncoded == "" || databaseURL == "" {
		log.Fatal("TASMANE_INTERNAL_BINDING_SECRET, 32-byte TASMANE_IMPACT_TOKEN_SECRET, base64 TASMANE_GOOGLE_TOKEN_ENCRYPTION_KEY, and DATABASE_URL are required")
	}
	tokenKey, err := base64.StdEncoding.DecodeString(tokenKeyEncoded)
	if err != nil {
		log.Fatal("TASMANE_GOOGLE_TOKEN_ENCRYPTION_KEY must be standard base64 for the API process")
	}
	tokenCipher, err := googleintegration.NewTokenCipher(tokenKey)
	if err != nil {
		log.Fatal(err)
	}
	database, err := sql.Open("pgx", databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()
	startupContext, cancelStartup := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelStartup()
	if err := database.PingContext(startupContext); err != nil {
		log.Fatal(err)
	}
	if err := db.Migrate(startupContext, database); err != nil {
		log.Fatal(err)
	}

	store := repository.NewPostgres(database, nil)
	handler := httpapi.NewRouter(httpapi.Handler{
		Tasks:         application.Tasks{Repo: store},
		Catalog:       application.Catalog{Repo: store},
		Relations:     application.Relations{Repo: store},
		Confirmations: application.Confirmations{Repo: store},
		ImpactTokens:  application.ImpactTokens{Secret: []byte(impactSecret)},
		GoogleJobs:    application.GoogleJobs{Queue: store},
		GoogleQueries: application.GoogleQueries{Repo: store},
		Advices:       application.Advices{Repo: store},
	})
	mux := http.NewServeMux()
	mux.Handle("/internal/auth/bootstrap", httpapi.NewAuthBootstrapHandler(database, tokenCipher, secret))
	mux.Handle("/", httpapi.InternalAuth(secret, handler))
	server := &http.Server{
		Addr:              "127.0.0.1:8080",
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-stop
		shutdownContext, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownContext); err != nil {
			log.Printf("shutdown: %v", err)
		}
	}()
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
