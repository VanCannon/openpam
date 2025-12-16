package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/VanCannon/openpam/orchestrator/internal/api"
	"github.com/VanCannon/openpam/orchestrator/internal/worker"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
)

func main() {
	log.Println("Starting Orchestrator Service on :8090")

	// Connect to database
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbname := os.Getenv("DB_NAME")

	psqlInfo := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err = db.Ping(); err != nil {
		log.Fatal(err)
	}
	log.Println("Connected to database")

	// Start Worker
	w := worker.NewWorker(db, 30*time.Second) // Check every 30 seconds
	go w.Start()
	defer w.Stop()

	r := mux.NewRouter()
	api.RegisterRoutes(r)

	log.Fatal(http.ListenAndServe(":8090", r))
}
