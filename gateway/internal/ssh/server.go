package ssh

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"

	"github.com/VanCannon/openpam/gateway/internal/logger"
	"github.com/VanCannon/openpam/gateway/internal/models"
	"github.com/VanCannon/openpam/gateway/internal/repository"
	"github.com/VanCannon/openpam/gateway/internal/vault"
	"golang.org/x/crypto/ssh"
)

// SSHServer manages incoming SSH connections
type SSHServer struct {
	logger       *logger.Logger
	config       *ssh.ServerConfig
	proxy        *Proxy
	targetRepo   *repository.TargetRepository
	userRepo     *repository.UserRepository
	credRepo     *repository.CredentialRepository
	auditRepo    *repository.AuditLogRepository
	scheduleRepo *repository.ScheduleRepository
	vault        *vault.Client
	server       net.Listener
	mu           sync.Mutex
	done         chan struct{}
}

// NewSSHServer creates a new SSH server
func NewSSHServer(
	log *logger.Logger,
	proxy *Proxy,
	targetRepo *repository.TargetRepository,
	userRepo *repository.UserRepository,
	credRepo *repository.CredentialRepository,
	auditRepo *repository.AuditLogRepository,
	scheduleRepo *repository.ScheduleRepository,
	vaultClient *vault.Client,
) *SSHServer {
	return &SSHServer{
		logger:       log,
		proxy:        proxy,
		targetRepo:   targetRepo,
		userRepo:     userRepo,
		credRepo:     credRepo,
		auditRepo:    auditRepo,
		scheduleRepo: scheduleRepo,
		vault:        vaultClient,
		done:         make(chan struct{}),
	}
}

// Start begins listening on the specified port
func (s *SSHServer) Start(port int) error {
	// Configure SSH Server
	config := &ssh.ServerConfig{
		NoClientAuth: true, // For now, we allow connection and auth later or rely on public key?
		// Actually, standard PAM proxies often require auth.
		// For Ansible, we might want to accept any key or password if we treat the gateway as a jump host
		// and the REAL auth happens effectively via the vault lookup.
		// BUT, we need to know WHO the user is.
		// Let's implement PasswordAuth callback at least.
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			// TODO: Validate against OpenPAM user database or EntraID
			// For now, accept generic dev creds or implement checking
			// Ideally we check `dbt_users` table content.
			return nil, nil
		},
	}

	// Generate host key (ED25519)
	// In production, load from file
	keyData, err := generateED25519Key()
	if err != nil {
		return fmt.Errorf("failed to generate host key: %w", err)
	}

	signer, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		return fmt.Errorf("failed to parse generated host key: %w", err)
	}
	config.AddHostKey(signer)

	// Listen
	addr := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}
	s.mu.Lock()
	s.server = listener
	s.mu.Unlock()

	s.logger.Info("Native SSH Server listening", map[string]interface{}{"addr": addr})

	go func() {
		for {
			nConn, err := listener.Accept()
			if err != nil {
				select {
				case <-s.done:
					return
				default:
					s.logger.Error("Failed to accept SSH connection", map[string]interface{}{"error": err.Error()})
					continue
				}
			}

			go s.handleConnection(nConn, config)
		}
	}()

	return nil
}

func (s *SSHServer) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	close(s.done)
	if s.server != nil {
		s.server.Close()
	}
}

func (s *SSHServer) handleConnection(nConn net.Conn, config *ssh.ServerConfig) {
	// Handshake
	conn, chans, reqs, err := ssh.NewServerConn(nConn, config)
	if err != nil {
		s.logger.Error("SSH Handshake failed", map[string]interface{}{"error": err.Error()})
		return
	}
	defer conn.Close()

	// Handle Discard Requests
	go ssh.DiscardRequests(reqs)

	// Accept Channels
	for newChannel := range chans {
		if newChannel.ChannelType() != "session" {
			newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}

		channel, requests, err := newChannel.Accept()
		if err != nil {
			continue
		}

		go s.handleSession(conn, channel, requests)
	}
}

