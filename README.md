# OpenPAM - Open Privileged Access Management

OpenPAM is a web-based Privileged Access Management tool designed to provide secure, clientless access to infrastructure. It acts as a central gateway, enforcing authentication via EntraID/AD before proxying connections to SSH and RDP targets.

## Features

- **Zero Trust Architecture** - Never expose internal networks directly
- **Clientless Access** - Browser-based SSH and RDP connections
- **Secret Isolation** - Credentials stored exclusively in HashiCorp Vault
- **Distributed Architecture** - Hub and spoke model for multi-zone deployment
- **Session Recording** - Full audit trails of all connection sessions
- **EntraID Integration** - Enterprise authentication and authorization

## Architecture

OpenPAM consists of several key components:

- **Web Client** - Next.js frontend with xterm.js (SSH) and Guacamole (RDP)
- **Gateway** - Golang backend handling authentication and protocol proxying
- **PostgreSQL** - Stores metadata (no secrets)
- **HashiCorp Vault** - Secure credential storage
- **Guacamole Daemon** - RDP protocol handling

See [docs/architecture.md](docs/architecture.md) for detailed architecture documentation.

## Quick Start

### Development Mode (Fastest)

For quick local testing without EntraID or Vault configuration:

```bash
# 1. Start services
docker-compose up -d postgres vault

# 2. Run migrations
cd gateway && go run cmd/migrate/main.go

# 3. Use development config
cp gateway/.env.dev gateway/.env

# 4. Start backend
go run cmd/server/main.go

# 5. Start frontend (in another terminal)
cd web && npm install && npm run dev
```

Visit http://localhost:3000 and click "Sign in with Microsoft" - you'll be auto-logged in!

See [docs/development.md](docs/development.md) for detailed development mode guide.

### RDP Connections

To enable RDP connections, you'll also need guacd:

```bash
# Start guacd daemon for RDP
docker-compose up -d guacd
```

RDP connections are fully browser-based using Apache Guacamole with:
- Mouse and keyboard input
- Dynamic resolution adjustment (automatically resizes to match browser window)
- Clipboard support (optional)
- Full session recording capability

### Production Setup

### Prerequisites

- Go 1.22+
- Node.js 18+
- Docker and Docker Compose
- Make
- Microsoft EntraID (Azure AD) tenant

### 1. Start Development Environment

```bash
# Start PostgreSQL and Vault
make dev-up

# Run database migrations
make migrate-up
```

### 2. Configure Environment

```bash
# Copy example environment file
cp gateway/.env.example gateway/.env

# Edit .env with your production settings
# For dev, defaults should work with Docker Compose
```

### 3. Initialize Vault (Development Only)

```bash
# Set Vault address and token
export VAULT_ADDR=http://localhost:8200
export VAULT_TOKEN=dev-root-token

# Enable KV v2 secrets engine
vault secrets enable -version=2 kv

# Create a test secret
vault kv put kv/servers/server1 username=admin password=changeme
```

### 4. Run the Gateway

```bash
make run
```

The gateway will be available at `http://localhost:8080`

### Available Endpoints

- `GET /health` - Basic health check
- `GET /ready` - Readiness check (includes DB and Vault)
- `GET /api/v1/targets` - List available targets (not implemented)
- `POST /api/v1/auth/login` - EntraID login (not implemented)
- `WS /api/ws/connect/{protocol}/{target_id}` - WebSocket tunnel (not implemented)

## Development

### Build

```bash
# Build binary
make build

# Binary will be at bin/openpam-gateway
./bin/openpam-gateway
```

### Run Tests

```bash
make test
```

### Database Migrations

```bash
# Apply migrations
make migrate-up

# Rollback last migration
make migrate-down

# Check migration status
make migrate-status
```

### Project Structure

```
gateway/
├── cmd/
│   ├── migrate/        # Migration CLI tool
│   └── server/         # Main server entry point
├── internal/
│   ├── api/            # API handlers (TODO)
│   ├── auth/           # Authentication (TODO)
│   ├── config/         # Configuration management
│   ├── database/       # Database layer
│   ├── logger/         # Structured logging
│   ├── middleware/     # HTTP middleware
│   ├── models/         # Database models
│   ├── rdp/            # RDP protocol handler (TODO)
│   ├── server/         # HTTP server
│   ├── ssh/            # SSH protocol handler (TODO)
│   └── vault/          # Vault client
└── go.mod
```

## Configuration

Configuration is loaded from environment variables:

### Database
- `DB_HOST` - PostgreSQL host (default: localhost)
- `DB_PORT` - PostgreSQL port (default: 5432)
- `DB_USER` - Database user (default: openpam)
- `DB_PASSWORD` - Database password
- `DB_NAME` - Database name (default: openpam)
- `DB_SSLMODE` - SSL mode (default: disable)

### Vault
- `VAULT_ADDR` - Vault address (default: http://localhost:8200)
- `VAULT_TOKEN` - Vault token (for dev)
- `VAULT_ROLE_ID` - AppRole role ID (for prod)
- `VAULT_SECRET_ID` - AppRole secret ID (for prod)

### EntraID
- `ENTRA_TENANT_ID` - Azure AD tenant ID
- `ENTRA_CLIENT_ID` - Application client ID
- `ENTRA_CLIENT_SECRET` - Application client secret

### Server
- `SERVER_HOST` - HTTP server host (default: 0.0.0.0)
- `SERVER_PORT` - HTTP server port (default: 8080)

### Zone
- `ZONE_TYPE` - Zone type: hub or satellite (default: hub)
- `ZONE_NAME` - Zone name (default: default)

## Database Schema

The database includes the following tables:

- **zones** - Network zones (hub/satellite gateways)
- **targets** - Servers/systems users can connect to
- **credentials** - Vault secret path references (no actual credentials)
- **users** - EntraID/AD user information
- **audit_logs** - Complete session audit trails

See the schema in [docs/architecture.md](docs/architecture.md) or [gateway/internal/database/migrations/](gateway/internal/database/migrations/)

## Security

- Credentials are **never** stored in the database
- All secrets are retrieved from HashiCorp Vault at connection time
- Sessions are fully audited with metadata stored in PostgreSQL
- Zero Trust model - no direct network access to targets
- EntraID authentication required for all access

## Roadmap

### Phase 1: Core Infrastructure ✅
- [x] Database schema and models
- [x] Configuration management
- [x] Vault integration
- [x] Basic HTTP server with health checks
- [x] Logging and middleware

### Phase 2: Authentication (TODO)
- [ ] EntraID OAuth2 integration
- [ ] JWT token management
- [ ] Session handling
- [ ] User management

### Phase 3: Protocol Handlers
- [x] WebSocket tunnel endpoint
- [x] SSH proxy implementation
- [x] RDP proxy with Guacamole
  - [x] Mouse and keyboard input
  - [x] Dynamic resolution adjustment
  - [x] Clipboard support
- [ ] Session recording

### Phase 4: Repository Layer (TODO)
- [ ] CRUD operations for all models
- [ ] Target listing API
- [ ] Audit log queries

### Phase 5: Satellite Gateway (TODO)
- [ ] Reverse tunnel mechanism
- [ ] Multi-zone support
- [ ] Hub-spoke communication

### Phase 6: Frontend (TODO)
- [ ] Next.js application
- [ ] Terminal emulator (xterm.js)
- [ ] RDP client (Guacamole JS)
- [ ] Target selection UI

## Contributing

This is a personal/learning project. Contributions are welcome!

## License

TBD

## Support

For issues and questions, see the [GitHub Issues](https://github.com/bvanc/openpam/issues)
