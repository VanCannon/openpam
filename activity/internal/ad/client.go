package ldap

import (
	"crypto/tls"
	"fmt"
	"log"

	"github.com/go-ldap/ldap/v3"
)

type Client struct {
	Host         string
	Port         int
	BaseDN       string
	BindDN       string
	BindPassword string
	Conn         *ldap.Conn
}

func NewClient(host string, port int, baseDN, bindDN, bindPassword string) *Client {
	return &Client{
		Host:         host,
		Port:         port,
		BaseDN:       baseDN,
		BindDN:       bindDN,
		BindPassword: bindPassword,
	}
}

func (c *Client) Connect() error {
	address := fmt.Sprintf("%s:%d", c.Host, c.Port)
	log.Printf("Connecting to LDAP at %s", address)

	// Try StartTLS first, fall back to plain if needed (or configure via struct)
	// For simplicity, assuming standard LDAP or LDAPS based on port
	var l *ldap.Conn
	var err error

	if c.Port == 636 {
		l, err = ldap.DialTLS("tcp", address, &tls.Config{InsecureSkipVerify: true})
	} else {
		l, err = ldap.Dial("tcp", address)
		if err == nil {
			// Try StartTLS
			if err = l.StartTLS(&tls.Config{InsecureSkipVerify: true}); err != nil {
				log.Printf("Failed to StartTLS: %v", err)
				// Continue anyway, maybe server allows plain auth (though unlikely given the error)
				// But in this case, we know it failed, so we should probably return error if StartTLS fails
				// However, for broad compatibility, let's log and proceed, or maybe return error?
				// The error "Strong Auth Required" happens at Bind.
				// So if StartTLS fails, Bind will likely fail too.
				return fmt.Errorf("failed to StartTLS: %v", err)
			}
		}
	}

	if err != nil {
		return fmt.Errorf("failed to dial: %v", err)
	}

	// Redundant StartTLS block removed

	// Bind
	err = l.Bind(c.BindDN, c.BindPassword)
	if err != nil {
		// Check for Strong Auth Required (LDAP Result Code 8)
		if ldapErr, ok := err.(*ldap.Error); ok && ldapErr.ResultCode == 8 {
			log.Println("Simple Bind failed (Strong Auth Required), attempting NTLM Bind...")
			// NTLM Bind requires DOMAIN\User or User@Domain
			// We'll try to use the BindDN as is, assuming the user provided it in a compatible format
			// If it's a DN, NTLM might fail or we might need to parse it, but let's try direct first
			err = l.NTLMBind(c.BaseDN, c.BindDN, c.BindPassword) // Note: NTLMBind signature might vary, checking...
			// Actually NTLMBind in v3 takes (domain, username, password)
			// We need to parse c.BindDN
			domain, username := parseBindDN(c.BindDN)
			if domain != "" && username != "" {
				err = l.NTLMBind(domain, username, c.BindPassword)
			} else {
				return fmt.Errorf("failed to bind (Strong Auth Required) and could not parse BindDN for NTLM: %v", err)
			}
		}
	}
	if err != nil {
		return fmt.Errorf("failed to bind: %v", err)
	}

	c.Conn = l
	return nil
}

func parseBindDN(bindDN string) (string, string) {
	// Support DOMAIN\User
	if idx := len(bindDN) - 1; idx >= 0 {
		// Check for backslash
		for i, r := range bindDN {
			if r == '\\' {
				return bindDN[:i], bindDN[i+1:]
			}
		}
	}
	// Support User@Domain (UPN) - simplistic mapping
	// NTLM usually wants NetBIOS domain, but UPN might work depending on implementation
	return "", ""
}

func (c *Client) Close() {
	if c.Conn != nil {
		c.Conn.Close()
	}
}

func (c *Client) SearchUsers(filter string) ([]*ldap.Entry, error) {
	searchRequest := ldap.NewSearchRequest(
		c.BaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		filter,
		[]string{"sAMAccountName", "mail", "displayName", "memberOf", "userPrincipalName", "userAccountControl", "distinguishedName", "pwdLastSet"},
		nil,
	)

	sr, err := c.Conn.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to search users: %v", err)
	}

	return sr.Entries, nil
}

func (c *Client) SearchComputers(filter string) ([]*ldap.Entry, error) {
	searchRequest := ldap.NewSearchRequest(
		c.BaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		filter,
		[]string{"name", "dNSHostName", "operatingSystem", "operatingSystemVersion", "distinguishedName"},
		nil,
	)

	sr, err := c.Conn.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to search computers: %v", err)
	}

	return sr.Entries, nil
}

func (c *Client) SearchGroups(filter string) ([]*ldap.Entry, error) {
	searchRequest := ldap.NewSearchRequest(
		c.BaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		filter,
		[]string{"name", "description", "member", "distinguishedName"},
		nil,
	)

	sr, err := c.Conn.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to search groups: %v", err)
	}

	return sr.Entries, nil
}

