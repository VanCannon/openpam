package rdp

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestRecorder_OutputFormat(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "rdp_recordings")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	recorder, err := NewRecorder(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create recorder: %v", err)
	}

	sessionID := "test-format"
	ctx := context.Background()

	if err := recorder.StartRecording(ctx, sessionID); err != nil {
		t.Fatalf("Failed to start recording: %v", err)
	}

	// Test instruction with commas in args
	// Guacamole args are strings, they can contain anything.
	// Our recorder joins them with commas.
	// If arg contains comma, it will break our simple CSV parser.
	if err := recorder.WriteInstruction(sessionID, "test", "arg1", "arg,with,commas", "arg3"); err != nil {
		t.Fatalf("Failed to write instruction: %v", err)
	}

	if err := recorder.StopRecording(sessionID); err != nil {
		t.Fatalf("Failed to stop recording: %v", err)
	}

	// session := recorder.sessions[sessionID] // This will be nil after StopRecording, need to get path before
	// Actually StartRecording sets it. StopRecording deletes it.
	// We need to reconstruct path or just list dir.

	files, err := os.ReadDir(tmpDir)
	if err != nil || len(files) == 0 {
		t.Fatalf("No recording file found")
	}
	filePath := tmpDir + "/" + files[0].Name()

	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read recording file: %v", err)
	}

	line := strings.TrimSpace(string(content))
	t.Logf("Recorded line: %s", line)

	// Expected: timestamp,test,arg1,arg,with,commas,arg3;
	// If we split by comma:
	// 0: timestamp
	// 1: test
	// 2: arg1
	// 3: arg
	// 4: with
	// 5: commas
	// 6: arg3

	// This confirms that commas in args BREAK the CSV format.
	// We should verify if this is what happens.
	if !strings.Contains(line, "arg,with,commas") {
		t.Errorf("Expected arg with commas to be present")
	}
}
