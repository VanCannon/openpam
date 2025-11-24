package tunnel

import (
	"encoding/json"
	"fmt"
)

// MessageType represents the type of tunnel message
type MessageType string

const (
	// MessageTypeRegister is sent by satellite to register with hub
	MessageTypeRegister MessageType = "register"

	// MessageTypeRegisterAck is sent by hub to acknowledge registration
	MessageTypeRegisterAck MessageType = "register_ack"

	// MessageTypeDialRequest is sent by hub to request satellite to dial a target
	MessageTypeDialRequest MessageType = "dial_request"

	// MessageTypeDialResponse is sent by satellite with dial result
	MessageTypeDialResponse MessageType = "dial_response"

	// MessageTypeData is for proxied connection data
	MessageTypeData MessageType = "data"

	// MessageTypeClose is sent to close a connection
	MessageTypeClose MessageType = "close"

	// MessageTypePing is sent for keepalive
	MessageTypePing MessageType = "ping"

	// MessageTypePong is the response to ping
	MessageTypePong MessageType = "pong"
)

// Message represents a tunnel protocol message
type Message struct {
	Type        MessageType     `json:"type"`
	ConnectionID string         `json:"connection_id,omitempty"`
	Payload     json.RawMessage `json:"payload,omitempty"`
}

// RegisterPayload is sent by satellite to register with hub
type RegisterPayload struct {
	ZoneID   string `json:"zone_id"`
	ZoneName string `json:"zone_name"`
	Version  string `json:"version"`
}

// RegisterAckPayload is sent by hub to acknowledge registration
type RegisterAckPayload struct {
	Accepted bool   `json:"accepted"`
	Message  string `json:"message,omitempty"`
}

// DialRequestPayload is sent by hub to request satellite to dial a target
type DialRequestPayload struct {
	TargetHost string `json:"target_host"`
	TargetPort int    `json:"target_port"`
	Protocol   string `json:"protocol"`
	Username   string `json:"username"`
	Password   string `json:"password,omitempty"`
	PrivateKey string `json:"private_key,omitempty"`
}

// DialResponsePayload is sent by satellite with dial result
type DialResponsePayload struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// DataPayload contains proxied data
type DataPayload struct {
	Data []byte `json:"data"`
}

// ClosePayload indicates a connection should be closed
type ClosePayload struct {
	Reason string `json:"reason,omitempty"`
}

// NewMessage creates a new message with the given type
func NewMessage(msgType MessageType) *Message {
	return &Message{
		Type: msgType,
	}
}

// SetPayload sets the message payload
func (m *Message) SetPayload(payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}
	m.Payload = data
	return nil
}

// GetPayload unmarshals the payload into the given struct
func (m *Message) GetPayload(v interface{}) error {
	if m.Payload == nil {
		return fmt.Errorf("no payload")
	}
	if err := json.Unmarshal(m.Payload, v); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}
	return nil
}

// Encode encodes the message to JSON
func (m *Message) Encode() ([]byte, error) {
	return json.Marshal(m)
}

// DecodeMessage decodes a message from JSON
func DecodeMessage(data []byte) (*Message, error) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}
	return &msg, nil
}
