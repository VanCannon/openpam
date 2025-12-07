package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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
	GroupFilter    string `json:"group_filter"`
}

type ConfigRequest struct {
	Host           string `json:"host"`
	Port           int    `json:"port"`
	BaseDN         string `json:"base_dn"`
	BindDN         string `json:"bind_dn"`
	BindPassword   string `json:"bind_password"`
	UserFilter     string `json:"user_filter"`
	ComputerFilter string `json:"computer_filter"`
	GroupFilter    string `json:"group_filter"`
}

func RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/api/v1/identity/sync", SyncAD).Methods("POST")
	r.HandleFunc("/api/v1/identity/config", SaveConfig).Methods("POST")
	r.HandleFunc("/api/v1/identity/config", GetConfig).Methods("GET")
	r.HandleFunc("/api/v1/users", GetUsers).Methods("GET")
	r.HandleFunc("/api/v1/computers", GetComputers).Methods("GET")
	r.HandleFunc("/api/v1/ad-users", GetADUsers).Methods("GET")
	r.HandleFunc("/api/v1/ad-computers", GetADComputers).Methods("GET")
	r.HandleFunc("/api/v1/ad-groups", GetADGroups).Methods("GET")
	r.HandleFunc("/api/v1/users/import", ImportADUser).Methods("POST")
	r.HandleFunc("/api/v1/groups/import", ImportADGroup).Methods("POST")
	r.HandleFunc("/api/v1/computers/import", ImportADComputer).Methods("POST")
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
	host, port, baseDN, bindDN, bindPassword, _, _, _, err := db.GetConfig()
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
			"groups":       getGroups(userEntry),
		},
	})
}

type ldapEntry interface {
	GetAttributeValues(string) []string
}

func getGroups(entry ldapEntry) string {
	// memberOf attribute contains list of DNs
	groups := entry.GetAttributeValues("memberOf")
	// Convert to JSON array string
	b, _ := json.Marshal(groups)
	return string(b)
}

func SaveConfig(w http.ResponseWriter, r *http.Request) {
	var req ConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := db.SaveConfig(req.Host, req.Port, req.BaseDN, req.BindDN, req.BindPassword, req.UserFilter, req.ComputerFilter, req.GroupFilter); err != nil {
		log.Printf("Failed to save config: %v", err)
		http.Error(w, "Failed to save config", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func GetConfig(w http.ResponseWriter, r *http.Request) {
	host, port, baseDN, bindDN, bindPassword, userFilter, computerFilter, groupFilter, err := db.GetConfig()
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
		GroupFilter:    groupFilter,
	})
}

func SyncAD(w http.ResponseWriter, r *http.Request) {
	// Try to get config from DB first
	host, port, baseDN, bindDN, bindPassword, userFilter, computerFilter, groupFilter, err := db.GetConfig()
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

	// Sync Groups
	ldapGroups, err := client.SearchGroups(groupFilter)
	if err != nil {
		log.Printf("Failed to search groups: %v", err)
	}

	// Parse AD Groups
	var adGroups []db.ADGroup
	for _, g := range ldapGroups {
		name := g.GetAttributeValue("name")
		id := uuid.NewSHA1(uuid.NameSpaceURL, []byte("ad-group:"+name)).String()
		members := g.GetAttributeValues("member")

		adGroups = append(adGroups, db.ADGroup{
			ID:          id,
			DN:          g.DN,
			Name:        name,
			Description: g.GetAttributeValue("description"),
			MemberCount: len(members),
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

	if err := db.SaveADGroups(adGroups); err != nil {
		log.Printf("Failed to save AD groups: %v", err)
		http.Error(w, "Failed to save AD groups", http.StatusInternalServerError)
		return
	}

	log.Printf("Synced %d users, %d computers, %d groups", len(adUsers), len(adComputers), len(adGroups))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":          "success",
		"users_count":     len(adUsers),
		"computers_count": len(adComputers),
		"groups_count":    len(adGroups),
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

func GetADGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := db.GetADGroups()
	if err != nil {
		log.Printf("Failed to get AD groups: %v", err)
		http.Error(w, "Failed to get AD groups", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"groups": groups,
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

func ImportADGroup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ADGroupID string `json:"ad_group_id"`
		Role      string `json:"role"`
	}
	// Debug logging
	bodyBytes, _ := io.ReadAll(r.Body)
	log.Printf("ImportADGroup received body: %s", string(bodyBytes))
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Failed to decode request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get AD group details
	adGroups, err := db.GetADGroups() // TODO: Optimize
	if err != nil {
		http.Error(w, "Failed to fetch AD groups", http.StatusInternalServerError)
		return
	}

	var targetGroup *db.ADGroup
	for _, g := range adGroups {
		if g.ID == req.ADGroupID {
			targetGroup = &g
			break
		}
	}

	if targetGroup == nil {
		log.Printf("AD group not found for ID: %s", req.ADGroupID)
		http.Error(w, "AD group not found", http.StatusNotFound)
		return
	}

	// Save to groups table
	group := db.Group{
		ID:          targetGroup.ID,
		Name:        targetGroup.Name,
		DN:          targetGroup.DN,
		Description: targetGroup.Description,
		Role:        req.Role,
		Source:      "active_directory",
	}

	if err := db.SaveGroups([]db.Group{group}); err != nil {
		log.Printf("Failed to import AD group: %v", err)
		http.Error(w, "Failed to import group", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func ImportADComputer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ADComputerID string `json:"ad_computer_id"`
		ZoneID       string `json:"zone_id"`
		Protocol     string `json:"protocol"`
		Port         int    `json:"port"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Defaults
	if req.Protocol == "" {
		req.Protocol = "rdp"
	}
	if req.Port == 0 {
		req.Port = 3389
	}

	// Get AD computer details
	adComputers, err := db.GetADComputers() // TODO: Optimize
	if err != nil {
		http.Error(w, "Failed to fetch AD computers", http.StatusInternalServerError)
		return
	}

	var targetComputer *db.ADComputer
	for _, c := range adComputers {
		if c.ID == req.ADComputerID {
			targetComputer = &c
			break
		}
	}

	if targetComputer == nil {
		log.Printf("AD computer not found for ID: %s", req.ADComputerID)
		http.Error(w, "AD computer not found", http.StatusNotFound)
		return
	}

	// Save to targets table
	target := db.Target{
		ID:          uuid.New().String(),
		ZoneID:      req.ZoneID,
		Name:        targetComputer.Name,
		Hostname:    targetComputer.DNSHostName,
		Protocol:    req.Protocol,
		Port:        req.Port,
		Description: fmt.Sprintf("Imported from AD: %s", targetComputer.DN),
		Enabled:     true,
	}

	if err := db.SaveTargets([]db.Target{target}); err != nil {
		log.Printf("Failed to import AD computer: %v", err)
		http.Error(w, "Failed to import computer", http.StatusInternalServerError)
		return
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
