package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"openpam/identity/internal/db"
	"openpam/identity/internal/ldap"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type SyncRequest struct {
	Host           string `json:"host"`
	Port           int    `json:"port"`
	BaseDN         string `json:"base_dn"`
	BindDN         string `json:"bind_dn"`
	BindPassword   string `json:"bind_password"`
	UserFilter     string `json:"user_filter"`
	ComputerFilter string `json:"computer_filter"`
}

type ConfigRequest struct {
	Host           string `json:"host"`
	Port           int    `json:"port"`
	BaseDN         string `json:"base_dn"`
	BindDN         string `json:"bind_dn"`
	BindPassword   string `json:"bind_password"`
	UserFilter     string `json:"user_filter"`
	ComputerFilter string `json:"computer_filter"`
}

func RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/api/v1/identity/sync", SyncAD).Methods("POST")
	r.HandleFunc("/api/v1/identity/config", SaveConfig).Methods("POST")
	r.HandleFunc("/api/v1/identity/config", GetConfig).Methods("GET")
	r.HandleFunc("/api/v1/users", GetUsers).Methods("GET")
	r.HandleFunc("/api/v1/computers", GetComputers).Methods("GET")
	r.HandleFunc("/api/v1/ad-users", GetADUsers).Methods("GET")
	r.HandleFunc("/api/v1/ad-computers", GetADComputers).Methods("GET")
	r.HandleFunc("/api/v1/users/import", ImportADUser).Methods("POST")
	r.HandleFunc("/api/v1/managed-accounts", GetManagedAccounts).Methods("GET")
	r.HandleFunc("/api/v1/identity/auth", VerifyCredentials).Methods("POST")
}

func VerifyCredentials(w http.ResponseWriter, r *http.Request) {
	var creds struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get config from DB
	host, port, baseDN, bindDN, bindPassword, _, _, err := db.GetConfig()
	if err != nil {
		log.Printf("Failed to get config for auth: %v", err)
		http.Error(w, "Failed to get configuration", http.StatusInternalServerError)
		return
	}

	if host == "" {
		http.Error(w, "AD configuration not found", http.StatusBadRequest)
		return
	}

	client := ldap.NewClient(host, port, baseDN, bindDN, bindPassword)
	if err := client.Connect(); err != nil {
		log.Printf("Failed to connect to LDAP: %v", err)
		http.Error(w, "Failed to connect to directory service", http.StatusInternalServerError)
		return
	}
	defer client.Close()

	// Authenticate
	userEntry, err := client.Authenticate(creds.Username, creds.Password)
	if err != nil {
		log.Printf("Authentication failed for user %s: %v", creds.Username, err)
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Return user details
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"valid": true,
		"user": map[string]string{
			"entra_id":     userEntry.GetAttributeValue("sAMAccountName"),
			"email":        userEntry.GetAttributeValue("mail"),
			"display_name": userEntry.GetAttributeValue("displayName"),
		},
	})
}

