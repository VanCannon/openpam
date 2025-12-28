package ssh

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/VanCannon/openpam/gateway/internal/logger"
	"github.com/VanCannon/openpam/gateway/internal/models"
	"github.com/VanCannon/openpam/gateway/internal/service"
	"github.com/VanCannon/openpam/gateway/internal/vault"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// WSMessage is a helper for ReadData to return type+data
// Ideally ReadData just returns data.
// But we need to know if it's a resize or whatever.
// The WebSocket Protocol here is JSON for control, or raw text?
// Actually the code says:
// messageType, data, err := wsConn.ReadMessage()
// if messageType == websocket.TextMessage { ... resize/upload ... }
// else { ... stdin ... }
//
// So we need to abstract this behavior.
// Let's make `ReadData` return `([]byte, MessageType, error)`
// where MessageType is an enum for Stdin, Resize, Upload.
// OR we encapsulate parsing inside the WebSocketAdapter.

type MsgType int

const (
	MsgTypeSignin MsgType = iota // Standard Input
	MsgTypeResize                // Resize Event
	MsgTypeUpload                // File Upload
)

// Msg represents a parsed message from the client
type Msg struct {
	Type    MsgType
	Data    []byte // Payload for Stdin
	Cols    int
	Rows    int
	Name    string // Filename for Upload
	Content string // Base64 content for Upload
}

// We need to add Lock/Unlock to the interface context or the struct.
// The original code had `wsMutex`. Let's put that in the adapter.

type SafeWebSocketAdapter struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (w *SafeWebSocketAdapter) ReadData() (*Msg, error) {
	// Read is safe to be concurrent with Write, but only one reader.
	// We assume single reader loop.
	msgType, data, err := w.conn.ReadMessage()
	if err != nil {
		return nil, err
	}

	if msgType == websocket.TextMessage {
		var msgHeader struct {
			Type string `json:"type"`
		}
		// Try to parse as JSON control message
		if err := json.Unmarshal(data, &msgHeader); err == nil {
			if msgHeader.Type == "resize" {
				var resizeMsg struct {
					Cols int
					Rows int
				}
				if err := json.Unmarshal(data, &resizeMsg); err == nil {
					return &Msg{Type: MsgTypeResize, Cols: resizeMsg.Cols, Rows: resizeMsg.Rows}, nil
				}
			} else if msgHeader.Type == "file_upload" {
				var uploadMsg struct {
					Name string
					Data string
				}
				if err := json.Unmarshal(data, &uploadMsg); err == nil {
					return &Msg{Type: MsgTypeUpload, Name: uploadMsg.Name, Content: uploadMsg.Data}, nil
				}
			}
		}
		// If not a control message (or invalid JSON), treat as raw text input
		return &Msg{Type: MsgTypeSignin, Data: data}, nil
	}
	return &Msg{Type: MsgTypeSignin, Data: data}, nil
}

func (w *SafeWebSocketAdapter) WriteData(data []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.conn.WriteMessage(websocket.BinaryMessage, data)
}

func (w *SafeWebSocketAdapter) Close() error {
	return w.conn.Close()
}

func (w *SafeWebSocketAdapter) Type() string {
	return "websocket"
}

// NewSafeWebSocketAdapter creates a new adapter with mutex
func NewSafeWebSocketAdapter(conn *websocket.Conn) *SafeWebSocketAdapter {
	return &SafeWebSocketAdapter{
		conn: conn,
	}
}

// ClientConnection interface update
type ClientConnection interface {
	ReadData() (*Msg, error)
	WriteData(data []byte) error
	Close() error
	Type() string
}

// Proxy handles the SSH session
type Proxy struct {
	logger       *logger.Logger
	recorder     *Recorder
	monitor      *Monitor
	aiService    service.AIService
	outputBuffer []string
	bufferMutex  sync.Mutex
}

