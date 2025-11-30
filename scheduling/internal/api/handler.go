package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

type ScheduleRequest struct {
	JobType  string `json:"job_type"` // e.g., "ad_sync"
	Interval string `json:"interval"` // e.g., "daily", "0 0 * * *"
	Payload  string `json:"payload"`  // JSON payload for the job
}

func RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/api/v1/schedules", CreateSchedule).Methods("POST")
}

func CreateSchedule(w http.ResponseWriter, r *http.Request) {
	var req ScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// TODO: Register job with a scheduler library (e.g., robfig/cron)
	log.Printf("Scheduled job '%s' with interval '%s'", req.JobType, req.Interval)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "schedule_created"})
}