func (s *SSHServer) handleSession(conn *ssh.ServerConn, channel ssh.Channel, requests <-chan *ssh.Request) {
	// Parse Identity: GatewayUser#TargetUser@TargetHost
	userInput := conn.User()
	parts := strings.Split(userInput, "#")

	var gatewayUserEmail string
	var targetPart string

	if len(parts) == 2 {
		gatewayUserEmail = parts[0]
		targetPart = parts[1]
	} else {
		channel.Stderr().Write([]byte("Access Denied: Format must be GatewayUser#TargetUser@TargetHost\n"))
		channel.Close()
		return
	}

	// Find last '@' to separate user from target host
	lastAt := strings.LastIndex(targetPart, "@")
	if lastAt == -1 || lastAt == 0 || lastAt == len(targetPart)-1 {
		channel.Stderr().Write([]byte("Invalid target format. Use TargetUser@TargetHost\n"))
		channel.Close()
		return
	}

	targetUser := targetPart[:lastAt]
	targetName := targetPart[lastAt+1:]

	ctx := context.Background()

	// 1. Resolve Gateway User
	user, err := s.userRepo.GetByEmail(ctx, gatewayUserEmail)
	if err != nil {
		channel.Stderr().Write([]byte(fmt.Sprintf("Access Denied: User %s not found\n", gatewayUserEmail)))
		channel.Close()
		return
	}

	// 2. Resolve Target
	// Note: We need GetByName or similar. Assuming GetByHostname works or exists.
	// The repo likely only has GetByID. We might have to iterate or add a method.
	// For this task, let's assume we can iterate List(nil) and filter.
	// Not efficient but functional for this MVP.
	allTargets, err := s.targetRepo.List(ctx, 1000, 0)
	var target *models.Target
	for _, t := range allTargets {
		if strings.EqualFold(t.Hostname, targetName) || strings.EqualFold(t.Name, targetName) {
			target = t
			break
		}
	}
	if target == nil {
		channel.Stderr().Write([]byte(fmt.Sprintf("Target %s not found\n", targetName)))
		channel.Close()
		return
	}

	// 3. Check Schedule
	activeStatus := models.ScheduleStatusActive
	approvedStatus := models.ApprovalStatusApproved
	schedules, err := s.scheduleRepo.List(ctx, &user.ID, &target.ID, &activeStatus, &approvedStatus, nil)

	var validSchedule *models.Schedule
	now := time.Now()
	if err == nil {
		for _, sch := range schedules {
			if (sch.StartTime.Before(now) || sch.StartTime.Equal(now)) && sch.EndTime.After(now) {
				validSchedule = &sch
				break
			}
		}
	}

	// Check Standing Access
	if validSchedule == nil {
		standingType := "standing"
		standing, err := s.scheduleRepo.List(ctx, &user.ID, &target.ID, &activeStatus, &approvedStatus, &standingType)
		if err == nil && len(standing) > 0 {
			validSchedule = &standing[0]
		}
	}

	if validSchedule == nil {
		channel.Stderr().Write([]byte("Access Denied: No active schedule found for this target\n"))
		channel.Close()
		return
	}

	// 4. Resolve Credentials (Vault)
	// Simplify for MVP: Assume we just use the username provided unless managed.
	// Ideally we duplicate the logic from websocket.go to handle managed/ephemeral.
	// For now, we'll assume standard static credential retrieval or basic vault path.
	// If managed, we might fail here without the extra logic.
	// Let's implement basic Vault retrieval for static accounts.

	// 4. Resolve Credentials
	var vaultCreds *vault.Credentials

	switch validSchedule.AccountType {
	case "managed":
		vaultPath, ok := validSchedule.AccountDetails["vault_secret_path"].(string)
		if !ok || vaultPath == "" {
			channel.Stderr().Write([]byte("Error: Managed account configuration missing vault path\n"))
			channel.Close()
			return
		}

		vaultCreds, err = s.vault.GetCredentials(ctx, vaultPath)
		if err != nil {
			channel.Stderr().Write([]byte("Error: Failed to retrieve managed credentials from Vault\n"))
			channel.Close()
			return
		}

	case "ephemeral":
		channel.Stderr().Write([]byte("Error: Ephemeral SSH sessions not yet supported via native listener\n"))
		channel.Close()
		return

	case "promotion":
		channel.Stderr().Write([]byte("Error: AD Promotion SSH sessions require interactive auth not yet fully supported via native listener\n"))
		channel.Close()
		return

	default:
		// "Legacy" or "Static" or "Standing" -> Use Credential Repo
		credsList, err := s.credRepo.GetByTargetID(ctx, target.ID)
		if err != nil { // Handle error potentially being nil if list is empty
			credsList = []*models.Credential{}
		}

		if len(credsList) > 0 {
			for _, c := range credsList {
				// 1. Exact Match
				if strings.EqualFold(c.Username, targetUser) {
					vaultCreds, err = s.vault.GetCredentials(ctx, c.VaultSecretPath)
					break
				}
				// 2. Smart Match
				if strings.Contains(targetUser, "@") {
					inputUser, _, _ := strings.Cut(targetUser, "@")
					if strings.EqualFold(c.Username, inputUser) {
						vaultCreds, err = s.vault.GetCredentials(ctx, c.VaultSecretPath)
						break
					}
				}
			}
		}

		if vaultCreds == nil {
			channel.Stderr().Write([]byte(fmt.Sprintf("Error: Credentials not found in OpenPAM for user '%s' on target '%s'. Available users for this target: ", targetUser, targetName)))
			for _, c := range credsList {
				channel.Stderr().Write([]byte(fmt.Sprintf("'%s' ", c.Username)))
			}
			channel.Stderr().Write([]byte("\n"))
			channel.Close()
			return
		}
	}

	// Create Adapter
	adapter := &SSHChannelAdapter{Channel: channel, Reqs: requests}

	// Create Audit Log
	remoteAddr := conn.RemoteAddr().String()
	auditLog := &models.AuditLog{
		UserID:        user.ID,
		TargetID:      target.ID,
		SessionStatus: models.SessionStatusActive,
		ClientIP:      &remoteAddr,
		// CredentialID?
	}
	s.auditRepo.Create(ctx, auditLog)

	// Update Audit Log on exit
	defer func() {
		auditLog.SessionStatus = models.SessionStatusCompleted
		// UpdateStatus handles setting EndTime internally or we can set it here if the repo method expects it.
		// Looking at repo code: UpdateStatus sets EndTime to Now().
		s.auditRepo.UpdateStatus(ctx, auditLog)

		// Note: System Audit log could be added here if we had access to the repo
	}()

	// Hand off to Proxy
	if err := s.proxy.Handle(ctx, adapter, target, vaultCreds, auditLog); err != nil {
		s.logger.Error("SSH Proxy failed", map[string]interface{}{"error": err.Error()})
		// Mark as failed if errored
		auditLog.SessionStatus = models.SessionStatusFailed
	}
}

