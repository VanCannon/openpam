package main

import (
	"log"
	"net/http"
	"openpam/identity/internal/api"
	"openpam/identity/internal/db"

	"github.com/gorilla/mux"
)

func main() {
	log.Println("Starting Identity Service on :8082")

	if err := db.InitDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	r := mux.NewRouter()
	api.RegisterRoutes(r)

	log.Fatal(http.ListenAndServe(":8082", r))
}
