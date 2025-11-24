-- Zones table: Represents network zones (hub or satellite gateways)
CREATE TABLE zones (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    type VARCHAR(50) NOT NULL CHECK (type IN ('hub', 'satellite')),
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Targets table: Represents servers/systems that users can connect to
CREATE TABLE targets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    zone_id UUID NOT NULL REFERENCES zones(id) ON DELETE RESTRICT,
    name VARCHAR(255) NOT NULL,
    hostname VARCHAR(255) NOT NULL,
    protocol VARCHAR(50) NOT NULL CHECK (protocol IN ('ssh', 'rdp')),
    port INTEGER NOT NULL CHECK (port > 0 AND port <= 65535),
    description TEXT,
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(zone_id, name)
);

-- Credentials table: Maps targets to their credentials stored in Vault
CREATE TABLE credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    target_id UUID NOT NULL REFERENCES targets(id) ON DELETE CASCADE,
    username VARCHAR(255) NOT NULL,
    vault_secret_path VARCHAR(500) NOT NULL,
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(target_id, username)
);

-- Users table: Stores user information from EntraID/AD
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entra_id VARCHAR(255) NOT NULL UNIQUE,
    email VARCHAR(255) NOT NULL UNIQUE,
    display_name VARCHAR(255),
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    last_login_at TIMESTAMP WITH TIME ZONE
);

-- Audit logs table: Records all connection sessions
CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    target_id UUID NOT NULL REFERENCES targets(id) ON DELETE RESTRICT,
    credential_id UUID REFERENCES credentials(id) ON DELETE SET NULL,
    start_time TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    end_time TIMESTAMP WITH TIME ZONE,
    bytes_sent BIGINT DEFAULT 0,
    bytes_received BIGINT DEFAULT 0,
    session_status VARCHAR(50) CHECK (session_status IN ('active', 'completed', 'failed', 'terminated')),
    client_ip VARCHAR(45),
    error_message TEXT,
    recording_path VARCHAR(500),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes for performance
CREATE INDEX idx_targets_zone_id ON targets(zone_id);
CREATE INDEX idx_targets_enabled ON targets(enabled);
CREATE INDEX idx_credentials_target_id ON credentials(target_id);
CREATE INDEX idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX idx_audit_logs_target_id ON audit_logs(target_id);
CREATE INDEX idx_audit_logs_start_time ON audit_logs(start_time DESC);
CREATE INDEX idx_audit_logs_status ON audit_logs(session_status);
CREATE INDEX idx_users_entra_id ON users(entra_id);
CREATE INDEX idx_users_email ON users(email);
