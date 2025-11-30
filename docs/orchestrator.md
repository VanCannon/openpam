# OpenPAM Orchestrator Architecture

## Overview

The OpenPAM Orchestrator is the central coordination layer that manages interactions between microservices, handles complex workflows, and ensures proper sequencing of operations across the platform.

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                      OpenPAM Orchestrator                        │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │              Core Orchestration Engine                      │ │
│  │  - Workflow Management                                      │ │
│  │  - Event Bus / Message Queue                               │ │
│  │  - Service Registry & Discovery                            │ │
│  │  - State Management                                         │ │
│  └────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
                              ▼
        ┌─────────────────────────────────────────────────┐
        │            Service Microservices                    │
        └─────────────────────────────────────────────────┘
                              ▼
┌──────────┬──────────┬──────────┬──────────┬──────────┬──────────┐
│Scheduling│ Identity │ Activity │Automation│  Comms   │ License  │
│  Service   │  Service   │  Service   │  Service   │  Service   │  Service   │
└──────────┴──────────┴──────────┴──────────┴──────────┴──────────┘
     │         │          │          │          │          │
     ▼         ▼          ▼          ▼          ▼          ▼
┌──────────┬──────────┬──────────┬──────────┬──────────┬──────────┐
│ Sessions │    AD    │   User   │ Ansible  │  Email   │ License  │
│Time-based│  Sync    │   Mgmt   │Playbooks │  Slack   │Validation│
│  Access  │  LDAP    │Scripts   │  Tasks   │  Teams   │  Usage   │
│  Windows │Resources │ Workflows│  Config  │   SIEM   │ Features │
└──────────┴──────────┴──────────┴──────────┴──────────┴──────────┘
```

## Core Orchestration Engine

### Components

1. **Workflow Manager**
   - Defines and executes multi-step workflows
   - Handles dependencies between services
   - Manages compensation/rollback logic
   - Tracks workflow state and history

2. **Event Bus**
   - Asynchronous message passing between services
   - Pub/Sub pattern for loose coupling
   - Event sourcing for audit trail
   - Technology: NATS, RabbitMQ, or Kafka

3. **Service Registry**
   - Dynamic service discovery
   - Health checking
   - Load balancing
   - Technology: Consul or etcd

4. **State Management**
   - Distributed state storage
   - Transaction coordination
   - Saga pattern implementation
   - Technology: Redis or etcd

## Microservices

### 1. Scheduling Service

**Purpose:** Manage time-based access control and session scheduling

**Responsibilities:**
- Schedule future session availability windows
- Time-based access control enforcement
- Session expiration management
- Recurring access patterns
- Calendar integration
- Timezone handling

**API Endpoints:**
```
POST   /api/v1/schedules
GET    /api/v1/schedules/{id}
PUT    /api/v1/schedules/{id}
DELETE /api/v1/schedules/{id}
GET    /api/v1/schedules/active
POST   /api/v1/schedules/validate
```

**Events Published:**
- `schedule.created`
- `schedule.activated`
- `schedule.expired`
- `schedule.modified`
- `session.scheduled`
- `session.available`
- `session.unavailable`

**Data Model:**
```go
type Schedule struct {
    ID            string
    UserID        string
    TargetID      string
    StartTime     time.Time
    EndTime       time.Time
    RecurrenceRule string // iCal RRULE format
    Timezone      string
    Status        string // pending, active, expired, cancelled
    CreatedBy     string
    Metadata      map[string]interface{}
}
```

---

### 2. Identity Service (AD/LDAP Connector)

**Purpose:** Synchronize users, groups, and resources from Active Directory/LDAP

**Responsibilities:**
- AD/LDAP connection management
- User synchronization (import/update)
- Group membership sync
- Organizational unit mapping
- Attribute mapping and transformation
- Incremental sync and full sync
- Conflict resolution

**API Endpoints:**
```
POST   /api/v1/identity/sync/users
POST   /api/v1/identity/sync/groups
POST   /api/v1/identity/sync/full
GET    /api/v1/identity/sync/status
POST   /api/v1/identity/config
GET    /api/v1/identity/users/{dn}
GET    /api/v1/identity/groups/{dn}
```

**Events Published:**
- `identity.user.synced`
- `identity.group.synced`
- `identity.sync.started`
- `identity.sync.completed`
- `identity.sync.failed`

**Configuration:**
```yaml
identity:
  provider: activedirectory
  host: ldap://dc.example.com
  port: 389
  base_dn: DC=example,DC=com
  bind_dn: CN=service,OU=Users,DC=example,DC=com
  bind_password: ${SECRET}
  sync_interval: 1h
  user_filter: (objectClass=user)
  group_filter: (objectClass=group)
  attribute_map:
    username: sAMAccountName
    email: mail
    full_name: displayName
    groups: memberOf
