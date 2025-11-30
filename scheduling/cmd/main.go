package main

import (
	"log"
	"net/http"

	"openpam/scheduling/internal/api"

	"github.com/gorilla/mux"
)

func main() {
	log.Println("Starting Scheduling Service on :8081")

	r := mux.NewRouter()
	api.RegisterRoutes(r)

	log.Fatal(http.ListenAndServe(":8081", r))
}