func SaveConfig(w http.ResponseWriter, r *http.Request) {
	var req ConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := db.SaveConfig(req.Host, req.Port, req.BaseDN, req.BindDN, req.BindPassword, req.UserFilter, req.ComputerFilter); err != nil {
		log.Printf("Failed to save config: %v", err)
		http.Error(w, "Failed to save config", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func GetConfig(w http.ResponseWriter, r *http.Request) {
	host, port, baseDN, bindDN, bindPassword, userFilter, computerFilter, err := db.GetConfig()
	if err != nil {
		log.Printf("Failed to get config: %v", err)
		http.Error(w, "Failed to get config", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ConfigRequest{
		Host:           host,
		Port:           port,
		BaseDN:         baseDN,
		BindDN:         bindDN,
		BindPassword:   bindPassword,
		UserFilter:     userFilter,
		ComputerFilter: computerFilter,
	})
}

func SyncAD(w http.ResponseWriter, r *http.Request) {
	// Try to get config from DB first
	host, port, baseDN, bindDN, bindPassword, userFilter, computerFilter, err := db.GetConfig()
	if err != nil {
		log.Printf("Failed to get config for sync: %v", err)
		http.Error(w, "Failed to get config", http.StatusInternalServerError)
		return
	}

	if host == "" {
		http.Error(w, "AD configuration not found", http.StatusBadRequest)
		return
	}

	client := ldap.NewClient(host, port, baseDN, bindDN, bindPassword)
	if err := client.Connect(); err != nil {
		log.Printf("Failed to connect to LDAP: %v", err)
		http.Error(w, "Failed to connect to LDAP", http.StatusInternalServerError)
		return
	}
	defer client.Close()

	// Sync Users
	ldapUsers, err := client.SearchUsers(userFilter)
	if err != nil {
		log.Printf("Failed to search users: %v", err)
		http.Error(w, "Failed to search users", http.StatusInternalServerError)
		return
	}

	// Parse AD Users
	var adUsers []db.ADUser
	for _, u := range ldapUsers {
		username := u.GetAttributeValue("sAMAccountName")
		// Generate deterministic UUID for ID
		id := uuid.NewSHA1(uuid.NameSpaceURL, []byte("ad-user:"+username)).String()

		// Parse UAC
		uacStr := u.GetAttributeValue("userAccountControl")
		status := "Active"
		passwordStatus := "Normal"

		if uacStr != "" {
			uac, err := strconv.Atoi(uacStr)
			if err == nil {
				// Status
				if uac&2 != 0 { // ACCOUNTDISABLE
					status = "Disabled"
				} else if uac&16 != 0 { // LOCKOUT
					status = "Locked Out"
				}

				// Password Status
				if uac&65536 != 0 { // DONT_EXPIRE_PASSWORD
					passwordStatus = "Never Expires"
				} else if uac&262144 != 0 { // SMARTCARD_REQUIRED
					passwordStatus = "Smart Card Required"
				}
			}
		}

		// Check pwdLastSet for Password Expired
		pwdLastSet := u.GetAttributeValue("pwdLastSet")
		if pwdLastSet == "0" {
			status = "Password Expired"
		}

		adUsers = append(adUsers, db.ADUser{
			ID:                id,
			DN:                u.DN,
			SAMAccountName:    username,
			UserPrincipalName: u.GetAttributeValue("userPrincipalName"),
			DisplayName:       u.GetAttributeValue("displayName"),
			Mail:              u.GetAttributeValue("mail"),
			OU:                parseOU(u.DN),
			Status:            status,
			PasswordStatus:    passwordStatus,
		})
	}

	// Sync Computers
	ldapComputers, err := client.SearchComputers(computerFilter)
	if err != nil {
		log.Printf("Failed to search computers: %v", err)
	}

	// Parse AD Computers
	var adComputers []db.ADComputer
	for _, c := range ldapComputers {
		name := c.GetAttributeValue("name")
		id := uuid.NewSHA1(uuid.NameSpaceURL, []byte("ad-computer:"+name)).String()

		adComputers = append(adComputers, db.ADComputer{
			ID:                     id,
			DN:                     c.DN,
			Name:                   name,
			DNSHostName:            c.GetAttributeValue("dNSHostName"),
			OperatingSystem:        c.GetAttributeValue("operatingSystem"),
			OperatingSystemVersion: c.GetAttributeValue("operatingSystemVersion"),
		})
	}

	// Save to DB
	if err := db.SaveADUsers(adUsers); err != nil {
		log.Printf("Failed to save AD users: %v", err)
		http.Error(w, "Failed to save AD users", http.StatusInternalServerError)
		return
	}

	if err := db.SaveADComputers(adComputers); err != nil {
		log.Printf("Failed to save AD computers: %v", err)
		http.Error(w, "Failed to save AD computers", http.StatusInternalServerError)
		return
	}

	log.Printf("Synced %d users and %d computers", len(adUsers), len(adComputers))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":          "success",
		"users_count":     len(adUsers),
		"computers_count": len(adComputers),
	})
}

func parseOU(dn string) string {
	// Extract OU from DN (e.g., "CN=User,OU=IT,DC=example,DC=com" -> "IT")
	// This is a simplified parser
	parts := strings.Split(dn, ",")
	for _, part := range parts {
		if strings.HasPrefix(strings.TrimSpace(part), "OU=") {
			return strings.TrimPrefix(strings.TrimSpace(part), "OU=")
		}
	}
	return ""
}

func GetADUsers(w http.ResponseWriter, r *http.Request) {
	users, err := db.GetADUsers()
	if err != nil {
		log.Printf("Failed to get AD users: %v", err)
		http.Error(w, "Failed to get AD users", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"users": users,
	})
}

func GetADComputers(w http.ResponseWriter, r *http.Request) {
	computers, err := db.GetADComputers()
	if err != nil {
		log.Printf("Failed to get AD computers: %v", err)
		http.Error(w, "Failed to get AD computers", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"computers": computers,
	})
}

func ImportADUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ADUserID string `json:"ad_user_id"`
		Role     string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get AD user details
	adUsers, err := db.GetADUsers() // TODO: Optimize to get single user
	if err != nil {
		http.Error(w, "Failed to fetch AD users", http.StatusInternalServerError)
		return
	}

	var targetUser *db.ADUser
	for _, u := range adUsers {
		if u.ID == req.ADUserID {
			targetUser = &u
			break
		}
	}

	if targetUser == nil {
		log.Printf("AD user not found for ID: %s", req.ADUserID)
		http.Error(w, "AD user not found", http.StatusNotFound)
		return
	}

	// Create OpenPAM user or Managed Account
	// Check if email exists, fallback to UPN or dummy
	email := targetUser.Mail
	if email == "" {
		email = targetUser.UserPrincipalName
	}
	if email == "" {
		email = fmt.Sprintf("%s@ad.local", targetUser.SAMAccountName)
	}

	if req.Role == "managed" {
		// Save to managed_accounts table
		account := db.ManagedAccount{
			ID:          targetUser.ID,
			EntraID:     targetUser.SAMAccountName,
			Email:       email,
			DisplayName: targetUser.DisplayName,
			Source:      "active_directory",
		}

		if err := db.SaveManagedAccounts([]db.ManagedAccount{account}); err != nil {
			log.Printf("Failed to import managed account: %v", err)
			http.Error(w, "Failed to import managed account", http.StatusInternalServerError)
			return
		}
	} else {
		// Save to users table
		user := db.User{
			ID:          targetUser.ID, // Use same ID
			EntraID:     targetUser.SAMAccountName,
			Email:       email,
			DisplayName: targetUser.DisplayName,
			Role:        req.Role,
			Enabled:     true, // Default to enabled
			Source:      "active_directory",
		}

		if err := db.SaveUsers([]db.User{user}); err != nil {
			log.Printf("Failed to import AD user: %v", err)
			http.Error(w, "Failed to import user", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func GetUsers(w http.ResponseWriter, r *http.Request) {
	users, err := db.GetUsers()
	if err != nil {
		log.Printf("Failed to get users: %v", err)
		http.Error(w, "Failed to get users", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"users": users,
	})
}

func GetManagedAccounts(w http.ResponseWriter, r *http.Request) {
	accounts, err := db.GetManagedAccounts()
	if err != nil {
		log.Printf("Failed to get managed accounts: %v", err)
		http.Error(w, "Failed to get managed accounts", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"accounts": accounts,
	})
}

func GetComputers(w http.ResponseWriter, r *http.Request) {
	computers, err := db.GetComputers()
	if err != nil {
		log.Printf("Failed to get computers: %v", err)
		http.Error(w, "Failed to get computers", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"computers": computers,
	})
}
