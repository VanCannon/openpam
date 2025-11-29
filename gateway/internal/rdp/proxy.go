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
	"time"

	"github.com/VanCannon/openpam/gateway/internal/logger"
	"github.com/VanCannon/openpam/gateway/internal/models"
	"github.com/VanCannon/openpam/gateway/internal/ssh"
	"github.com/VanCannon/openpam/gateway/internal/vault"

	"github.com/gorilla/websocket"
)

// Proxy handles RDP protocol proxying via Apache Guacamole daemon
type Proxy struct {
	guacdAddress string
	logger       *logger.Logger
	recorder     *Recorder
	monitor      *ssh.Monitor
}

// NewProxy creates a new RDP proxy
func NewProxy(guacdAddress string, log *logger.Logger, recorder *Recorder, monitor *ssh.Monitor) *Proxy {
	return &Proxy{
		guacdAddress: guacdAddress,
		logger:       log,
		recorder:     recorder,
		monitor:      monitor,
	}
}

// Handle proxies an RDP connection over WebSocket using Guacamole protocol
func (p *Proxy) Handle(
	ctx context.Context,
	wsConn *websocket.Conn,
	target *models.Target,
	creds *vault.Credentials,
	auditLog *models.AuditLog,
	width int,
	height int,
) error {
	// Connect to guacd
	guacdConn, err := net.Dial("tcp", p.guacdAddress)
	if err != nil {
		return fmt.Errorf("failed to connect to guacd: %w", err)
	}
	defer guacdConn.Close()

	// Use buffered reader for guacd connection
	guacdReader := bufio.NewReader(guacdConn)

	p.logger.Info("Connected to guacd", map[string]interface{}{
		"address": p.guacdAddress,
		"target":  target.Hostname,
	})

	// Start recording if recorder is available
	if p.recorder != nil {
		if err := p.recorder.StartRecording(ctx, auditLog.ID.String()); err != nil {
			p.logger.Error("Failed to start recording", map[string]interface{}{
				"error": err.Error(),
			})
		} else {
			defer p.recorder.StopRecording(auditLog.ID.String())
		}
	}

	// 1. Handshake with guacd
	// ... (rest of handshake logic remains the same until proxy loop)

	// Send "select" instruction
	if err := p.sendInstruction(guacdConn, "select", "rdp"); err != nil {
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

	p.logger.Info("Received args from guacd", map[string]interface{}{
		"args": args,
	})

	// Construct "size" instruction (client screen size)
	// We must record and broadcast this so monitors/replay know the screen size
	if p.recorder != nil {
		p.recorder.WriteInstruction(auditLog.ID.String(), "size", "0", fmt.Sprintf("%d", width), fmt.Sprintf("%d", height), "96")
	}

	// Keep track of header messages to send to new subscribers
	var headerBuilder strings.Builder

	if p.monitor != nil {
		// Broadcast size: 4.size,1.0,4.1024,3.768,2.96;
		msg := fmt.Sprintf("4.size,1.0,%d.%d,%d.%d,2.96;", len(fmt.Sprintf("%d", width)), width, len(fmt.Sprintf("%d", height)), height)
		headerBuilder.WriteString(msg)
		p.monitor.SetHeader(auditLog.ID.String(), []byte(headerBuilder.String()))
		p.monitor.Broadcast(auditLog.ID.String(), []byte(msg))
	}

	if err := p.sendInstruction(guacdConn, "size", fmt.Sprintf("%d", width), fmt.Sprintf("%d", height), "96"); err != nil {
		return fmt.Errorf("failed to send size to guacd: %w", err)
	}

	// Construct "audio" and "video" instructions (supported formats)
	if err := p.sendInstruction(guacdConn, "audio", "audio/L16", "rate=44100", "channels=2"); err != nil {
		return fmt.Errorf("failed to send audio to guacd: %w", err)
	}
	if err := p.sendInstruction(guacdConn, "video", "image/jpeg", "image/png", "image/webp"); err != nil {
		return fmt.Errorf("failed to send video to guacd: %w", err)
	}

	// Construct "image" instruction (supported image formats)
	if err := p.sendInstruction(guacdConn, "image", "image/png", "image/jpeg"); err != nil {
		return fmt.Errorf("failed to send image to guacd: %w", err)
	}

	// Connection parameters
	config := map[string]string{
		"hostname":               target.Hostname,
		"port":                   fmt.Sprintf("%d", target.Port),
		"username":               creds.Username,
		"password":               creds.Password,
		"ignore-cert":            "true",
		"security":               "any",
		"disable-bitmap-caching": "true",
		"enable-wallpaper":       "true",
		"enable-theming":         "true",
		"enable-menu-animations": "true",
		"color-depth":            "32",
		"width":                  fmt.Sprintf("%d", width),
		"height":                 fmt.Sprintf("%d", height),
		"dpi":                    "96",
		"resize-method":          "display-update",
	}

	// Respond to "args" with "connect"
	// Match the reference implementation exactly - treat all args the same
	connectArgs := make([]string, len(args))
	for i, argName := range args {
		if val, ok := config[argName]; ok {
			connectArgs[i] = val
		} else {
			connectArgs[i] = ""
		}
	}

	p.logger.Info("Sending connect instruction to guacd", map[string]interface{}{
		"instruction": "connect",
		"num_args":    len(connectArgs),
	})

	if err := p.sendInstruction(guacdConn, "connect", connectArgs...); err != nil {
		return fmt.Errorf("failed to send connect to guacd: %w", err)
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

	// Record and broadcast "ready"
	if p.recorder != nil {
		p.recorder.WriteInstruction(auditLog.ID.String(), "ready", readyArgs...)
	}
	if p.monitor != nil {
		var sb strings.Builder
		sb.WriteString("5.ready")
		for _, arg := range readyArgs {
			sb.WriteString(fmt.Sprintf(",%d.%s", len(arg), arg))
		}
		sb.WriteString(";")
		msg := sb.String()

		headerBuilder.WriteString(msg)
		p.monitor.SetHeader(auditLog.ID.String(), []byte(headerBuilder.String()))
		p.monitor.Broadcast(auditLog.ID.String(), []byte(msg))
	}

	// Send "ready" to client
	if err := p.sendInstruction(&wsWriter{wsConn}, "ready", readyArgs...); err != nil {
		return fmt.Errorf("failed to send ready to client: %w", err)
	}

	// Send "size" to client to ensure display is sized correctly
	// layer 0, width, height
	if err := p.sendInstruction(&wsWriter{wsConn}, "size", "0", fmt.Sprintf("%d", width), fmt.Sprintf("%d", height)); err != nil {
		return fmt.Errorf("failed to send size to client: %w", err)
	}

	// Proxy loop
	var wg sync.WaitGroup
	wg.Add(2)

	// Use a channel to signal that one side has closed the connection
	// This allows us to unblock the other side
	doneChan := make(chan struct{})
	errChan := make(chan error, 2)
	var bytesSent, bytesReceived int64

	// Ensure connections are closed when we exit to unblock goroutines
	defer wsConn.Close()
	defer guacdConn.Close()

	// guacd -> websocket
	go func() {
		defer wg.Done()
		// We parse instructions here to record them
		for {
			opcode, args, err := p.readInstruction(guacdReader)
			if err != nil {
				if err != io.EOF {
					// Only log real errors, not normal EOF
					if !strings.Contains(err.Error(), "use of closed network connection") {
						p.logger.Error("guacd read error", map[string]interface{}{"error": err.Error()})
						errChan <- err
					}
				} else {
					p.logger.Info("guacd connection closed (EOF)")
				}
				return
			}

			// Record instruction
			if p.recorder != nil {
				if err := p.recorder.WriteInstruction(auditLog.ID.String(), opcode, args...); err != nil {
					p.logger.Error("Failed to record instruction", map[string]interface{}{
						"error": err.Error(),
					})
				}
			}

			// Broadcast to monitor
			if p.monitor != nil {
				// We need to reconstruct the instruction string to broadcast it
				// Format: opcode,arg,arg;
				// Note: The monitor expects raw bytes to send to the websocket.
				// The frontend player will expect the standard Guacamole protocol stream.
				var sb strings.Builder
				sb.WriteString(fmt.Sprintf("%d.%s", len(opcode), opcode))
				for _, arg := range args {
					sb.WriteString(fmt.Sprintf(",%d.%s", len(arg), arg))
				}
				sb.WriteString(";")
				p.monitor.Broadcast(auditLog.ID.String(), []byte(sb.String()))
			}

			// Forward to WebSocket
			if err := p.sendInstruction(&wsWriter{wsConn}, opcode, args...); err != nil {
				if !strings.Contains(err.Error(), "use of closed network connection") {
					p.logger.Error("ws write error", map[string]interface{}{"error": err.Error()})
					errChan <- err
				}
				return
			}

			// Estimate bytes received (rough approximation since we re-serialized)
			bytesReceived += 100 // Placeholder
		}
	}()

	// websocket -> guacd
	go func() {
		defer wg.Done()

		// Create a ticker to send keep-alives to guacd
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		// Goroutine to handle keep-alives
		go func() {
			for {
				select {
				case <-ticker.C:
					// Send nop to guacd to keep connection alive
					err := p.sendInstruction(guacdConn, "nop")
					if err != nil {
						// If error (e.g. closed connection), stop
						return
					}
				case <-doneChan:
					return
				}
			}
		}()

		for {
			_, message, err := wsConn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					p.logger.Error("ws read error", map[string]interface{}{"error": err.Error()})
					errChan <- err
				} else {
					p.logger.Info("WebSocket closed normally")
				}
				return
			}

			// Parse and filter instructions
			reader := bufio.NewReader(bytes.NewReader(message))
			for {
				opcode, args, err := p.readInstruction(reader)
				if err != nil {
					if err != io.EOF && err.Error() != "EOF" {
						p.logger.Error("Error parsing instruction from ws", map[string]interface{}{"error": err.Error()})
					}
					break
				}

				// Ignore internal "empty" opcode (used for keep-alive/internal)
				if opcode == "" {
					// Respond to keep-alive
					err = p.sendInstruction(&wsWriter{wsConn}, "nop")
					if err != nil {
						return
					}
					continue
				}

				// Forward instruction to guacd
				err = p.sendInstruction(guacdConn, opcode, args...)
				if err != nil {
					if !strings.Contains(err.Error(), "use of closed network connection") {
						p.logger.Error("guacd write error", map[string]interface{}{"error": err.Error()})
						errChan <- err
					}
					return
				}
			}
		}
	}()

	// Wait for completion
	go func() {
		wg.Wait()
		close(doneChan)
	}()

	select {
	case <-ctx.Done():
		p.logger.Info("RDP session cancelled by context")
		return ctx.Err()
	case err := <-errChan:
		return err
	case <-doneChan:
		// Success
		auditLog.BytesSent = bytesSent
		auditLog.BytesReceived = bytesReceived
		return nil
	}
}

