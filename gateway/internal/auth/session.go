package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sync"
	"time"
)

// Session represents an active user session
type Session struct {
	ID          string
	UserID      string
	Email       string
	DisplayName string
	CreatedAt   time.Time
	ExpiresAt   time.Time
	Data        map[string]interface{}
}

// SessionStore manages user sessions
type SessionStore interface {
	Create(ctx context.Context, session *Session) error
	Get(ctx context.Context, sessionID string) (*Session, error)
	Delete(ctx context.Context, sessionID string) error
	DeleteByUserID(ctx context.Context, userID string) error
	Cleanup(ctx context.Context) error
}

// MemorySessionStore is an in-memory session store (for development)
type MemorySessionStore struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

// NewMemorySessionStore creates a new in-memory session store
func NewMemorySessionStore() *MemorySessionStore {
	return &MemorySessionStore{
		sessions: make(map[string]*Session),
	}
}

// Create creates a new session
func (s *MemorySessionStore) Create(ctx context.Context, session *Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sessions[session.ID] = session
	return nil
}

// Get retrieves a session by ID
func (s *MemorySessionStore) Get(ctx context.Context, sessionID string) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, exists := s.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found")
	}

	// Check if session is expired
	if time.Now().After(session.ExpiresAt) {
		return nil, fmt.Errorf("session expired")
	}

	return session, nil
}

// Delete deletes a session by ID
func (s *MemorySessionStore) Delete(ctx context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.sessions, sessionID)
	return nil
}

// DeleteByUserID deletes all sessions for a user
func (s *MemorySessionStore) DeleteByUserID(ctx context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, session := range s.sessions {
		if session.UserID == userID {
			delete(s.sessions, id)
		}
	}

	return nil
}

// Cleanup removes expired sessions
func (s *MemorySessionStore) Cleanup(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for id, session := range s.sessions {
		if now.After(session.ExpiresAt) {
			delete(s.sessions, id)
		}
	}

	return nil
}

// StartCleanup starts a background goroutine to periodically clean up expired sessions
func (s *MemorySessionStore) StartCleanup(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.Cleanup(ctx)
			}
		}
	}()
}

// GenerateSessionID generates a random session ID
func GenerateSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate session ID: %w", err)
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// StateStore manages OAuth2 state parameters
type StateStore interface {
	Create(ctx context.Context, state string, expiresAt time.Time) error
	Validate(ctx context.Context, state string) (bool, error)
	Delete(ctx context.Context, state string) error
}

// MemoryStateStore is an in-memory OAuth2 state store
type MemoryStateStore struct {
	states map[string]time.Time
	mu     sync.RWMutex
}

// NewMemoryStateStore creates a new in-memory state store
func NewMemoryStateStore() *MemoryStateStore {
	return &MemoryStateStore{
		states: make(map[string]time.Time),
	}
}

// Create creates a new state
func (s *MemoryStateStore) Create(ctx context.Context, state string, expiresAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.states[state] = expiresAt
	return nil
}

// Validate checks if a state is valid
func (s *MemoryStateStore) Validate(ctx context.Context, state string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	expiresAt, exists := s.states[state]
	if !exists {
		return false, nil
	}

	if time.Now().After(expiresAt) {
		return false, nil
	}

	return true, nil
}

// Delete deletes a state
func (s *MemoryStateStore) Delete(ctx context.Context, state string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.states, state)
	return nil
}

// Cleanup removes expired states
func (s *MemoryStateStore) Cleanup(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for state, expiresAt := range s.states {
		if now.After(expiresAt) {
			delete(s.states, state)
		}
	}

	return nil
}

// StartCleanup starts a background goroutine to periodically clean up expired states
func (s *MemoryStateStore) StartCleanup(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.Cleanup(ctx)
			}
		}
	}()
}

// GenerateState generates a random OAuth2 state parameter
func GenerateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate state: %w", err)
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
