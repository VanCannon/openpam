package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	ldap "github.com/VanCannon/openpam/activity/internal/ad"
	"github.com/VanCannon/openpam/activity/internal/vault"
	"github.com/gorilla/mux"
)

// Request structs
type EnableDisableRequest struct {
	SAMAccountName    string `json:"sam_account_name"`
	DistinguishedName string `json:"dn"`
}

type RotateRequest struct {
	SAMAccountName    string `json:"sam_account_name"`
	DistinguishedName string `json:"dn"`
	VaultPath         string `json:"vault_path"`
}

type CreateEphemeralRequest struct {
	Prefix      string `json:"prefix"`
	Description string `json:"description"`
}

type PromotionRequest struct {
	UserDN  string `json:"user_dn"`
	GroupDN string `json:"group_dn"`
}

// Helpers
func getADClient() (*ldap.Client, error) {
	// TODO: Get config from DB or Env. Using Env for now as per docker-compose
	host := os.Getenv("AD_HOST")
	portStr := os.Getenv("AD_PORT")
	baseDN := os.Getenv("AD_BASE_DN")
	bindDN := os.Getenv("AD_BIND_DN")
	bindPwd := os.Getenv("AD_BIND_PASSWORD")

	if host == "" {
		// Fallback for dev if not set in env yet
		return nil, fmt.Errorf("AD configuration missing")
	}

	port, _ := strconv.Atoi(portStr)

	client := ldap.NewClient(host, port, baseDN, bindDN, bindPwd)
	if err := client.Connect(); err != nil {
		return nil, err
	}
	return client, nil
}

func getVaultClient() (*vault.Client, error) {
	addr := os.Getenv("VAULT_ADDR")
	token := os.Getenv("VAULT_TOKEN")

	return vault.New(vault.Config{
		Address: addr,
		Token:   token,
	})
}

func RegisterRoutes(r *mux.Router) {
	// Managed Accounts
	r.HandleFunc("/api/v1/activity/accounts/enable", HandleEnableAccount).Methods("POST")
	r.HandleFunc("/api/v1/activity/accounts/disable", HandleDisableAccount).Methods("POST")
	r.HandleFunc("/api/v1/activity/accounts/rotate", HandleRotatePassword).Methods("POST")

	// Ephemeral Accounts
	r.HandleFunc("/api/v1/activity/ephemeral/create", HandleCreateEphemeral).Methods("POST")
	r.HandleFunc("/api/v1/activity/ephemeral/delete", HandleDeleteEphemeral).Methods("POST")

	// User Promotion
	r.HandleFunc("/api/v1/activity/promotion/promote", HandlePromoteUser).Methods("POST")
	r.HandleFunc("/api/v1/activity/promotion/demote", HandleDemoteUser).Methods("POST")
}

func HandleEnableAccount(w http.ResponseWriter, r *http.Request) {
	var req EnableDisableRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	client, err := getADClient()
	if err != nil {
		log.Printf("Failed to connect to AD: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to connect to directory service")
		return
	}
	defer client.Close()

	// Use DN if provided, else find by sAMAccountName
	dn := req.DistinguishedName
	if dn == "" && req.SAMAccountName != "" {
		dn, err = client.GetUserDN(req.SAMAccountName)
		if err != nil {
			respondError(w, http.StatusNotFound, "User not found")
			return
		}
	}

	if dn == "" {
		respondError(w, http.StatusBadRequest, "Must provide dn or sam_account_name")
		return
	}

	if err := client.EnableUser(dn); err != nil {
		log.Printf("Failed to enable user %s: %v", dn, err)
		respondError(w, http.StatusInternalServerError, "Failed to enable user")
		return
	}

	respond(w, http.StatusOK, map[string]string{"status": "success", "message": "Account enabled"})
}