```

---

### 3. Activity Service

**Purpose:** Execute administrative actions on users and resources

**Responsibilities:**
- User lifecycle management (create/delete/disable/enable)
- Group membership management
- Password management
- Permission assignment
- Resource provisioning
- PowerShell script execution (Windows)
- Bash script execution (Linux)
- Activity logging and audit trail

**API Endpoints:**
```
POST   /api/v1/activity/users
PUT    /api/v1/activity/users/{id}
DELETE /api/v1/activity/users/{id}
POST   /api/v1/activity/users/{id}/enable
POST   /api/v1/activity/users/{id}/disable
POST   /api/v1/activity/users/{id}/groups
DELETE /api/v1/activity/users/{id}/groups/{groupId}
POST   /api/v1/activity/scripts/powershell
POST   /api/v1/activity/scripts/bash
GET    /api/v1/activity/history
```

**Events Published:**
- `activity.user.created`
- `activity.user.updated`
- `activity.user.deleted`
- `activity.user.enabled`
- `activity.user.disabled`
- `activity.group.added`
- `activity.group.removed`
- `activity.script.executed`

**Script Execution Model:**
```go
type ScriptExecution struct {
    ID          string
    Type        string // powershell, bash
    Script      string
    Target      string // AD server, Linux host
    Parameters  map[string]interface{}
    Timeout     time.Duration
    RunAs       string
    Status      string // queued, running, completed, failed
    Output      string
    Error       string
    StartTime   time.Time
    EndTime     time.Time
    ExecutedBy  string
}
```

**Security Considerations:**
- Script approval workflow
- Privileged execution sandboxing
- Input validation and sanitization
- Audit logging of all actions
- Credential management (vaulted)

---

### 4. Automation Service (Ansible Integration)

**Purpose:** Execute infrastructure automation tasks via Ansible

**Responsibilities:**
- Ansible playbook execution
- Inventory management
- Credential vault integration
- Task scheduling
- Execution history and logging
- Configuration management
- Infrastructure provisioning
- Application deployment

**API Endpoints:**
```
POST   /api/v1/automation/playbooks
GET    /api/v1/automation/playbooks/{id}
POST   /api/v1/automation/playbooks/{id}/execute
GET    /api/v1/automation/executions/{id}
GET    /api/v1/automation/executions/{id}/logs
POST   /api/v1/automation/inventory
GET    /api/v1/automation/inventory/{id}
```

**Events Published:**
- `automation.playbook.created`
- `automation.execution.started`
- `automation.execution.completed`
- `automation.execution.failed`
- `automation.task.completed`

**Playbook Execution Model:**
```go
type PlaybookExecution struct {
    ID            string
    PlaybookID    string
    PlaybookName  string
    Inventory     string
    ExtraVars     map[string]interface{}
    Tags          []string
    SkipTags      []string
    Limit         string
    Status        string // queued, running, completed, failed
    Output        string
    FailedTasks   []string
    ChangedCount  int
    FailedCount   int
    OkCount       int
    StartTime     time.Time
    EndTime       time.Time
    ExecutedBy    string
    TriggeredBy   string // manual, scheduled, event
}
```

**Integration:**
- Ansible AWX/Tower API integration option
- Direct ansible-playbook execution
- Dynamic inventory from OpenPAM resources
- Credential injection from vault

---

### 5. Communications Service

**Purpose:** Multi-channel notification and alerting system

**Responsibilities:**
- Email notifications
- Slack messaging
- Microsoft Teams integration
- SMS alerts (via Twilio)
- Webhook notifications
- SIEM log forwarding
- Alert aggregation and routing
- Template management
- Delivery tracking

**API Endpoints:**
```
POST   /api/v1/comms/email
POST   /api/v1/comms/slack
POST   /api/v1/comms/teams
POST   /api/v1/comms/sms
POST   /api/v1/comms/webhook
POST   /api/v1/comms/siem
GET    /api/v1/comms/templates
POST   /api/v1/comms/templates
GET    /api/v1/comms/history
```

**Events Subscribed:**
- `session.started`
- `session.ended`
- `authentication.failed`
- `privilege.escalated`
- `user.created`
- `schedule.expired`
- `license.expiring`
- `security.alert`

**Notification Channels:**

1. **Email**
   - SMTP configuration
   - HTML/Plain text templates
   - Attachments support
   - Batch sending

2. **Slack**
   - Webhook integration
   - Bot API integration
   - Rich message formatting
   - Interactive buttons

3. **Microsoft Teams**
   - Webhook integration
   - Adaptive cards
   - Channel routing

4. **SIEM Integration**
   - Syslog forwarding (RFC 5424)
   - CEF (Common Event Format)
   - LEEF (Log Event Extended Format)
   - JSON format
   - Splunk HEC
   - Elastic Stack
   - Azure Sentinel

**Log Format Configuration:**
```yaml
comms:
  siem:
    enabled: true
    format: cef # cef, leef, syslog, json
    destination: syslog://siem.example.com:514
    tls: true
    protocol: tcp
    batch_size: 100
    flush_interval: 5s
    fields:
      facility: local0
      severity: info
      app_name: openpam
