package rdp

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRecorder_IdleTimeOptimization(t *testing.T) {
	// Create temporary directory for recordings
	tmpDir, err := os.MkdirTemp("", "rdp_recordings")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	recorder, err := NewRecorder(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create recorder: %v", err)
	}

	sessionID := "test-session"
	ctx := context.Background()

	if err := recorder.StartRecording(ctx, sessionID); err != nil {
		t.Fatalf("Failed to start recording: %v", err)
	}

	// 1. Write first instruction (t=0)
	if err := recorder.WriteInstruction(sessionID, "size", "1024", "768"); err != nil {
		t.Fatalf("Failed to write instruction: %v", err)
	}

	// 2. Simulate short delay (100ms)
	// We need to manually manipulate the LastRealTime to simulate time passing
	// because sleeping in tests is flaky and slow.
	recorder.mu.RLock()
	session := recorder.sessions[sessionID]
	recorder.mu.RUnlock()

	session.mu.Lock()
	session.LastRealTime = session.LastRealTime.Add(-100 * time.Millisecond)
	session.mu.Unlock()

	if err := recorder.WriteInstruction(sessionID, "mouse", "100", "100"); err != nil {
		t.Fatalf("Failed to write instruction: %v", err)
	}

	// 3. Simulate long idle time (10 seconds)
	// MaxIdleTime is 5 seconds
	session.mu.Lock()
	session.LastRealTime = session.LastRealTime.Add(-10 * time.Second)
	session.mu.Unlock()

	if err := recorder.WriteInstruction(sessionID, "key", "65", "1"); err != nil {
		t.Fatalf("Failed to write instruction: %v", err)
	}

	if err := recorder.StopRecording(sessionID); err != nil {
		t.Fatalf("Failed to stop recording: %v", err)
	}

	// Read the file and verify timestamps
	content, err := os.ReadFile(session.FilePath)
	if err != nil {
		t.Fatalf("Failed to read recording file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 3 {
		t.Fatalf("Expected 3 lines, got %d", len(lines))
	}

	// Verify line 1: timestamp 0
	if !strings.HasPrefix(lines[0], "0,size") {
		t.Errorf("Line 1 mismatch: %s", lines[0])
	}

	// Verify line 2: timestamp ~100
	// Since we manipulated LastRealTime relative to Now, and WriteInstruction uses Now,
	// the delta should be exactly what we subtracted.
	// However, WriteInstruction sets LastRealTime = Now.
	// So:
	// T0: StartRecording (LastRealTime = Now)
	// T1: WriteInstruction (delta = 0) -> LastRealTime = Now
	// T2: Manually subtract 100ms from LastRealTime. LastRealTime = Now - 100ms.
	// T3: WriteInstruction. Now - (Now - 100ms) = 100ms.
	// So timestamp should be 0 + 100 = 100.
	if !strings.HasPrefix(lines[1], "100,mouse") {
		t.Errorf("Line 2 mismatch: %s", lines[1])
	}

	// Verify line 3: timestamp 100 + 5000 (MaxIdleTime) = 5100
	// T4: Manually subtract 10s from LastRealTime. LastRealTime = Now - 10s.
	// T5: WriteInstruction. Now - (Now - 10s) = 10s.
	// Delta (10s) > MaxIdleTime (5s).
	// CurrentTime += 5s.
	// Previous CurrentTime was 100. New is 5100.
	if !strings.HasPrefix(lines[2], "5100,key") {
		t.Errorf("Line 3 mismatch: %s", lines[2])
	}
}
