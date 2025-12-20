package rdp

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/VanCannon/openpam/gateway/internal/logger"
	"github.com/VanCannon/openpam/gateway/internal/models"
	"github.com/VanCannon/openpam/gateway/internal/vault"

	"github.com/gorilla/websocket"
)

// FastProxy handles RDP protocol proxying with minimal overhead
// Key optimization: raw byte passthrough without parsing in the critical path
type FastProxy struct {
	guacdAddress  string
	logger        *logger.Logger
	asyncRecorder *AsyncRecorder
	asyncMonitor  *AsyncMonitor
}

// NewFastProxy creates a new optimized RDP proxy
func NewFastProxy(guacdAddress string, log *logger.Logger, asyncRecorder *AsyncRecorder, asyncMonitor *AsyncMonitor) *FastProxy {
	return &FastProxy{
		guacdAddress:  guacdAddress,
		logger:        log,
		asyncRecorder: asyncRecorder,
		asyncMonitor:  asyncMonitor,
	}
}

// Handle proxies an RDP connection with minimal overhead
func (p *FastProxy) Handle(
	ctx context.Context,
	wsConn *websocket.Conn,
	target *models.Target,
	creds *vault.Credentials,
	auditLog *models.AuditLog,
	width int,
	height int,
) error {
	sessionID := auditLog.ID.String()

	p.logger.Info("FastProxy RDP Handle() called", map[string]interface{}{
		"session_id": sessionID,
		"target":     target.Hostname,
	})

	// Connect to guacd
	guacdConn, err := net.Dial("tcp", p.guacdAddress)
	if err != nil {
		return fmt.Errorf("failed to connect to guacd: %w", err)
	}
	defer guacdConn.Close()

	guacdReader := bufio.NewReaderSize(guacdConn, 65536) // 64KB buffer

	p.logger.Info("Connected to guacd", map[string]interface{}{
		"address": p.guacdAddress,
		"target":  target.Hostname,
	})

	// Start async recording
	if p.asyncRecorder != nil {
		if err := p.asyncRecorder.StartRecording(ctx, sessionID); err != nil {
			p.logger.Error("Failed to start async recording", map[string]interface{}{
				"error": err.Error(),
			})
		} else {
			defer p.asyncRecorder.StopRecording(sessionID)
		}
	}

	// === HANDSHAKE PHASE (uses parsing, but only once) ===

	// Send "select" instruction
	if _, err := guacdConn.Write([]byte("6.select,3.rdp;")); err != nil {
		return fmt.Errorf("failed to send select to guacd: %w", err)
	}

	// Read "args" instruction from guacd
	opcode, args, err := p.readInstruction(guacdReader)
	if err != nil {
		return fmt.Errorf("failed to read args from guacd: %w", err)
	}
	if opcode != "args" {
		return fmt.Errorf("expected args instruction, got: %s", opcode)
	}

	widthStr := fmt.Sprintf("%d", width)
	heightStr := fmt.Sprintf("%d", height)

	// Connection parameters - balanced for quality and performance
	config := map[string]string{
		"hostname":                   target.Hostname,
		"port":                       fmt.Sprintf("%d", target.Port),
		"username":                   creds.Username,
		"password":                   creds.Password,
		"ignore-cert":                "true",
		"security":                   "any",
		"disable-bitmap-caching":     "false",
		"disable-offscreen-caching":  "false",
		"disable-glyph-caching":      "false",
		"enable-wallpaper":           "true",
		"enable-theming":             "true",
		"enable-menu-animations":     "false",
		"enable-font-smoothing":      "true",
		"enable-desktop-composition": "false",
		"enable-full-window-drag":    "false",
		"color-depth":                "24",
		"width":                      widthStr,
		"height":                     heightStr,
		"dpi":                        "96",
		"resize-method":              "display-update",
	}

	// Build and send connect instruction
	// Handle Guacamole 1.5.0+ version negotiation: if args[0] starts with "VERSION_", echo it back
	connectArgs := make([]string, len(args))
	for i, argName := range args {
		if strings.HasPrefix(argName, "VERSION_") {
			// Echo version string back for version negotiation
			connectArgs[i] = argName
		} else if val, ok := config[argName]; ok {
			connectArgs[i] = val
		} else {
			connectArgs[i] = ""
		}
	}

	// Build all handshake instructions and send as one write
	sizeInstr := fmt.Sprintf("4.size,%d.%s,%d.%s,2.96;", len(widthStr), widthStr, len(heightStr), heightStr)
	audioInstr := "5.audio;"
	videoInstr := "5.video;"
	imageInstr := "5.image,9.image/png,10.image/jpeg;"
	connectInstr := p.buildInstruction("connect", connectArgs...)

	handshake := sizeInstr + audioInstr + videoInstr + imageInstr + string(connectInstr)
	if _, err := guacdConn.Write([]byte(handshake)); err != nil {
		return fmt.Errorf("failed to send handshake to guacd: %w", err)
	}

	// Wait for "ready" instruction
	opcode, readyArgs, err := p.readInstruction(guacdReader)
	if err != nil {
		return fmt.Errorf("failed to read ready from guacd: %w", err)
	}
	if opcode != "ready" {
		return fmt.Errorf("expected ready instruction, got: %s", opcode)
	}

	p.logger.Info("Guacamole connection established (ready received)")

	// Build header for monitoring
	var headerBuilder strings.Builder
	sizeMsg := fmt.Sprintf("4.size,1.0,%d.%s,%d.%s,2.96;", len(widthStr), widthStr, len(heightStr), heightStr)
	headerBuilder.WriteString(sizeMsg)

	// Record size and ready
	if p.asyncRecorder != nil {
		p.asyncRecorder.WriteInstruction(sessionID, "size", "0", widthStr, heightStr, "96")
		p.asyncRecorder.WriteInstruction(sessionID, "ready", readyArgs...)
	}

	// Build and send ready to client
	readyInstr := p.buildInstruction("ready", readyArgs...)
	headerBuilder.Write(readyInstr)
	if p.asyncMonitor != nil {
		p.asyncMonitor.SetHeader(sessionID, []byte(headerBuilder.String()))
		p.asyncMonitor.Broadcast(sessionID, readyInstr)
	}

	if err := wsConn.WriteMessage(websocket.TextMessage, readyInstr); err != nil {
		return fmt.Errorf("failed to send ready to client: %w", err)
	}

	// Send size to client
	clientSizeInstr := p.buildInstruction("size", "0", widthStr, heightStr)
	if err := wsConn.WriteMessage(websocket.TextMessage, clientSizeInstr); err != nil {
		return fmt.Errorf("failed to send size to client: %w", err)
	}

	// === PROXY LOOP (raw byte passthrough) ===

	var wg sync.WaitGroup
	errChan := make(chan error, 2)
	var bytesReceived, bytesSent int64

	var shutdownOnce sync.Once
	shutdown := func() {
		shutdownOnce.Do(func() {
			wsConn.Close()
			guacdConn.Close()
		})
	}

	// Raw byte buffer for forwarding - reused to avoid allocations
	// Using a pool would be even better for high concurrency

	wg.Add(2)

	// Channel for async processing (recording/monitoring)
	asyncChan := make(chan []byte, 10000)
	asyncDone := make(chan struct{})

	// Async worker for recording/monitoring
	go func() {
		defer close(asyncDone)
		for instrBytes := range asyncChan {
			opcode, args, err := p.parseInstruction(instrBytes)
			if err != nil {
				continue
			}
			if p.asyncRecorder != nil {
				p.asyncRecorder.WriteInstruction(sessionID, opcode, args...)
			}
			if p.asyncMonitor != nil {
				p.asyncMonitor.Broadcast(sessionID, instrBytes)
			}
		}
	}()

	// guacd -> websocket (RAW BYTE PASSTHROUGH)
	go func() {
		defer wg.Done()
		defer close(asyncChan)

		// Read buffer - we read in chunks and scan for semicolons
		buf := make([]byte, 65536)
		pending := make([]byte, 0, 65536)

		for {
			n, err := guacdReader.Read(buf)
			if err != nil {
				if err != io.EOF && !strings.Contains(err.Error(), "use of closed network connection") {
					p.logger.Error("guacd read error", map[string]interface{}{"error": err.Error()})
					errChan <- err
				}
				shutdown()
				return
			}

			if n == 0 {
				continue
			}

			// Append to pending buffer
			pending = append(pending, buf[:n]...)

			// Process complete instructions
			for {
				semicolonIdx := bytes.IndexByte(pending, ';')
				if semicolonIdx == -1 {
					break // No complete instruction
				}

				// Extract instruction including semicolon
				instr := pending[:semicolonIdx+1]
				pending = pending[semicolonIdx+1:]

				// Send to async channel (non-blocking)
				select {
				case asyncChan <- append([]byte(nil), instr...):
				default:
					// Channel full, skip recording (live stream takes priority)
				}

				// Forward to WebSocket immediately
				if err := wsConn.WriteMessage(websocket.TextMessage, instr); err != nil {
					if !strings.Contains(err.Error(), "use of closed network connection") {
						p.logger.Error("ws write error", map[string]interface{}{"error": err.Error()})
						errChan <- err
					}
					shutdown()
					return
				}

				atomic.AddInt64(&bytesReceived, int64(len(instr)))
			}
		}
	}()

	// websocket -> guacd
	go func() {
		defer wg.Done()

		for {
			_, message, err := wsConn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					p.logger.Error("ws read error", map[string]interface{}{"error": err.Error()})
					errChan <- err
				}
				shutdown()
				return
			}

			// Check for keep-alive (empty instruction: "0.;")
			if bytes.Equal(message, []byte("0.;")) {
				wsConn.WriteMessage(websocket.TextMessage, []byte("3.nop;"))
				continue
			}

			// Forward raw bytes to guacd
			if _, err := guacdConn.Write(message); err != nil {
				if !strings.Contains(err.Error(), "use of closed network connection") {
					p.logger.Error("guacd write error", map[string]interface{}{"error": err.Error()})
					errChan <- err
				}
				shutdown()
				return
			}

			atomic.AddInt64(&bytesSent, int64(len(message)))
		}
	}()

	// Wait for completion
	doneChan := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneChan)
	}()

	var finalErr error
	select {
	case <-ctx.Done():
		p.logger.Info("RDP session cancelled by context")
		finalErr = ctx.Err()
	case err := <-errChan:
		finalErr = err
	case <-doneChan:
		auditLog.BytesSent = bytesSent
		auditLog.BytesReceived = bytesReceived
	}

	shutdown()

	// Log stats
	if p.asyncRecorder != nil {
		if written, dropped, ok := p.asyncRecorder.GetStats(sessionID); ok {
			p.logger.Info("Recording stats", map[string]interface{}{
				"session_id":     sessionID,
				"written":        written,
				"dropped":        dropped,
				"bytes_received": bytesReceived,
				"bytes_sent":     bytesSent,
			})
		}
	}

	return finalErr
}

