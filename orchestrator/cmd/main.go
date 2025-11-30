package main

import (
	"log"
	"net/http"

	"openpam/orchestrator/internal/api"

	"github.com/gorilla/mux"
)

func main() {
	log.Println("Starting Orchestrator Service on :8090")

	r := mux.NewRouter()
	api.RegisterRoutes(r)

	log.Fatal(http.ListenAndServe(":8090", r))
}
