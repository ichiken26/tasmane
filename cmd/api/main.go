package main

import (
	"context"
	"database/sql"
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
	"tasmane/internal/repository"
	"tasmane/internal/transport/httpapi"
)

func main() {
	secret := os.Getenv("TASMANE_INTERNAL_BINDING_SECRET")
	databaseURL := os.Getenv("DATABASE_URL")
	if secret == "" || databaseURL == "" {
		log.Fatal("TASMANE_INTERNAL_BINDING_SECRET and DATABASE_URL are required")
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
	})
	server := &http.Server{
		Addr:              "127.0.0.1:8080",
		Handler:           httpapi.InternalAuth(secret, handler),
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
