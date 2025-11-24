package ssh

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/bvanc/openpam/gateway/internal/logger"
	"github.com/bvanc/openpam/gateway/internal/models"
	"github.com/bvanc/openpam/gateway/internal/vault"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
)

// Proxy handles SSH protocol proxying over WebSocket
type Proxy struct {
	logger   *logger.Logger
	recorder *Recorder
}

// NewProxy creates a new SSH proxy
func NewProxy(log *logger.Logger, recorder *Recorder) *Proxy {
	return &Proxy{
		logger:   log,
		recorder: recorder,
	}
}

// Handle proxies an SSH connection over WebSocket
func (p *Proxy) Handle(
	ctx context.Context,
	wsConn *websocket.Conn,
	target *models.Target,
	creds *vault.Credentials,
	auditLog *models.AuditLog,
) error {
	// Build SSH client config
	config, err := p.buildSSHConfig(creds)
	if err != nil {
		return fmt.Errorf("failed to build SSH config: %w", err)
	}

	// Connect to SSH server
	addr := fmt.Sprintf("%s:%d", target.Hostname, target.Port)
	sshConn, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return fmt.Errorf("failed to connect to SSH server: %w", err)
	}
	defer sshConn.Close()

	p.logger.Info("Connected to SSH server", map[string]interface{}{
		"target": target.Hostname,
	})

	// Open SSH session
	session, err := sshConn.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	// Set up terminal modes
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	// Request PTY
	if err := session.RequestPty("xterm-256color", 40, 80, modes); err != nil {
		return fmt.Errorf("failed to request PTY: %w", err)
	}

	// Set up pipes
	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	stderr, err := session.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	// Start shell
	if err := session.Shell(); err != nil {
		return fmt.Errorf("failed to start shell: %w", err)
	}

	// Set up recording if enabled
	var recWriter io.Writer
	if p.recorder != nil {
		recWriter, err = p.recorder.StartRecording(ctx, auditLog.ID.String())
		if err != nil {
			p.logger.Error("Failed to start recording", map[string]interface{}{
				"error": err.Error(),
			})
		}
		defer p.recorder.StopRecording(auditLog.ID.String())
	}

	// Proxy data between WebSocket and SSH
	var wg sync.WaitGroup
	var bytesSent, bytesReceived int64

	// WebSocket -> SSH (user input)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			_, data, err := wsConn.ReadMessage()
			if err != nil {
				p.logger.Debug("WebSocket read error", map[string]interface{}{
					"error": err.Error(),
				})
				return
			}

			bytesSent += int64(len(data))

			// Write to SSH stdin
			if _, err := stdin.Write(data); err != nil {
				p.logger.Error("Failed to write to SSH stdin", map[string]interface{}{
					"error": err.Error(),
				})
				return
			}

			// Record input if enabled
			if recWriter != nil {
				recWriter.Write(data)
			}
		}
	}()

	// SSH stdout -> WebSocket
	wg.Add(1)
	go func() {
		defer wg.Done()
		buffer := make([]byte, 4096)
		for {
			n, err := stdout.Read(buffer)
			if err != nil {
				if err != io.EOF {
					p.logger.Debug("SSH stdout read error", map[string]interface{}{
						"error": err.Error(),
					})
				}
				return
			}

			bytesReceived += int64(n)

			data := buffer[:n]

			// Send to WebSocket
			if err := wsConn.WriteMessage(websocket.BinaryMessage, data); err != nil {
				p.logger.Error("Failed to write to WebSocket", map[string]interface{}{
					"error": err.Error(),
				})
				return
			}

			// Record output if enabled
			if recWriter != nil {
				recWriter.Write(data)
			}
		}
	}()

	// SSH stderr -> WebSocket
	wg.Add(1)
	go func() {
		defer wg.Done()
		buffer := make([]byte, 4096)
		for {
			n, err := stderr.Read(buffer)
			if err != nil {
				if err != io.EOF {
					p.logger.Debug("SSH stderr read error", map[string]interface{}{
						"error": err.Error(),
					})
				}
				return
			}

			data := buffer[:n]

			// Send to WebSocket
			if err := wsConn.WriteMessage(websocket.BinaryMessage, data); err != nil {
				p.logger.Error("Failed to write stderr to WebSocket", map[string]interface{}{
					"error": err.Error(),
				})
				return
			}
		}
	}()

	// Wait for session to complete or context cancellation
	done := make(chan error, 1)
	go func() {
		done <- session.Wait()
	}()

	select {
	case <-ctx.Done():
		p.logger.Info("SSH session cancelled by context")
		return ctx.Err()
	case err := <-done:
		wg.Wait() // Wait for goroutines to finish
		auditLog.BytesSent = bytesSent
		auditLog.BytesReceived = bytesReceived
		if err != nil {
			return fmt.Errorf("SSH session error: %w", err)
		}
	}

	return nil
}

// buildSSHConfig creates SSH client configuration
func (p *Proxy) buildSSHConfig(creds *vault.Credentials) (*ssh.ClientConfig, error) {
	config := &ssh.ClientConfig{
		User:            creds.Username,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: Implement proper host key verification
		Timeout:         10 * time.Second,
	}

	// Use password or private key
	if creds.Password != "" {
		config.Auth = []ssh.AuthMethod{
			ssh.Password(creds.Password),
		}
	} else if creds.PrivateKey != "" {
		signer, err := ssh.ParsePrivateKey([]byte(creds.PrivateKey))
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
		config.Auth = []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		}
	} else {
		return nil, fmt.Errorf("no authentication method available")
	}

	return config, nil
}

// HandleResize handles terminal resize requests
func (p *Proxy) HandleResize(session *ssh.Session, width, height int) error {
	return session.WindowChange(height, width)
}
