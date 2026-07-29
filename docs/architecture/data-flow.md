# Data Flow Documentation

## Network Topology

All remote traffic flows through the Nginx reverse proxy to the Relay, which bridges client connections to the user's Daemon:

```
Client Layer (Web / Mobile / CLI)
        │  App-Bridge (TypeScript), WebSocket + optional E2EE
        ▼
┌─────────────────────────────────────────┐
│  Nginx (Reverse Proxy)                  │
│  solo.up2ai.top:443                     │
│  - SSL termination (Let's Encrypt)      │
│  - Reverse proxy to localhost:8081      │
└───────────────────┬─────────────────────┘
                    │  WebSocket (localhost:8081)
                    ▼
┌─────────────────────────────────────────┐
│  Solo Relay Server (Go)                 │
│  127.0.0.1:8081                         │
│  - Control socket / Data socket         │
│  - Session management & message routing │
└───────────────────┬─────────────────────┘
                    │  WebSocket
                    ▼
┌─────────────────────────────────────────┐
│  Solo Daemon (Go, user machine)         │
│  127.0.0.1:17612                        │
│  - Agent / Workspace / Terminal         │
│  - Relay Client (control + data conns)  │
└─────────────────────────────────────────┘
```

Key points:

- **App → Nginx**: WSS (WebSocket over TLS) on port 443.
- **Nginx → Relay**: plain WS on `localhost:8081`; the Relay binds to `127.0.0.1` only and is not reachable from the public internet.
- **Daemon → Relay**: the Daemon dials out over WSS (port 443) via its Relay Client, so it works behind NAT.

For server details, Nginx/systemd configuration, and port access control, see [Deployment Architecture](deployment.md).

## WebSocket Message Flow

### Connection Establishment

```
Client                              Relay                              Daemon
  │                                  │                                  │
  │  1. WebSocket connection          │                                  │
  │─────────────────────────────────►│                                  │
  │                                  │                                  │
  │                                  │  2. Validate parameters           │
  │                                  │  (serverId, role, connectionId)   │
  │                                  │                                  │
  │  3. Connection confirmation       │                                  │
  │◄─────────────────────────────────│                                  │
  │                                  │                                  │
  │                                  │  4. If Server role                │
  │                                  │     wait for Client connection    │
  │                                  │                                  │
  │                                  │  5. If Client role                │
  │                                  │     match to Server session       │
  │                                  │                                  │
  │                                  │  6. Establish data channel        │
  │                                  │◄────────────────────────────────►│
```

### Message Transmission

```
┌─────────┐     ┌─────────────┐     ┌─────────┐
│  Client │◄───►│    Relay    │◄───►│  Daemon │
│         │     │             │     │         │
└────┬────┘     └──────┬──────┘     └────┬────┘
     │                 │                 │
     │  1. Send(msg)   │                 │
     │────────────────►│                 │
     │                 │  2. Forward     │
     │                 │────────────────►│
     │                 │                 │
     │                 │  3. Process     │
     │                 │                 │
     │                 │  4. Response    │
     │                 │◄────────────────│
     │                 │                 │
     │  5. Receive     │                 │
     │◄────────────────│                 │
```

## Message Types

### Control Messages

**Direction**: Daemon ↔ Relay

| Message | Description |
|---------|-------------|
| `hello` | Handshake, exchange protocol version and authentication info |
| `ping` | Heartbeat keepalive |
| `pong` | Heartbeat response |
| `attach` | Request to establish data connection |
| `detach` | Disconnect data connection |

### Data Messages

**Direction**: Client ↔ Daemon (via Relay)

| Message | Description |
|---------|-------------|
| `auth` | Authentication |
| `request` | Request |
| `response` | Response |
| `event` | Event notification |
| `error` | Error |

### Protocol Constants

Defined in `protocol/protocol.go`; these govern the handshake and session lifecycle below:

