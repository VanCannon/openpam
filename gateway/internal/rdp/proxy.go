package rdp

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/bvanc/openpam/gateway/internal/logger"
	"github.com/bvanc/openpam/gateway/internal/models"
	"github.com/bvanc/openpam/gateway/internal/vault"
	"github.com/gorilla/websocket"
)

// Proxy handles RDP protocol proxying via Apache Guacamole daemon
type Proxy struct {
	guacdAddress string
	logger       *logger.Logger
}

// NewProxy creates a new RDP proxy
func NewProxy(guacdAddress string, log *logger.Logger) *Proxy {
	return &Proxy{
		guacdAddress: guacdAddress,
		logger:       log,
	}
}

// Handle proxies an RDP connection over WebSocket using Guacamole protocol
func (p *Proxy) Handle(
	ctx context.Context,
	wsConn *websocket.Conn,
	target *models.Target,
	creds *vault.Credentials,
	auditLog *models.AuditLog,
) error {
	// Connect to guacd
	guacdConn, err := net.Dial("tcp", p.guacdAddress)
	if err != nil {
		return fmt.Errorf("failed to connect to guacd: %w", err)
	}
	defer guacdConn.Close()

	p.logger.Info("Connected to guacd", map[string]interface{}{
		"address": p.guacdAddress,
	})

	reader := bufio.NewReader(guacdConn)

	// Send Guacamole handshake
	handshake := p.buildHandshake(target, creds)
	if _, err := guacdConn.Write([]byte(handshake)); err != nil {
		return fmt.Errorf("failed to send handshake: %w", err)
	}

	p.logger.Debug("Sent Guacamole handshake")

	// Read handshake response
	response, err := p.readInstruction(reader)
	if err != nil {
		return fmt.Errorf("failed to read handshake response: %w", err)
	}

	p.logger.Debug("Received handshake response", map[string]interface{}{
		"response": response,
	})

	// Proxy data between WebSocket and guacd
	var wg sync.WaitGroup
	var bytesSent, bytesReceived int64

	errChan := make(chan error, 2)

	// WebSocket -> guacd
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			_, data, err := wsConn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					p.logger.Debug("WebSocket closed normally")
				} else {
					errChan <- fmt.Errorf("WebSocket read error: %w", err)
				}
				return
			}

			bytesSent += int64(len(data))

			if _, err := guacdConn.Write(data); err != nil {
				errChan <- fmt.Errorf("guacd write error: %w", err)
				return
			}
		}
	}()

	// guacd -> WebSocket
	wg.Add(1)
	go func() {
		defer wg.Done()
		buffer := make([]byte, 8192)
		for {
			n, err := guacdConn.Read(buffer)
			if err != nil {
				if err != io.EOF {
					errChan <- fmt.Errorf("guacd read error: %w", err)
				}
				return
			}

			bytesReceived += int64(n)

			data := buffer[:n]
			if err := wsConn.WriteMessage(websocket.BinaryMessage, data); err != nil {
				errChan <- fmt.Errorf("WebSocket write error: %w", err)
				return
			}
		}
	}()

	// Wait for completion or error
	select {
	case <-ctx.Done():
		p.logger.Info("RDP session cancelled by context")
		return ctx.Err()
	case err := <-errChan:
		wg.Wait()
		auditLog.BytesSent = bytesSent
		auditLog.BytesReceived = bytesReceived
		return err
	}
}

// buildHandshake builds the Guacamole protocol handshake
func (p *Proxy) buildHandshake(target *models.Target, creds *vault.Credentials) string {
	// Guacamole handshake format:
	// select,<protocol>,<arg1>,<value1>,<arg2>,<value2>,...;

	params := []string{
		"hostname", target.Hostname,
		"port", fmt.Sprintf("%d", target.Port),
		"username", creds.Username,
	}

	if creds.Password != "" {
		params = append(params, "password", creds.Password)
	}

	// Additional RDP parameters
	params = append(params,
		"security", "any",
		"ignore-cert", "true",
		"enable-drive", "false",
		"enable-printing", "false",
		"create-drive-path", "false",
	)

	// Build instruction
	handshake := "4.select,3.rdp"
	for i := 0; i < len(params); i += 2 {
		key := params[i]
		value := params[i+1]
		handshake += fmt.Sprintf(",%d.%s,%d.%s", len(key), key, len(value), value)
	}
	handshake += ";"

	return handshake
}

// readInstruction reads a Guacamole protocol instruction
func (p *Proxy) readInstruction(reader *bufio.Reader) (string, error) {
	instruction := ""
	for {
		b, err := reader.ReadByte()
		if err != nil {
			return "", err
		}
		instruction += string(b)
		if b == ';' {
			break
		}
	}
	return instruction, nil
}
