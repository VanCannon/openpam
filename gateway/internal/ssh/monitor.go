package ssh

import (
	"sync"
)

// Monitor manages live session monitoring by broadcasting session data to multiple subscribers
type Monitor struct {
	// subscribers maps session ID to a list of subscriber channels
	subscribers map[string][]chan []byte
	mu          sync.RWMutex
}

// NewMonitor creates a new session monitor
func NewMonitor() *Monitor {
	return &Monitor{
		subscribers: make(map[string][]chan []byte),
	}
}

// Subscribe adds a new subscriber for a session and returns a channel to receive data
func (m *Monitor) Subscribe(sessionID string) chan []byte {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Create a buffered channel to prevent blocking if subscriber is slow
	ch := make(chan []byte, 100)

	if m.subscribers[sessionID] == nil {
		m.subscribers[sessionID] = []chan []byte{}
	}

	m.subscribers[sessionID] = append(m.subscribers[sessionID], ch)

	return ch
}

// Unsubscribe removes a subscriber channel for a session
func (m *Monitor) Unsubscribe(sessionID string, ch chan []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()

	subs, ok := m.subscribers[sessionID]
	if !ok {
		return
	}

	// Find and remove the channel
	for i, subscriber := range subs {
		if subscriber == ch {
			// Close the channel
			close(ch)

			// Remove from slice
			m.subscribers[sessionID] = append(subs[:i], subs[i+1:]...)

			// Clean up empty session entries
			if len(m.subscribers[sessionID]) == 0 {
				delete(m.subscribers, sessionID)
			}

			return
		}
	}
}

// Broadcast sends data to all subscribers of a session
func (m *Monitor) Broadcast(sessionID string, data []byte) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	subs, ok := m.subscribers[sessionID]
	if !ok || len(subs) == 0 {
		return
	}

	// Send to all subscribers
	// Use non-blocking send to prevent slow subscribers from blocking the session
	for _, ch := range subs {
		select {
		case ch <- data:
			// Successfully sent
		default:
			// Channel buffer is full, skip this send
			// In production, you might want to log this or disconnect slow subscribers
		}
	}
}

// HasSubscribers returns true if a session has any active subscribers
func (m *Monitor) HasSubscribers(sessionID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	subs, ok := m.subscribers[sessionID]
	return ok && len(subs) > 0
}

// SubscriberCount returns the number of active subscribers for a session
func (m *Monitor) SubscriberCount(sessionID string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	subs, ok := m.subscribers[sessionID]
	if !ok {
		return 0
	}
	return len(subs)
}
