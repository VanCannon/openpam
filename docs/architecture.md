Architecture Specification: OpenPAM

1. System Overview

OpenPAM is a web-based Privileged Access Management tool designed to provide secure, clientless access to infrastructure. It acts as a central gateway, enforcing authentication (via EntraID/AD) before proxying connections to SSH and RDP targets.

The system is evolving into a comprehensive PAM platform with an orchestrator-based microservices architecture, supporting advanced features including scheduling, automation, identity management, and multi-channel communications.

2. High-Level Architecture

## Core Components

### Web Client (Frontend)

Tech: Next.js 16 (React 19), Tailwind CSS.

Role: Renders the UI, manages user sessions, and renders remote terminals/desktops.

Libraries: xterm.js (SSH - dynamically imported for React 19 compatibility), guacamole-common-js (RDP).

Communication: HTTPS (REST) for metadata, Secure WebSockets (WSS) for live sessions.

### Unified Gateway (Backend - The "Hub")

Tech: Golang.

Role:

Auth Enforcement: Validates EntraID tokens.

Protocol Translation: Converts WebSockets -> Raw TCP (SSH) or Guacamole Protocol (RDP).

Audit: Records session metadata and raw streams to storage.

Secret Retrieval: Authenticates to and fetches credentials from the Secrets Vault on demand.

Libraries: golang.org/x/crypto/ssh, github.com/gorilla/websocket, github.com/hashicorp/vault/api (for secrets retrieval).

### Satellite Gateways (Optional - The "Spokes")

Tech: Golang (Same binary, different config).

Role: Deployed in isolated networks. Establishes a reverse tunnel back to the Hub to allow access to targets without opening inbound firewall ports.

### RDP Engine (Sidecar)

Tech: Apache Guacamole Daemon (guacd - C++).

Role: Handles the complex parsing of the RDP protocol.

Communication: The Go Gateway connects to guacd via TCP port 4822.

### Data Store (PostgreSQL)

PostgreSQL: Stores user roles, connection profiles (hostname, port, protocol), and audit logs. It stores references to secrets, but not the secrets themselves.

Redis (Optional): Hot cache for active session states, distributed state management for orchestrator.

### Secrets Vault (HashiCorp Vault)

Tech: HashiCorp Vault (with KV Secrets Engine).

Role: Dedicated, centralized store for all sensitive credentials (passwords, private keys, service account tokens). Vault manages encryption at rest and enforces policies for retrieval by the Gateway.

## Orchestration Layer (Planned)

### Orchestrator

Tech: Golang with NATS event bus, Consul service registry.

Role: Central coordination layer managing workflows, service communication, and complex multi-step operations.

Components:
- Workflow Manager: Executes multi-step workflows with dependency management
- Event Bus: Asynchronous pub/sub messaging (NATS)
- Service Registry: Dynamic service discovery and health checking (Consul)
- State Management: Distributed state storage (Redis/etcd)

### Microservice Agents

The orchestrator coordinates six specialized agents:

1. **Scheduling Agent** (Port 8081)
   - Time-based access control
   - Session scheduling windows
   - Recurring access patterns
   - Calendar integration

2. **Identity Agent** (Port 8082)
   - Active Directory / LDAP synchronization
   - User and group import
   - Organizational unit mapping
   - Incremental and full sync

3. **Activity Agent** (Port 8083)
   - User lifecycle management (create/delete/enable/disable)
   - Group membership management
   - PowerShell script execution (Windows)
   - Bash script execution (Linux)
   - Audit trail logging

4. **Automation Agent** (Port 8084)
   - Ansible playbook execution
   - Infrastructure provisioning
   - Configuration management
   - Task scheduling

5. **Communications Agent** (Port 8085)
   - Email notifications (SMTP)
   - Slack integration
   - Microsoft Teams integration
   - SIEM log forwarding (CEF, LEEF, Syslog, JSON)
   - Multi-format log aggregation

6. **License Agent** (Port 8086)
   - License validation and enforcement
   - Feature flag management
   - Usage tracking and limits
   - Concurrent session enforcement
   - Expiration notifications

