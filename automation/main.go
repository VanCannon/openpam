package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/VanCannon/openpam/automation/internal/ansible"
)

type PlaybookExecutionRequest struct {
	Target         string            `json:"target"`    // Can be empty if inventory provided
	Inventory      string            `json:"inventory"` // Name of inventory file in playbooks dir
	Playbook       string            `json:"playbook"`  // Name of playbook or module
	Vars           map[string]string `json:"vars"`
	BecomePassword string            `json:"become_password"` // Sudo password
	SSHPassword    string            `json:"ssh_password"`    // SSH login password
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8084"
	}

	ansibleRunner := ansible.NewRunner()

	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "healthy",
			"service": "automation-service",
		})
	})

	// Execution API
	mux.HandleFunc("/api/v1/automation/playbooks/execute", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req PlaybookExecutionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Target == "" && req.Inventory == "" {
			http.Error(w, "Target or Inventory is required", http.StatusBadRequest)
			return
		}

		var output string
		var err error

		if req.Vars == nil {
			req.Vars = make(map[string]string)
		}

		if req.BecomePassword != "" {
			req.Vars["ansible_become_password"] = req.BecomePassword
			req.Vars["ansible_become_pass"] = req.BecomePassword
		}

		if req.SSHPassword != "" {
			req.Vars["ansible_ssh_pass"] = req.SSHPassword
			req.Vars["ansible_password"] = req.SSHPassword
		}

		// Check if the 'Playbook' field looks like a file (ends in .yml or .yaml)
		if strings.HasSuffix(req.Playbook, ".yml") || strings.HasSuffix(req.Playbook, ".yaml") {
			// It's a playbook file
			// We assume the playbook is in the mounted directory /ansible/playbooks
			playbookPath := fmt.Sprintf("/ansible/playbooks/%s", req.Playbook)

			var inventoryPath string
			if req.Inventory != "" {
				inventoryPath = fmt.Sprintf("/ansible/playbooks/%s", req.Inventory)
			}

			log.Printf("Executing playbook %s against %s (inventory: %s)", playbookPath, req.Target, inventoryPath)

			output, err = ansibleRunner.ExecutePlaybook(req.Target, inventoryPath, playbookPath, req.Vars)
		} else {
			// It's an ad-hoc module
			// Default to "ping" module if no playbook/module specified for this logical test
			module := "ping"
			if req.Playbook != "" {
				module = req.Playbook
			}

			log.Printf("Executing ansible module %s against %s", module, req.Target)
			output, err = ansibleRunner.ExecuteAdHoc(req.Target, module, "")
		}

		if err != nil {
			log.Printf("Execution failed: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"status": "error",
				"error":  err.Error(),
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status": "success",
			"output": output,
		})
	})

	serverAddr := fmt.Sprintf("0.0.0.0:%s", port)
	log.Printf("Automation Service starting on %s...", serverAddr)
	if err := http.ListenAndServe(serverAddr, mux); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
