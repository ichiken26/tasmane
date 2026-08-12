package main

import (
	"log"
	"net/http"
	"os"
	"tasmane/internal/application"
	"tasmane/internal/repository"
	"tasmane/internal/transport/httpapi"
)

func main() {
	secret := os.Getenv("TASMANE_INTERNAL_BINDING_SECRET")
	if secret == "" {
		log.Fatal("TASMANE_INTERNAL_BINDING_SECRET is required")
	}
	h := httpapi.Handler{Tasks: application.Tasks{Repo: repository.NewMemory()}}
	log.Fatal(http.ListenAndServe("127.0.0.1:8080", httpapi.InternalAuth(secret, h)))
}