3. Extended Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           User Interface Layer                          │
│                     Next.js 16 + React 19 Frontend                     │
│              (xterm.js terminal, Admin panels, Dashboard)              │
└────────────────────────────────┬────────────────────────────────────────┘
                                 │ HTTPS/WSS
                                 ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                         Gateway Layer (Port 8080)                       │
│                          Golang Gateway (Hub)                           │
│   - Authentication (EntraID/AD)                                        │
│   - SSH/RDP Protocol Translation                                       │
│   - WebSocket Management                                               │
│   - Credential Retrieval from Vault                                    │
└────────────┬────────────────────────────────┬───────────────────────────┘
             │                                │
             │ API Calls                      │ Events
             ▼                                ▼
┌─────────────────────────┐    ┌──────────────────────────────────────────┐
│   PostgreSQL Database   │    │        Orchestrator (Port 8090)          │
│  - Users                │    │  ┌────────────────────────────────────┐  │
│  - Targets              │◄───┤  │   Core Orchestration Engine        │  │
│  - Zones                │    │  │  - Workflow Manager                │  │
│  - Credentials (refs)   │    │  │  - NATS Event Bus                  │  │
│  - Audit Logs           │    │  │  - Consul Service Registry         │  │
│  - Schedules            │    │  │  - Redis State Management          │  │
└─────────────────────────┘    │  └────────────────────────────────────┘  │
                                └──────────────┬───────────────────────────┘
┌─────────────────────────┐                   │
│   HashiCorp Vault       │                   │ Coordinates
│  - Credentials          │                   ▼
│  - Private Keys         │    ┌──────────────────────────────────────────┐
│  - Service Tokens       │    │         Microservice Agents              │
└─────────────────────────┘    └──────────────────────────────────────────┘
                                │
              ┌─────────────────┴─────────────────┬───────────────┬──────────┐
              │                 │                 │               │          │
              ▼                 ▼                 ▼               ▼          ▼
    ┌─────────────────┐ ┌──────────────┐ ┌─────────────┐ ┌──────────┐ ┌────────┐
    │  Scheduling     │ │   Identity   │ │  Activity   │ │  Comms   │ │License │
    │    Agent        │ │    Agent     │ │   Agent     │ │  Agent   │ │ Agent  │
    │  (Port 8081)    │ │ (Port 8082)  │ │(Port 8083)  │ │(Port 8085)│ │(8086) │
    │                 │ │              │ │             │ │          │ │        │
    │ - Time windows  │ │ - AD/LDAP    │ │ - User Mgmt │ │ - Email  │ │-Limits │
    │ - Schedules     │ │ - Sync users │ │ - Scripts   │ │ - Slack  │ │-Features│
    │ - Recurring     │ │ - Sync groups│ │ - Workflows │ │ - Teams  │ │-Usage  │
    └─────────────────┘ └──────┬───────┘ └──────┬──────┘ │ - SIEM   │ └────────┘
                               │                │        └─────┬────┘
                               ▼                ▼              │
                        ┌─────────────┐  ┌──────────┐         │
                        │Active       │  │PowerShell│         ▼
                        │Directory/   │  │  Bash    │  ┌──────────────┐
                        │LDAP Servers │  │ Scripts  │  │Splunk/Elastic│
                        └─────────────┘  └──────────┘  │Azure Sentinel│
                                                       │  Syslog      │
         ┌────────────────────────────────────────────┤  SIEM        │
         │     Automation Agent (Port 8084)           └──────────────┘
         │
         │  - Ansible Playbooks
         │  - Infrastructure Automation
         │  - Configuration Management
         └────────┬──────────────────────────
                  ▼
         ┌──────────────────┐
         │ Target Infrastructure
         │ - SSH Servers
         │ - RDP Servers
         │ - Cloud Resources
         └──────────────────┘
