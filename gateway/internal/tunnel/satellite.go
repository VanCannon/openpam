package tunnel

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/bvanc/openpam/gateway/internal/logger"
	"github.com/gorilla/websocket"
)

// SatelliteClient connects to the hub and maintains a reverse tunnel
type SatelliteClient struct {
	hubAddress string
	zoneID     string
	zoneName   string
	logger     *logger.Logger
	conn       *websocket.Conn
	connections map[string]net.Conn
}

// NewSatelliteClient creates a new satellite client
func NewSatelliteClient(hubAddress, zoneID, zoneName string, log *logger.Logger) *SatelliteClient {
	return &SatelliteClient{
		hubAddress:  hubAddress,
		zoneID:      zoneID,
		zoneName:    zoneName,
		logger:      log,
		connections: make(map[string]net.Conn),
	}
}

// Connect establishes connection to the hub
func (s *SatelliteClient) Connect(ctx context.Context) error {
	s.logger.Info("Connecting to hub", map[string]interface{}{
		"hub_address": s.hubAddress,
		"zone_name":   s.zoneName,
	})

	// Connect via WebSocket
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, s.hubAddress, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to hub: %w", err)
	}

	s.conn = conn

	// Send registration message
	if err := s.register(); err != nil {
		s.conn.Close()
		return fmt.Errorf("failed to register with hub: %w", err)
	}

	s.logger.Info("Successfully connected and registered with hub")

	// Start message handler
	go s.handleMessages(ctx)

	return nil
}

// register sends registration message to hub
func (s *SatelliteClient) register() error {
	msg := NewMessage(MessageTypeRegister)
	payload := RegisterPayload{
		ZoneID:   s.zoneID,
		ZoneName: s.zoneName,
		Version:  "0.1.0",
	}

	if err := msg.SetPayload(payload); err != nil {
		return err
	}

	data, err := msg.Encode()
	if err != nil {
		return err
	}

	return s.conn.WriteMessage(websocket.TextMessage, data)
}

// handleMessages processes messages from the hub
func (s *SatelliteClient) handleMessages(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			_, data, err := s.conn.ReadMessage()
			if err != nil {
				s.logger.Error("Error reading message from hub", map[string]interface{}{
					"error": err.Error(),
				})
				return
			}

			msg, err := DecodeMessage(data)
			if err != nil {
				s.logger.Error("Failed to decode message", map[string]interface{}{
					"error": err.Error(),
				})
				continue
			}

			if err := s.handleMessage(ctx, msg); err != nil {
				s.logger.Error("Failed to handle message", map[string]interface{}{
					"type":  msg.Type,
					"error": err.Error(),
				})
			}
		}
	}
}

// handleMessage processes a single message
func (s *SatelliteClient) handleMessage(ctx context.Context, msg *Message) error {
	switch msg.Type {
	case MessageTypeRegisterAck:
		return s.handleRegisterAck(msg)
	case MessageTypeDialRequest:
		return s.handleDialRequest(ctx, msg)
	case MessageTypeData:
		return s.handleData(msg)
	case MessageTypeClose:
		return s.handleClose(msg)
	case MessageTypePing:
		return s.handlePing()
	default:
		s.logger.Warn("Unknown message type", map[string]interface{}{
			"type": msg.Type,
		})
	}
	return nil
}

// handleRegisterAck processes registration acknowledgment
func (s *SatelliteClient) handleRegisterAck(msg *Message) error {
	var payload RegisterAckPayload
	if err := msg.GetPayload(&payload); err != nil {
		return err
	}

	if !payload.Accepted {
		return fmt.Errorf("registration rejected: %s", payload.Message)
	}

	s.logger.Info("Registration accepted by hub")
	return nil
}

// handleDialRequest dials a target and establishes connection
func (s *SatelliteClient) handleDialRequest(ctx context.Context, msg *Message) error {
	var payload DialRequestPayload
	if err := msg.GetPayload(&payload); err != nil {
		return err
	}

	s.logger.Info("Dialing target", map[string]interface{}{
		"host":       payload.TargetHost,
		"port":       payload.TargetPort,
		"protocol":   payload.Protocol,
		"connection": msg.ConnectionID,
	})

	// Dial the target
	addr := fmt.Sprintf("%s:%d", payload.TargetHost, payload.TargetPort)
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)

	response := NewMessage(MessageTypeDialResponse)
	response.ConnectionID = msg.ConnectionID

	if err != nil {
		responsePayload := DialResponsePayload{
			Success: false,
			Error:   err.Error(),
		}
		response.SetPayload(responsePayload)
	} else {
		s.connections[msg.ConnectionID] = conn
		responsePayload := DialResponsePayload{
			Success: true,
		}
		response.SetPayload(responsePayload)

		// Start proxying data
		go s.proxyConnection(ctx, msg.ConnectionID, conn)
	}

	data, _ := response.Encode()
	return s.conn.WriteMessage(websocket.TextMessage, data)
}

// proxyConnection proxies data between target and hub
func (s *SatelliteClient) proxyConnection(ctx context.Context, connectionID string, targetConn net.Conn) {
	defer func() {
		targetConn.Close()
		delete(s.connections, connectionID)

		// Send close message
		closeMsg := NewMessage(MessageTypeClose)
		closeMsg.ConnectionID = connectionID
		closeMsg.SetPayload(ClosePayload{Reason: "connection closed"})
		data, _ := closeMsg.Encode()
		s.conn.WriteMessage(websocket.TextMessage, data)
	}()

	// Read from target and send to hub
	buffer := make([]byte, 8192)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			targetConn.SetReadDeadline(time.Now().Add(30 * time.Second))
			n, err := targetConn.Read(buffer)
			if err != nil {
				return
			}

			dataMsg := NewMessage(MessageTypeData)
			dataMsg.ConnectionID = connectionID
			dataMsg.SetPayload(DataPayload{Data: buffer[:n]})

			msgData, _ := dataMsg.Encode()
			if err := s.conn.WriteMessage(websocket.TextMessage, msgData); err != nil {
				return
			}
		}
	}
}

// handleData receives data from hub and writes to target
func (s *SatelliteClient) handleData(msg *Message) error {
	conn, exists := s.connections[msg.ConnectionID]
	if !exists {
		return fmt.Errorf("connection not found: %s", msg.ConnectionID)
	}

	var payload DataPayload
	if err := msg.GetPayload(&payload); err != nil {
		return err
	}

	_, err := conn.Write(payload.Data)
	return err
}

// handleClose closes a connection
func (s *SatelliteClient) handleClose(msg *Message) error {
	conn, exists := s.connections[msg.ConnectionID]
	if !exists {
		return nil
	}

	conn.Close()
	delete(s.connections, msg.ConnectionID)
	return nil
}

// handlePing responds to ping with pong
func (s *SatelliteClient) handlePing() error {
	pongMsg := NewMessage(MessageTypePong)
	data, _ := pongMsg.Encode()
	return s.conn.WriteMessage(websocket.TextMessage, data)
}

// Close closes the satellite client
func (s *SatelliteClient) Close() error {
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}