```

**CEF Format Example:**
```
CEF:0|OpenPAM|Gateway|1.0|SESSION_START|Privileged Session Started|5|
src=192.168.1.100 suser=john.doe dst=10.0.1.50 duser=root
proto=ssh msg=User john.doe started privileged session to root@server01
```

---

### 6. License Service

**Purpose:** License validation, enforcement, and usage tracking

**Responsibilities:**
- License key validation
- Feature flag management
- Usage tracking and limits
- Concurrent session limits
- User count enforcement
- Expiration management
- License renewal notifications
- Telemetry collection (opt-in)

**API Endpoints:**
```
POST   /api/v1/license/validate
GET    /api/v1/license/status
GET    /api/v1/license/features
GET    /api/v1/license/usage
POST   /api/v1/license/activate
POST   /api/v1/license/deactivate
GET    /api/v1/license/limits
```

**Events Published:**
- `license.validated`
- `license.expired`
- `license.expiring`
- `license.limit.reached`
- `license.feature.disabled`

**License Model:**
```go
type License struct {
    LicenseKey      string
    CustomerID      string
    CustomerName    string
    ProductEdition  string // community, professional, enterprise
    IssueDate       time.Time
    ExpirationDate  time.Time
    Features        []string
    Limits          LicenseLimits
    Status          string // active, expired, suspended, revoked
    ActivationCount int
    MaxActivations  int
}

