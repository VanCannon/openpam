Architecture Specification: OpenPAM

1. System Overview

OpenPAM is a web-based Privileged Access Management tool designed to provide secure, clientless access to infrastructure. It acts as a central gateway, enforcing authentication (via EntraID/AD) before proxying connections to SSH and RDP targets.

2. High-Level Architecture

Components

Web Client (Frontend)

Tech: Next.js (React), Tailwind CSS.

Role: Renders the UI, manages user sessions, and renders remote terminals/desktops.

Libraries: xterm.js (SSH), guacamole-common-js (RDP).

Communication: HTTPS (REST) for metadata, Secure WebSockets (WSS) for live sessions.

Unified Gateway (Backend - The "Hub")

Tech: Golang.

Role:

Auth Enforcement: Validates EntraID tokens.

Protocol Translation: Converts WebSockets -> Raw TCP (SSH) or Guacamole Protocol (RDP).

Audit: Records session metadata and raw streams to storage.

Secret Retrieval: Authenticates to and fetches credentials from the Secrets Vault on demand.

Libraries: golang.org/x/crypto/ssh, github.com/gorilla/websocket, github.com/hashicorp/vault/api (for secrets retrieval).

Satellite Gateways (Optional - The "Spokes")

Tech: Golang (Same binary, different config).

Role: Deployed in isolated networks. Establishes a reverse tunnel back to the Hub to allow access to targets without opening inbound firewall ports.

RDP Engine (Sidecar)

Tech: Apache Guacamole Daemon (guacd - C++).

Role: Handles the complex parsing of the RDP protocol.

Communication: The Go Gateway connects to guacd via TCP port 4822.

Data Store (PostgreSQL)

PostgreSQL: Stores user roles, connection profiles (hostname, port, protocol), and audit logs. It stores references to secrets, but not the secrets themselves.

Redis (Optional): Hot cache for active session states.

Secrets Vault (HashiCorp Vault)

Tech: HashiCorp Vault (with KV Secrets Engine).

Role: Dedicated, centralized store for all sensitive credentials (passwords, private keys, service account tokens). Vault manages encryption at rest and enforces policies for retrieval by the Gateway.

3. Data Flow Diagrams

Flow A: Direct Connection (Hub Network)

User clicks "Connect to Server A".

Hub Gateway validates user session.

Hub Gateway looks up target_id in PostgreSQL to find the corresponding vault_secret_path.

Hub Gateway authenticates to Secrets Vault and retrieves credentials.

Hub Gateway dials "Server A" locally (Direct TCP) using the retrieved credentials.

Flow B: Distributed Connection (Remote Network)

User clicks "Connect to Remote Server B".

Next.js connects to Hub Gateway.

Hub Gateway looks up "Server B" in Postgres and finds its Zone and vault_secret_path.

Hub Gateway authenticates to Secrets Vault and retrieves credentials.

Hub Gateway finds the active WebSocket connection from the Manufacturing Satellite.

Hub Gateway sends a "Dial Request" frame and the retrieved credentials (temporarily) down the tunnel to the Satellite.

Satellite connects to "Server B" locally using the provided credentials.

Data Path: User <-> Hub <-> Satellite <-> Target.

4. API & Interface Contracts

Internal API (Frontend <-> Gateway)

GET /api/v1/targets: List available servers for the user.

POST /api/v1/auth/login: Exchange EntraID code for Session Cookie.

WS /api/ws/connect/{protocol}/{target_id}: The main tunnel endpoint.

Database Schema

```sql
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
```

5. Security Principles

Zero Trust: The Gateway never exposes the internal network directly. It only tunnels specific protocols after auth.

Secret Isolation: Private keys and passwords never leave the Gateway backend, are never sent to the Frontend, and are stored exclusively in HashiCorp Vault. PostgreSQL only stores non-sensitive references (vault_secret_path).

Vault Integration: The Gateway will use a secure authentication method (e.g., AppRole or Kubernetes Auth) to obtain short-lived tokens from Vault before retrieving credentials.

6. Network Deployment Topology

Physical Separation Strategy

This model ensures that the sensitive Gateway logic resides in a trusted network, while public access is mediated by a DMZ.

[ Internet ]
      │ HTTPS (443)
      ▼
┌───────────────────────────────┐
│ DMZ Segment                   │
│  [ NGINX Reverse Proxy ]      │
└────────────┬──────────────────┘
             ▼
┌──────────────────────────────────────┐
│ Internal HQ (Hub)                    │
│   [ Next.js Frontend ]               │
│   [ Hub Gateway (Golang) ] <──┐      │
│   [ Postgres DB ]             │      │
│   [ Secrets Vault (HashiCorp) ]      │
└────────────▲──────────────────┼──────┘
             │ (Reverse Tunnel) │
             │                  ▼
             │          ┌──────────────┐
             │          │ Local Target │
             │          └──────────────┘
             │
      [ Internet / WAN ]
             │
┌────────────┴─────────────────────────┐
│ Isolated Network (e.g., Cloud/Branch)│
│                                      │
│   [ Satellite Gateway (Golang) ]     │
│             │                        │
│             ▼                        │
│      ┌──────────────┐                │
│      │ Remote Target│                │
│      └──────────────┘                │
└──────────────────────────────────────┘
