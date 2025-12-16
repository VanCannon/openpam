package worker

import (
	"database/sql"
	"log"
	"time"

	_ "github.com/lib/pq"
)

type Worker struct {
	db       *sql.DB
	interval time.Duration
	stopChan chan struct{}
}

func NewWorker(db *sql.DB, interval time.Duration) *Worker {
	return &Worker{
		db:       db,
		interval: interval,
		stopChan: make(chan struct{}),
	}
}

func (w *Worker) Start() {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	log.Println("Orchestrator worker started")

	for {
		select {
		case <-ticker.C:
			w.processSchedules()
		case <-w.stopChan:
			log.Println("Orchestrator worker stopped")
			return
		}
	}
}

func (w *Worker) Stop() {
	close(w.stopChan)
}

func (w *Worker) processSchedules() {
	// 1. Provision pending active schedules
	if err := w.provisionSchedules(); err != nil {
		log.Printf("Failed to provision schedules: %v", err)
	}

	// 2. Deprovision expired schedules
	if err := w.deprovisionSchedules(); err != nil {
		log.Printf("Failed to deprovision schedules: %v", err)
	}
}

func (w *Worker) provisionSchedules() error {
	rows, err := w.db.Query(`
		SELECT id, user_id, type, account_type, account_details
		FROM schedules
		WHERE status = 'active' AND provision_status = 'pending'
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id, userID, scheduleType, accountType string
		var accountDetailsJSON []byte
		if err := rows.Scan(&id, &userID, &scheduleType, &accountType, &accountDetailsJSON); err != nil {
			log.Printf("Failed to scan schedule: %v", err)
			continue
		}

		log.Printf("Provisioning schedule %s (type: %s, account: %s)", id, scheduleType, accountType)

		// Perform provisioning logic based on AccountType
		if err := w.provision(userID, accountType, accountDetailsJSON); err != nil {
			log.Printf("Failed to provision schedule %s: %v", id, err)
			continue
		}

		// Update provision_status
		_, err := w.db.Exec(`UPDATE schedules SET provision_status = 'provisioned' WHERE id = $1`, id)
		if err != nil {
			log.Printf("Failed to update provision_status for schedule %s: %v", id, err)
		}
	}
	return nil
}

func (w *Worker) deprovisionSchedules() error {
	rows, err := w.db.Query(`
		SELECT id, user_id, type, account_type, account_details
		FROM schedules
		WHERE status = 'expired' AND provision_status = 'provisioned'
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id, userID, scheduleType, accountType string
		var accountDetailsJSON []byte
		if err := rows.Scan(&id, &userID, &scheduleType, &accountType, &accountDetailsJSON); err != nil {
			log.Printf("Failed to scan schedule: %v", err)
			continue
		}

		log.Printf("Deprovisioning schedule %s (type: %s, account: %s)", id, scheduleType, accountType)

		// Perform deprovisioning logic based on AccountType
		if err := w.deprovision(userID, accountType, accountDetailsJSON); err != nil {
			log.Printf("Failed to deprovision schedule %s: %v", id, err)
			continue
		}

		// Update provision_status
		_, err := w.db.Exec(`UPDATE schedules SET provision_status = 'deprovisioned' WHERE id = $1`, id)
		if err != nil {
			log.Printf("Failed to update provision_status for schedule %s: %v", id, err)
		}
	}
	return nil
}

func (w *Worker) provision(userID, accountType string, detailsJSON []byte) error {
	switch accountType {
	case "user_promotion":
		// TODO: Call Identity Service to add user to AD group
		log.Printf("Adding user %s to AD group (stub)", userID)
	case "ephemeral":
		// TODO: Call Identity Service to create ephemeral user
		log.Printf("Creating ephemeral user for %s (stub)", userID)
	case "managed":
		// TODO: Checkout credentials
		log.Printf("Checking out credentials for %s (stub)", userID)
	}
	return nil
}

func (w *Worker) deprovision(userID, accountType string, detailsJSON []byte) error {
	switch accountType {
	case "user_promotion":
		// TODO: Call Identity Service to remove user from AD group
		log.Printf("Removing user %s from AD group (stub)", userID)
	case "ephemeral":
		// TODO: Call Identity Service to delete ephemeral user
		log.Printf("Deleting ephemeral user for %s (stub)", userID)
	case "managed":
		// TODO: Checkin credentials
		log.Printf("Checking in credentials for %s (stub)", userID)
	}
	return nil
}
