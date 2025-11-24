# Development Mode Guide

This guide explains how to run OpenPAM in development mode, which bypasses EntraID and Vault authentication for quick local testing.

## Overview

Development mode provides:
- **No EntraID Required**: Auto-login as a test user
- **No Vault Required**: Mock credentials (still requires Vault service for API calls, but validation is skipped)
- **Quick Setup**: Get started in minutes
- **Full Functionality**: All features work as normal

**WARNING**: Never enable DEV_MODE in production environments!

## Quick Start

### 1. Start Required Services

```bash
# Start PostgreSQL and Vault with docker-compose
docker-compose up -d postgres vault
```

### 2. Run Database Migrations

```bash
cd gateway
go run cmd/migrate/main.go
```

### 3. Copy Development Config

```bash
# Use the pre-configured development environment
cp .env.dev .env
```

The `.env.dev` file has `DEV_MODE=true` enabled.

### 4. Start Backend

```bash
cd gateway
go run cmd/server/main.go
```

You should see:
```
WARNING: Development mode enabled. Authentication and Vault validation disabled!
INFO: Server starting on 0.0.0.0:8080
```

### 5. Start Frontend

```bash
cd web
npm install
npm run dev
```

Frontend will be available at http://localhost:3000

### 6. Login

Click "Sign in with Microsoft" - you'll be automatically logged in as:
- **Email**: dev@example.com
- **Display Name**: Development User
- **User ID**: dev-user-123

## What Dev Mode Does

### Backend Changes

1. **Configuration Validation** ([internal/config/config.go](c:\Users\bvanc\PAM\gateway\internal\config\config.go))
   - Skips EntraID credential validation
   - Skips Vault credential validation
   - Shows warning in logs

2. **Authentication Handler** ([internal/handlers/auth.go](c:\Users\bvanc\PAM\gateway\internal\handlers\auth.go))
   - `/api/v1/auth/login` auto-creates test user
   - Generates valid JWT token
   - Redirects to frontend with token
   - No EntraID OAuth flow

3. **Test User**
   - Automatically created in database on first login
   - Stored like any other user
   - Can be used for all operations

### What Still Works

- ✅ All API endpoints
- ✅ Database operations
- ✅ Target management
- ✅ WebSocket connections
- ✅ SSH/RDP proxying (if targets are reachable)
- ✅ Audit logging
- ✅ Frontend UI

### What Doesn't Work

- ❌ Actual EntraID authentication
- ❌ Real user provisioning from Azure AD
- ❌ Vault secret retrieval (you'll need mock credentials or test data)

## Adding Test Data

### Create a Zone

```bash
curl -X POST http://localhost:8080/api/v1/zones \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "dev-zone",
    "type": "hub",
    "description": "Development zone"
  }'
```

### Create a Target

```bash
curl -X POST http://localhost:8080/api/v1/targets \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "zone_id": "ZONE_UUID",
    "name": "test-server",
    "hostname": "localhost",
    "protocol": "ssh",
    "port": 22,
    "description": "Local SSH server"
  }'
```

### Create Credentials

Note: In dev mode, you'll still need Vault running for the credential storage API to work. Use the dev Vault token from `.env.dev`:

```bash
# Store secret in Vault
vault kv put secret/targets/test-server \
  username=testuser \
  password=testpass

# Link to target in database
curl -X POST http://localhost:8080/api/v1/credentials \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "target_id": "TARGET_UUID",
    "username": "testuser",
    "vault_secret_path": "secret/data/targets/test-server"
  }'
```

## Testing WebSocket Connections

With a target and credentials set up, you can test SSH/RDP connections:

1. Go to http://localhost:3000/dashboard
2. Click on a target
3. Select credentials
4. Click "Connect"
5. Terminal/RDP viewer should open

## Switching Back to Production Mode

1. Set `DEV_MODE=false` in `.env`
2. Configure EntraID credentials
3. Configure Vault credentials
4. Restart backend

Or use `.env.example` as a template:
```bash
cp .env.example .env
# Edit .env with production values
```

## Troubleshooting

### "Failed to create dev user"

Check that PostgreSQL is running and migrations have been applied:
```bash
docker-compose ps postgres
go run cmd/migrate/main.go
```

### "Connection refused" to Vault

Even in dev mode, Vault service must be running for credential operations:
```bash
docker-compose up -d vault
```

### Frontend Shows "Unauthorized"

Make sure the token is being set correctly. Check browser console for errors. The dev login should redirect to:
```
http://localhost:3000/auth/callback?token=JWT_TOKEN_HERE
```

### Changes Not Taking Effect

Make sure you've restarted the backend after changing `.env`:
```bash
# Stop with Ctrl+C
go run cmd/server/main.go
```

## Security Notes

- **Never commit `.env` files** to version control
- **Never enable DEV_MODE in production**
- Dev mode bypasses all authentication - anyone can access the API
- The development user is stored in the real database
- JWT tokens are still validated (just auto-generated)

## Next Steps

Once you have dev mode working, you can:

1. **Configure Real EntraID**: See [authentication.md](authentication.md)
2. **Set up Vault Properly**: Configure AppRole or other auth methods
3. **Add Satellite Gateways**: See [satellite.md](satellite.md)
4. **Deploy to Production**: Disable DEV_MODE and use real credentials
