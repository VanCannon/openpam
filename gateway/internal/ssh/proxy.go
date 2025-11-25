package ssh

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	monitor  *Monitor
}

// NewProxy creates a new SSH proxy
func NewProxy(log *logger.Logger, recorder *Recorder, monitor *Monitor) *Proxy {
	return &Proxy{
		logger:   log,
		recorder: recorder,
		monitor:  monitor,
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
	p.logger.Info("Requesting PTY", map[string]interface{}{"target": target.Hostname})
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
	p.logger.Info("Starting shell", map[string]interface{}{"target": target.Hostname})
	if err := session.Shell(); err != nil {
		return fmt.Errorf("failed to start shell: %w", err)
	}
	p.logger.Info("Shell started", map[string]interface{}{"target": target.Hostname})

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
	var wsMutex sync.Mutex // Mutex to synchronize WebSocket writes
	wsClosedChan := make(chan struct{}) // Signal when WebSocket closes

	// WebSocket -> SSH (user input)
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer stdin.Close() // Close SSH stdin when WebSocket closes
		defer close(wsClosedChan) // Signal that WebSocket closed
		p.logger.Info("Starting WebSocket -> SSH loop")
		for {
			messageType, data, err := wsConn.ReadMessage()
			if err != nil {
				// Check if it's a normal WebSocket close
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					p.logger.Info("WebSocket closed by client (user clicked X)")
				} else {
					p.logger.Debug("WebSocket read error", map[string]interface{}{
						"error": err.Error(),
					})
				}
				return
			}

			p.logger.Debug("Received data from WebSocket", map[string]interface{}{
				"bytes":        len(data),
				"message_type": messageType,
			})

			// Handle text messages as potential control messages
			if messageType == websocket.TextMessage {
				// Try to parse as JSON control message
				var controlMsg struct {
					Type string `json:"type"`
					Cols int    `json:"cols"`
					Rows int    `json:"rows"`
				}
				if err := json.Unmarshal(data, &controlMsg); err == nil && controlMsg.Type == "resize" {
					p.logger.Debug("Handling terminal resize", map[string]interface{}{
						"cols": controlMsg.Cols,
						"rows": controlMsg.Rows,
					})
					// Handle resize
					if err := session.WindowChange(controlMsg.Rows, controlMsg.Cols); err != nil {
						p.logger.Error("Failed to resize terminal", map[string]interface{}{
							"error": err.Error(),
						})
					}
					continue
				}
				// If not a control message, treat as terminal input
			}

			bytesSent += int64(len(data))

			// Write to SSH stdin
			if _, err := stdin.Write(data); err != nil {
				p.logger.Error("Failed to write to SSH stdin", map[string]interface{}{
					"error": err.Error(),
				})
				return
			}

			// Don't record input - the terminal echo in stdout already captures it
			// Recording input here causes duplicate keystrokes in the replay
		}
	}()

	// SSH stdout -> WebSocket
	wg.Add(1)
	go func() {
		defer wg.Done()
		p.logger.Info("Starting SSH stdout -> WebSocket loop")
		buffer := make([]byte, 4096)
		for {
			p.logger.Debug("Waiting to read from SSH stdout...")
			n, err := stdout.Read(buffer)
			if err != nil {
				if err != io.EOF {
					p.logger.Debug("SSH stdout read error", map[string]interface{}{
						"error": err.Error(),
					})
				} else {
					p.logger.Debug("SSH stdout EOF")
				}
				return
			}

			p.logger.Info("Received data from SSH stdout", map[string]interface{}{
				"bytes": n,
				"data":  string(buffer[:n]),
			})

			bytesReceived += int64(n)

			data := buffer[:n]

			// Send to WebSocket
			p.logger.Debug("Sending data to WebSocket", map[string]interface{}{"bytes": n})
			wsMutex.Lock()
			err = wsConn.WriteMessage(websocket.BinaryMessage, data)
			wsMutex.Unlock()

			if err != nil {
				p.logger.Error("Failed to write to WebSocket", map[string]interface{}{
					"error": err.Error(),
				})
				return
			}
			p.logger.Debug("Successfully sent data to WebSocket")

			// Record output if enabled
			if recWriter != nil {
				recWriter.Write(data)
			}

			// Broadcast to live monitors
			if p.monitor != nil {
				p.monitor.Broadcast(auditLog.ID.String(), data)
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
			wsMutex.Lock()
			err = wsConn.WriteMessage(websocket.BinaryMessage, data)
			wsMutex.Unlock()

			if err != nil {
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
		wsConn.Close()
		wg.Wait()
		return ctx.Err()
	case <-wsClosedChan:
		// WebSocket closed by client (user clicked X) - terminate SSH session
		p.logger.Info("WebSocket closed by client, terminating SSH session")
		session.Close()
		wg.Wait()
		auditLog.BytesSent = bytesSent
		auditLog.BytesReceived = bytesReceived
		// Treat user-initiated close as successful completion
		return nil
	case err := <-done:
		// SSH session ended - close WebSocket immediately to unblock goroutines
		p.logger.Info("SSH session ended, closing WebSocket")
		wsConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "SSH session ended"))
		wsConn.Close()

		wg.Wait() // Wait for goroutines to finish (they'll exit when WebSocket closes)
		auditLog.BytesSent = bytesSent
		auditLog.BytesReceived = bytesReceived

		// Check if the error is an ExitError with status 0 (normal exit)
		if err != nil {
			// Check if it's an SSH exit status error
			if exitErr, ok := err.(*ssh.ExitError); ok {
				exitStatus := exitErr.ExitStatus()
				p.logger.Info("SSH session exited", map[string]interface{}{
					"exit_status": exitStatus,
				})
				// Exit status 0 means success (user typed "exit")
				// Exit status 127 means last command not found (common when exiting shell)
				// Exit status 130 means user pressed Ctrl+C (also acceptable)
				if exitStatus == 0 || exitStatus == 127 || exitStatus == 130 {
					p.logger.Info("Treating as successful session exit", map[string]interface{}{
						"exit_status": exitStatus,
					})
					return nil
				}
				// Other non-zero exit statuses are actual errors
				return fmt.Errorf("SSH session exited with status %d", exitStatus)
			}
			// Other errors are real failures
			return fmt.Errorf("SSH session error: %w", err)
		}
		// Normal exit - return nil
		p.logger.Info("SSH session completed normally")
		return nil
	}
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