```

4. Data Flow Diagrams

## Flow A: Direct Connection (Hub Network)

User clicks "Connect to Server A".

Hub Gateway validates user session.

Hub Gateway looks up target_id in PostgreSQL to find the corresponding vault_secret_path.

Hub Gateway authenticates to Secrets Vault and retrieves credentials.

Hub Gateway dials "Server A" locally (Direct TCP) using the retrieved credentials.

## Flow B: Distributed Connection (Remote Network)

User clicks "Connect to Remote Server B".

Next.js connects to Hub Gateway.

Hub Gateway looks up "Server B" in Postgres and finds its Zone and vault_secret_path.

Hub Gateway authenticates to Secrets Vault and retrieves credentials.

Hub Gateway finds the active WebSocket connection from the Manufacturing Satellite.

Hub Gateway sends a "Dial Request" frame and the retrieved credentials (temporarily) down the tunnel to the Satellite.

Satellite connects to "Server B" locally using the provided credentials.

Data Path: User <-> Hub <-> Satellite <-> Target.

## Flow C: Scheduled Session with Orchestration (New)

User requests scheduled access to Server C for next Tuesday 9 AM - 5 PM.

Frontend sends schedule request to Gateway API.

Gateway publishes `schedule.requested` event to NATS Event Bus.

Orchestrator Workflow Manager starts "scheduled_access" workflow:
  - Step 1: License Agent validates user count and concurrent session limits
  - Step 2: Scheduling Agent creates schedule record with time window
  - Step 3: Identity Agent checks if user exists in AD (sync if needed)
  - Step 4: Activity Agent grants target access permissions
  - Step 5: Communications Agent sends email confirmation to user
  - Step 6: Communications Agent logs event to SIEM

When Tuesday 9 AM arrives:
  - Scheduling Agent publishes `schedule.activated` event
  - Gateway allows user connection to Server C
  - Communications Agent sends Slack notification: "Access to Server C is now available"

When Tuesday 5 PM arrives:
  - Scheduling Agent publishes `schedule.expired` event
  - Gateway blocks further connections
  - Active sessions are gracefully terminated
  - Communications Agent sends Slack notification: "Access to Server C has expired"

## Flow D: User Provisioning with Automation (New)

Admin requests new user provisioning via Frontend.

Gateway publishes `user.provision.requested` event to NATS.

Orchestrator Workflow Manager starts "user_provisioning" workflow:
  - Step 1: License Agent checks if user count is below license limit
  - Step 2: Identity Agent creates user in Active Directory with groups
  - Step 3: Activity Agent creates corresponding OpenPAM user record
  - Step 4: Automation Agent runs Ansible playbook to create home directory
  - Step 5: Automation Agent runs PowerShell script to set email permissions
  - Step 6: Activity Agent assigns target access based on user role
  - Step 7: Communications Agent sends welcome email to new user
  - Step 8: Communications Agent logs provisioning event to SIEM

If any step fails, Orchestrator executes compensation logic:
  - Rollback AD user creation
  - Delete OpenPAM user record
  - Log failure to SIEM
  - Send alert email to admin

## Flow E: Session with Real-time Notifications (New)

User starts SSH session to production database server.

Gateway publishes `session.started` event to NATS Event Bus.

Communications Agent subscribes to session events:
  - Sends Slack message to #security channel: "User john.doe started privileged session to prod-db-01"
  - Forwards CEF-formatted log to Splunk SIEM
  - Sends email to DBA team (if configured for critical servers)

During session, Activity Agent monitors commands executed.

When session ends, Gateway publishes `session.ended` event.

Communications Agent:
  - Sends Slack message: "Session ended. Duration: 15 minutes"
  - Forwards session summary to SIEM with command count and data transferred

5. API & Interface Contracts

## Core Gateway API (Frontend <-> Gateway)

**Authentication & Users:**
- `POST /api/v1/auth/login` - Exchange EntraID code for Session Cookie
- `GET /api/v1/auth/me` - Get current user information
- `POST /api/v1/auth/logout` - Logout current user

**Targets & Connections:**
- `GET /api/v1/targets` - List available servers for the user
- `GET /api/v1/targets/{id}` - Get target details
- `WS /api/ws/connect/{protocol}/{target_id}` - Main tunnel endpoint for SSH/RDP

**Admin Management:**
- `GET /api/v1/admin/zones` - List all zones
- `POST /api/v1/admin/zones` - Create new zone
- `GET /api/v1/admin/targets` - List all targets (admin view)
- `POST /api/v1/admin/targets` - Create new target
- `GET /api/v1/admin/credentials` - List all credentials
- `POST /api/v1/admin/credentials` - Create new credential

**Audit:**
- `GET /api/v1/audit` - List audit logs with filtering

## Orchestrator API (Gateway <-> Orchestrator)

**Service Registry:**
- `GET /api/v1/orchestrator/services` - List registered services
- `GET /api/v1/orchestrator/services/{name}/health` - Service health check

**Workflow Management:**
- `POST /api/v1/orchestrator/workflows` - Trigger a workflow
- `GET /api/v1/orchestrator/workflows/{id}` - Get workflow status
- `GET /api/v1/orchestrator/workflows/{id}/history` - Get workflow execution history

**Events:**
- NATS Subjects (Pub/Sub):
  - `openpam.session.*` - Session lifecycle events
  - `openpam.schedule.*` - Scheduling events
  - `openpam.user.*` - User management events
  - `openpam.license.*` - License events
  - `openpam.automation.*` - Automation events

## Agent-Specific APIs

**Scheduling Agent (Port 8081):**
- `POST /api/v1/schedules` - Create schedule
- `GET /api/v1/schedules/{id}` - Get schedule
- `GET /api/v1/schedules/active` - List active schedules
- `POST /api/v1/schedules/validate` - Validate access window

**Identity Agent (Port 8082):**
- `POST /api/v1/identity/sync/users` - Sync users from AD
- `POST /api/v1/identity/sync/groups` - Sync groups from AD
- `GET /api/v1/identity/sync/status` - Get sync status
- `POST /api/v1/identity/config` - Update AD connection config

**Activity Agent (Port 8083):**
- `POST /api/v1/activity/users` - Create user
- `PUT /api/v1/activity/users/{id}` - Update user
- `POST /api/v1/activity/users/{id}/enable` - Enable user
- `POST /api/v1/activity/users/{id}/disable` - Disable user
- `POST /api/v1/activity/scripts/powershell` - Execute PowerShell
- `POST /api/v1/activity/scripts/bash` - Execute Bash script

**Automation Agent (Port 8084):**
- `POST /api/v1/automation/playbooks` - Create playbook
- `POST /api/v1/automation/playbooks/{id}/execute` - Execute playbook
- `GET /api/v1/automation/executions/{id}` - Get execution status
- `GET /api/v1/automation/executions/{id}/logs` - Get execution logs

**Communications Agent (Port 8085):**
- `POST /api/v1/comms/email` - Send email
- `POST /api/v1/comms/slack` - Send Slack message
- `POST /api/v1/comms/teams` - Send Teams message
- `POST /api/v1/comms/siem` - Forward log to SIEM
- `GET /api/v1/comms/templates` - List notification templates

**License Agent (Port 8086):**
- `POST /api/v1/license/validate` - Validate license key
- `GET /api/v1/license/status` - Get license status
- `GET /api/v1/license/features` - List enabled features
- `GET /api/v1/license/usage` - Get current usage stats
- `GET /api/v1/license/limits` - Get license limits

6. Database Schema

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

-- Schedules table: Time-based access control (New)
CREATE TABLE schedules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    target_id UUID NOT NULL REFERENCES targets(id) ON DELETE CASCADE,
    start_time TIMESTAMP WITH TIME ZONE NOT NULL,
    end_time TIMESTAMP WITH TIME ZONE NOT NULL,
    recurrence_rule VARCHAR(500), -- iCal RRULE format
    timezone VARCHAR(100) DEFAULT 'UTC',
    status VARCHAR(50) NOT NULL CHECK (status IN ('pending', 'active', 'expired', 'cancelled')),
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    metadata JSONB
);

CREATE INDEX idx_schedules_user_id ON schedules(user_id);
CREATE INDEX idx_schedules_target_id ON schedules(target_id);
CREATE INDEX idx_schedules_status ON schedules(status);
CREATE INDEX idx_schedules_time_range ON schedules(start_time, end_time);

-- Workflow executions table: Track orchestrator workflows (New)
CREATE TABLE workflow_executions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_name VARCHAR(255) NOT NULL,
    trigger_event VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL CHECK (status IN ('running', 'completed', 'failed', 'cancelled')),
    input_data JSONB,
    output_data JSONB,
    error_message TEXT,
    started_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    completed_at TIMESTAMP WITH TIME ZONE,
    triggered_by UUID REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX idx_workflow_executions_name ON workflow_executions(workflow_name);
CREATE INDEX idx_workflow_executions_status ON workflow_executions(status);
CREATE INDEX idx_workflow_executions_started_at ON workflow_executions(started_at DESC);

-- AD sync jobs table: Track identity synchronization (New)
CREATE TABLE ad_sync_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sync_type VARCHAR(50) NOT NULL CHECK (sync_type IN ('full', 'incremental', 'users', 'groups')),
    status VARCHAR(50) NOT NULL CHECK (status IN ('running', 'completed', 'failed')),
    users_synced INTEGER DEFAULT 0,
    groups_synced INTEGER DEFAULT 0,
    errors_count INTEGER DEFAULT 0,
    error_details JSONB,
    started_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    completed_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_ad_sync_jobs_status ON ad_sync_jobs(status);
CREATE INDEX idx_ad_sync_jobs_started_at ON ad_sync_jobs(started_at DESC);

-- Script executions table: Track PowerShell/Bash execution (New)
CREATE TABLE script_executions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    script_type VARCHAR(50) NOT NULL CHECK (script_type IN ('powershell', 'bash')),
    script_content TEXT NOT NULL,
    target_host VARCHAR(255),
    status VARCHAR(50) NOT NULL CHECK (status IN ('queued', 'running', 'completed', 'failed')),
    output TEXT,
    error_message TEXT,
    exit_code INTEGER,
    started_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    completed_at TIMESTAMP WITH TIME ZONE,
    executed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    timeout_seconds INTEGER DEFAULT 300
);

CREATE INDEX idx_script_executions_status ON script_executions(status);
CREATE INDEX idx_script_executions_executed_by ON script_executions(executed_by);
CREATE INDEX idx_script_executions_started_at ON script_executions(started_at DESC);

-- Ansible playbook executions table (New)
CREATE TABLE ansible_executions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    playbook_name VARCHAR(255) NOT NULL,
    playbook_path VARCHAR(500) NOT NULL,
    inventory TEXT NOT NULL,
    extra_vars JSONB,
    tags VARCHAR(500),
    status VARCHAR(50) NOT NULL CHECK (status IN ('queued', 'running', 'completed', 'failed')),
    output TEXT,
    failed_tasks JSONB,
    stats JSONB, -- ok_count, failed_count, changed_count
    started_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    completed_at TIMESTAMP WITH TIME ZONE,
    executed_by UUID REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX idx_ansible_executions_status ON ansible_executions(status);
CREATE INDEX idx_ansible_executions_playbook_name ON ansible_executions(playbook_name);
CREATE INDEX idx_ansible_executions_started_at ON ansible_executions(started_at DESC);

-- Notifications log table (New)
CREATE TABLE notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    channel VARCHAR(50) NOT NULL CHECK (channel IN ('email', 'slack', 'teams', 'sms', 'siem')),
    recipient VARCHAR(500) NOT NULL,
    subject VARCHAR(500),
    message TEXT NOT NULL,
    status VARCHAR(50) NOT NULL CHECK (status IN ('pending', 'sent', 'failed')),
    error_message TEXT,
    event_type VARCHAR(255),
    related_entity_type VARCHAR(100), -- session, user, schedule, etc.
    related_entity_id UUID,
    sent_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    metadata JSONB
);

CREATE INDEX idx_notifications_channel ON notifications(channel);
CREATE INDEX idx_notifications_status ON notifications(status);
CREATE INDEX idx_notifications_sent_at ON notifications(sent_at DESC);
CREATE INDEX idx_notifications_event_type ON notifications(event_type);

-- License information table (New)
CREATE TABLE license_info (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    license_key VARCHAR(500) NOT NULL UNIQUE,
    customer_id VARCHAR(255) NOT NULL,
    customer_name VARCHAR(500),
    product_edition VARCHAR(100) NOT NULL CHECK (product_edition IN ('community', 'professional', 'enterprise')),
    issue_date TIMESTAMP WITH TIME ZONE NOT NULL,
    expiration_date TIMESTAMP WITH TIME ZONE NOT NULL,
    max_users INTEGER,
    max_concurrent_sessions INTEGER,
    max_targets INTEGER,
    features JSONB NOT NULL, -- List of enabled features
    status VARCHAR(50) NOT NULL CHECK (status IN ('active', 'expired', 'suspended', 'revoked')),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_license_info_status ON license_info(status);
CREATE INDEX idx_license_info_expiration ON license_info(expiration_date);
```

