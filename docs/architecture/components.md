# Component Details

## 1. App (Client Application)

**Directory**: `app/`

**Tech Stack**: React Native + Expo

**Responsibilities**:
- Provides the user interface (Web, iOS, Android)
- Communicates with Daemon via App-Bridge
- Manages user sessions and workspaces

**Key Directories**:
- `src/screens/` - Page components
- `src/components/` - Reusable components
- `src/app/` - Expo Router routes

## 2. App-Bridge (Client Communication Library)

**Directory**: `app-bridge/`

**Tech Stack**: TypeScript

**Responsibilities**:
- Encapsulates WebSocket communication details
- Supports direct connections and Relay connections
- Implements end-to-end encryption (E2EE)

**Key Modules**:

### 2.1 Client

**Directory**: `src/client/`

| File | Responsibility |
|------|---------------|
| `daemon-client.ts` | Main client, manages connection state |
| `daemon-client-websocket-transport.ts` | WebSocket transport implementation |
| `daemon-client-relay-e2ee-transport.ts` | Relay E2EE transport |
| `daemon-client-transport-types.ts` | Transport layer type definitions |
| `daemon-client-transport-utils.ts` | Transport utility functions |

### 2.2 Relay

**Directory**: `src/relay/`

| File | Responsibility |
|------|---------------|
| `e2ee.ts` | End-to-end encryption implementation |
| `encrypted-channel.ts` | Encrypted channel |
| `crypto.ts` | Encryption utilities |
| `base64.ts` | Base64 encoding utilities |

### 2.3 Server

**Directory**: `src/server/`

| Directory | Responsibility |
|-----------|---------------|
| `agent/` | Agent management |
| `chat/` | Chat functionality |
| `loop/` | Main loop |
| `schedule/` | Scheduler |

## 3. Daemon

**Directory**: `daemon/`

**Tech Stack**: Go

**Responsibilities**:
- Core service, manages all business logic
- WebSocket server
- Agent lifecycle management
- Workspace and project management

**Architecture**:

```
daemon/
├── main.go              # Entry point
└── internal/
    ├── agent/           # Agent management
    ├── config/          # Configuration (includes MemoryConfig)
    ├── memory/          # Session memory: TurnRecorder / bridge / filebackend / redact
    ├── memorysetup/     # Assembles recorder+redactor+bridge from MemoryConfig
    ├── metrics/         # Metrics
    ├── pidlock/         # PID lock
    ├── push/            # Push notifications
    ├── relayclient/     # Relay client
    ├── server/          # WebSocket server
    ├── terminal/        # Terminal management
    ├── workspace/       # Workspace management
    └── wsconn/          # WebSocket connection abstraction
```

### 3.1 Server (WebSocket Server)

**Directory**: `internal/server/`

Core files:
- `daemon.go` - Daemon main structure, service orchestration
- `session.go` - Session management
- `session_agent.go` - Agent sessions
- `session_terminal.go` - Terminal sessions
- `handler_registry.go` - Handler registry

### 3.2 Relay Client

**Directory**: `internal/relayclient/`

Core files:
- `client.go` - Relay client implementation
- `e2ee.go` - End-to-end encryption
- `e2ee_test.go` - E2EE tests

Features:
- Maintains control connection (Control Connection)
- Manages data connection (Data Connection)
- Auto-reconnection
- Keepalive heartbeat

### 3.3 Agent Manager

**Directory**: `internal/agent/`

Features:
- Agent lifecycle management
- Provider registration and discovery
- Model configuration

### 3.4 Workspace

**Directory**: `internal/workspace/`

Features:
- Workspace management
- Git integration
- Script execution

### 3.5 Session Memory

**Directory**: `internal/memory/` (assembled in `internal/memorysetup/`)

Features:
- Persists each user / assistant turn as Markdown + YAML frontmatter
- Disk path: `~/.solo/memory/sessions/{YYYY-MM-DD}/{sessionID}/turns/{seq:04d}-{role}.md`, index at `~/.solo/memory/sessions.jsonl`
- Enabled by default, opt-out via config.json `"memory": {"enabled": false}`

Core structure:
- `recorder.go` - `TurnRecorder` stable interface (Phase 1 implemented as `filebackend`)
- `filebackend/` - Async channel writer + directory layout + `sessions.jsonl`
- `redact/` - Pre-write redaction (regex / env / multi, includes OpenAI/GitHub/Anthropic/AWS default patterns)
- `bridge/` - Session→turn bridge: seq/parent chain, streaming chunk merging; `SafeBridge` provides panic recovery + circuit breaker
- `internal/server/memorybridge*.go` - Session scheduler layer hook injection

See [Session Memory Persistence](session-memory-persistence.md) and [`../product/session-memory-spec.md`](../product/session-memory-spec.md).

## 4. Relay (Relay Server)

**Directory**: `relay-go/`

**Tech Stack**: Go

**Responsibilities**:
- WebSocket connection relay
- Session management
- Message buffering
- NAT traversal support

**Architecture**:

```
relay-go/
├── cmd/relay/
│   └── main.go          # Entry point
└── internal/
    ├── config/          # Configuration
    ├── e2ee/            # End-to-end encryption
    ├── metrics/         # Metrics
    └── relay/           # Core implementation
        ├── server.go    # HTTP/WebSocket server
        ├── session.go   # Session management
        ├── session_manager.go # Session manager
        ├── control.go   # Control connection logic
        └── buffer.go    # Message buffering
```

### 4.1 Server

**File**: `internal/relay/server.go`

Features:
- HTTP server
- WebSocket upgrade
- Health check endpoint (`/health`)
- **Prometheus metrics endpoint (`/metrics`)**: sessions, connections, messages counts

### 4.2 Session

**File**: `internal/relay/session.go`

Features:
- Session state management
- Message routing
- Connection pairing

## 5. CLI (Command Line Interface)

**Directory**: `cli/`

**Tech Stack**: Go

**Responsibilities**:
- Command line interaction
- Session management
- Configuration management

## 6. Protocol (Protocol Definitions)

**Directory**: `protocol/`

**Tech Stack**: Go

**Responsibilities**:
- Defines shared protocol constants
- Message structures
- Type definitions

**Core Files**:
- `protocol.go` - Protocol constants
- `message.go` - Message types
- `message_*.go` - Various message definitions

## Component Interaction

```
┌─────────┐     ┌─────────────┐     ┌─────────┐
│   App   │◄───►│ App-Bridge  │◄───►│  Relay  │
│         │     │             │     │         │
└─────────┘     └─────────────┘     └────┬────┘
                                         │
                                    ┌────┴────┐
                                    │  Daemon │
                                    └─────────┘
```

## Data Flow

1. **User action** → App
2. **App** → App-Bridge (message encapsulation)
3. **App-Bridge** → Relay (optional, public network mode)
4. **Relay** → Daemon (message forwarding)
5. **Daemon** → Business processing → Returns result
6. **Result** → Relay → App-Bridge → App