type LicenseLimits struct {
    MaxUsers            int
    MaxConcurrentSessions int
    MaxTargets          int
    MaxZones            int
    MaxSchedules        int
    MaxIntegrations     int
}
```

**Feature Flags:**
```go
var Features = map[string]string{
    "ssh_access":           "all",
    "rdp_access":           "professional",
    "database_access":      "professional",
    "kubernetes_access":    "enterprise",
    "session_recording":    "professional",
    "activity_management":  "enterprise",
    "automation":           "enterprise",
    "ad_sync":              "professional",
    "siem_integration":     "enterprise",
    "mfa":                  "professional",
    "rbac":                 "professional",
    "audit_logs":           "all",
    "api_access":           "professional",
    "high_availability":    "enterprise",
}
```

---

## Orchestration Workflows

### Example: Scheduled User Provisioning Workflow

```yaml
workflow:
  name: scheduled_user_provisioning
  trigger: schedule.activated
  steps:
    - name: check_license
      service: license
      action: validate_user_limit
      on_failure: notify_admin

    - name: create_ad_user
      service: identity
      action: create_user
      input:
        username: ${event.user.username}
        email: ${event.user.email}
        groups: ${event.user.groups}
      on_failure: rollback

    - name: create_openpam_user
      service: activity
      action: create_user
      input:
        username: ${event.user.username}
        ad_sync: true
      depends_on: create_ad_user

    - name: grant_access
      service: activity
      action: assign_targets
      input:
        user_id: ${steps.create_openpam_user.output.id}
        targets: ${event.targets}

    - name: notify_user
      service: comms
      action: send_email
      template: user_provisioned
      input:
        to: ${event.user.email}
        username: ${event.user.username}

    - name: log_siem
      service: comms
      action: send_siem
      input:
        event_type: user_provisioned
        user: ${event.user.username}
```

### Example: Session Lifecycle with Activity Tracking

```yaml
workflow:
  name: session_lifecycle
  trigger: session.requested
  steps:
    - name: check_schedule
      service: scheduling
      action: validate_access_window
      input:
        user_id: ${event.user_id}
        target_id: ${event.target_id}
      on_failure: deny_access

    - name: check_license
      service: license
      action: check_concurrent_sessions
      on_failure: deny_access

    - name: create_session
      service: gateway
      action: establish_connection

    - name: notify_start
      service: comms
      action: send_notifications
      parallel:
        - send_slack
        - send_siem

    - name: wait_session_end
      service: gateway
      action: monitor_session

    - name: capture_activity
      service: activity
      action: log_session_commands

    - name: notify_end
      service: comms
      action: send_notifications
      parallel:
        - send_slack
        - send_siem
```

---

## Technology Stack Recommendations

### Message Queue / Event Bus
- **NATS**: Lightweight, high-performance, Go-native
- **RabbitMQ**: Mature, feature-rich, good for complex routing
- **Kafka**: Best for high-throughput, event sourcing

**Recommendation:** NATS for simplicity and performance with Go

### Service Registry
- **Consul**: Service discovery, health checking, KV store
- **etcd**: Kubernetes-native, consistent, reliable

**Recommendation:** Consul for standalone, etcd if Kubernetes-based

### State Management
- **Redis**: Fast, in-memory, pub/sub support
- **etcd**: Distributed, consistent, watch support

**Recommendation:** Redis for caching and state, etcd for coordination

### Workflow Engine Options
1. **Custom Go-based** - Full control, tailored to needs
2. **Temporal**: Durable workflow execution (more complex)
3. **Cadence**: Similar to Temporal
4. **Argo Workflows**: Kubernetes-native

**Recommendation:** Start with custom Go-based for simplicity

---

## Communication Patterns

### Synchronous (Request/Response)
- User-facing API calls
- License validation
- Schedule validation

### Asynchronous (Event-driven)
- User provisioning
- Notifications
- Audit logging
- SIEM forwarding

### Choreography vs Orchestration
- **Choreography**: Services react to events independently
  - Good for: Notifications, logging
- **Orchestration**: Central orchestrator controls flow
  - Good for: Multi-step workflows with dependencies

**Approach:** Hybrid - orchestration for complex workflows, choreography for notifications

---

## Deployment Architecture

### Microservices Structure
```
openpam/
├── gateway/              # Existing SSH/WebSocket gateway
├── orchestrator/         # Central orchestrator
│   ├── cmd/
│   │   └── orchestrator/
│   ├── internal/
│   │   ├── workflow/
│   │   ├── eventbus/
│   │   ├── registry/
│   │   └── state/
│   └── pkg/
│       └── client/
├── scheduling-service/
├── identity-service/
├── activity-service/
├── automation-service/
├── comms-service/
└── license-service/
```

### Communication Flow
```
User Request → Gateway → Orchestrator → Event Bus → Microservices
                  ↓
            License Check
                  ↓
            Schedule Check
                  ↓
          Execute Workflow
                  ↓
          Update State
                  ↓
        Notify via Comms Service
