package ssh

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Recorder records SSH sessions for audit purposes
type Recorder struct {
	recordingsPath string
	sessions       map[string]*RecordingSession
	mu             sync.RWMutex
}

// RecordingSession represents an active recording session
type RecordingSession struct {
	SessionID string
	FilePath  string
	File      *os.File
	StartTime time.Time
}

// NewRecorder creates a new session recorder
func NewRecorder(recordingsPath string) (*Recorder, error) {
	// Create recordings directory if it doesn't exist
	if err := os.MkdirAll(recordingsPath, 0750); err != nil {
		return nil, fmt.Errorf("failed to create recordings directory: %w", err)
	}

	return &Recorder{
		recordingsPath: recordingsPath,
		sessions:       make(map[string]*RecordingSession),
	}, nil
}

// StartRecording starts recording a session
func (r *Recorder) StartRecording(ctx context.Context, sessionID string) (io.Writer, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Generate filename with timestamp
	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("%s-%s.log", sessionID, timestamp)
	filePath := filepath.Join(r.recordingsPath, filename)

	// Create recording file
	file, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create recording file: %w", err)
	}

	// Write session header
	header := fmt.Sprintf("=== SSH Session Recording ===\n")
	header += fmt.Sprintf("Session ID: %s\n", sessionID)
	header += fmt.Sprintf("Start Time: %s\n", time.Now().Format(time.RFC3339))
	header += fmt.Sprintf("=============================\n\n")
	file.WriteString(header)

	session := &RecordingSession{
		SessionID: sessionID,
		FilePath:  filePath,
		File:      file,
		StartTime: time.Now(),
	}

	r.sessions[sessionID] = session

	return file, nil
}

// StopRecording stops recording a session
func (r *Recorder) StopRecording(sessionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	session, exists := r.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Write session footer
	footer := fmt.Sprintf("\n=============================\n")
	footer += fmt.Sprintf("End Time: %s\n", time.Now().Format(time.RFC3339))
	footer += fmt.Sprintf("Duration: %s\n", time.Since(session.StartTime).String())
	footer += fmt.Sprintf("=============================\n")
	session.File.WriteString(footer)

	// Close file
	if err := session.File.Close(); err != nil {
		return fmt.Errorf("failed to close recording file: %w", err)
	}

	delete(r.sessions, sessionID)

	return nil
}

// GetRecordingPath returns the path to a recording file
func (r *Recorder) GetRecordingPath(sessionID string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	session, exists := r.sessions[sessionID]
	if !exists {
		return "", fmt.Errorf("session not found: %s", sessionID)
	}

	return session.FilePath, nil
}