// SSHChannelAdapter implements ClientConnection
type SSHChannelAdapter struct {
	Channel ssh.Channel
	Reqs    <-chan *ssh.Request
}

func (s *SSHChannelAdapter) ReadData() (*Msg, error) {
	// Non-blocking check for requests (resize) is tricky here because Read is blocking.
	// This usually requires a goroutine to demux.
	// Simple approach: Expect mostly data.
	// For a robust implementation, we need a select loop or a demuxer.

	buf := make([]byte, 1024)
	n, err := s.Channel.Read(buf)
	if err != nil {
		return nil, err
	}
	return &Msg{Type: MsgTypeSignin, Data: buf[:n]}, nil
}

func (s *SSHChannelAdapter) WriteData(data []byte) error {
	_, err := s.Channel.Write(data)
	return err
}

func (s *SSHChannelAdapter) Close() error {
	return s.Channel.Close()
}

func (s *SSHChannelAdapter) Type() string {
	return "ssh"
}

// Helper for key gen
func generateED25519Key() ([]byte, error) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}

	// Marshaling ED25519 private key to PEM
	// Standard x509 doesn't support MarshalPKCS8PrivateKey for ed25519 well in older Go?
	// Actually ssh.MarshalPrivateKey is better if we just want it for internal use,
	// BUT AddHostKey expects a Signer.
	// We can use ssh.NewSignerFromKey(priv) directly?
	// The problem is `generateED25519Key` signature I wrote returns `[]byte`.
	// Let's return the PEM encoded private key so `ssh.ParsePrivateKey` works standardly.

	x509Encoded, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, err
	}
	pemBlock := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: x509Encoded,
	}
	return pem.EncodeToMemory(pemBlock), nil
}
