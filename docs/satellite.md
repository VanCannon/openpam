# Satellite Gateway Architecture

OpenPAM supports distributed deployments using a hub-and-spoke model. Satellite gateways in remote or isolated networks connect back to the central hub via reverse tunnels, eliminating the need for inbound firewall rules.

## Architecture Overview

```
                           Internet/WAN
                                │
┌───────────────────────────────┼───────────────────────────────┐
│ Hub Network (HQ)              │                               │
│                               │                               │
│  ┌─────────────────────┐      │      ┌──────────────────────┐│
│  │   Hub Gateway       │◄─────┴──────┤  Satellite Gateway  ││
│  │   (Primary)         │   Reverse   │  (Branch Office)    ││
│  │                     │   Tunnel    │                      ││
│  │  - EntraID Auth     │   (WSS)     │  - No Public IP     ││
│  │  - Database         │             │  - Local Targets    ││
│  │  - Vault            │             │                      ││
│  │  - Tunnel Server    │             │                      ││
│  └─────────────────────┘             └──────────────────────┘│
│           │                                     │             │
│           │                                     │             │
│    ┌──────▼──────┐                       ┌─────▼──────┐      │
│    │ Local       │                       │ Remote     │      │
│    │ Targets     │                       │ Targets    │      │
│    └─────────────┘                       └────────────┘      │
└───────────────────────────────────────────────────────────────┘
```

## Key Features

✅ **No Inbound Firewall Rules** - Satellites initiate outbound connections
✅ **Automatic Reconnection** - Resilient to network interruptions
✅ **Credential Forwarding** - Hub retrieves from Vault and forwards to satellite
✅ **Protocol Agnostic** - Works with SSH and RDP
✅ **Session Auditing** - All sessions logged centrally at hub

## Tunnel Protocol

### Message Types

**Registration:**
- `register` - Satellite → Hub: Initial registration
- `register_ack` - Hub → Satellite: Registration acknowledgment

**Connection Management:**
- `dial_request` - Hub → Satellite: Request to dial target
- `dial_response` - Satellite → Hub: Dial result
- `data` - Bidirectional: Proxied data
- `close` - Bidirectional: Close connection

**Keepalive:**
- `ping` - Hub → Satellite: Keepalive check
- `pong` - Satellite → Hub: Keepalive response

### Message Format

All messages are JSON over WebSocket:

```json
{
  "type": "dial_request",
  "connection_id": "uuid",
  "payload": {
    "target_host": "192.168.1.10",
    "target_port": 22,
    "protocol": "ssh",
    "username": "admin",
    "password": "secret"
  }
}
```

## Hub Configuration

The hub accepts satellite connections and maintains the tunnel server.

**Environment Variables:**
```bash
ZONE_TYPE=hub
ZONE_NAME=headquarters
```

**Tunnel Endpoint:**
- `WS /api/tunnel` - Satellite connection endpoint

**Hub Responsibilities:**
1. Accept satellite WebSocket connections
2. Authenticate and register satellites
3. Route connection requests to appropriate satellite
4. Forward credentials from Vault to satellite
5. Proxy data between user and satellite
6. Record audit logs

## Satellite Configuration

Satellites connect to the hub and proxy connections to local targets.

**Environment Variables:**
```bash
ZONE_TYPE=satellite
ZONE_NAME=branch-office
ZONE_ID=uuid-of-zone-from-database
HUB_ADDRESS=wss://hub.example.com/api/tunnel
```

**Satellite Responsibilities:**
1. Establish WebSocket connection to hub
2. Register with zone information
3. Accept dial requests from hub
4. Connect to local targets
5. Proxy data bidirectionally
6. Handle disconnections gracefully

## Setup Guide

### 1. Create Zone in Database

First, create a zone entry in the hub database:

```sql
INSERT INTO zones (id, name, type, description)
VALUES (
  'a1b2c3d4-e5f6-7890-abcd-ef1234567890',
  'branch-office',
  'satellite',
  'Branch office in remote location'
);
```

### 2. Configure Hub

On the hub server, ensure tunnel endpoint is enabled (done automatically).

### 3. Configure Satellite

Create `.env` file on satellite:

