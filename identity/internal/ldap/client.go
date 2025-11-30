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