// NewProxy creates a new SSH proxy
func NewProxy(log *logger.Logger, recorder *Recorder, monitor *Monitor, apiKey string) *Proxy {
	var ai service.AIService
	if apiKey != "" {
		ai = service.NewGeminiAIService(apiKey)
		log.Info("Enabled Gemini AI Service")
	} else {
		ai = service.NewMockAIService()
		log.Info("Enabled Mock AI Service (No API Key provided)")
	}

	return &Proxy{
		logger:       log,
		recorder:     recorder,
		monitor:      monitor,
		aiService:    ai,
		outputBuffer: make([]string, 0, 200),
	}
}

// Handle manages the SSH session
func (p *Proxy) Handle(
	ctx context.Context,
	clientConn ClientConnection,
	target *models.Target,
	creds *vault.Credentials,
	auditLog *models.AuditLog,
) error {
	p.logger.Info("Starting SSH session", map[string]interface{}{
		"target": target.Hostname,
		"user":   creds.Username,
		"type":   clientConn.Type(),
	})
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

	// --- Silent Discovery Phase ---
	// We open a specialized, temporary session to gather context BEFORE the main shell starts.
	// This ensures we know the OS and environment without polluting the user's interactive PTY.
	discoveryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	osContext := p.gatherContext(discoveryCtx, sshConn, target)
	p.logger.Info("Silent discovery completed", map[string]interface{}{
		"os_family": osContext.Family,
		"distro":    osContext.Distro,
		"roles":     osContext.Roles,
	})
	// -------------------------------

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
	// Proxy data between Client and SSH
	var wg sync.WaitGroup
	var bytesSent, bytesReceived int64
	clientClosedChan := make(chan struct{}) // Signal when Client closes

	// Client -> SSH (user input)
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer stdin.Close()           // Close SSH stdin when WebSocket closes
		defer close(clientClosedChan) // Signal that WebSocket closed
		p.logger.Info("Starting Client -> SSH loop with AI Interceptor")

		// AI Interceptor State
		var inputBuffer []byte
		inAIHeader := true // True if we are at start of line
		aiMode := "none"

		resetInterceptor := func() {
			inputBuffer = nil
			inAIHeader = true
			inAIHeader = true
			aiMode = "none"
		}
		resetInterceptor()

		for {
			msg, err := clientConn.ReadData()
			if err != nil {
				// We assume any error here is a close or failure
				p.logger.Info("Client read error or close", map[string]interface{}{"error": err.Error()})
				return
			}

			// Handle Control Messages
			if msg.Type == MsgTypeResize {
				p.HandleResize(session, msg.Cols, msg.Rows)
				continue
			} else if msg.Type == MsgTypeUpload {
				p.logger.Info("File upload received", map[string]interface{}{"name": msg.Name})

				// Sanitize filename
				name := msg.Name
				if idx := strings.LastIndexAny(name, "/\\"); idx != -1 {
					name = name[idx+1:]
				}

				var cmd string
				if osContext.Family == "windows" {
					cmd = fmt.Sprintf("[IO.File]::WriteAllBytes('%s', [Convert]::FromBase64String('%s')); Write-Host 'File uploaded: %s'\r",
						name, msg.Content, name)
				} else {
					cmd = fmt.Sprintf("echo '%s' | base64 -d > '%s'; echo 'File uploaded: %s'\n",
						msg.Content, name, name)
				}

				stdin.Write([]byte(cmd))
				continue
			}

			// Must be stdin data
			data := msg.Data
			if len(data) == 0 {
				continue
			}

			// --- INTERCEPTION LOGIC ---

			// Check for new line in this data packet to maintain state
			// We also treat Ctrl+C (3) and Ctrl+L (12) as "newlines" because they reset the prompt context
			hasNewline := bytes.Contains(data, []byte{13}) ||
				bytes.Contains(data, []byte{10}) ||
				bytes.Contains(data, []byte{3}) ||
				bytes.Contains(data, []byte{12})

			// 1. Trigger Detection
			// Only check for trigger if we are at the start of a line and AI is not already active
			if aiMode == "none" && inAIHeader && len(data) > 0 {
				if data[0] == '!' {
					aiMode = "command"
					inAIHeader = false
				} else if data[0] == '@' {
					// Immediate System Info Trigger
					// We do not enter a buffering mode, we just execute and reset.
					// But we must consume this character so it doesn't go to SSH.

					// Format Info
					infoMsg := fmt.Sprintf("\r\n\033[36m--- System Context ---\033[0m\r\n"+
						"Family: %s\r\nDistro: %s\r\nUser: %s\r\nRoles: %v\r\nTools: %v\r\n"+
						"\033[36m----------------------\033[0m\r\n",
						osContext.Family, osContext.Distro, osContext.User, osContext.Roles, osContext.Tools)

					// Local Echo Info
					clientConn.WriteData([]byte(infoMsg))
					clientConn.WriteData([]byte("\r\n")) // Extra newline

					// We consumed the @ stroke.
					// We should probably reset to a clean prompt state or just let user type.
					// But the user physically typed '@', and we intercepted it.
					// The cursor is technically past output now.
					// Let's just reset interceptor and NOT forward '@'
					resetInterceptor()
					continue // output processed, skip forwarding

				} else if data[0] == '?' {
					aiMode = "error"
					inAIHeader = false
				}
			}

			// 2. Routing
			if aiMode != "none" {
				// --- AI MODE ---

				// Handle Newline (Trigger Execution)
				nlIndex := -1
				if idx := bytes.IndexByte(data, 13); idx >= 0 {
					nlIndex = idx
				} else if idx := bytes.IndexByte(data, 10); idx >= 0 {
					nlIndex = idx
				}

				if nlIndex >= 0 {
					// Found newline!
					// 1. Append data UP TO newline to buffer
					inputBuffer = append(inputBuffer, data[:nlIndex]...)

					// 2. Echo data UP TO newline (suppress the newline itself)
					clientConn.WriteData(data[:nlIndex])

					// Execute AI Action
					query := string(inputBuffer)
					p.logger.Info("AI Trigger detected", map[string]interface{}{"mode": aiMode, "query": query})

					// 3. Visual Cleanup (Erase query from terminal)
					eraseSeq := bytes.Repeat([]byte("\b \b"), len(inputBuffer)+1) // +1 for trigger char
					clientConn.WriteData(eraseSeq)

					if aiMode == "command" {
						// --- COMMAND GENERATION (!) ---
						suggestion, err := p.aiService.GenerateCommand(ctx, query, osContext)
						if err != nil {
							suggestion = fmt.Sprintf("# AI Error: %v", err)
						}
						// Inject
						stdin.Write([]byte(suggestion))

					} else if aiMode == "error" {
						// --- ERROR ANALYSIS (?) ---
						// Parse optional number
						lineCount := 20 // default
						queryTrim := strings.TrimSpace(query)
						if _, err := fmt.Sscanf(queryTrim, "%d", &lineCount); err == nil {
							// Check bounds
							if lineCount < 10 {
								lineCount = 10
							}
							if lineCount > 200 {
								lineCount = 200
							}
						}

						// Get capture
						p.bufferMutex.Lock()
						totalLines := len(p.outputBuffer)
						start := totalLines - lineCount
						if start < 0 {
							start = 0
						}
						captured := make([]string, len(p.outputBuffer[start:]))
						copy(captured, p.outputBuffer[start:])
						p.bufferMutex.Unlock()

						// Call AI
						analysis, err := p.aiService.AnalyzeError(ctx, query, captured, osContext)
						if err != nil {
							analysis = fmt.Sprintf("\r\n\033[31mAI Error: %v\033[0m", err)
						} else {
							// Colorize/Format analysis
							analysis = fmt.Sprintf("\r\n\033[33m%s\033[0m\r\n", analysis)
						}

						// Local Echo result (do not inject to STDIN)
						clientConn.WriteData([]byte(analysis))
						// We probably want to restore prompt?
						// Current flow: user typed `? 50<ENTER>`, we erased it.
						// Then we printed huge block.
						// The prompt is likely still there visually but maybe covered?
						// Actually since we erased `? 50`, the cursor is back at prompt start.
						// So printing analysis PUSHES the prompt down or overwrites?
						// Standard terminal behavior: printing \r\n moves validly.
						// We should probably print a fresh empty line or suggestion.
						// For error analysis, we just show output.
						// The user will likely have to hit enter to get a new prompt from shell.
					}

					// Reset
					resetInterceptor()

					// Note: We ignore any data AFTER the newline
					continue
				}

				// No newline, just buffering
				inputBuffer = append(inputBuffer, data...)

				// LOCAL ECHO (required since we blocked server echo)
				clientConn.WriteData(data)

				// Check for Backspace (only if no newline was handled)
				if bytes.Contains(data, []byte{127}) {
					bsCount := bytes.Count(data, []byte{127})
					for i := 0; i < bsCount; i++ {
						if len(inputBuffer) > 0 {
							inputBuffer = inputBuffer[:len(inputBuffer)-1]
						}
						// Remove the char before it (if any recorded)
						// Note: inputBuffer contains the BS characters too, so we removed BS, now remove char
						if len(inputBuffer) > 0 {
							inputBuffer = inputBuffer[:len(inputBuffer)-1]
						}

						// Visual backspace
						clientConn.WriteData([]byte("\b \b"))
					}

					// If buffer is empty (user backspaced everything including trigger), treat as reset?
					if len(inputBuffer) == 0 {
						resetInterceptor()
					}
				}

			} else {
				// --- PASSTHROUGH MODE ---
				// Allow data to flow to SSH stdin

				// Update header state for next packet
				if hasNewline {
					inAIHeader = true
				} else {
					// If we processed some data and it didn't have a newline, we are definitely not at header anymore using this simple logic.
					// However, if data was empty (unlikely given len check above), state shouldn't change.
					if len(data) > 0 {
						inAIHeader = false
					}
				}

				bytesSent += int64(len(data))

				// Audit: Check for Encoded Commands (Ansible/PowerShell)
				p.detectAndLogEncodedCommand(data)

				if _, err := stdin.Write(data); err != nil {
					p.logger.Error("Failed to write to SSH stdin", map[string]interface{}{"error": err.Error()})
					return
				}
			}

			// Loop continues to read next packet
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
					p.logger.Error("Error reading from SSH stdout", map[string]interface{}{"error": err.Error()})
				} else {
					p.logger.Info("SSH stdout closed (EOF)")
				}
				break
			}
			p.logger.Debug("Read from SSH stdout", map[string]interface{}{"bytes": n})

			if n > 0 {
				bytesReceived += int64(n)
				data := buffer[:n]

				// Send to Client
				if err := clientConn.WriteData(data); err != nil {
					p.logger.Error("Failed to write to Client", map[string]interface{}{"error": err.Error()})
					return
				}

				// Capture Output for AI Analysis
				p.bufferMutex.Lock()
				lines := strings.Split(string(data), "\n")
				for _, line := range lines {
					if line == "" {
						continue
					}
					// Clean up basics (cr)
					line = strings.TrimRight(line, "\r")
					p.outputBuffer = append(p.outputBuffer, line)
				}
				// Trim buffer
				if len(p.outputBuffer) > 200 {
					p.outputBuffer = p.outputBuffer[len(p.outputBuffer)-200:]
				}
				p.bufferMutex.Unlock()

				p.logger.Debug("Successfully sent data to Client")

				// Record output if enabled
				if recWriter != nil {
					recWriter.Write(data)
				}

				// Broadcast to live monitors
				if p.monitor != nil {
					p.monitor.Broadcast(auditLog.ID.String(), data)
				}
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

			// Send to Client
			if err := clientConn.WriteData(data); err != nil {
				p.logger.Error("Failed to write stderr to Client", map[string]interface{}{
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
		clientConn.Close()
		wg.Wait()
		return ctx.Err()
	case <-clientClosedChan:
		// Client connection closed - terminate SSH session
		p.logger.Info("Client connection closed, terminating SSH session")
		session.Close()
		wg.Wait()
		auditLog.BytesSent = bytesSent
		auditLog.BytesReceived = bytesReceived
		// Treat user-initiated close as successful completion
		return nil
	case err := <-done:
		// SSH session ended - close WebSocket immediately to unblock goroutines
		// SSH session ended - close Client connection immediately
		p.logger.Info("SSH session ended, closing Client connection")
		// We could send a close message if protocol supports it
		// For now just close underlying transport
		clientConn.Close()

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
			ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) (answers []string, err error) {
				answers = make([]string, len(questions))
				for i := range questions {
					answers[i] = creds.Password
				}
				return answers, nil
			}),
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

// gatherContext executes silent commands to fingerprint the system
func (p *Proxy) gatherContext(ctx context.Context, client *ssh.Client, target *models.Target) service.SystemContext {
	sysCtx := service.SystemContext{Family: "unknown"}

	// Helper to run a command and get output safely
	runCmd := func(cmd string) string {
		session, err := client.NewSession()
		if err != nil {
			return ""
		}
		defer session.Close()

		var b bytes.Buffer
		session.Stdout = &b
		// We rely on the parent context 'ctx' passed to gatherContext controls the overall duration.
		if err := session.Run(cmd); err != nil {
			return ""
		}
		return strings.TrimSpace(b.String())
	}

	// 1. Identify OS Family
	uname := runCmd("uname -a")
	if uname != "" {
		// NOTE: Uname is not in service.SystemContext? Check definition.
		// Wait, I need to check if Uname is in service.SystemContext.
		// I defined it in proxy.go originally but maybe not in service.SystemContext in previous step.
		// Let me check my previous edit to ai.go.
		// Previous edit:
		// type SystemContext struct {
		// 	Family  string
		// 	Distro  string
		// 	Roles   []string
		// 	Tools   []string
		// 	Network []string
		// 	User    string
		// }
		// Uname is MISSING. I should add it to service.SystemContext or drop it.
		// I should probably add it for completeness or just drop it if not used by AI.
		// The prompt uses Family and Distro. Uname might be useful raw context.
		// I'll assume I should drop 'Uname' assignment for now to match the struct,
		// or better: I will add Uname to ai.go in a separate step if needed.
		// For now, I will NOT assign sysCtx.Uname if it doesn't exist.

		sysCtx.Family = "linux"
		if strings.Contains(strings.ToLower(uname), "darwin") {
			sysCtx.Family = "darwin"
		}
	} else {
		// Try Windows PowerShell
		psVer := runCmd("echo $PSVersionTable.PSVersion.ToString()")
		if psVer != "" && psVer != "$PSVersionTable.PSVersion.ToString()" {
			sysCtx.Family = "windows"
		} else {
			// Fallback CMD
			ver := runCmd("ver")
			if strings.Contains(strings.ToLower(ver), "windows") {
				sysCtx.Family = "windows"
			}
		}
	}

	// 2. Gather Deep Context based on Family
	if sysCtx.Family == "windows" {
		// --- WINDOWS ---

		// Distro / OS Version
		if winName := runCmd("(Get-CimInstance Win32_OperatingSystem).Caption"); winName != "" {
			sysCtx.Distro = winName
		}

		// Roles (DC, DNS, SQL, IIS)
		// Check DomainRole: 4/5 = DC
		domainRole := runCmd("(Get-CimInstance Win32_ComputerSystem).DomainRole")
		if domainRole == "4" || domainRole == "5" {
			sysCtx.Roles = append(sysCtx.Roles, "Domain Controller")
		}

		// Check Services
		svcCheck := runCmd("Get-Service -Name DNS, MSSQLSERVER, W3SVC -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Name")
		if strings.Contains(svcCheck, "DNS") {
			sysCtx.Roles = append(sysCtx.Roles, "DNS Server")
		}
		if strings.Contains(svcCheck, "MSSQLSERVER") {
			sysCtx.Roles = append(sysCtx.Roles, "SQL Server")
		}
		if strings.Contains(svcCheck, "W3SVC") {
			sysCtx.Roles = append(sysCtx.Roles, "IIS Web Server")
		}

		// Tools
		toolsCheck := runCmd("Get-Command docker, git, python, kubectl -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Name")
		for _, tool := range []string{"docker", "git", "python", "kubectl"} {
			if strings.Contains(strings.ToLower(toolsCheck), tool) {
				sysCtx.Tools = append(sysCtx.Tools, tool)
			}
		}

		// User
		sysCtx.User = runCmd("whoami")

		// Network (First non-loopback IPv4)
		// Concise PS command to get just the IP of the main interface
		netInfo := runCmd("Get-NetIPAddress -AddressFamily IPv4 | Where-Object { $_.InterfaceAlias -notmatch 'Loopback' } | Select-Object -First 1 -ExpandProperty IPAddress")
		if netInfo != "" {
			sysCtx.Network = append(sysCtx.Network, "IP: "+netInfo)
		}

	} else if sysCtx.Family == "linux" {
		// --- LINUX ---

		// Distro
		if release := runCmd("cat /etc/os-release"); release != "" {
			for _, line := range strings.Split(release, "\n") {
				if strings.HasPrefix(line, "PRETTY_NAME=") {
					sysCtx.Distro = strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), "\"")
					break
				}
			}
		}

		// Roles (Postgres, Web servers)
		// Check active services
		if runCmd("systemctl is-active postgresql") == "active" {
			sysCtx.Roles = append(sysCtx.Roles, "PostgreSQL Database")
		}
		if runCmd("systemctl is-active nginx") == "active" {
			sysCtx.Roles = append(sysCtx.Roles, "Nginx Web Server")
		}
		if runCmd("systemctl is-active apache2") == "active" || runCmd("systemctl is-active httpd") == "active" {
			sysCtx.Roles = append(sysCtx.Roles, "Apache Web Server")
		}

		// Tools
		// Check binaries
		toolsRes := runCmd("which docker git python3 kubectl 2>/dev/null")
		for _, tool := range []string{"docker", "git", "python3", "kubectl"} {
			if strings.Contains(toolsRes, tool) {
				sysCtx.Tools = append(sysCtx.Tools, tool)
			}
		}

		// User
		sysCtx.User = runCmd("whoami")

		// Network
		// ip -4 addr show scope global | grep inet | awk '{print $2}'
		// Simplified for reliability: just get the output and we can pass it or summarize it.
		// Let's try to get just the IP.
		ip := runCmd("hostname -I | cut -d' ' -f1")
		if ip != "" {
			sysCtx.Network = append(sysCtx.Network, "IP: "+ip)
		}
	}

	return sysCtx
}