func (c *Client) Authenticate(username, password string) (*ldap.Entry, error) {
	// 1. Bind as read-only user (already done in Connect, but we might need a fresh connection or re-bind)
	// Ideally, we should use a separate connection for the user bind to avoid messing up the main connection
	// But for now, we can try to find the user first, then attempt a bind with their DN

	log.Printf("Authenticating user: %s", username)

	// Find the user to get their DN
	// Search by sAMAccountName, userPrincipalName, or mail
	filter := fmt.Sprintf("(&(objectClass=user)(|(sAMAccountName=%s)(userPrincipalName=%s)(mail=%s)))", username, username, username)

	log.Printf("Searching for user with filter: %s", filter)
	users, err := c.SearchUsers(filter)
	if err != nil {
		log.Printf("Search failed: %v", err)
		return nil, fmt.Errorf("failed to search for user: %v", err)
	}

	if len(users) == 0 {
		log.Printf("User not found: %s", username)
		return nil, fmt.Errorf("user not found")
	}

	userEntry := users[0]
	userDN := userEntry.DN
	log.Printf("Found user DN: %s", userDN)

	// 2. Attempt to bind as the user
	// Create a new connection for this authentication attempt
	address := fmt.Sprintf("%s:%d", c.Host, c.Port)
	var l *ldap.Conn

	if c.Port == 636 {
		l, err = ldap.DialTLS("tcp", address, &tls.Config{InsecureSkipVerify: true})
	} else {
		l, err = ldap.Dial("tcp", address)
		if err == nil {
			err = l.StartTLS(&tls.Config{InsecureSkipVerify: true})
		}
	}

	if err != nil {
		log.Printf("Failed to connect for auth bind: %v", err)
		return nil, fmt.Errorf("failed to connect for auth: %v", err)
	}
	defer l.Close()

	// Attempt Bind
	log.Printf("Attempting Bind with DN: %s", userDN)
	err = l.Bind(userDN, password)
	if err != nil {
		log.Printf("Bind failed for %s: %v", userDN, err)
		return nil, fmt.Errorf("authentication failed: %v", err)
	}

	log.Printf("Successfully authenticated user: %s", username)
	return userEntry, nil
}

// EnableUser enables a user account in AD
func (c *Client) EnableUser(userDN string) error {
	// userAccountControl: clear the ACCOUNTDISABLE bit (2)
	// We need to fetch current UAC first
	searchRequest := ldap.NewSearchRequest(
		userDN,
		ldap.ScopeBaseObject, ldap.NeverDerefAliases, 0, 0, false,
		"(objectClass=*)",
		[]string{"userAccountControl"},
		nil,
	)

	sr, err := c.Conn.Search(searchRequest)
	if err != nil {
		return fmt.Errorf("failed to fetch user UAC: %v", err)
	}

	if len(sr.Entries) == 0 {
		return fmt.Errorf("user not found")
	}

	uacStr := sr.Entries[0].GetAttributeValue("userAccountControl")
	uac := 512 // Default NORMAL_ACCOUNT
	if uacStr != "" {
		fmt.Sscanf(uacStr, "%d", &uac)
	}

	// Clear DISABLE bit (2)
	newUAC := uac &^ 2

	modify := ldap.NewModifyRequest(userDN, nil)
	modify.Replace("userAccountControl", []string{fmt.Sprintf("%d", newUAC)})

	if err := c.Conn.Modify(modify); err != nil {
		return fmt.Errorf("failed to enable user: %v", err)
	}

	return nil
}

// DisableUser disables a user account in AD
func (c *Client) DisableUser(userDN string) error {
	// userAccountControl: set the ACCOUNTDISABLE bit (2)
	searchRequest := ldap.NewSearchRequest(
		userDN,
		ldap.ScopeBaseObject, ldap.NeverDerefAliases, 0, 0, false,
		"(objectClass=*)",
		[]string{"userAccountControl"},
		nil,
	)

	sr, err := c.Conn.Search(searchRequest)
	if err != nil {
		return fmt.Errorf("failed to fetch user UAC: %v", err)
	}

	if len(sr.Entries) == 0 {
		return fmt.Errorf("user not found")
	}

	uacStr := sr.Entries[0].GetAttributeValue("userAccountControl")
	uac := 512 // Default NORMAL_ACCOUNT
	if uacStr != "" {
		fmt.Sscanf(uacStr, "%d", &uac)
	}

	// Set DISABLE bit (2)
	newUAC := uac | 2

	modify := ldap.NewModifyRequest(userDN, nil)
	modify.Replace("userAccountControl", []string{fmt.Sprintf("%d", newUAC)})

	if err := c.Conn.Modify(modify); err != nil {
		return fmt.Errorf("failed to disable user: %v", err)
	}

	return nil
}

