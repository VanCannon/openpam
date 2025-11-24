package tunnel

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/bvanc/openpam/gateway/internal/logger"
	"github.com/gorilla/websocket"
	"github.com/google/uuid"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true // TODO: Implement proper origin checking
	},
}

// HubServer manages satellite connections
type HubServer struct {
	logger     *logger.Logger
	satellites map[string]*SatelliteConnection
	mu         sync.RWMutex
}

// SatelliteConnection represents a connected satellite
type SatelliteConnection struct {
	ZoneID      string
	ZoneName    string
	Conn        *websocket.Conn
	Connections map[string]chan []byte // connection_id -> data channel
	mu          sync.RWMutex
}

// NewHubServer creates a new hub server
func NewHubServer(log *logger.Logger) *HubServer {
	return &HubServer{
		logger:     log,
		satellites: make(map[string]*SatelliteConnection),
	}
}

// HandleSatelliteConnection handles a new satellite WebSocket connection
func (h *HubServer) HandleSatelliteConnection() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			h.logger.Error("Failed to upgrade satellite connection", map[string]interface{}{
				"error": err.Error(),
			})
			return
		}

		// Wait for registration message
		_, data, err := conn.ReadMessage()
		if err != nil {
			h.logger.Error("Failed to read registration", map[string]interface{}{
				"error": err.Error(),
			})
			conn.Close()
			return
		}

		msg, err := DecodeMessage(data)
		if err != nil || msg.Type != MessageTypeRegister {
			h.logger.Error("Invalid registration message")
			conn.Close()
			return
		}

		var payload RegisterPayload
		if err := msg.GetPayload(&payload); err != nil {
			h.logger.Error("Invalid registration payload", map[string]interface{}{
				"error": err.Error(),
			})
			conn.Close()
			return
		}

		h.logger.Info("Satellite registering", map[string]interface{}{
			"zone_id":   payload.ZoneID,
			"zone_name": payload.ZoneName,
			"version":   payload.Version,
		})

		// Create satellite connection
		satellite := &SatelliteConnection{
			ZoneID:      payload.ZoneID,
			ZoneName:    payload.ZoneName,
			Conn:        conn,
			Connections: make(map[string]chan []byte),
		}

		h.mu.Lock()
		h.satellites[payload.ZoneID] = satellite
		h.mu.Unlock()

		// Send registration acknowledgment
		ackMsg := NewMessage(MessageTypeRegisterAck)
		ackMsg.SetPayload(RegisterAckPayload{
			Accepted: true,
			Message:  "Registration successful",
		})
		ackData, _ := ackMsg.Encode()
		conn.WriteMessage(websocket.TextMessage, ackData)

		h.logger.Info("Satellite registered successfully", map[string]interface{}{
			"zone_name": payload.ZoneName,
		})

		// Handle messages from satellite
		h.handleSatelliteMessages(context.Background(), satellite)

		// Cleanup on disconnect
		h.mu.Lock()
		delete(h.satellites, payload.ZoneID)
		h.mu.Unlock()

		h.logger.Info("Satellite disconnected", map[string]interface{}{
			"zone_name": payload.ZoneName,
		})
	}
}

// handleSatelliteMessages processes messages from a satellite
func (h *HubServer) handleSatelliteMessages(ctx context.Context, satellite *SatelliteConnection) {
	for {
		_, data, err := satellite.Conn.ReadMessage()
		if err != nil {
			h.logger.Error("Error reading from satellite", map[string]interface{}{
				"error":     err.Error(),
				"zone_name": satellite.ZoneName,
			})
			return
		}

		msg, err := DecodeMessage(data)
		if err != nil {
			h.logger.Error("Failed to decode satellite message", map[string]interface{}{
				"error": err.Error(),
			})
			continue
		}

		switch msg.Type {
		case MessageTypeDialResponse:
			h.handleDialResponse(satellite, msg)
		case MessageTypeData:
			h.handleSatelliteData(satellite, msg)
		case MessageTypeClose:
			h.handleSatelliteClose(satellite, msg)
		case MessageTypePong:
			// Keepalive response
		default:
			h.logger.Warn("Unknown message type from satellite", map[string]interface{}{
				"type": msg.Type,
			})
		}
	}
}