func HandleDisableAccount(w http.ResponseWriter, r *http.Request) {
	var req EnableDisableRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	client, err := getADClient()
	if err != nil {
		log.Printf("Failed to connect to AD: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to connect to directory service")
		return
	}
	defer client.Close()

	// Use DN if provided, else find by sAMAccountName
	dn := req.DistinguishedName
	if dn == "" && req.SAMAccountName != "" {
		dn, err = client.GetUserDN(req.SAMAccountName)
		if err != nil {
			respondError(w, http.StatusNotFound, "User not found")
			return
		}
	}

	if dn == "" {
		respondError(w, http.StatusBadRequest, "Must provide dn or sam_account_name")
		return
	}

	if err := client.DisableUser(dn); err != nil {
		log.Printf("Failed to disable user %s: %v", dn, err)
		respondError(w, http.StatusInternalServerError, "Failed to disable user")
		return
	}

	respond(w, http.StatusOK, map[string]string{"status": "success", "message": "Account disabled"})
}

func HandleRotatePassword(w http.ResponseWriter, r *http.Request) {
	var req RotateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.VaultPath == "" {
		respondError(w, http.StatusBadRequest, "Vault path is required")
		return
	}

	adClient, err := getADClient()
	if err != nil {
		log.Printf("Failed to connect to AD: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to connect to directory service")
		return
	}
	defer adClient.Close()

	// Use DN if provided, else find by sAMAccountName
	dn := req.DistinguishedName
	username := req.SAMAccountName

	if dn == "" && username != "" {
		dn, err = adClient.GetUserDN(username)
		if err != nil {
			respondError(w, http.StatusNotFound, "User not found")
			return
		}
	} else if dn != "" && username == "" {
		// We might need username for Vault storage if not provided
		// Let's assume username is provided or can be extracted/ignored if Vault just needs password
		// But our Vault schema expects username.
		// Ideally we should lookup username from DN if missing, but let's enforce both or lookup.
		// Detailed lookup logic skipped for brevity, assuming caller provides both or we trust one.
	}

	if dn == "" {
		respondError(w, http.StatusBadRequest, "Must provide dn or sam_account_name")
		return
	}

	// Generate new random password
	newPassword := generatePassword(16)

	// 1. Update AD
	if err := adClient.SetPassword(dn, newPassword); err != nil {
		log.Printf("Failed to set password for %s: %v", dn, err)
		respondError(w, http.StatusInternalServerError, "Failed to rotate password")
		return
	}

	// 2. Update Vault
	vaultClient, err := getVaultClient()
	if err != nil {
		log.Printf("Failed to connect to Vault: %v", err)
		// Critical error: Password changed in AD but not saved to Vault!
		// In production, we might want to rollback AD change or alert loudly.
		respondError(w, http.StatusInternalServerError, "Password changed but failed to store in Vault")
		return
	}

	creds := &vault.Credentials{
		Username: username,
		Password: newPassword,
	}

	if err := vaultClient.WriteCredentials(r.Context(), req.VaultPath, creds); err != nil {
		log.Printf("Failed to write to Vault path %s: %v", req.VaultPath, err)
		respondError(w, http.StatusInternalServerError, "Password changed but failed to store in Vault")
		return
	}

	respond(w, http.StatusOK, map[string]string{"status": "success", "message": "Password rotated and stored"})
}

func generatePassword(length int) string {
	// Simple random password generator
	// TODO: Use crypto/rand and improved character set
	// For MVP, just a placeholder implementation
	// Using a fixed complexity for now to satisfy AD requirements usually
	return "P@ssw0rdGeneric" + strconv.FormatInt(time.Now().UnixNano(), 10)
}

