# OpenPAM Agent Services

This document provides an overview of all agent services implemented for the OpenPAM system.

## Architecture Overview

The OpenPAM system uses a microservices architecture with the following components:

- **Gateway** (Port 8080): Main API gateway and SSH proxy
- **Orchestrator** (Port 8090): Coordinates workflows across all agents
- **License Agent** (Port 8086): License validation and feature flags
- **Scheduling Agent** (Port 8081): Time-based access control
- **Identity Agent** (Port 8082): AD/LDAP synchronization
- **Activity Agent** (Port 8083): User lifecycle management and script execution
- **Automation Agent** (Port 8084): Ansible playbook execution
- **Communications Agent** (Port 8085): Email, Slack, Teams, SIEM integration

## Quick Start

### Development Setup

1. **Start all services with Docker Compose:**
```bash
docker-compose up -d
```

2. **Check service health:**
```bash
docker-compose ps
```

3. **View logs:**
```bash
docker-compose logs -f [service-name]
```

### Individual Agent Development

Each agent can be run independently for development:

```bash
cd license-agent
go run cmd/license-agent/main.go
```

## Agent Details

### 1. License Agent (Port 8086)

**Purpose**: License validation, feature flags, usage tracking

**Key Endpoints:**
- `POST /api/v1/license/validate` - Validate license key
- `GET /api/v1/license/usage` - Get usage statistics
- `POST /api/v1/license/feature` - Check feature availability
- `GET /api/v1/license` - Get active license

**NATS Events Published:**
- `openpam.license.validation` - License validation results
- `openpam.license.threshold` - Usage threshold alerts
- `openpam.license.feature` - Feature access events

**NATS Events Subscribed:**
- `openpam.session.started` - Track concurrent sessions
- `openpam.session.ended` - Update session counts

### 2. Scheduling Agent (Port 8081)

**Purpose**: Time-based access control and scheduling

**Key Endpoints:**
- `POST /api/v1/schedules` - Create schedule
- `GET /api/v1/schedules` - List schedules
- `GET /api/v1/schedules/{id}` - Get schedule
- `PUT /api/v1/schedules/{id}` - Update schedule
- `DELETE /api/v1/schedules/{id}` - Delete schedule
- `POST /api/v1/schedule/check` - Check if access is allowed

**NATS Events Published:**
- `openpam.schedule.created` - Schedule created
- `openpam.schedule.activated` - Schedule became active
- `openpam.schedule.expired` - Schedule expired
- `openpam.schedule.updated` - Schedule updated
- `openpam.schedule.deleted` - Schedule deleted

**Background Tasks:**
- Checks schedules every 60 seconds
- Automatically activates pending schedules
- Expires schedules that have passed their end time

### 3. Identity Agent (Port 8082)

**Purpose**: AD/LDAP synchronization and identity management

**Status**: Directory structure created. Implementation includes:
- LDAP/AD connection and authentication
- User and group synchronization
- Sync job scheduling and status tracking
- Differential sync support

**Configuration:**
```yaml
ldap:
  enabled: true
  server: "ldap://ad.example.com:389"
  bind_dn: "cn=admin,dc=example,dc=com"
  bind_password: "password"
  sync_interval: "1h"
```

### 4. Activity Agent (Port 8083)

**Purpose**: User lifecycle management and script execution

**Status**: Directory structure created. Implementation includes:
- User provisioning (create, disable, enable, delete)
- Group membership management
- PowerShell and Bash script execution
- Script execution history and logging

### 5. Automation Agent (Port 8084)

**Purpose**: Ansible playbook execution

**Status**: Directory structure created. Implementation includes:
- Ansible playbook execution
- Inventory management
- Execution tracking and logging
- Integration with vault for secrets

### 6. Communications Agent (Port 8085)

**Purpose**: Multi-channel notifications and SIEM integration

**Status**: Directory structure created. Implementation includes:
- Email notifications (SMTP)
- Slack webhooks
- Microsoft Teams webhooks
- SIEM log forwarding (CEF, LEEF, Syslog, JSON)

### 7. Orchestrator (Port 8090)

**Purpose**: Workflow coordination across all agents

**Status**: Directory structure created. Implementation includes:
- Workflow definition and execution
- Event-driven workflow triggers
- State management with Redis
- Service discovery via Consul
- Workflow execution tracking

## Database Schema

All agents share the same PostgreSQL database with the following tables:

- `license_info` - License information
- `schedules` - Scheduled access windows
- `ad_sync_jobs` - AD/LDAP sync job history
- `script_executions` - Script execution logs
- `ansible_executions` - Ansible playbook execution logs
- `notifications` - Notification queue and history
- `workflow_executions` - Workflow execution state

## Service Discovery

All agents register with Consul for service discovery:

```go
serviceName := "license-agent"
serviceID := "license-agent-1"
port := 8086
```

Health checks are automatically configured at `/health` endpoint.

## Event-Driven Communication

All agents use NATS for pub/sub messaging:

**Subject Naming Convention:**
- `openpam.{domain}.{event}`
- Example: `openpam.session.started`, `openpam.license.validation`

## Configuration

Each agent uses a `config.yaml` file with environment variable overrides:

```yaml
server:
  port: 8086
  host: "0.0.0.0"

database:
  host: "${DB_HOST:-localhost}"
  password: "${DB_PASSWORD}"

nats:
  url: "${NATS_URL:-nats://localhost:4222}"

consul:
  address: "${CONSUL_ADDRESS:-localhost:8500}"
```

## Development Workflow

1. **Make changes** to an agent
2. **Rebuild the agent:**
   ```bash
   cd license-agent
   docker-compose build license-agent
   ```
3. **Restart the agent:**
   ```bash
   docker-compose up -d license-agent
   ```
4. **View logs:**
   ```bash
   docker-compose logs -f license-agent
   ```

## Testing

Each agent exposes a `/health` endpoint for health checks:

```bash
curl http://localhost:8086/health
```

## Next Steps

1. **Implement remaining agents**: Complete Identity, Activity, Automation, Communications, and Orchestrator agents
2. **Add authentication**: Integrate JWT authentication across all agents
3. **Add metrics**: Implement Prometheus metrics endpoints
4. **Add tracing**: Integrate distributed tracing (Jaeger/Zipkin)
5. **Add tests**: Unit and integration tests for each agent
6. **Add API documentation**: OpenAPI/Swagger specs

## Troubleshooting

**Agent won't start:**
- Check database connection: `docker-compose logs postgres`
- Check NATS connection: `docker-compose logs nats`
- Check Consul connection: `docker-compose logs consul`

**Agent not registered in Consul:**
- Verify Consul is running: `curl http://localhost:8500/v1/agent/services`
- Check agent logs for registration errors

**NATS events not being received:**
- Check NATS is running: `curl http://localhost:8222/healthz`
- Verify subject names match between publisher and subscriber
- Check NATS logs: `docker-compose logs nats`

## Production Considerations

1. **High Availability**: Run multiple instances of each agent
2. **Load Balancing**: Use Consul for service discovery and load balancing
3. **Secrets Management**: Use Vault for all secrets
4. **Monitoring**: Implement comprehensive logging and metrics
5. **Security**: Enable mTLS between services
6. **Backups**: Regular database backups
7. **Disaster Recovery**: Document and test DR procedures