```

---

## Security Considerations

1. **Authentication & Authorization**
   - mTLS between services
   - Service-to-service authentication
   - JWT tokens for service identity

2. **Secrets Management**
   - HashiCorp Vault integration
   - Encrypted credentials
   - Automatic rotation

3. **Network Security**
   - Service mesh (optional: Istio/Linkerd)
   - Network policies
   - Zero-trust architecture

4. **Audit Logging**
   - All orchestrator decisions logged
   - Immutable audit trail
   - SIEM integration

---

## Monitoring & Observability

### Metrics
- Service health
- Workflow execution times
- Event bus throughput
- License usage
- API latency

### Logging
- Structured logging (JSON)
- Centralized log aggregation
- Correlation IDs across services

### Tracing
- Distributed tracing (Jaeger/Tempo)
- Request flow visualization
- Performance bottleneck identification

---

## Implementation Phases

### Phase 1: Foundation (Weeks 1-4)
- Orchestrator core engine
- Event bus setup (NATS)
- Service registry (Consul)
- License agent (basic)

### Phase 2: Identity & Activity (Weeks 5-8)
- Identity agent (AD/LDAP sync)
- Activity agent (user management)
- Basic PowerShell/Bash execution

### Phase 3: Scheduling & Automation (Weeks 9-12)
- Scheduling agent
- Automation agent (Ansible)
- Workflow engine enhancements

### Phase 4: Communications & SIEM (Weeks 13-16)
- Communications agent
- Email/Slack/Teams integration
- SIEM forwarding (CEF/LEEF/Syslog)

### Phase 5: Polish & Scale (Weeks 17-20)
- Performance optimization
- High availability setup
- Documentation
- Testing & hardening

---

## API Gateway Pattern

Consider adding an API Gateway in front of microservices:

```
Client → API Gateway → Orchestrator → Microservices
            ↓
       - Rate limiting
       - Authentication
       - Request routing
       - Response caching
       - API versioning
```

**Options:**
- Kong
- Traefik
- Custom Go-based

---

## Configuration Management

### Centralized Configuration
```yaml
# config/orchestrator.yaml
orchestrator:
  event_bus:
    type: nats
    url: nats://localhost:4222
  registry:
    type: consul
    url: http://localhost:8500
  state:
    type: redis
    url: redis://localhost:6379

services:
  scheduling:
    url: http://scheduling-service:8081
    health_check: /health
  identity:
    url: http://identity-service:8082
  activity:
    url: http://activity-service:8083
  automation:
    url: http://automation-service:8084
  comms:
    url: http://comms-service:8085
  license:
    url: http://license-service:8086
```

---

## Future Enhancements

1. **Machine Learning Integration**
   - Anomaly detection in session behavior
   - Predictive access scheduling
   - Risk scoring

2. **Multi-tenancy**
   - Tenant isolation
   - Per-tenant configuration
   - Resource quotas

3. **Global Distribution**
   - Multi-region deployment
   - Geo-routing
   - Data residency compliance

4. **Advanced Automation**
   - Terraform integration
   - CloudFormation support
   - Kubernetes operator

---

## References

- Event-Driven Architecture: https://martinfowler.com/articles/201701-event-driven.html
- Saga Pattern: https://microservices.io/patterns/data/saga.html
- CQRS: https://martinfowler.com/bliki/CQRS.html
- Service Mesh: https://servicemesh.io/