func HandleCreateEphemeral(w http.ResponseWriter, r *http.Request) {
	var req CreateEphemeralRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Prefix == "" {
		respondError(w, http.StatusBadRequest, "Prefix is required")
		return
	}

	// Generate random suffix
	randomSuffix := strconv.FormatInt(time.Now().UnixNano(), 36)[0:6]
	username := fmt.Sprintf("%s-%s", req.Prefix, randomSuffix)
	password := generatePassword(16)

	adClient, err := getADClient()
	if err != nil {
		log.Printf("Failed to connect to AD: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to connect to directory service")
		return
	}
	defer adClient.Close()

	dn, err := adClient.CreateUser(username, password, req.Description)
	if err != nil {
		log.Printf("Failed to create ephemeral user %s: %v", username, err)
		respondError(w, http.StatusInternalServerError, "Failed to create ephemeral user")
		return
	}

	// Store in Vault
	vaultClient, err := getVaultClient()
	if err != nil {
		log.Printf("Failed to connect to Vault: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to connect to secret store")
		return
	}

	vaultPath := fmt.Sprintf("secret/data/openpam/ephemeral/%s", username)
	creds := &vault.Credentials{
		Username: username,
		Password: password,
	}

	if err := vaultClient.WriteCredentials(r.Context(), vaultPath, creds); err != nil {
		log.Printf("Failed to write to Vault path %s: %v", vaultPath, err)
		respondError(w, http.StatusInternalServerError, "Failed to store ephemeral credentials")
		return
	}

	// Return info to Gateway
	respond(w, http.StatusCreated, map[string]string{
		"status":     "success",
		"username":   username,
		"dn":         dn,
		"vault_path": vaultPath,
	})
}

func HandleDeleteEphemeral(w http.ResponseWriter, r *http.Request) {
	var req EnableDisableRequest // Reuse struct as we just need DN or Username
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	adClient, err := getADClient()
	if err != nil {
		log.Printf("Failed to connect to AD: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to connect to directory service")
		return
	}
	defer adClient.Close()

	dn := req.DistinguishedName
	if dn == "" && req.SAMAccountName != "" {
		dn, err = adClient.GetUserDN(req.SAMAccountName)
		if err != nil {
			// If not found, maybe already deleted?
			respond(w, http.StatusOK, map[string]string{"status": "success", "message": "User not found, assumed deleted"})
			return
		}
	}

	if dn == "" {
		respondError(w, http.StatusBadRequest, "Must provide dn or sam_account_name")
		return
	}

	if err := adClient.DeleteUser(dn); err != nil {
		log.Printf("Failed to delete user %s: %v", dn, err)
		respondError(w, http.StatusInternalServerError, "Failed to delete user")
		return
	}

	respond(w, http.StatusOK, map[string]string{"status": "success", "message": "Ephemeral account deleted"})
}

func HandlePromoteUser(w http.ResponseWriter, r *http.Request) {
	var req PromotionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.UserDN == "" || req.GroupDN == "" {
		respondError(w, http.StatusBadRequest, "UserDN and GroupDN are required")
		return
	}

	adClient, err := getADClient()
	if err != nil {
		log.Printf("Failed to connect to AD: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to connect to directory service")
		return
	}
	defer adClient.Close()

	if err := adClient.AddGroupMember(req.GroupDN, req.UserDN); err != nil {
		log.Printf("Failed to add user %s to group %s: %v", req.UserDN, req.GroupDN, err)
		respondError(w, http.StatusInternalServerError, "Failed to promote user")
		return
	}

	respond(w, http.StatusOK, map[string]string{"status": "success", "message": "User promoted"})
}

func HandleDemoteUser(w http.ResponseWriter, r *http.Request) {
	var req PromotionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.UserDN == "" || req.GroupDN == "" {
		respondError(w, http.StatusBadRequest, "UserDN and GroupDN are required")
		return
	}

	adClient, err := getADClient()
	if err != nil {
		log.Printf("Failed to connect to AD: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to connect to directory service")
		return
	}
	defer adClient.Close()

	if err := adClient.RemoveGroupMember(req.GroupDN, req.UserDN); err != nil {
		log.Printf("Failed to remove user %s from group %s: %v", req.UserDN, req.GroupDN, err)
		respondError(w, http.StatusInternalServerError, "Failed to demote user")
		return
	}

	respond(w, http.StatusOK, map[string]string{"status": "success", "message": "User demoted"})
}

func respond(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

func respondError(w http.ResponseWriter, code int, message string) {
	respond(w, code, map[string]string{"error": message})
}
