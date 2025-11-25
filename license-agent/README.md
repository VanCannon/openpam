# License Agent

The License Agent is responsible for license validation, feature flag management, and usage tracking in the OpenPAM system.

## Features

- **License Validation**: Validate license keys and check expiration
- **Feature Flags**: Enable/disable features based on license
- **Usage Tracking**: Monitor users, targets, and concurrent sessions
- **NATS Integration**: Publish validation events and subscribe to session events
- **Consul Integration**: Service discovery and health checks

## API Endpoints

### Health Check
```
GET /health
```

### Validate License
```
POST /api/v1/license/validate
{
  "license_key": "your-license-key"
}
```

### Get Usage Statistics
```
GET /api/v1/license/usage
```

### Check Feature
```
POST /api/v1/license/feature
{
  "feature": "scheduling"
}
```

### Get Active License
```
GET /api/v1/license
```

## Configuration

Configuration is loaded from `config.yaml` and can be overridden with environment variables:

- `DB_HOST`: Database host
- `DB_PASSWORD`: Database password
- `NATS_URL`: NATS server URL
- `CONSUL_ADDRESS`: Consul address

## Running

### Local Development
```bash
go run cmd/license-agent/main.go
```

### Docker
```bash
docker build -t openpam/license-agent .
docker run -p 8086:8086 -v $(pwd)/config.yaml:/root/config.yaml openpam/license-agent
```

## NATS Events

### Published Events
- `openpam.license.validation`: License validation results
- `openpam.license.threshold`: Usage threshold alerts
- `openpam.license.feature`: Feature access events

### Subscribed Events
- `openpam.session.started`: Session started (for concurrent session tracking)
- `openpam.session.ended`: Session ended

## Database Schema

The agent uses the `license_info` table from the main OpenPAM database:

```sql
CREATE TABLE license_info (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    license_key VARCHAR(500) NOT NULL UNIQUE,
    license_type VARCHAR(100) NOT NULL,
    issued_to VARCHAR(255) NOT NULL,
    issued_at TIMESTAMP WITH TIME ZONE NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE,
    max_users INTEGER,
    max_targets INTEGER,
    max_sessions INTEGER,
    features JSONB DEFAULT '{}',
    is_active BOOLEAN DEFAULT true,
    activated_at TIMESTAMP WITH TIME ZONE,
    last_checked_at TIMESTAMP WITH TIME ZONE,
    validation_errors JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
```
