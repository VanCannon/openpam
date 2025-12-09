package rdp

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	// MaxIdleTime is the maximum amount of time (in milliseconds) that will be
	// recorded for a single gap between instructions. If the real gap is larger,
	// it will be condensed to this value.
	MaxIdleTime = 5000 * time.Millisecond
)

// Recorder records RDP sessions in Guacamole protocol format
type Recorder struct {
	recordingsPath string
	sessions       map[string]*RecordingSession
	mu             sync.RWMutex
}

// RecordingSession represents an active recording session
type RecordingSession struct {
	SessionID       string
	FilePath        string
	File            *os.File
	Writer          *bufio.Writer // Buffered writer for better performance
	StartTime       time.Time
	LastRealTime    time.Time
	CurrentTime     time.Duration // Accumulated recorded time
	InstructionCount int64         // Count of instructions written
	mu              sync.Mutex
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
func (r *Recorder) StartRecording(ctx context.Context, sessionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Generate filename with timestamp
	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("%s-%s.guac", sessionID, timestamp)
	filePath := filepath.Join(r.recordingsPath, filename)

	// Create recording file
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create recording file: %w", err)
	}

	// Create buffered writer for better I/O performance (64KB buffer)
	writer := bufio.NewWriterSize(file, 65536)

	session := &RecordingSession{
		SessionID:        sessionID,
		FilePath:         filePath,
		File:             file,
		Writer:           writer,
		StartTime:        time.Now(),
		LastRealTime:     time.Now(),
		CurrentTime:      0,
		InstructionCount: 0,
	}

	r.sessions[sessionID] = session

	return nil
}

// WriteInstruction writes a Guacamole instruction to the recording
func (r *Recorder) WriteInstruction(sessionID string, opcode string, args ...string) error {
	r.mu.RLock()
	session, exists := r.sessions[sessionID]
	r.mu.RUnlock()

	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	now := time.Now()
	delta := now.Sub(session.LastRealTime)

	// Idle time optimization:
	// If the time since the last instruction is greater than MaxIdleTime,
	// we only advance the recorded time by MaxIdleTime.
	if delta > MaxIdleTime {
		session.CurrentTime += MaxIdleTime
	} else {
		session.CurrentTime += delta
	}

	session.LastRealTime = now

	// Format instruction: timestamp,len.opcode,len.arg,len.arg;
	// Example: 120,4.size,1.0,4.1024,3.768,2.96;
	var sb strings.Builder

	// Timestamp (in milliseconds) followed by comma
	sb.WriteString(fmt.Sprintf("%d,", session.CurrentTime.Milliseconds()))

	// Standard Guacamole instruction format
	// Opcode
	sb.WriteString(fmt.Sprintf("%d.%s", len(opcode), opcode))

	// Args
	for _, arg := range args {
		sb.WriteString(fmt.Sprintf(",%d.%s", len(arg), arg))
	}

	sb.WriteString(";\n")

	if _, err := session.Writer.WriteString(sb.String()); err != nil {
		return fmt.Errorf("failed to write to recording file: %w", err)
	}

	session.InstructionCount++

	// Flush every 100 instructions to ensure data doesn't sit in buffer too long
	// This prevents buffer buildup while still benefiting from buffering
	if session.InstructionCount%100 == 0 {
		if err := session.Writer.Flush(); err != nil {
			return fmt.Errorf("failed to flush recording buffer: %w", err)
		}
	}

	return nil
}

// StopRecording stops recording a session
func (r *Recorder) StopRecording(sessionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	session, exists := r.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Flush any remaining data in buffer
	if err := session.Writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush recording buffer: %w", err)
	}

	// Close file
	if err := session.File.Close(); err != nil {
		return fmt.Errorf("failed to close recording file: %w", err)
	}

	delete(r.sessions, sessionID)

	return nil
}
