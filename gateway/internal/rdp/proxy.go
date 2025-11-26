package rdp

import (
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
		"target":  target.Hostname,
	})

	// Store connection parameters for injection
	// We'll intercept the client's connect instruction and inject our credentials
	connectionParams := map[string]string{
		"hostname": target.Hostname,
		"port":     fmt.Sprintf("%d", target.Port),
		"username": creds.Username,
		"password": creds.Password,
	}

	// Proxy data between WebSocket and guacd
	var wg sync.WaitGroup
	var bytesSent, bytesReceived int64

	errChan := make(chan error, 2)

	// Send initial "select" instruction to guacd to start the handshake
	// This initiates the protocol and tells guacd we want RDP
	selectInstruction := "6.select,3.rdp;"
	p.logger.Info("Initiating handshake with guacd", map[string]interface{}{"select": selectInstruction})
	if _, err := guacdConn.Write([]byte(selectInstruction)); err != nil {
		return fmt.Errorf("failed to send select to guacd: %w", err)
	}

	// Read the args response from guacd and forward it to the client
	// This tells the client what parameters are available
	buffer := make([]byte, 8192)
	n, err := guacdConn.Read(buffer)
	if err != nil {
		return fmt.Errorf("failed to read args from guacd: %w", err)
	}
	argsResponse := buffer[:n]
	p.logger.Info("Received args from guacd", map[string]interface{}{
		"args":   string(argsResponse),
		"length": n,
	})

	// Forward the args instruction to the WebSocket client
	if err := wsConn.WriteMessage(websocket.TextMessage, argsResponse); err != nil {
		return fmt.Errorf("failed to send args to client: %w", err)
	}
	p.logger.Info("Forwarded args to client")

	// WebSocket -> guacd
	wg.Add(1)
	go func() {
		defer wg.Done()
		messageCount := 0
		for {
			_, data, err := wsConn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					p.logger.Info("WebSocket closed normally")
				} else {
					p.logger.Error("WebSocket read error", map[string]interface{}{"error": err.Error()})
					errChan <- fmt.Errorf("WebSocket read error: %w", err)
				}
				return
			}

			bytesSent += int64(len(data))
			messageCount++

			dataStr := string(data)

			// Log first few messages from client
			if messageCount <= 10 {
				p.logger.Info("Client -> guacd", map[string]interface{}{
					"message_num": messageCount,
					"data":        dataStr,
					"length":      len(data),
				})
			}

			// Intercept connect instruction to inject credentials
			if len(dataStr) > 7 && dataStr[:7] == "7.connect" {
				// Parse and modify the connect instruction
				// The client sends: 7.connect,hostname,port,...
				// We need to inject our actual credentials
				modifiedConnect := p.injectCredentials(dataStr, connectionParams)
				p.logger.Info("Modified connect instruction", map[string]interface{}{
					"original": dataStr,
					"modified": modifiedConnect,
				})
				data = []byte(modifiedConnect)
			}

			if _, err := guacdConn.Write(data); err != nil {
				p.logger.Error("guacd write error", map[string]interface{}{"error": err.Error()})
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
		messageCount := 0
		for {
			n, err := guacdConn.Read(buffer)
			if err != nil {
				if err == io.EOF {
					p.logger.Info("guacd connection closed (EOF)")
				} else {
					p.logger.Error("guacd read error", map[string]interface{}{"error": err.Error()})
					errChan <- fmt.Errorf("guacd read error: %w", err)
				}
				return
			}

			bytesReceived += int64(n)
			messageCount++

			data := buffer[:n]

			// Log first few messages from guacd
			if messageCount <= 5 {
				p.logger.Info("guacd -> Client", map[string]interface{}{
					"message_num": messageCount,
					"data":        string(data),
					"length":      n,
				})
			}

			if err := wsConn.WriteMessage(websocket.TextMessage, data); err != nil {
				p.logger.Error("WebSocket write error", map[string]interface{}{"error": err.Error()})
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

// injectCredentials modifies a Guacamole connect instruction to inject our credentials
func (p *Proxy) injectCredentials(connectMsg string, params map[string]string) string {
	// Parse the Guacamole connect instruction
	// Format: 7.connect,14.192.168.10.184,4.3389,0.,0.,...;
	// We need to replace hostname (position 0), port (position 1), username (position 3), password (position 4)

	// For simplicity, we'll rebuild the connect instruction with our parameters
	// The order from the args instruction is: hostname, port, domain, username, password, width, height, ...

	// Extract everything after "7.connect," and before the final ";"
	if len(connectMsg) < 10 || connectMsg[:10] != "7.connect," {
		return connectMsg
	}

	// Build new connect instruction with injected credentials
	result := "7.connect"
	result += fmt.Sprintf(",%d.%s", len(params["hostname"]), params["hostname"])
	result += fmt.Sprintf(",%d.%s", len(params["port"]), params["port"])
	result += ",0." // domain (empty)
	result += fmt.Sprintf(",%d.%s", len(params["username"]), params["username"])
	result += fmt.Sprintf(",%d.%s", len(params["password"]), params["password"])

	// Extract the rest of the parameters from the original message (width, height, etc.)
	// Skip the first 5 parameters (hostname, port, domain, username, password)
	rest := connectMsg[10:] // Skip "7.connect,"
	paramCount := 0
	i := 0
	for i < len(rest) {
		// Find the length prefix
		dotPos := -1
		for j := i; j < len(rest) && j < i+10; j++ {
			if rest[j] == '.' {
				dotPos = j
				break
			}
		}
		if dotPos == -1 {
			break
		}

		lengthStr := rest[i:dotPos]
		length := 0
		fmt.Sscanf(lengthStr, "%d", &length)

		// Skip this parameter's value
		valueStart := dotPos + 1
		valueEnd := valueStart + length

		paramCount++
		if paramCount > 5 {
			// Include this parameter in the result
			if valueEnd <= len(rest) {
				result += "," + rest[i:valueEnd]
			}
		}

		// Move to next parameter
		i = valueEnd
		if i < len(rest) && rest[i] == ',' {
			i++
		}
		if i < len(rest) && rest[i] == ';' {
			break
		}
	}

	result += ";"
	return result
}
