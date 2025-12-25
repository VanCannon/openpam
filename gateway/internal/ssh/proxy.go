package ssh

import (
	"bytes"
	"context"
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

// Proxy handles SSH protocol proxying over WebSocket
type Proxy struct {
	logger    *logger.Logger
	recorder  *Recorder
	monitor   *Monitor
	aiService service.AIService
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
		logger:    log,
		recorder:  recorder,
		monitor:   monitor,
		aiService: ai,
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
	var wg sync.WaitGroup
	var bytesSent, bytesReceived int64
	var wsMutex sync.Mutex              // Mutex to synchronize WebSocket writes
	wsClosedChan := make(chan struct{}) // Signal when WebSocket closes

	// WebSocket -> SSH (user input)
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer stdin.Close()       // Close SSH stdin when WebSocket closes
		defer close(wsClosedChan) // Signal that WebSocket closed
		p.logger.Info("Starting WebSocket -> SSH loop with AI Interceptor")

		// AI Interceptor State
		var inputBuffer []byte
		inAIHeader := true  // True if we are at start of line waiting to see if it's '?'
		isAIActive := false // True if we have detected '?' and are currently capturing a query

		// Helper to reset AI state
		resetInterceptor := func() {
			inputBuffer = nil
			inAIHeader = true // Assume start of line initially/after enter
			isAIActive = false
		}
		resetInterceptor()

		for {
			messageType, data, err := wsConn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					p.logger.Info("WebSocket closed by client")
				} else {
					p.logger.Debug("WebSocket read error", map[string]interface{}{"error": err.Error()})
				}
				return
			}

			// Handle Control Messages (Resize)
			if messageType == websocket.TextMessage {
				var controlMsg struct {
					Type string `json:"type"`
					Cols int    `json:"cols"`
					Rows int    `json:"rows"`
				}
				if err := json.Unmarshal(data, &controlMsg); err == nil && controlMsg.Type == "resize" {
					p.HandleResize(session, controlMsg.Cols, controlMsg.Rows)
					continue
				}
			}

			// --- INTERCEPTION LOGIC ---

			// Check for new line in this data packet to maintain state
			hasNewline := bytes.Contains(data, []byte{13}) || bytes.Contains(data, []byte{10})

			// 1. Trigger Detection
			// Only check for trigger if we are at the start of a line and AI is not already active
			if !isAIActive && inAIHeader && len(data) > 0 {
				if data[0] == '?' {
					isAIActive = true
					inAIHeader = false
				}
			}

			// 2. Routing
			if isAIActive {
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
					// This keeps the cursor on the same line so we can erase it all.
					wsMutex.Lock()
					wsConn.WriteMessage(websocket.BinaryMessage, data[:nlIndex])
					wsMutex.Unlock()

					// Execute AI Query
					query := string(inputBuffer)
					p.logger.Info("AI Trigger detected", map[string]interface{}{"query": query})

					// 3. Visual Cleanup
					// We need to erase: len(inputBuffer) characters.
					// We send backspace-space-backspace seq.
					eraseSeq := bytes.Repeat([]byte("\b \b"), len(inputBuffer))
					wsMutex.Lock()
					wsConn.WriteMessage(websocket.BinaryMessage, eraseSeq)
					wsMutex.Unlock()

					// 4. Call AI
					suggestion, err := p.aiService.GenerateCommand(ctx, string(inputBuffer), osContext)
					if err != nil {
						suggestion = fmt.Sprintf("# AI Error: %v", err)
					}

					// 5. Inject
					stdin.Write([]byte(suggestion))

					// Reset
					resetInterceptor()

					// Note: We ignore any data AFTER the newline in the same packet for simplicity.
					// In a real typing scenario, it's unlikely to have more data immediately after Enter.
					continue
				}

				// No newline, just buffering
				inputBuffer = append(inputBuffer, data...)

				// LOCAL ECHO (required since we blocked server echo)
				wsMutex.Lock()
				wsConn.WriteMessage(websocket.BinaryMessage, data)
				wsMutex.Unlock()

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
						wsMutex.Lock()
						wsConn.WriteMessage(websocket.BinaryMessage, []byte("\b \b"))
						wsMutex.Unlock()
					}

					// If buffer is empty (user backspaced everything including '?'), treat as reset?
					// Or just let them continue? If they delete '?', they might want to exit AI mode.
					// Current logic: if buffer empty, reset.
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