```go
const (
    WSProtocolVersion        = 1
    HelloTimeoutMs           = 15000
    SessionDisconnectGraceMs = 90000

    WSEndpoint           = "/ws"
    RelayProtocolVersion = "2"
)
```

## Session Lifecycle

### 1. Create Session

```
Client          Relay           Daemon
  │              │               │
  │── connect ──►│               │
  │              │── attach ────►│
  │              │               │
  │              │◄── accept ────│
  │◄─ connected ─│               │
```

### 2. Data Transmission

```
Client          Relay           Daemon
  │              │               │
  │── message ──►│── forward ───►│
  │              │               │
  │◄─ response ──│◄── result ────│
```

### 3. Close Session

```
Client          Relay           Daemon
  │              │               │
  │── close ────►│── detach ────►│
  │              │               │
  │◄─ closed ────│◄── ack ───────│
```

## End-to-End Encryption (E2EE) Flow

### Key Exchange

```
Client                                          Daemon
  │                                              │
  │  1. Generate ephemeral key pair (X25519)     │
  │                                              │
  │  2. Send public key (via Relay control conn) │
  │─────────────────────────────────────────────►│
  │                                              │
  │                                              │  3. Generate ephemeral key pair
  │                                              │
  │                                              │  4. Send public key
  │◄─────────────────────────────────────────────│
  │                                              │
  │  5. Compute shared secret                    │
  │     (X25519 key exchange)                    │
  │                                              │
  │                                              │  6. Compute shared secret
  │                                              │
  │  7. Derive encryption key (XSalsa20-Poly1305)│
  │                                              │
  │                                              │  8. Derive encryption key
```

### Encrypted Transmission

```
Client                      Relay                      Daemon
  │                          │                          │
  │  1. Encrypt message      │                          │
  │     (XSalsa20-Poly1305)  │                          │
  │                          │                          │
  │── ciphertext ───────────►│── forward ──────────────►│
  │                          │                          │
  │                          │                          │  2. Decrypt message
  │                          │                          │
  │                          │                          │  3. Process request
  │                          │                          │
  │                          │                          │  4. Encrypt response
  │                          │                          │
  │◄── ciphertext ──────────│◄── forward ──────────────│
  │                          │                          │
  │  5. Decrypt response     │                          │
```

## Pairing Link Flow

The Pairing Link is how a mobile/desktop client bootstraps a connection to a Daemon: it packages the network configuration and the Daemon's public key into a single URL, which then feeds directly into the E2EE handshake above.

### Generation

```
User ──► solo pair (CLI) ──► GeneratePairingOffer (cli/internal/client)
                                    │
                                    ▼
                  ConnectionOfferV2 (JSON)
                  {
                    "v": 2,
                    "serverId": "75df32ee",
                    "daemonPublicKeyB64": "LbDipkESA0+8Mzs57k0EnIW8...",
                    "relay": { "endpoint": "solo.up2ai.top:443" }
                  }
                                    │
                                    ▼  Base64URL encode
        https://solo.up2ai.top/#offer=eyJ2IjoyLCJzZXJ2ZXJJZCI6...
                                    │
                                    ▼
                          QR Code (shown in terminal)
```

| Component | Location | Responsibility |
|-----------|----------|----------------|
| CLI `pair` command | `cli/cmd/daemon_pair.go` | User entry point, reads config |
| Pairing logic | `cli/internal/client/pairing.go` | Generate/parse pairing link |
| Key management | `cli/internal/client/pairing.go` | Curve25519 key pair |
| Offer schema | `app-bridge/src/shared/connection-offer.ts` | Type definition and validation |

### Usage Sequence

```
Daemon (server)        User             Mobile App (client)
     │                  │                       │
     │  1. solo pair    │                       │
     │◄─────────────────│                       │
     │                  │                       │
     │  2. Generate Pairing Link + QR           │
     │─────────────────►│                       │
     │                  │                       │
     │                  │  3. Scan QR / paste link
     │──────────────────────────────────────────►│
     │                  │                       │  4. Parse offer
     │                  │                       │  5. Connect to Relay
     │                  │                       │  6. E2EE handshake
     │◄──────────────────────────────────────────│
     │                  │                       │
     │  7. Establish data channel               │
     │  8. Normal communication                 │
     │◄────────────────────────────────────────►│
```

