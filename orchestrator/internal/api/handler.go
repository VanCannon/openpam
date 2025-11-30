package api

import (
	"bytes"
	"fmt"
	"net/http"
	"os"

	"github.com/gorilla/mux"
)

func RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/api/v1/orchestrator/sync/ad", TriggerADSync).Methods("POST")
}

func TriggerADSync(w http.ResponseWriter, r *http.Request) {
	// In a real scenario, we'd fetch config from DB or receive it here
	// For now, we'll forward the request body to the Identity Service

	// Get Identity Service URL from env, default to docker service name
	identityServiceURL := os.Getenv("IDENTITY_SERVICE_URL")
	if identityServiceURL == "" {
		identityServiceURL = "http://identity:8082/api/v1/identity/sync"
	}

	// Forward request
	resp, err := http.Post(identityServiceURL, "application/json", r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to call Identity Service: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Copy response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)

	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	w.Write(buf.Bytes())
}