// RequestDial requests satellite to dial a target
func (h *HubServer) RequestDial(zoneID, targetHost string, targetPort int, protocol, username, password, privateKey string) (string, chan []byte, error) {
	h.mu.RLock()
	satellite, exists := h.satellites[zoneID]
	h.mu.RUnlock()

	if !exists {
		return "", nil, fmt.Errorf("satellite not connected: %s", zoneID)
	}

	connectionID := uuid.New().String()

	// Create data channel for this connection
	dataChan := make(chan []byte, 100)
	satellite.mu.Lock()
	satellite.Connections[connectionID] = dataChan
	satellite.mu.Unlock()

	// Send dial request
	dialMsg := NewMessage(MessageTypeDialRequest)
	dialMsg.ConnectionID = connectionID
	dialMsg.SetPayload(DialRequestPayload{
		TargetHost: targetHost,
		TargetPort: targetPort,
		Protocol:   protocol,
		Username:   username,
		Password:   password,
		PrivateKey: privateKey,
	})

	msgData, _ := dialMsg.Encode()
	if err := satellite.Conn.WriteMessage(websocket.TextMessage, msgData); err != nil {
		satellite.mu.Lock()
		delete(satellite.Connections, connectionID)
		satellite.mu.Unlock()
		return "", nil, fmt.Errorf("failed to send dial request: %w", err)
	}

	return connectionID, dataChan, nil
}

// SendData sends data through a tunnel connection
func (h *HubServer) SendData(zoneID, connectionID string, data []byte) error {
	h.mu.RLock()
	satellite, exists := h.satellites[zoneID]
	h.mu.RUnlock()

	if !exists {
		return fmt.Errorf("satellite not connected")
	}

	dataMsg := NewMessage(MessageTypeData)
	dataMsg.ConnectionID = connectionID
	dataMsg.SetPayload(DataPayload{Data: data})

	msgData, _ := dataMsg.Encode()
	return satellite.Conn.WriteMessage(websocket.TextMessage, msgData)
}

// CloseConnection closes a tunnel connection
func (h *HubServer) CloseConnection(zoneID, connectionID string) error {
	h.mu.RLock()
	satellite, exists := h.satellites[zoneID]
	h.mu.RUnlock()

	if !exists {
		return nil
	}

	satellite.mu.Lock()
	if ch, exists := satellite.Connections[connectionID]; exists {
		close(ch)
		delete(satellite.Connections, connectionID)
	}
	satellite.mu.Unlock()

	closeMsg := NewMessage(MessageTypeClose)
	closeMsg.ConnectionID = connectionID
	closeMsg.SetPayload(ClosePayload{Reason: "connection closed"})

	msgData, _ := closeMsg.Encode()
	return satellite.Conn.WriteMessage(websocket.TextMessage, msgData)
}

// handleDialResponse processes dial response from satellite
func (h *HubServer) handleDialResponse(satellite *SatelliteConnection, msg *Message) {
	var payload DialResponsePayload
	if err := msg.GetPayload(&payload); err != nil {
		h.logger.Error("Failed to parse dial response", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	if !payload.Success {
		h.logger.Error("Satellite failed to dial target", map[string]interface{}{
			"connection": msg.ConnectionID,
			"error":      payload.Error,
		})

		// Close data channel
		satellite.mu.Lock()
		if ch, exists := satellite.Connections[msg.ConnectionID]; exists {
			close(ch)
			delete(satellite.Connections, msg.ConnectionID)
		}
		satellite.mu.Unlock()
	}
}

// handleSatelliteData processes data from satellite
func (h *HubServer) handleSatelliteData(satellite *SatelliteConnection, msg *Message) {
	var payload DataPayload
	if err := msg.GetPayload(&payload); err != nil {
		h.logger.Error("Failed to parse data payload", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	satellite.mu.RLock()
	dataChan, exists := satellite.Connections[msg.ConnectionID]
	satellite.mu.RUnlock()

	if exists {
		select {
		case dataChan <- payload.Data:
		default:
			h.logger.Warn("Data channel full, dropping data")
		}
	}
}

// handleSatelliteClose processes close message from satellite
func (h *HubServer) handleSatelliteClose(satellite *SatelliteConnection, msg *Message) {
	satellite.mu.Lock()
	if ch, exists := satellite.Connections[msg.ConnectionID]; exists {
		close(ch)
		delete(satellite.Connections, msg.ConnectionID)
	}
	satellite.mu.Unlock()
}

// GetSatellite returns a satellite connection by zone ID
func (h *HubServer) GetSatellite(zoneID string) (*SatelliteConnection, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	satellite, exists := h.satellites[zoneID]
	return satellite, exists
}