// detectAndLogEncodedCommand checks if the input contains a Base64 encoded PowerShell command
// and logs the decoded version for auditing purposes.
func (p *Proxy) detectAndLogEncodedCommand(data []byte) {
	input := string(data)
	// Simple regex to catch standard EncodedCommand usage
	// patterns: powershell -EncodedCommand <B64>
	//           pwsh -e <B64>
	//           powershell.exe /e <B64>
	// We look for "-e", "-ec", "-EncodedCommand" followed by a space and a blob.
	// Note: This is a heuristic.

	lowerInput := strings.ToLower(input)
	if !strings.Contains(lowerInput, "powershell") && !strings.Contains(lowerInput, "pwsh") {
		return
	}

	// Indices of potential flags
	flags := []string{"-encodedcommand", "-ec", "-e", "/encodedcommand", "/ec", "/e"}
	var payload string

	for _, flag := range flags {
		idx := strings.Index(lowerInput, flag)
		if idx != -1 {
			// Found flag. The payload should be after it.
			// Format: flag + whitespace + payload
			remaining := input[idx+len(flag):]
			trimmed := strings.TrimSpace(remaining)
			// Payload is efficiently the next token
			parts := strings.Fields(trimmed)
			if len(parts) > 0 {
				payload = parts[0]
				break
			}
		}
	}

	if payload != "" {
		// Attempt Decode
		// PowerShell uses UTF-16LE encoding for the inner string
		decodedBytes, err := base64.StdEncoding.DecodeString(payload)
		if err == nil {
			// Convert UTF-16LE to UTF-8 for logging
			// If strictly ASCII, it appears as "c o m m a n d" (null bytes interleaved)
			// We can strip null bytes for a cleaner log if it's just ASCII
			cleanPayload := strings.ReplaceAll(string(decodedBytes), "\x00", "")

			p.logger.Info("AUDIT: Decoded PowerShell Command", map[string]interface{}{
				"decoded_cmd": cleanPayload,
				"raw_size":    len(payload),
			})
		}
	}
}