7. Security Principles

Zero Trust: The Gateway never exposes the internal network directly. It only tunnels specific protocols after auth.

Secret Isolation: Private keys and passwords never leave the Gateway backend, are never sent to the Frontend, and are stored exclusively in HashiCorp Vault. PostgreSQL only stores non-sensitive references (vault_secret_path).

Vault Integration: The Gateway will use a secure authentication method (e.g., AppRole or Kubernetes Auth) to obtain short-lived tokens from Vault before retrieving credentials.

Service-to-Service Authentication: All microservices use mTLS for encrypted communication and service identity verification.

Audit Everything: All orchestrator workflows, agent actions, and state changes are logged immutably for compliance and forensics.

8. Network Deployment Topology

Physical Separation Strategy

This model ensures that the sensitive Gateway logic resides in a trusted network, while public access is mediated by a DMZ.

## Traditional Deployment (Current)

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

## Extended Deployment with Orchestration (Planned)

[ Internet ]
      │ HTTPS (443)
      ▼
┌───────────────────────────────┐
│ DMZ Segment                   │
│  [ NGINX Reverse Proxy ]      │
│  [ API Gateway (Optional) ]   │
└────────────┬──────────────────┘
             ▼
┌────────────────────────────────────────────────────────┐
│ Internal HQ Network (Hub)                              │
│                                                        │
│  ┌────────────────────┐                               │
│  │  Frontend Tier     │                               │
│  │  [ Next.js Web ]   │                               │
│  └─────────┬──────────┘                               │
│            │                                           │
│  ┌─────────▼──────────────────────────────────────┐   │
│  │  Application Tier                              │   │
│  │  ┌──────────────────┐  ┌──────────────────┐  │   │
│  │  │  Hub Gateway     │  │   Orchestrator   │  │   │
│  │  │  (Port 8080)     │◄─┤   (Port 8090)    │  │   │
│  │  └──────────────────┘  └────────┬─────────┘  │   │
│  │                                  │            │   │
│  │         ┌────────────────────────┼────────────┼───┼─────┐
│  │         │                        │            │   │     │
│  │  ┌──────▼──────┐  ┌──────▼──────▼────┬───────▼───▼──┐  │
│  │  │ Scheduling  │  │  Identity Agent  │   Activity   │  │
│  │  │   Agent     │  │  (Port 8082)     │    Agent     │  │
│  │  │ (Port 8081) │  │  ┌──────────┐    │ (Port 8083)  │  │
│  │  └─────────────┘  │  │  AD/LDAP │    └───┬──────────┘  │
│  │                   │  │  Sync    │        │             │
│  │  ┌─────────────┐  │  └──────────┘    ┌───▼──────────┐  │
│  │  │ Automation  │  └─────────────────►│ PowerShell/  │  │
│  │  │   Agent     │                     │ Bash Scripts │  │
│  │  │ (Port 8084) │                     └──────────────┘  │
│  │  │ ┌─────────┐ │                                       │
│  │  │ │ Ansible │ │  ┌──────────────┐  ┌──────────────┐  │
│  │  │ └─────────┘ │  │ Comms Agent  │  │License Agent │  │
│  │  └─────────────┘  │ (Port 8085)  │  │ (Port 8086)  │  │
│  │                   │ ┌──────────┐ │  └──────────────┘  │
│  │                   │ │Email/Slack│ │                    │
│  │                   │ │Teams/SIEM│ │                    │
│  │                   │ └──────────┘ │                    │
│  │                   └──────────────┘                    │
│  └────────────────────────────────────────────────────────┘
│                                                            │
│  ┌─────────────────────────────────────────────────────┐  │
│  │  Infrastructure Services Tier                       │  │
│  │  ┌──────────┐  ┌───────────┐  ┌─────────────────┐  │  │
│  │  │PostgreSQL│  │   Redis   │  │  HashiCorp      │  │  │
│  │  │ Database │  │  (State)  │  │    Vault        │  │  │
│  │  └──────────┘  └───────────┘  │  (Secrets)      │  │  │
│  │                                └─────────────────┘  │  │
│  │  ┌──────────┐  ┌──────────┐                        │  │
│  │  │   NATS   │  │  Consul  │                        │  │
│  │  │Event Bus │  │ Registry │                        │  │
│  │  └──────────┘  └──────────┘                        │  │
│  └─────────────────────────────────────────────────────┘  │
│                                                            │
│          │                                  │              │
│          ▼                                  ▼              │
│   ┌──────────────┐                  ┌──────────────┐      │
│   │ Local Target │                  │ Local Target │      │
│   │   (SSH/RDP)  │                  │   (SSH/RDP)  │      │
│   └──────────────┘                  └──────────────┘      │
└────────────────────────────────────────────────────────────┘
             │
             │ (Reverse Tunnel)
             │
      [ Internet / WAN ]
             │