```bash
# Database (can use same as hub or local cache)
DB_HOST=hub.example.com
DB_PORT=5432
DB_USER=openpam
DB_PASSWORD=openpam
DB_NAME=openpam

# Vault (must point to hub's vault)
VAULT_ADDR=https://vault.hub.example.com:8200
VAULT_TOKEN=satellite-token

# Zone Configuration
ZONE_TYPE=satellite
ZONE_NAME=branch-office
ZONE_ID=a1b2c3d4-e5f6-7890-abcd-ef1234567890
HUB_ADDRESS=wss://hub.example.com/api/tunnel

# No need for EntraID config on satellite
# No need for session secrets on satellite
```

### 4. Create Targets

Create target entries that reference the satellite zone:

```sql
INSERT INTO targets (id, zone_id, name, hostname, protocol, port)
VALUES (
  gen_random_uuid(),
  'a1b2c3d4-e5f6-7890-abcd-ef1234567890',  -- satellite zone_id
  'remote-server-01',
  '10.50.1.10',  -- private IP in satellite network
  'ssh',
  22
);
```

### 5. Start Services

```bash
# On hub
make run

# On satellite
make run
```

## Connection Flow

1. **User Initiates Connection**
   - User authenticates to hub via EntraID
   - User selects target in satellite zone
   - Frontend opens WebSocket to hub

2. **Hub Routes to Satellite**
   - Hub identifies target is in satellite zone
   - Hub retrieves credentials from Vault
   - Hub sends `dial_request` to satellite with credentials

3. **Satellite Dials Target**
   - Satellite receives dial request
   - Satellite connects to local target
   - Satellite sends `dial_response` to hub

4. **Data Proxying**
   - User ↔ Hub ↔ Satellite ↔ Target
   - All data flows through established tunnels
   - Hub records session audit log

5. **Session Termination**
   - Either side sends `close` message
   - Connections cleaned up
   - Audit log updated with final stats

## Security Considerations

### Credential Handling
- ✅ Credentials retrieved by hub from Vault
- ✅ Forwarded to satellite only for active connection
- ✅ Never persisted on satellite
- ✅ Cleared from memory after use

### Network Security
- ✅ Satellite initiates outbound connection (no inbound ports)
- ✅ WebSocket over TLS (WSS) required in production
- ✅ Hub validates satellite registration
- ✅ Each zone has unique ID

### Authentication
- ⚠️ Currently satellites auto-register
- 🔧 TODO: Implement satellite authentication tokens
- 🔧 TODO: Implement certificate-based mutual TLS

## Monitoring

### Hub Monitoring

Check connected satellites:
```bash
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/satellites
```

### Satellite Logs

Satellite logs connection status:
```
[2025-01-23T20:00:00Z] INFO: Connecting to hub hub_address=wss://hub.example.com/api/tunnel
[2025-01-23T20:00:01Z] INFO: Successfully connected and registered with hub
```

## Troubleshooting

### Satellite Cannot Connect to Hub

**Symptoms:** Satellite logs "failed to connect to hub"

**Solutions:**
- Check `HUB_ADDRESS` is correct WebSocket URL (wss://)
- Verify network connectivity to hub
- Check firewall allows outbound HTTPS/WSS
- Verify hub is running and tunnel endpoint is active

### Dial Requests Failing

**Symptoms:** Hub logs "satellite failed to dial target"

**Solutions:**
- Verify target hostname/IP is reachable from satellite
- Check satellite has network access to target
- Verify target port is correct and service is running
- Check credentials are valid

### Connection Drops

**Symptoms:** Sessions disconnect unexpectedly

**Solutions:**
- Check network stability between satellite and hub
- Verify no aggressive firewalls/proxies timing out WebSocket
- Increase keepalive interval if needed
- Check satellite has sufficient resources

## Performance

### Latency
- Connection latency = User→Hub + Hub→Satellite + Satellite→Target
- Typical overhead: 50-200ms depending on satellite location
- Local targets have minimal overhead vs direct connection

### Throughput
- Limited by slowest link in chain
- WebSocket adds minimal overhead (~5%)
- No buffering delays - data proxied in real-time

### Scalability
- Hub can support 100+ satellite connections
- Each satellite can handle 50+ concurrent sessions
- Bottleneck typically network bandwidth, not CPU

## Future Enhancements

- [ ] Satellite authentication tokens
- [ ] Certificate-based mutual TLS
- [ ] Satellite health monitoring dashboard
- [ ] Automatic satellite discovery
- [ ] Load balancing across multiple satellites
- [ ] Satellite-to-satellite tunneling
- [ ] Local credential caching on satellite
- [ ] Compression for low-bandwidth links
