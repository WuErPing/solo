# Data Flow Documentation

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
