# Protocol Handlers

OpenPAM supports SSH and RDP connections through WebSocket-based protocol handlers. The gateway acts as a proxy, injecting credentials from Vault and providing session recording.

## Architecture

```
┌──────────────┐         ┌──────────────┐         ┌──────────────┐
│   Browser    │  WSS    │   Gateway    │   TCP   │    Target    │
│  (xterm.js)  │<------->│  (SSH Proxy) │<------->│ SSH Server   │
└──────────────┘         └──────────────┘         └──────────────┘
                                │
                                │ Vault API
                                ▼
                         ┌──────────────┐
                         │    Vault     │
                         │  (Secrets)   │
                         └──────────────┘
```

## SSH Protocol Handler

### Features
- **WebSocket-to-SSH Translation**: Converts WebSocket messages to SSH protocol
- **Credential Injection**: Retrieves credentials from Vault and authenticates automatically
- **PTY Support**: Full terminal emulation with xterm-256color
- **Session Recording**: Records all input/output for audit purposes
- **Authentication Methods**: Password and private key support

### Implementation

Location: [internal/ssh/proxy.go](../gateway/internal/ssh/proxy.go)

The SSH proxy:
1. Receives WebSocket connection from client
2. Retrieves target details from database
3. Fetches credentials from Vault
4. Establishes SSH connection to target
5. Proxies data bidirectionally
6. Records session for audit
7. Updates audit log with bytes transferred

### SSH Recording

Location: [internal/ssh/recorder.go](../gateway/internal/ssh/recorder.go)

Sessions are recorded to `./recordings/<session-id>-<timestamp>.log` with:
- Session metadata (ID, start time, end time)
- Full input/output stream
- Duration statistics

### Terminal Resize

The SSH proxy supports terminal resize events. Frontend can send resize requests via WebSocket control messages.

## RDP Protocol Handler

### Features
- **Guacamole Integration**: Uses Apache Guacamole daemon for RDP protocol handling
- **WebSocket-to-Guacamole**: Translates WebSocket to Guacamole protocol
- **Credential Injection**: Automatically provides credentials from Vault
- **Security Options**: Configurable ignore-cert for self-signed certificates

### Implementation

Location: [internal/rdp/proxy.go](../gateway/internal/rdp/proxy.go)

The RDP proxy:
1. Receives WebSocket connection from client
2. Retrieves target details from database
3. Fetches credentials from Vault
4. Connects to guacd daemon
5. Sends Guacamole handshake with target/credential info
6. Proxies Guacamole protocol bidirectionally
7. Updates audit log with bytes transferred

### Guacamole Protocol

The proxy uses the Guacamole protocol to communicate with guacd:

**Handshake Format:**
```
4.select,3.rdp,8.hostname,12.example.com,4.port,4.3389,8.username,5.admin,8.password,8.mypasswd;
```

Components:
- Length-prefixed strings (e.g., `8.username` = "username" with length 8)
- Semicolon-terminated instructions
- Key-value pairs for connection parameters

### Guacamole Daemon Setup

The RDP proxy requires Apache Guacamole daemon (guacd) to be running:

```bash
# Using Docker
docker run -d --name guacd \
  -p 4822:4822 \
  guacamole/guacd:1.5.4

# Or using Docker Compose (already in docker-compose.yml)
docker-compose up -d guacd
```

## WebSocket Connection Flow

### Connection Endpoint

`WS /api/ws/connect/{protocol}/{target_id}`

**Path Parameters:**
- `protocol`: `ssh` or `rdp`
- `target_id`: UUID of the target system

**Authentication:**
- Requires JWT token in cookie or Authorization header
- Token must be valid and not expired

### Connection Process

1. **Authentication Check**
   - Middleware validates JWT token
   - Extracts user information

2. **Target Validation**
   - Looks up target in database
   - Verifies target is enabled
   - Checks protocol matches

3. **Credential Retrieval**
   - Finds credentials for target
   - Fetches actual secrets from Vault
   - Never exposes credentials to client