// SetPassword rotates the password for a user
func (c *Client) SetPassword(userDN, newPassword string) error {
	// AD requires password to be enclosed in quotes and UTF-16LE encoded
	utf16 := encodePassword(newPassword)

	modify := ldap.NewModifyRequest(userDN, nil)
	modify.Replace("unicodePwd", []string{string(utf16)})

	if err := c.Conn.Modify(modify); err != nil {
		return fmt.Errorf("failed to set password: %v", err)
	}

	return nil
}

func encodePassword(password string) string {
	quoted := "\"" + password + "\""
	// Convert to UTF-16LE
	// Go strings are UTF-8.
	// Simplified implementation:
	var utf16 []byte
	for _, r := range quoted {
		// Basic Multilingual Plane characters only for now
		utf16 = append(utf16, byte(r), 0)
	}
	return string(utf16)
}

// AddGroupMember adds a user to a group
func (c *Client) AddGroupMember(groupDN, userDN string) error {
	modify := ldap.NewModifyRequest(groupDN, nil)
	modify.Add("member", []string{userDN})

	if err := c.Conn.Modify(modify); err != nil {
		// specialized error handling?
		// e.g. checking if already member (LDAP result 68)
		if ldapErr, ok := err.(*ldap.Error); ok && ldapErr.ResultCode == 68 {
			return nil // Already member
		}
		return fmt.Errorf("failed to add group member: %v", err)
	}
	return nil
}

// RemoveGroupMember removes a user from a group
func (c *Client) RemoveGroupMember(groupDN, userDN string) error {
	modify := ldap.NewModifyRequest(groupDN, nil)
	modify.Delete("member", []string{userDN})

	if err := c.Conn.Modify(modify); err != nil {
		if ldapErr, ok := err.(*ldap.Error); ok && ldapErr.ResultCode == 53 {
			// server is unwilling to perform - confusing, but sometimes happens if not member?
			// Usually 53 is constraint violation
			return fmt.Errorf("failed to remove group member: %v", err)
		}
		// If attribute value doesn't exist (not a member), LDAP might return error
		// We should probably check specific code or just return error
		return fmt.Errorf("failed to remove group member: %v", err)
	}
	return nil
}

// CreateUser creates a new user (Ephemeral)
func (c *Client) CreateUser(username, password, description string) (string, error) {
	// Construct DN: CN=username,CN=Users,BaseDN (Simplification: putting in Users container)
	// Or maybe a specific OU for Ephemeral accounts? Let's use BaseDN for now or assume its an OU.

	// Need to determine parent DN.
	// HACK: Assuming BaseDN is the parent container for users for now.
	dn := fmt.Sprintf("CN=%s,%s", username, c.BaseDN)

	add := ldap.NewAddRequest(dn, []ldap.Control{})
	add.Attribute("objectClass", []string{"top", "person", "organizationalPerson", "user"})
	add.Attribute("cn", []string{username})
	add.Attribute("sAMAccountName", []string{username})
	add.Attribute("userPrincipalName", []string{fmt.Sprintf("%s@example.com", username)}) // TODO: get real domain
	add.Attribute("description", []string{description})
	// Initial UAC: NORMAL_ACCOUNT (512) | ACCOUNTDISABLE (2) -> 514
	// Must set password before enabling
	add.Attribute("userAccountControl", []string{"514"})

	if err := c.Conn.Add(add); err != nil {
		return "", fmt.Errorf("failed to create user: %v", err)
	}

	// Set password
	if err := c.SetPassword(dn, password); err != nil {
		// cleanup
		c.DeleteUser(dn)
		return "", fmt.Errorf("failed to set initial password: %v", err)
	}

	// Enable account
	if err := c.EnableUser(dn); err != nil {
		c.DeleteUser(dn)
		return "", fmt.Errorf("failed to enable new user: %v", err)
	}

	return dn, nil
}

// DeleteUser deletes a user
func (c *Client) DeleteUser(dn string) error {
	del := ldap.NewDelRequest(dn, []ldap.Control{})
	if err := c.Conn.Del(del); err != nil {
		return fmt.Errorf("failed to delete user: %v", err)
	}
	return nil
}

// Helper to find user DN by sAMAccountName
func (c *Client) GetUserDN(username string) (string, error) {
	filter := fmt.Sprintf("(&(objectClass=user)(sAMAccountName=%s))", username)
	entries, err := c.SearchUsers(filter)
	if err != nil {
		return "", err
	}
	if len(entries) == 0 {
		return "", fmt.Errorf("user not found")
	}
	return entries[0].DN, nil
}

// Helper to find group DN by Name
func (c *Client) GetGroupDN(name string) (string, error) {
	filter := fmt.Sprintf("(&(objectClass=group)(name=%s))", name)
	entries, err := c.SearchGroups(filter)
	if err != nil {
		return "", err
	}
	if len(entries) == 0 {
		return "", fmt.Errorf("group not found")
	}
	return entries[0].DN, nil
}