// wsWriter wraps websocket.Conn to satisfy io.Writer
type wsWriter struct {
	*websocket.Conn
}

func (w *wsWriter) Write(p []byte) (int, error) {
	err := w.Conn.WriteMessage(websocket.TextMessage, p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

// sendInstruction sends a Guacamole instruction to the writer
func (p *Proxy) sendInstruction(w io.Writer, opcode string, args ...string) error {
	var sb strings.Builder

	// Opcode
	sb.WriteString(fmt.Sprintf("%d.%s", len(opcode), opcode))

	// Args
	for _, arg := range args {
		sb.WriteString(fmt.Sprintf(",%d.%s", len(arg), arg))
	}

	sb.WriteString(";")

	_, err := w.Write([]byte(sb.String()))
	return err
}

// readInstruction reads a Guacamole instruction from the reader
func (p *Proxy) readInstruction(reader *bufio.Reader) (string, []string, error) {
	var elements []string
	var currentElement strings.Builder
	var length int

	for {
		// Read length
		lenStr, err := reader.ReadString('.')
		if err != nil {
			return "", nil, err
		}
		lenStr = strings.TrimSuffix(lenStr, ".")

		if _, err := fmt.Sscanf(lenStr, "%d", &length); err != nil {
			return "", nil, fmt.Errorf("invalid length: %w", err)
		}

		// Read content
		content := make([]byte, length)
		if _, err := io.ReadFull(reader, content); err != nil {
			return "", nil, err
		}
		currentElement.Write(content)
		elements = append(elements, currentElement.String())
		currentElement.Reset()

		// Check delimiter
		delim, err := reader.ReadByte()
		if err != nil {
			return "", nil, err
		}

		if delim == ';' {
			break
		} else if delim != ',' {
			return "", nil, fmt.Errorf("unexpected delimiter: %c", delim)
		}
	}

	if len(elements) == 0 {
		return "", nil, fmt.Errorf("empty instruction")
	}

	return elements[0], elements[1:], nil
}
