# OpenPAM API Documentation

Base URL: `http://localhost:8080`

All API endpoints require authentication unless specified otherwise.

## Authentication

### Login
`GET /api/v1/auth/login`

Initiates OAuth2 login flow with EntraID.

**Response:** Redirect to EntraID login

---

### Callback
`GET /api/v1/auth/callback?code=CODE&state=STATE`

OAuth2 callback endpoint.

**Response:**
```json
{
  "success": true,
  "user": {
    "id": "uuid",
    "email": "user@example.com",
    "display_name": "User Name"
  }
}
```

---

### Logout
`POST /api/v1/auth/logout`

Logs out the current user.

**Response:**
```json
{
  "success": true,
  "message": "Logged out successfully"
}
```

---

### Get Current User
`GET /api/v1/auth/me`

Returns information about the authenticated user.

**Headers:** `Authorization: Bearer <token>` or Cookie

**Response:**
```json
{
  "id": "uuid",
  "email": "user@example.com",
  "display_name": "User Name",
  "enabled": true
}
```

---

## Zones

### List Zones
`GET /api/v1/zones`

Lists all zones.

**Response:**
```json
{
  "zones": [
    {
      "id": "uuid",
      "name": "headquarters",
      "type": "hub",
      "description": "Main HQ zone",
      "created_at": "2025-01-23T19:00:00Z",
      "updated_at": "2025-01-23T19:00:00Z"
    }
  ],
  "count": 1
}
```

---

### Create Zone
`POST /api/v1/zones/create`

Creates a new zone.

**Body:**
```json
{
  "name": "branch-office",
  "type": "satellite",
  "description": "Branch office zone"
}
```

**Response:** `201 Created` with zone object

---

### Get Zone
`GET /api/v1/zones/get?id=UUID`

Gets a specific zone.

**Response:** Zone object

---

### Update Zone
`PUT /api/v1/zones/update?id=UUID`

Updates a zone.

**Body:**
```json
{
  "name": "updated-name",
  "type": "hub",
  "description": "Updated description"
}
```

**Response:** Updated zone object

---

### Delete Zone
`DELETE /api/v1/zones/delete?id=UUID`

Deletes a zone.

**Response:** `204 No Content`

---

## Targets

### List Targets
`GET /api/v1/targets?limit=50&offset=0`

Lists available targets (servers).

**Query Parameters:**
- `limit`: Max results (default: 50, max: 100)
- `offset`: Pagination offset (default: 0)

**Response:**
```json
{
  "targets": [
    {
      "id": "uuid",
      "zone_id": "uuid",
      "name": "production-server",
      "hostname": "10.0.1.5",
      "protocol": "ssh",
      "port": 22,
      "description": "Main production server",
      "enabled": true
    }
  ],
  "count": 1,
  "limit": 50,
  "offset": 0
}
```

---

### Create Target
`POST /api/v1/targets/create`

Creates a new target.

**Body:**
```json
{
  "zone_id": "uuid",
  "name": "web-server-01",
  "hostname": "192.168.1.10",
  "protocol": "ssh",
  "port": 22,
  "description": "Web server"
}
```

**Response:** `201 Created` with target object

---

### Get Target
`GET /api/v1/targets/get?id=UUID`

Gets a specific target.

**Response:** Target object

---

### Update Target
`PUT /api/v1/targets/update?id=UUID`

Updates a target.

**Body:**
```json
{
  "zone_id": "uuid",
  "name": "web-server-01-updated",
  "hostname": "192.168.1.10",
  "protocol": "ssh",
  "port": 22,
  "description": "Updated description",
  "enabled": true
}
```

**Response:** Updated target object

---

### Delete Target
`DELETE /api/v1/targets/delete?id=UUID`

Deletes a target.

**Response:** `204 No Content`

---

## Credentials

### List Credentials by Target
`GET /api/v1/credentials?target_id=UUID`

Lists credentials for a specific target.

**Response:**
```json
{
  "credentials": [
    {
      "id": "uuid",
      "target_id": "uuid",
      "username": "admin",
      "description": "Administrator account"
    }
  ],
  "count": 1
}
```

**Note:** `vault_secret_path` is never exposed via API

---

### Create Credential
`POST /api/v1/credentials/create`

Creates a new credential.

**Body:**
```json
{
  "target_id": "uuid",
  "username": "admin",
  "vault_secret_path": "kv/servers/prod-server",
  "description": "Admin credentials"
}
```

**Response:** `201 Created` with credential object

---

### Delete Credential
`DELETE /api/v1/credentials/delete?id=UUID`

Deletes a credential.

**Response:** `204 No Content`

---

## Audit Logs

### List Audit Logs
`GET /api/v1/audit-logs?limit=50&offset=0`

Lists audit logs with pagination.

**Response:**
```json
{
  "logs": [
    {
      "id": "uuid",
      "user_id": "uuid",
      "target_id": "uuid",
      "credential_id": "uuid",
      "start_time": "2025-01-23T19:30:00Z",
      "end_time": "2025-01-23T19:45:00Z",
      "bytes_sent": 1024,
      "bytes_received": 4096,
      "session_status": "completed",
      "client_ip": "192.168.1.100",
      "error_message": null,
      "recording_path": "/recordings/session-uuid.log",
      "created_at": "2025-01-23T19:30:00Z"
    }
  ],
  "count": 1,
  "limit": 50,
  "offset": 0
}
```

---

### List Audit Logs by User
`GET /api/v1/audit-logs/user?user_id=UUID&limit=50&offset=0`

Lists audit logs for a specific user.

**Response:** Same as List Audit Logs

---

### List Active Sessions
`GET /api/v1/audit-logs/active`

Lists all currently active sessions.

**Response:**
```json
{
  "sessions": [
    {
      "id": "uuid",
      "user_id": "uuid",
      "target_id": "uuid",
      "start_time": "2025-01-23T20:00:00Z",
      "session_status": "active",
      "client_ip": "192.168.1.100"
    }
  ],
  "count": 1
}
```

---

## WebSocket Connection

### Connect to Target
`WS /api/ws/connect/{protocol}/{target_id}`

Establishes a WebSocket connection to a target server.

**Path Parameters:**
- `protocol`: `ssh` or `rdp`
- `target_id`: UUID of target

**Headers:**
- `Authorization: Bearer <token>` or Cookie with JWT

**WebSocket Protocol:**
- Binary frames for data transfer
- Text frames for control messages (resize, etc.)

**Example:**
```javascript
const ws = new WebSocket(
  'wss://gateway.example.com/api/ws/connect/ssh/target-uuid',
  null,
  { headers: { 'Authorization': `Bearer ${token}` } }
);
```

---

## Error Responses

All endpoints return standard HTTP status codes:

- `200 OK`: Success
- `201 Created`: Resource created
- `204 No Content`: Success with no response body
- `400 Bad Request`: Invalid request
- `401 Unauthorized`: Authentication required
- `403 Forbidden`: Access denied
- `404 Not Found`: Resource not found
- `405 Method Not Allowed`: Wrong HTTP method
- `500 Internal Server Error`: Server error

**Error Format:**
```
Plain text error message
```

---

## Rate Limiting

Currently not implemented. Future versions will include rate limiting per user.

## Versioning

API is versioned via URL path (`/api/v1/`). Breaking changes will increment the version number.
