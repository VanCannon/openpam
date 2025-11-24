# Authentication & Authorization

OpenPAM uses Microsoft EntraID (Azure AD) for authentication via OAuth2/OpenID Connect, with JWT tokens for session management.

## Authentication Flow

```
┌──────────┐                ┌──────────┐                ┌──────────┐
│  Client  │                │ Gateway  │                │ EntraID  │
└────┬─────┘                └────┬─────┘                └────┬─────┘
     │                           │                           │
     │  GET /api/v1/auth/login   │                           │
     │───────────────────────────>│                           │
     │                           │                           │
     │  Redirect to EntraID      │                           │
     │<───────────────────────────│                           │
     │                           │                           │
     │  User authenticates       │                           │
     │───────────────────────────────────────────────────────>│
     │                           │                           │
     │  Redirect to callback     │                           │
     │<───────────────────────────────────────────────────────│
     │                           │                           │
     │  GET /api/v1/auth/callback?code=xyz&state=abc         │
     │───────────────────────────>│                           │
     │                           │   Exchange code for token │
     │                           │───────────────────────────>│
     │                           │<───────────────────────────│
     │                           │                           │
     │                           │  Get user info            │
     │                           │───────────────────────────>│
     │                           │<───────────────────────────│
     │                           │                           │
     │  Set JWT cookie           │                           │
     │<───────────────────────────│                           │
     │                           │                           │
```

## Components

### 1. JWT Token Manager
- Generates JWT tokens for authenticated users
- Validates tokens on protected endpoints
- Default expiration: 1 hour (configurable via SESSION_TIMEOUT)
- Signing algorithm: HS256

### 2. EntraID Client
- OAuth2/OpenID Connect integration
- Exchanges authorization codes for access tokens
- Retrieves user information from Microsoft Graph API
- Supports token refresh

### 3. Session Store
- In-memory session storage (current implementation)
- Tracks active user sessions
- Automatic cleanup of expired sessions
- Future: Redis-backed for distributed deployments

### 4. User Repository
- Stores user information in PostgreSQL
- Auto-creates users on first login
- Tracks last login timestamp
- Supports user enable/disable

## API Endpoints

### Authentication Endpoints (Public)

#### `GET /api/v1/auth/login`
Initiates the OAuth2 login flow.

**Response**: HTTP 302 redirect to EntraID login page

---

#### `GET /api/v1/auth/callback`
Handles the OAuth2 callback from EntraID.

**Query Parameters**:
- `code` - Authorization code from EntraID
- `state` - CSRF protection token

**Response**:
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

**Cookie Set**: `openpam_token` - JWT token (HttpOnly, SameSite=Lax)

---

#### `POST /api/v1/auth/logout`
Logs out the current user.

**Response**:
```json
{
  "success": true,
  "message": "Logged out successfully"
}
```

---

### Protected Endpoints (Authentication Required)

#### `GET /api/v1/auth/me`
Returns information about the currently authenticated user.

**Headers**:
- `Authorization: Bearer <token>` OR Cookie: `openpam_token=<token>`

**Response**:
```json
{
  "id": "uuid",
  "email": "user@example.com",
  "display_name": "User Name",
  "enabled": true
}
```

## Configuration

### Required Environment Variables

```bash
# EntraID Configuration
ENTRA_TENANT_ID=your-tenant-id
ENTRA_CLIENT_ID=your-app-client-id
ENTRA_CLIENT_SECRET=your-app-client-secret
ENTRA_REDIRECT_URL=http://localhost:8080/api/v1/auth/callback

# Session Configuration
SESSION_SECRET=long-random-string-change-in-production
SESSION_TIMEOUT=3600s
```

### Setting Up Azure AD Application

1. **Register Application** in Azure Portal:
   - Go to Azure Active Directory > App registrations
   - Click "New registration"
   - Name: "OpenPAM Gateway"
   - Supported account types: Single tenant
   - Redirect URI: `http://localhost:8080/api/v1/auth/callback`

2. **Configure API Permissions**:
   - Add "Microsoft Graph" permissions:
     - `User.Read` (Delegated)
     - `openid` (Delegated)
     - `profile` (Delegated)
     - `email` (Delegated)

3. **Create Client Secret**:
   - Go to Certificates & secrets
   - New client secret
   - Copy the value (shown only once)

4. **Get Configuration Values**:
   - `ENTRA_TENANT_ID`: Directory (tenant) ID from Overview
   - `ENTRA_CLIENT_ID`: Application (client) ID from Overview
   - `ENTRA_CLIENT_SECRET`: The secret value you copied

## Security Features

### CSRF Protection
- State parameter validation on OAuth2 callback
- State tokens expire after 10 minutes
- One-time use enforcement

### Token Security
- JWT tokens signed with HS256
- HttpOnly cookies prevent XSS attacks
- SameSite=Lax prevents CSRF
- Secure flag on HTTPS

### Session Management
- Automatic session cleanup every 15 minutes
- Configurable session timeout
- Per-user session tracking
- Logout invalidates all user sessions

## Testing Authentication

### With EntraID (Production)

1. Configure EntraID app registration
2. Set environment variables
3. Start the gateway
4. Visit `http://localhost:8080/api/v1/auth/login`
5. Authenticate with your Microsoft account

### Without EntraID (Development)

For development without EntraID, you can temporarily modify the handlers to issue tokens directly. This is NOT recommended for production.

## Middleware Usage

Protected endpoints use the `RequireAuth` middleware:

```go
// Example from server.go
s.router.Handle("/api/v1/targets", s.requireAuth(s.handleTargets()))
```

The middleware:
1. Checks for JWT in cookie or Authorization header
2. Validates the token signature and expiration
3. Extracts user claims
4. Adds user info to request context
5. Passes request to handler OR returns 401 Unauthorized

Access user info in handlers:

```go
func (s *Server) handleExample() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        userID := middleware.GetUserID(r.Context())
        email := middleware.GetUserEmail(r.Context())
        displayName := middleware.GetDisplayName(r.Context())

        // Use user info...
    }
}
```

## Future Enhancements

- [ ] Redis-backed session store for horizontal scaling
- [ ] Role-based access control (RBAC)
- [ ] Multi-factor authentication (MFA)
- [ ] API key authentication for programmatic access
- [ ] Audit logging of authentication events
- [ ] Token refresh endpoint
- [ ] Session management UI
