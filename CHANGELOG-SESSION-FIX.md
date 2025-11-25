# Session Exit Status Fix

## Problem
Sessions were being marked as "failed" even when users exited normally (typing "exit" or clicking the X button). This made audit logs inaccurate.

## Root Cause
The SSH library's `session.Wait()` returns an `ssh.ExitError` even for successful exits (exit status 0). The code was treating any error from `session.Wait()` as a failure.

## Solution

### Backend Changes ([gateway/internal/ssh/proxy.go](gateway/internal/ssh/proxy.go))

1. **Exit Status Handling** (lines 286-306):
   - Check if error is `ssh.ExitError`
   - If exit status is 0, treat as success (user typed "exit")
   - If exit status is non-zero, treat as error
   - Return `nil` for successful session completion

```go
// Check if the error is an ExitError with status 0 (normal exit)
if err != nil {
    // Check if it's an SSH exit status error
    if exitErr, ok := err.(*ssh.ExitError); ok {
        exitStatus := exitErr.ExitStatus()
        p.logger.Info("SSH session exited", map[string]interface{}{
            "exit_status": exitStatus,
        })
        // Exit status 0 means success (user typed "exit")
        if exitStatus == 0 {
            return nil
        }
        // Non-zero exit status is still an error
        return fmt.Errorf("SSH session exited with status %d", exitStatus)
    }
    // Other errors are real failures
    return fmt.Errorf("SSH session error: %w", err)
}
```

2. **WebSocket Close Handling** (lines 124-137):
   - Detect normal WebSocket closes vs errors
   - Close SSH stdin when WebSocket closes
   - Log appropriately for debugging

```go
defer stdin.Close() // Close SSH stdin when WebSocket closes
// ...
if err != nil {
    // Check if it's a normal WebSocket close
    if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
        p.logger.Info("WebSocket closed normally")
    } else {
        p.logger.Debug("WebSocket read error", map[string]interface{}{
            "error": err.Error(),
        })
    }
    return
}
```

### Frontend Changes ([web/components/terminal-client.tsx](web/components/terminal-client.tsx))

1. **Graceful WebSocket Close** (lines 318-324):
   - When user clicks X, close WebSocket with code 1000 (normal closure)
   - Let `ws.onclose` handler trigger the redirect to dashboard
   - Prevents abrupt disconnection

```typescript
onClick={() => {
  // Close WebSocket gracefully before calling onClose
  if (wsRef.current?.readyState === WebSocket.OPEN) {
    wsRef.current.close(1000, 'User closed connection')
  }
  // onClose will be called by ws.onclose handler
}}
```

## Testing

1. **User types "exit"**:
   - SSH session exits with status 0
   - Backend logs: "SSH session exited" with exit_status: 0
   - Session marked as "completed" in audit log
   - Frontend shows "Session ended. Redirecting to dashboard..."
   - User returns to dashboard after 2 seconds

2. **User clicks X button**:
   - WebSocket closes with code 1000 (normal closure)
   - Backend logs: "WebSocket closed normally"
   - SSH stdin closes, triggering session exit
   - Session marked as "completed" in audit log
   - User returns to dashboard immediately via onclose handler

3. **Actual error (connection lost, server crash, etc.)**:
   - Error caught and logged
   - Session marked as "failed" in audit log
   - Error message stored in audit log

## Expected Behavior

### Successful Session Completion
- **Trigger**: User types "exit" OR clicks X button
- **Backend**: Session status = "completed"
- **Frontend**: Shows "Session ended" message, returns to dashboard
- **Audit Log**: `session_status = "completed"`, `error_message = null`

### Failed Session
- **Trigger**: Network error, authentication failure, permission denied, server crash
- **Backend**: Session status = "failed"
- **Frontend**: Shows error message
- **Audit Log**: `session_status = "failed"`, `error_message = "SSH session error: <details>"`

## Files Modified
- `gateway/internal/ssh/proxy.go` - Exit status handling and WebSocket close detection
- `web/components/terminal-client.tsx` - Graceful WebSocket close on X button

## Dependencies
- No new dependencies
- Uses existing `golang.org/x/crypto/ssh` library
- Uses existing `github.com/gorilla/websocket` library
