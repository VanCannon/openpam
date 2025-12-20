package rdp

import (
	"sync"
	"sync/atomic"
)

const (
	// MonitorBufferSize is the buffer size for each subscriber
	MonitorBufferSize = 1000

	// MaxSubscribersPerSession limits subscribers to prevent memory issues
	MaxSubscribersPerSession = 50
)

// Subscriber represents a single monitoring subscriber
type Subscriber struct {
	ID       string
	Channel  chan []byte
	Dropped  int64 // Count of dropped messages
	Closed   int32 // Atomic flag indicating if closed
}

// AsyncMonitor manages live session monitoring with non-blocking broadcasts
type AsyncMonitor struct {
	// sessions maps session ID to session state
	sessions sync.Map // map[string]*MonitorSession
}

// MonitorSession holds state for a single monitored session
type MonitorSession struct {
	sessionID   string
	header      []byte
	headerMu    sync.RWMutex
	subscribers sync.Map // map[string]*Subscriber
	subCount    int32    // Atomic counter for fast subscriber count check
}

// NewAsyncMonitor creates a new async monitor
func NewAsyncMonitor() *AsyncMonitor {
	return &AsyncMonitor{}
}

// getOrCreateSession gets or creates a session
func (m *AsyncMonitor) getOrCreateSession(sessionID string) *MonitorSession {
	val, _ := m.sessions.LoadOrStore(sessionID, &MonitorSession{
		sessionID: sessionID,
	})
	return val.(*MonitorSession)
}

// SetHeader sets the header for a session (sent to new subscribers)
func (m *AsyncMonitor) SetHeader(sessionID string, data []byte) {
	session := m.getOrCreateSession(sessionID)
	session.headerMu.Lock()
	session.header = make([]byte, len(data))
	copy(session.header, data)
	session.headerMu.Unlock()
}

// Subscribe creates a new subscriber for a session
func (m *AsyncMonitor) Subscribe(sessionID string) chan []byte {
	session := m.getOrCreateSession(sessionID)

	// Check subscriber limit
	if atomic.LoadInt32(&session.subCount) >= MaxSubscribersPerSession {
		return nil
	}

	sub := &Subscriber{
		ID:      generateSubID(),
		Channel: make(chan []byte, MonitorBufferSize),
	}

	session.subscribers.Store(sub.ID, sub)
	atomic.AddInt32(&session.subCount, 1)

	// Send header if exists
	session.headerMu.RLock()
	if session.header != nil {
		headerCopy := make([]byte, len(session.header))
		copy(headerCopy, session.header)
		select {
		case sub.Channel <- headerCopy:
		default:
			// Header couldn't be sent, subscriber will miss initial state
		}
	}
	session.headerMu.RUnlock()

	return sub.Channel
}

// Unsubscribe removes a subscriber
func (m *AsyncMonitor) Unsubscribe(sessionID string, ch chan []byte) {
	val, ok := m.sessions.Load(sessionID)
	if !ok {
		return
	}

	session := val.(*MonitorSession)

	// Find and remove subscriber by channel
	session.subscribers.Range(func(key, value interface{}) bool {
		sub := value.(*Subscriber)
		if sub.Channel == ch {
			if atomic.CompareAndSwapInt32(&sub.Closed, 0, 1) {
				session.subscribers.Delete(key)
				atomic.AddInt32(&session.subCount, -1)
				close(sub.Channel)
			}
			return false // Stop iteration
		}
		return true
	})

	// Clean up empty sessions
	if atomic.LoadInt32(&session.subCount) == 0 {
		m.sessions.Delete(sessionID)
	}
}

// Broadcast sends data to all subscribers of a session
// This method is completely non-blocking - slow subscribers will have messages dropped
func (m *AsyncMonitor) Broadcast(sessionID string, data []byte) {
	val, ok := m.sessions.Load(sessionID)
	if !ok {
		return
	}

	session := val.(*MonitorSession)

	// Quick check - no subscribers means no work
	if atomic.LoadInt32(&session.subCount) == 0 {
		return
	}

	// Make a copy of data since we're sending to multiple subscribers
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)

	// Send to all subscribers (non-blocking)
	session.subscribers.Range(func(key, value interface{}) bool {
		sub := value.(*Subscriber)

		// Skip closed subscribers
		if atomic.LoadInt32(&sub.Closed) == 1 {
			return true
		}

		// Non-blocking send
		select {
		case sub.Channel <- dataCopy:
			// Sent successfully
		default:
			// Channel full - drop message and increment counter
			atomic.AddInt64(&sub.Dropped, 1)
		}

		return true
	})
}

// HasSubscribers returns true if a session has any active subscribers
func (m *AsyncMonitor) HasSubscribers(sessionID string) bool {
	val, ok := m.sessions.Load(sessionID)
	if !ok {
		return false
	}
	session := val.(*MonitorSession)
	return atomic.LoadInt32(&session.subCount) > 0
}

// SubscriberCount returns the number of active subscribers for a session
func (m *AsyncMonitor) SubscriberCount(sessionID string) int {
	val, ok := m.sessions.Load(sessionID)
	if !ok {
		return 0
	}
	session := val.(*MonitorSession)
	return int(atomic.LoadInt32(&session.subCount))
}

// CleanupSession removes all state for a session
func (m *AsyncMonitor) CleanupSession(sessionID string) {
	val, ok := m.sessions.LoadAndDelete(sessionID)
	if !ok {
		return
	}

	session := val.(*MonitorSession)

	// Close all subscriber channels
	session.subscribers.Range(func(key, value interface{}) bool {
		sub := value.(*Subscriber)
		if atomic.CompareAndSwapInt32(&sub.Closed, 0, 1) {
			close(sub.Channel)
		}
		return true
	})
}

// Simple ID generator for subscribers
var subIDCounter int64

func generateSubID() string {
	id := atomic.AddInt64(&subIDCounter, 1)
	return string(rune(id))
}