┌────────────┴─────────────────────────────────────────────┐
│ Isolated Network (e.g., Cloud/Branch/Manufacturing)      │
│                                                          │
│   [ Satellite Gateway (Golang) ]                        │
│             │                                            │
│             ▼                                            │
│      ┌─────────────────────────────────────┐            │
│      │ Remote Targets (SSH/RDP)            │            │
│      │ - Database Servers                  │            │
│      │ - Application Servers               │            │
│      │ - Network Devices                   │            │
│      └─────────────────────────────────────┘            │
└──────────────────────────────────────────────────────────┘
             │
             ▼
   ┌──────────────────────┐
   │ External Systems     │
   │ - Active Directory   │
   │ - LDAP Servers       │
   │ - Email/SMTP         │
   │ - Slack/Teams APIs   │
   │ - SIEM (Splunk, ELK) │
   └──────────────────────┘

## Deployment Considerations

**Service Ports Summary:**
- Gateway: 8080
- Orchestrator: 8090
- Scheduling Agent: 8081
- Identity Agent: 8082
- Activity Agent: 8083
- Automation Agent: 8084
- Communications Agent: 8085
- License Agent: 8086
- PostgreSQL: 5432
- Redis: 6379
- NATS: 4222
- Consul: 8500
- Vault: 8200

**High Availability Options:**
- Load-balanced Gateway instances
- Clustered NATS for event bus
- PostgreSQL replication (primary/standby)
- Redis Sentinel for automatic failover
- Consul cluster (3 or 5 nodes)

**Scalability:**
- Horizontal scaling: Run multiple instances of each agent
- Event-driven architecture enables independent scaling
- Agent auto-discovery via Consul service registry

**Monitoring & Observability:**
- Prometheus metrics from all services
- Grafana dashboards for visualization
- Jaeger distributed tracing
- Centralized logging (ELK or Loki)