4. **Audit Log Creation**
   - Creates audit_logs entry with status="active"
   - Records user, target, timestamp, client IP

5. **WebSocket Upgrade**
   - Upgrades HTTP to WebSocket connection
   - Starts protocol-specific proxy

6. **Session Proxying**
   - Bidirectional data transfer
   - Byte counting for audit
   - Optional session recording

7. **Session Termination**
   - Updates audit log with end_time
   - Sets final status (completed/failed)
   - Records total bytes transferred

## Session Recording

### SSH Recording

All SSH sessions can be recorded for compliance and audit purposes.

**Configuration:**
```bash
RECORDINGS_PATH=./recordings
```

**Recording Files:**
- Format: `<session-id>-<timestamp>.log`
- Contains: Full terminal I/O
- Includes: Session metadata header/footer

**Example:**
```
=== SSH Session Recording ===
Session ID: 123e4567-e89b-12d3-a456-426614174000
Start Time: 2025-01-23T19:30:00Z
=============================

[session output here]

=============================
End Time: 2025-01-23T19:45:23Z
Duration: 15m23s
=============================
```

### RDP Recording

RDP recording is not currently implemented. Guacamole supports screen recording which could be integrated in future versions.

## Audit Logging

All connection sessions are logged to the `audit_logs` table:

**Fields:**
- `user_id`: Who connected
- `target_id`: Where they connected
- `credential_id`: Which credential was used
- `start_time`: When session started
- `end_time`: When session ended
- `bytes_sent`: Data sent to target
- `bytes_received`: Data received from target
- `session_status`: active/completed/failed/terminated
- `client_ip`: Source IP address
- `error_message`: Error if session failed
- `recording_path`: Path to session recording file

## Security Considerations

### Credential Handling
- ✅ Credentials never sent to frontend
- ✅ Retrieved from Vault only when needed
- ✅ Injected directly into protocol connection
- ✅ Not logged or stored in gateway

### Host Key Verification
- ⚠️ Currently uses `InsecureIgnoreHostKey()` for SSH
- 🔧 TODO: Implement proper host key verification and TOFU

### Certificate Validation
- ⚠️ RDP proxy sets `ignore-cert=true` for Guacamole
- 🔧 TODO: Implement proper certificate validation

### Session Isolation
- ✅ Each session has unique audit log entry
- ✅ Sessions are isolated per-user
- ✅ No session data crosses between connections

## Example Usage

### Connect to SSH Target

```javascript
// Frontend example using xterm.js
const socket = new WebSocket(
  'wss://gateway.example.com/api/ws/connect/ssh/123e4567-e89b-12d3-a456-426614174000',
  null,
  { headers: { 'Authorization': `Bearer ${token}` } }
);

const term = new Terminal();
term.open(document.getElementById('terminal'));

socket.onmessage = (event) => {
  term.write(new Uint8Array(event.data));
};

term.onData((data) => {
  socket.send(data);
});
```

### Connect to RDP Target

```javascript
// Frontend example using Guacamole client
const socket = new WebSocket(
  'wss://gateway.example.com/api/ws/connect/rdp/987fcdeb-51d2-4f3e-b123-789012345678',
  null,
  { headers: { 'Authorization': `Bearer ${token}` } }
);

const tunnel = new Guacamole.WebSocketTunnel(socket);
const client = new Guacamole.Client(tunnel);

const display = document.getElementById('display');
display.appendChild(client.getDisplay().getElement());

client.connect();
```

## Future Enhancements

- [ ] SSH host key verification with TOFU
- [ ] RDP certificate validation
- [ ] Session recording playback UI
- [ ] Real-time session monitoring
- [ ] Session sharing/collaboration
- [ ] Clipboard support for both protocols
- [ ] File transfer support
- [ ] Multi-factor authentication challenge
- [ ] Just-in-time access (temporary credentials)
- [ ] Breakglass emergency access
