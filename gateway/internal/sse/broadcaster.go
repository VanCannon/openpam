package sse

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Client represents an SSE client connection
type Client struct {
	ID      string
	UserID  string
	Channel chan []byte
}

// Broadcaster manages SSE connections and broadcasts events
type Broadcaster struct {
	clients map[string]*Client
	mu      sync.RWMutex
}

// NewBroadcaster creates a new SSE broadcaster
func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		clients: make(map[string]*Client),
	}
}

// AddClient adds a new SSE client
func (b *Broadcaster) AddClient(clientID, userID string) *Client {
	b.mu.Lock()
	defer b.mu.Unlock()

	client := &Client{
		ID:      clientID,
		UserID:  userID,
		Channel: make(chan []byte, 10), // Buffer to prevent blocking
	}

	b.clients[clientID] = client
	return client
}

// RemoveClient removes an SSE client
func (b *Broadcaster) RemoveClient(clientID string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if client, exists := b.clients[clientID]; exists {
		close(client.Channel)
		delete(b.clients, clientID)
	}
}

// BroadcastScheduleUpdate broadcasts a schedule update to all connected clients
func (b *Broadcaster) BroadcastScheduleUpdate(scheduleID, userID, eventType string, data interface{}) {
	// Use defer+recover to prevent panics from crashing the server
	defer func() {
		if r := recover(); r != nil {
			// Log the panic but don't crash
			fmt.Printf("Panic in BroadcastScheduleUpdate: %v\n", r)
		}
	}()

	b.mu.RLock()
	defer b.mu.RUnlock()

	event := map[string]interface{}{
		"type":        eventType,
		"schedule_id": scheduleID,
		"user_id":     userID,
		"data":        data,
		"timestamp":   time.Now().Format(time.RFC3339),
	}

	eventData, err := json.Marshal(event)
	if err != nil {
		fmt.Printf("Failed to marshal SSE event: %v\n", err)
		return
	}

	// Format as SSE message
	message := fmt.Sprintf("data: %s\n\n", eventData)

	// Send to all clients for this user
	sent := 0
	for _, client := range b.clients {
		if client.UserID == userID {
			select {
			case client.Channel <- []byte(message):
				sent++
			default:
				// Channel full, skip this client to prevent blocking
			}
		}
	}

	if sent > 0 {
		fmt.Printf("Broadcast %s event to %d client(s) for user %s\n", eventType, sent, userID)
	}
}

// BroadcastToAll broadcasts a message to all connected clients
func (b *Broadcaster) BroadcastToAll(eventType string, data interface{}) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	event := map[string]interface{}{
		"type":      eventType,
		"data":      data,
		"timestamp": time.Now().Format(time.RFC3339),
	}

	eventData, err := json.Marshal(event)
	if err != nil {
		return
	}

	// Format as SSE message
	message := fmt.Sprintf("data: %s\n\n", eventData)

	for _, client := range b.clients {
		select {
		case client.Channel <- []byte(message):
		default:
			// Channel full, skip this client
		}
	}
}

// ClientCount returns the number of connected clients
func (b *Broadcaster) ClientCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.clients)
}
