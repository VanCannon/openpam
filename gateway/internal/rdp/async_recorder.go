package rdp

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// MaxIdleTimeAsync is the maximum idle time recorded between instructions
	MaxIdleTimeAsync = 5000 * time.Millisecond

	// InstructionBufferSize is the size of the async instruction buffer per session
	// Large buffer to handle video bursts without dropping
	InstructionBufferSize = 50000

	// BatchSize is the number of instructions to batch before writing
	BatchSize = 500

	// FlushInterval is how often to flush even if batch isn't full
	FlushInterval = 100 * time.Millisecond
)

// AsyncInstruction represents a single Guacamole instruction with timing
type AsyncInstruction struct {
	Timestamp time.Time
	Opcode    string
	Args      []string
}

// AsyncRecordingSession represents an active recording session with async writes
type AsyncRecordingSession struct {
	SessionID   string
	FilePath    string
	file        *os.File
	writer      *bufio.Writer
	startTime   time.Time
	lastTime    time.Time
	currentTime time.Duration

	// Async instruction channel - large buffer for burst handling
	instrChan chan AsyncInstruction
	doneChan  chan struct{}
	wg        sync.WaitGroup

	// Stats
	instructionsWritten int64
	instructionsDropped int64
}

// AsyncRecorder records RDP sessions asynchronously with batched writes
type AsyncRecorder struct {
	recordingsPath string
	sessions       sync.Map // map[string]*AsyncRecordingSession
}

// NewAsyncRecorder creates a new async session recorder
func NewAsyncRecorder(recordingsPath string) (*AsyncRecorder, error) {
	if err := os.MkdirAll(recordingsPath, 0750); err != nil {
		return nil, fmt.Errorf("failed to create recordings directory: %w", err)
	}

	return &AsyncRecorder{
		recordingsPath: recordingsPath,
	}, nil
}

// StartRecording starts an async recording session
func (r *AsyncRecorder) StartRecording(ctx context.Context, sessionID string) error {
	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("%s-%s.guac", sessionID, timestamp)
	filePath := filepath.Join(r.recordingsPath, filename)

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create recording file: %w", err)
	}

	// Large buffer for efficient disk writes (256KB)
	writer := bufio.NewWriterSize(file, 262144)

	session := &AsyncRecordingSession{
		SessionID:   sessionID,
		FilePath:    filePath,
		file:        file,
		writer:      writer,
		startTime:   time.Now(),
		lastTime:    time.Now(),
		currentTime: 0,
		instrChan:   make(chan AsyncInstruction, InstructionBufferSize),
		doneChan:    make(chan struct{}),
	}

	r.sessions.Store(sessionID, session)

	// Start the async writer goroutine
	session.wg.Add(1)
	go session.writerLoop()

	return nil
}

// WriteInstruction queues an instruction for async recording
// This method is non-blocking - if buffer is full, instruction is dropped
func (r *AsyncRecorder) WriteInstruction(sessionID string, opcode string, args ...string) error {
	val, ok := r.sessions.Load(sessionID)
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session := val.(*AsyncRecordingSession)

	// Non-blocking send - drop if buffer is full (should rarely happen with 50K buffer)
	select {
	case session.instrChan <- AsyncInstruction{
		Timestamp: time.Now(),
		Opcode:    opcode,
		Args:      args,
	}:
		// Successfully queued
	default:
		// Buffer full - drop instruction (better than blocking live session)
		atomic.AddInt64(&session.instructionsDropped, 1)
	}

	return nil
}

// StopRecording stops the async recording session
func (r *AsyncRecorder) StopRecording(sessionID string) error {
	val, ok := r.sessions.LoadAndDelete(sessionID)
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session := val.(*AsyncRecordingSession)

	// Signal writer to stop
	close(session.instrChan)

	// Wait for writer to finish (with timeout)
	done := make(chan struct{})
	go func() {
		session.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Writer finished normally
	case <-time.After(5 * time.Second):
		// Timeout - force close
	}

	close(session.doneChan)

	// Flush and close file
	if err := session.writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush recording buffer: %w", err)
	}

	if err := session.file.Close(); err != nil {
		return fmt.Errorf("failed to close recording file: %w", err)
	}

	return nil
}

// writerLoop is the async goroutine that batches and writes instructions
func (s *AsyncRecordingSession) writerLoop() {
	defer s.wg.Done()

	batch := make([]AsyncInstruction, 0, BatchSize)
	flushTicker := time.NewTicker(FlushInterval)
	defer flushTicker.Stop()

	writeBatch := func() {
		if len(batch) == 0 {
			return
		}

		var sb strings.Builder
		sb.Grow(len(batch) * 200) // Pre-allocate roughly 200 bytes per instruction

		for _, instr := range batch {
			// Calculate timing
			delta := instr.Timestamp.Sub(s.lastTime)
			if delta > MaxIdleTimeAsync {
				s.currentTime += MaxIdleTimeAsync
			} else if delta > 0 {
				s.currentTime += delta
			}
			s.lastTime = instr.Timestamp

			// Format: timestamp,len.opcode,len.arg,len.arg;\n
			sb.WriteString(fmt.Sprintf("%d,%d.%s", s.currentTime.Milliseconds(), len(instr.Opcode), instr.Opcode))
			for _, arg := range instr.Args {
				sb.WriteString(fmt.Sprintf(",%d.%s", len(arg), arg))
			}
			sb.WriteString(";\n")

			atomic.AddInt64(&s.instructionsWritten, 1)
		}

		// Write entire batch at once
		s.writer.WriteString(sb.String())

		// Clear batch
		batch = batch[:0]
	}

	for {
		select {
		case instr, ok := <-s.instrChan:
			if !ok {
				// Channel closed - write remaining and exit
				writeBatch()
				return
			}

			batch = append(batch, instr)

			// Write batch when full
			if len(batch) >= BatchSize {
				writeBatch()
			}

		case <-flushTicker.C:
			// Periodic flush to ensure data gets written even during quiet periods
			writeBatch()
			s.writer.Flush() // Force to disk periodically
		}
	}
}

// GetStats returns recording statistics for a session
func (r *AsyncRecorder) GetStats(sessionID string) (written, dropped int64, ok bool) {
	val, exists := r.sessions.Load(sessionID)
	if !exists {
		return 0, 0, false
	}

	session := val.(*AsyncRecordingSession)
	return atomic.LoadInt64(&session.instructionsWritten),
		atomic.LoadInt64(&session.instructionsDropped),
		true
}
