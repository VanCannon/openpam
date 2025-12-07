package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
)

var DB *sql.DB

func InitDB() error {
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbname := os.Getenv("DB_NAME")

	psqlInfo := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	var err error
	DB, err = sql.Open("postgres", psqlInfo)
	if err != nil {
		return fmt.Errorf("failed to open database: %v", err)
	}

	if err = DB.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %v", err)
	}

	log.Println("Connected to database")
	return createTables()
}

type User struct {
	ID          string `json:"id"`
	EntraID     string `json:"entra_id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	Enabled     bool   `json:"enabled"`
	Source      string `json:"source"`
	CreatedAt   string `json:"created_at"`
	LastLoginAt string `json:"last_login_at,omitempty"`
}

type ManagedAccount struct {
	ID          string `json:"id"`
	EntraID     string `json:"entra_id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Source      string `json:"source"`
	CreatedAt   string `json:"created_at"`
}

type Computer struct {
	ID                     string `json:"id"`
	Name                   string `json:"name"`
	DNSHostName            string `json:"dns_host_name"`
	OperatingSystem        string `json:"operating_system"`
	OperatingSystemVersion string `json:"operating_system_version"`
	CreatedAt              string `json:"created_at"`
}

type ADUser struct {
	ID                string `json:"id"` // GUID
	DN                string `json:"dn"`
	SAMAccountName    string `json:"sam_account_name"`
	UserPrincipalName string `json:"user_principal_name"`
	DisplayName       string `json:"display_name"`
	Mail              string `json:"mail"`
	OU                string `json:"ou"`
	Status            string `json:"status"`          // Active, Disabled, Locked Out, Password Expired
	PasswordStatus    string `json:"password_status"` // Never Expires, Cannot Change, Smart Card Required
	LastSync          string `json:"last_sync"`
}

type ADComputer struct {
	ID                     string `json:"id"` // GUID
	DN                     string `json:"dn"`
	Name                   string `json:"name"`
	DNSHostName            string `json:"dns_host_name"`
	OperatingSystem        string `json:"operating_system"`
	OperatingSystemVersion string `json:"operating_system_version"`
	LastSync               string `json:"last_sync"`
}

type ADGroup struct {
	ID          string `json:"id"` // GUID
	DN          string `json:"dn"`
	Name        string `json:"name"`
	Description string `json:"description"`
	MemberCount int    `json:"member_count"`
	LastSync    string `json:"last_sync"`
}

type Group struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DN          string `json:"dn"`
	Description string `json:"description"`
	Role        string `json:"role"`
	Source      string `json:"source"`
	CreatedAt   string `json:"created_at"`
}