// parseInstruction parses a Guacamole instruction from bytes
func (p *FastProxy) parseInstruction(data []byte) (string, []string, error) {
	if len(data) == 0 || data[len(data)-1] != ';' {
		return "", nil, fmt.Errorf("invalid instruction")
	}

	// Remove trailing semicolon
	data = data[:len(data)-1]

	var elements []string
	pos := 0

	for pos < len(data) {
		// Find the dot
		dotPos := bytes.IndexByte(data[pos:], '.')
		if dotPos == -1 {
			return "", nil, fmt.Errorf("missing dot in instruction")
		}
		dotPos += pos

		// Parse length
		lenStr := string(data[pos:dotPos])
		var length int
		if _, err := fmt.Sscanf(lenStr, "%d", &length); err != nil {
			return "", nil, fmt.Errorf("invalid length: %s", lenStr)
		}

		// Extract content
		contentStart := dotPos + 1
		contentEnd := contentStart + length
		if contentEnd > len(data) {
			return "", nil, fmt.Errorf("content exceeds data length")
		}

		elements = append(elements, string(data[contentStart:contentEnd]))

		// Move past content
		pos = contentEnd

		// Check for comma or end
		if pos < len(data) {
			if data[pos] == ',' {
				pos++ // Skip comma
			}
		}
	}

	if len(elements) == 0 {
		return "", nil, fmt.Errorf("empty instruction")
	}

	return elements[0], elements[1:], nil
}

// buildInstruction builds a Guacamole instruction
func (p *FastProxy) buildInstruction(opcode string, args ...string) []byte {
	var sb strings.Builder
	sb.Grow(len(opcode) + len(args)*20) // Pre-allocate

	sb.WriteString(fmt.Sprintf("%d.%s", len(opcode), opcode))
	for _, arg := range args {
		sb.WriteString(fmt.Sprintf(",%d.%s", len(arg), arg))
	}
	sb.WriteByte(';')

	return []byte(sb.String())
}

// readInstruction reads a complete instruction (used during handshake only)
func (p *FastProxy) readInstruction(reader *bufio.Reader) (string, []string, error) {
	var instrBytes []byte

	for {
		b, err := reader.ReadByte()
		if err != nil {
			return "", nil, err
		}
		instrBytes = append(instrBytes, b)
		if b == ';' {
			break
		}
	}

	return p.parseInstruction(instrBytes)
}