### Security

1. **E2EE**: the Pairing Link carries the Daemon public key used for end-to-end encryption.
2. **ServerID validation**: uniquely identifies the Daemon and prevents MITM attacks.
3. **TLS transport**: Relay communication uses WSS (WebSocket over TLS).
4. **Key persistence**: the Daemon key pair is stored at `~/.solo/daemon-keypair.json`.

Configuration details (Relay endpoint, `app.baseUrl`, etc.) are documented in [Deployment Architecture](deployment.md#pairing-link-配置).

## Agent Message Flow

### Agent Execution Flow

```
User → App → App-Bridge → Relay → Daemon → Agent Manager → Agent Provider
                                                          │
                                                          ▼
User ← App ← App-Bridge ← Relay ← Daemon ← Agent Manager ← Agent
```

### State Change Notification

```
Agent → Agent Manager → Daemon → Relay → App-Bridge → App → UI Update
```

### Agent Stall Detection Flow (StallMonitor)

```
Agent Provider ──SSE events──► AgentManager.handleStreamEvent()
                                      │
                                      ▼
                           StallMonitor.RecordEvent()
                                      │
                     ┌─────────────────┴─────────────────┐
                     │ Scan every 30s                    │
                     ▼                                    ▼
          ┌─────────────────────┐            ┌─────────────────────┐
          │  Inactivity Check   │            │  Repetition Check   │
          │  > 2 min no events  │            │  ≥ 6 identical / 10 │
          └──────────┬──────────┘            └──────────┬──────────┘
                     │                                  │
                     └────────────────┬─────────────────┘
                                      ▼
                           StallMonitor.interruptFn()
                                      │
                                      ▼
                           AgentManager.CancelAgentRun()
                                      │
                                      ▼
                           session.Interrupt()
                                      │
                                      ▼
                           emit turn_failed / turn_canceled
                                      │
                                      ▼
                            StallMonitor.UnregisterAgent()
```

**Grace Period Tightening:**

```
Session.expireGrace()
  │
  ▼
hasRunningAgentsWithProgress()?  ← Checks both LifecycleRunning and events within last 2 min
  │
  ├─ YES → Extend grace
  │
  └─ NO  → End grace, execute fullCleanup()
```

## Schedule Assistant Flow

Natural-language schedule parse uses one correlated RPC pair (`schedule/assist`) over the standard pipeline; the daemon then calls the host's configured LLM endpoint. The LLM only proposes — mutation happens on user confirm via the existing schedule RPCs.

### Parse (schedule/assist)

```
App (ScheduleAssistantPanel)
  │
  ▼
useScheduleAssist (stores thread in schedule-assistant-store, keyed by serverId)
  │
  ▼  WebSocket (schedule/assist, 120s timeout)
App-Bridge scheduleAssist() → Relay → Daemon
  │
  ▼
handleScheduleAssist → Assistant (per-session, lazy via sync.Once)
  │
  ├─ guards (message ≤2000 chars, timezone, transcript ≤10)
  ├─ rate limit (10/min) + single-flight per connection
  ├─ resolve default provider/model from config.llmProviders
  ├─ build prompt (system contract + agents/schedules context block)
  │
  ▼  HTTPS chat completion (OpenAI-compatible, non-streaming)
Configured LLM endpoint (Settings → General → LLM Providers)
  │
  ▼
extract + validate (schema + semantic; one retry on failure)
  │
  ▼  WebSocket (schedule/assist/response)
Return kind=proposal | clarify | answer | error (+ llmProvider/model echo)
```

### Confirm (existing RPCs — unchanged)

```
ProposalCard [Confirm]
  │
  ▼  op → schedule/create | update | pause | resume | delete
  │    (cron cadence converted local→UTC via cronToUTC;
  │     update merges proposal over schedule/inspect)
Daemon schedule.Store ─ Executor ─ daemonRunner ─▶ Target Agent
  (fire-time execution unchanged: agent → running agent;
   new-agent/provider → ephemeral agent via AgentManager)
```

See [Schedule Assistant](schedule-assistant.md) for the full architecture.

## Tmux RPC Message Flow

Tmux operations follow the standard Client → App-Bridge → Relay → Daemon pipeline using correlated request/response messages.

### Agent Discovery

```
App (TmuxDashboardScreen)
  │
  ▼
useAggregatedTmuxAgents (useQueries per host)
  │
  ├──► DaemonClient.tmuxListAgents(hostA)
  ├──► DaemonClient.tmuxListAgents(hostB)
  └──► DaemonClient.tmuxListAgents(hostC)
           │
           ▼  WebSocket (tmux/list_agents)
      Relay → Daemon
           │
           ▼
      scanTmuxAgents() → tmux list-panes -a -F "..."
           │
           ▼
      parseTmuxPaneLines() → 3-layer detection
           │
           ▼  WebSocket (tmux/list_agents/response)
      Return []TmuxAgentInfo
```

### Pane Content Capture

```
App (TmuxPaneScreen)
  │
  ▼
useTmuxCapturePane(paneId, startLine?)
  │
  ▼  WebSocket (tmux/capture_pane)
Relay → Daemon
  │
  ▼
captureTmuxPane(paneID) → tmux capture-pane -t {paneId} -p -e -S {startLine}
  │
  ▼  WebSocket (tmux/capture_pane/response)
Return content string (with ANSI codes)
```

### Keystroke Injection

```
App (TmuxPaneScreen)
  │
  ▼
onSendKeys(keys, sendEnter)
  │
  ▼  WebSocket (tmux/send_keys)
Relay → Daemon
  │
  ▼
sendKeysToTmuxPane(paneID, keys, sendEnter) → tmux send-keys -t {paneId} {keys} [Enter]
  │
  ▼  WebSocket (tmux/send_keys/response)
Return success / error
```

### Status Line Query

```
App (TmuxDashboardScreen)
  │
  ▼
useTmuxStatusLine(sessionId)
  │
  ▼  WebSocket (tmux/get_status_line)
Relay → Daemon
  │
  ▼
tmux display-message -p "#{status-left}" / "#{status-right}" / window list
  │
  ▼  WebSocket (tmux/get_status_line/response)
Return parsed status line segments with ANSI codes
```

See [Tmux Pane Content Loading](tmux-pane-content-loading.md) for the complete tmux subsystem documentation.

## Push Notification Flow

```
Daemon → Expo Push Service → Apple/Google Push → Mobile App
```

## File Operation Flow

### File Browsing

```
App → App-Bridge → Relay → Daemon → File System → Response
```

### File Editing

```
App → App-Bridge → Relay → Daemon → Editor → File System → Response
```

## Terminal Session Flow

```
App → App-Bridge → Relay → Daemon → Terminal Manager → Shell → Output
```

## Heartbeat Mechanism

### Control Connection Heartbeat

```
Daemon          Relay
  │              │
  │── ping ────►│
  │              │
  │◄── pong ────│
  │              │
  │  (every 10s) │
```

### Data Connection Heartbeat

```
Client          Relay          Daemon
  │              │              │
  │── ping ────►│── forward ──►│
  │              │              │
  │◄── pong ─────│◄── forward ──│
  │              │              │
  │  (every 30s) │              │
```

## Error Handling Flow

### Connection Disconnection

```
1. Detect disconnection (timeout or network error)
2. Mark session status as disconnected
3. Attempt automatic reconnection (exponential backoff)
4. Notify user (if reconnection fails)
```

### Message Loss

```
1. Relay buffers messages (default 200)
2. Client reconnects and restores session
3. Replay buffered messages
```