type Target struct {
	ID          string `json:"id"`
	ZoneID      string `json:"zone_id"`
	Name        string `json:"name"`
	Hostname    string `json:"hostname"`
	Protocol    string `json:"protocol"`
	Port        int    `json:"port"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func createTables() error {
	query := `
	CREATE TABLE IF NOT EXISTS ad_config (
		id SERIAL PRIMARY KEY,
		host TEXT NOT NULL,
		port INTEGER NOT NULL,
		base_dn TEXT NOT NULL,
		bind_dn TEXT NOT NULL,
		bind_password TEXT NOT NULL,
		user_filter TEXT NOT NULL,
		group_filter TEXT NOT NULL DEFAULT '(objectClass=group)',
		computer_filter TEXT NOT NULL DEFAULT '(objectClass=computer)',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		entra_id TEXT,
		email TEXT NOT NULL,
		display_name TEXT,
		role TEXT DEFAULT 'user',
		enabled BOOLEAN DEFAULT TRUE,
		source TEXT DEFAULT 'local',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		last_login_at TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS managed_accounts (
		id TEXT PRIMARY KEY,
		entra_id TEXT,
		email TEXT NOT NULL,
		display_name TEXT,
		source TEXT DEFAULT 'active_directory',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS computers (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		dns_host_name TEXT,
		operating_system TEXT,
		operating_system_version TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS ad_users (
		id TEXT PRIMARY KEY,
		dn TEXT,
		sam_account_name TEXT,
		user_principal_name TEXT,
		display_name TEXT,
		mail TEXT,
		ou TEXT,
		status TEXT,
		password_status TEXT,
		last_sync TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS ad_computers (
		id TEXT PRIMARY KEY,
		dn TEXT,
		name TEXT,
		dns_host_name TEXT,
		operating_system TEXT,
		operating_system_version TEXT,
		last_sync TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS ad_groups (
		id TEXT PRIMARY KEY,
		dn TEXT,
		name TEXT,
		description TEXT,
		member_count INTEGER,
		last_sync TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS groups (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		dn TEXT,
		description TEXT,
		role TEXT DEFAULT 'user',
		source TEXT DEFAULT 'active_directory',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err := DB.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create tables: %v", err)
	}

	// Migration: Add computer_filter if it doesn't exist (for existing deployments)
	// We can ignore error if column already exists
	_, _ = DB.Exec(`ALTER TABLE ad_config ADD COLUMN IF NOT EXISTS computer_filter TEXT NOT NULL DEFAULT '(objectClass=computer)'`)
	_, _ = DB.Exec(`ALTER TABLE ad_config ADD COLUMN IF NOT EXISTS group_filter TEXT NOT NULL DEFAULT '(objectClass=group)'`)

	// Migration: Add source column to users table if it doesn't exist
	_, _ = DB.Exec(`ALTER TABLE users ADD COLUMN IF NOT EXISTS source TEXT DEFAULT 'local'`)

	// Migration: Add entra_id column to users table if it doesn't exist
	_, _ = DB.Exec(`ALTER TABLE users ADD COLUMN IF NOT EXISTS entra_id TEXT`)

	// Migration: Add dn and description columns to groups table if they don't exist
	_, _ = DB.Exec(`ALTER TABLE groups ADD COLUMN IF NOT EXISTS dn TEXT`)
	_, _ = DB.Exec(`ALTER TABLE groups ADD COLUMN IF NOT EXISTS description TEXT`)
	_, _ = DB.Exec(`ALTER TABLE groups ADD COLUMN IF NOT EXISTS role TEXT DEFAULT 'user'`)
	_, _ = DB.Exec(`ALTER TABLE groups ADD COLUMN IF NOT EXISTS source TEXT DEFAULT 'active_directory'`)

	return nil
}

func SaveConfig(host string, port int, baseDN, bindDN, bindPassword, userFilter, computerFilter, groupFilter string) error {
	// Upsert logic: check if exists, update if so, else insert
	// For simplicity, we'll assume single config row for now and just delete/insert or update ID=1

	// Check if config exists
	var count int
	err := DB.QueryRow("SELECT COUNT(*) FROM ad_config").Scan(&count)
	if err != nil {
		return err
	}

	if count > 0 {
		_, err = DB.Exec(`
			UPDATE ad_config 
			SET host=$1, port=$2, base_dn=$3, bind_dn=$4, bind_password=$5, user_filter=$6, computer_filter=$7, group_filter=$8, updated_at=CURRENT_TIMESTAMP
		`, host, port, baseDN, bindDN, bindPassword, userFilter, computerFilter, groupFilter)
	} else {
		_, err = DB.Exec(`
			INSERT INTO ad_config (host, port, base_dn, bind_dn, bind_password, user_filter, computer_filter, group_filter)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, host, port, baseDN, bindDN, bindPassword, userFilter, computerFilter, groupFilter)
	}
	return err
}

func GetConfig() (string, int, string, string, string, string, string, string, error) {
	var host, baseDN, bindDN, bindPassword, userFilter, computerFilter, groupFilter string
	var port int

	err := DB.QueryRow(`
		SELECT host, port, base_dn, bind_dn, bind_password, user_filter, computer_filter, group_filter 
		FROM ad_config 
		ORDER BY id DESC LIMIT 1
	`).Scan(&host, &port, &baseDN, &bindDN, &bindPassword, &userFilter, &computerFilter, &groupFilter)

	if err == sql.ErrNoRows {
		return "", 0, "", "", "", "", "", "", nil
	}
	return host, port, baseDN, bindDN, bindPassword, userFilter, computerFilter, groupFilter, err
}

func SaveADUsers(users []ADUser) error {
	stmt, err := DB.Prepare(`
		INSERT INTO ad_users (id, dn, sam_account_name, user_principal_name, display_name, mail, ou, status, password_status, last_sync)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, CURRENT_TIMESTAMP)
		ON CONFLICT (id) DO UPDATE SET
		dn = EXCLUDED.dn,
		sam_account_name = EXCLUDED.sam_account_name,
		user_principal_name = EXCLUDED.user_principal_name,
		display_name = EXCLUDED.display_name,
		mail = EXCLUDED.mail,
		ou = EXCLUDED.ou,
		status = EXCLUDED.status,
		password_status = EXCLUDED.password_status,
		last_sync = CURRENT_TIMESTAMP
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, u := range users {
		_, err := stmt.Exec(u.ID, u.DN, u.SAMAccountName, u.UserPrincipalName, u.DisplayName, u.Mail, u.OU, u.Status, u.PasswordStatus)
		if err != nil {
			log.Printf("Failed to save AD user %s: %v", u.SAMAccountName, err)
		}
	}
	return nil
}

func SaveADComputers(computers []ADComputer) error {
	stmt, err := DB.Prepare(`
		INSERT INTO ad_computers (id, dn, name, dns_host_name, operating_system, operating_system_version, last_sync)
		VALUES ($1, $2, $3, $4, $5, $6, CURRENT_TIMESTAMP)
		ON CONFLICT (id) DO UPDATE SET
		dn = EXCLUDED.dn,
		name = EXCLUDED.name,
		dns_host_name = EXCLUDED.dns_host_name,
		operating_system = EXCLUDED.operating_system,
		operating_system_version = EXCLUDED.operating_system_version,
		last_sync = CURRENT_TIMESTAMP
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, c := range computers {
		_, err := stmt.Exec(c.ID, c.DN, c.Name, c.DNSHostName, c.OperatingSystem, c.OperatingSystemVersion)
		if err != nil {
			log.Printf("Failed to save AD computer %s: %v", c.Name, err)
		}
	}
	return nil
}

func GetADUsers() ([]ADUser, error) {
	rows, err := DB.Query(`SELECT id, dn, sam_account_name, user_principal_name, display_name, mail, ou, status, password_status, last_sync FROM ad_users`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []ADUser
	for rows.Next() {
		var u ADUser
		if err := rows.Scan(&u.ID, &u.DN, &u.SAMAccountName, &u.UserPrincipalName, &u.DisplayName, &u.Mail, &u.OU, &u.Status, &u.PasswordStatus, &u.LastSync); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

func GetADComputers() ([]ADComputer, error) {
	rows, err := DB.Query(`SELECT id, dn, name, dns_host_name, operating_system, operating_system_version, last_sync FROM ad_computers`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var computers []ADComputer
	for rows.Next() {
		var c ADComputer
		if err := rows.Scan(&c.ID, &c.DN, &c.Name, &c.DNSHostName, &c.OperatingSystem, &c.OperatingSystemVersion, &c.LastSync); err != nil {
			return nil, err
		}
		computers = append(computers, c)
	}
	return computers, nil
}

// Keep existing SaveUsers/GetUsers/SaveComputers/GetComputers for OpenPAM users/computers
// But we might want to clean up SaveComputers if we are moving to ad_computers exclusively for sync
// For now, let's keep them but maybe SyncAD will write to AD tables instead.

func SaveUsers(users []User) error {
	// ... (existing implementation)
	// We don't use a transaction here so that a failure in one record (e.g. duplicate email)
	// doesn't rollback the entire batch. We want partial success.

	stmt, err := DB.Prepare(`
		INSERT INTO users (id, entra_id, email, display_name, role, enabled, source)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
		entra_id = EXCLUDED.entra_id,
		email = EXCLUDED.email,
		display_name = EXCLUDED.display_name,
		source = EXCLUDED.source
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, u := range users {
		_, err := stmt.Exec(u.ID, u.EntraID, u.Email, u.DisplayName, u.Role, u.Enabled, u.Source)
		if err != nil {
			log.Printf("Failed to save user %s (%s): %v", u.Email, u.ID, err)
			// Continue with other users
		}
	}

	return nil
}

func SaveManagedAccounts(accounts []ManagedAccount) error {
	stmt, err := DB.Prepare(`
		INSERT INTO managed_accounts (id, entra_id, email, display_name, source)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO UPDATE SET
		entra_id = EXCLUDED.entra_id,
		email = EXCLUDED.email,
		display_name = EXCLUDED.display_name,
		source = EXCLUDED.source
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, a := range accounts {
		_, err := stmt.Exec(a.ID, a.EntraID, a.Email, a.DisplayName, a.Source)
		if err != nil {
			log.Printf("Failed to save managed account %s (%s): %v", a.Email, a.ID, err)
		}
	}
	return nil
}

func SaveComputers(computers []Computer) error {
	// ... (existing implementation)
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO computers (id, name, dns_host_name, operating_system, operating_system_version)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO UPDATE SET
		name = EXCLUDED.name,
		dns_host_name = EXCLUDED.dns_host_name,
		operating_system = EXCLUDED.operating_system,
		operating_system_version = EXCLUDED.operating_system_version
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, c := range computers {
		_, err := stmt.Exec(c.ID, c.Name, c.DNSHostName, c.OperatingSystem, c.OperatingSystemVersion)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func GetUsers() ([]User, error) {
	// ... (existing implementation)
	rows, err := DB.Query(`SELECT id, email, display_name, role, enabled, source, created_at, last_login_at FROM users`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		var lastLoginAt sql.NullString
		if err := rows.Scan(&u.ID, &u.Email, &u.DisplayName, &u.Role, &u.Enabled, &u.Source, &u.CreatedAt, &lastLoginAt); err != nil {
			return nil, err
		}
		if lastLoginAt.Valid {
			u.LastLoginAt = lastLoginAt.String
		}
		users = append(users, u)
	}
	return users, nil
}

func GetManagedAccounts() ([]ManagedAccount, error) {
	rows, err := DB.Query(`SELECT id, entra_id, email, display_name, source, created_at FROM managed_accounts`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []ManagedAccount
	for rows.Next() {
		var a ManagedAccount
		if err := rows.Scan(&a.ID, &a.EntraID, &a.Email, &a.DisplayName, &a.Source, &a.CreatedAt); err != nil {
			return nil, err
		}
		accounts = append(accounts, a)
	}
	return accounts, nil
}

func GetComputers() ([]Computer, error) {
	// ... (existing implementation)
	rows, err := DB.Query(`SELECT id, name, dns_host_name, operating_system, operating_system_version, created_at FROM computers`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var computers []Computer
	for rows.Next() {
		var c Computer
		if err := rows.Scan(&c.ID, &c.Name, &c.DNSHostName, &c.OperatingSystem, &c.OperatingSystemVersion, &c.CreatedAt); err != nil {
			return nil, err
		}
		computers = append(computers, c)
	}
	return computers, nil
}

func SaveADGroups(groups []ADGroup) error {
	stmt, err := DB.Prepare(`
		INSERT INTO ad_groups (id, dn, name, description, member_count, last_sync)
		VALUES ($1, $2, $3, $4, $5, CURRENT_TIMESTAMP)
		ON CONFLICT (id) DO UPDATE SET
		dn = EXCLUDED.dn,
		name = EXCLUDED.name,
		description = EXCLUDED.description,
		member_count = EXCLUDED.member_count,
		last_sync = CURRENT_TIMESTAMP
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, g := range groups {
		_, err := stmt.Exec(g.ID, g.DN, g.Name, g.Description, g.MemberCount)
		if err != nil {
			log.Printf("Failed to save AD group %s: %v", g.Name, err)
		}
	}
	return nil
}

func GetADGroups() ([]ADGroup, error) {
	rows, err := DB.Query(`SELECT id, dn, name, description, member_count, last_sync FROM ad_groups`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []ADGroup
	for rows.Next() {
		var g ADGroup
		if err := rows.Scan(&g.ID, &g.DN, &g.Name, &g.Description, &g.MemberCount, &g.LastSync); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, nil
}

func SaveGroups(groups []Group) error {
	stmt, err := DB.Prepare(`
		INSERT INTO groups (id, name, dn, description, role, source)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO UPDATE SET
		name = EXCLUDED.name,
		dn = EXCLUDED.dn,
		description = EXCLUDED.description,
		role = EXCLUDED.role,
		source = EXCLUDED.source
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, g := range groups {
		_, err := stmt.Exec(g.ID, g.Name, g.DN, g.Description, g.Role, g.Source)
		if err != nil {
			log.Printf("Failed to save group %s: %v", g.Name, err)
		}
	}
	return nil
}

func GetGroups() ([]Group, error) {
	rows, err := DB.Query(`SELECT id, name, dn, description, role, source, created_at FROM groups`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []Group
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.Name, &g.DN, &g.Description, &g.Role, &g.Source, &g.CreatedAt); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, nil
}

func DeleteGroup(id string) error {
	_, err := DB.Exec("DELETE FROM groups WHERE id = $1", id)
	return err
}

func SaveTargets(targets []Target) error {
	stmt, err := DB.Prepare(`
		INSERT INTO targets (id, zone_id, name, hostname, protocol, port, description, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT (id) DO UPDATE SET
		zone_id = EXCLUDED.zone_id,
		name = EXCLUDED.name,
		hostname = EXCLUDED.hostname,
		protocol = EXCLUDED.protocol,
		port = EXCLUDED.port,
		description = EXCLUDED.description,
		enabled = EXCLUDED.enabled,
		updated_at = CURRENT_TIMESTAMP
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, t := range targets {
		_, err := stmt.Exec(t.ID, t.ZoneID, t.Name, t.Hostname, t.Protocol, t.Port, t.Description, t.Enabled)
		if err != nil {
			log.Printf("Failed to save target %s: %v", t.Name, err)
		}
	}
	return nil
}
