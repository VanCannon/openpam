# Database Package

This package provides database connectivity and migration management for OpenPAM.

## Features

- PostgreSQL connection pooling with configurable settings
- Embedded migration files (no external migration tool required)
- Transaction-safe migrations with automatic rollback on failure
- Health check functionality

## Usage

### Setting up the database

#### Option 1: Using Docker

```bash
docker run --name openpam-postgres \
  -e POSTGRES_DB=openpam \
  -e POSTGRES_USER=openpam \
  -e POSTGRES_PASSWORD=openpam \
  -p 5432:5432 \
  -d postgres:16
```

#### Option 2: Local PostgreSQL

```bash
createdb openpam
psql -d openpam -c "CREATE USER openpam WITH PASSWORD 'openpam';"
psql -d openpam -c "GRANT ALL PRIVILEGES ON DATABASE openpam TO openpam;"
```

### Running migrations

From the project root:

```bash
# Apply all pending migrations
make migrate-up

# Rollback the last migration
make migrate-down

# Check migration status
make migrate-status
```

### Using the database in code

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/bvanc/openpam/gateway/internal/database"
)

func main() {
    cfg := database.Config{
        Host:            "localhost",
        Port:            5432,
        User:            "openpam",
        Password:        "openpam",
        Database:        "openpam",
        SSLMode:         "disable",
        MaxOpenConns:    25,
        MaxIdleConns:    5,
        ConnMaxLifetime: 5 * time.Minute,
        ConnMaxIdleTime: 1 * time.Minute,
    }

    db, err := database.New(cfg)
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // Health check
    ctx := context.Background()
    if err := db.HealthCheck(ctx); err != nil {
        log.Fatal(err)
    }

    // Use db for queries...
}
```

## Migration Files

Migrations are stored in `internal/database/migrations/` with the naming pattern:

- `001_initial_schema.up.sql` - Forward migration
- `001_initial_schema.down.sql` - Rollback migration

The number prefix determines execution order.

## Schema

The database schema includes:

- **zones** - Network zones (hub/satellite)
- **targets** - Servers/systems to connect to
- **credentials** - Vault secret path references
- **users** - EntraID/AD user information
- **audit_logs** - Session audit trails

See the migration files for complete schema details.
