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
  - `src/screens/agent/` - Agent detail and interaction screens
  - `src/screens/dashboard/` - Main dashboard
  - `src/screens/schedules/` - Schedule automation dashboard
  - `src/screens/settings/*-section.tsx` - Settings sections (operations, tmux agents, providers, keyboard shortcuts)
  - `src/screens/tmux-dashboard/` - Tmux agent discovery dashboard
  - `src/screens/workspace/` - Workspace management screens
- `src/components/` - Reusable components
- `src/app/` - Expo Router routes
  - `src/app/h/[serverId]/` - Per-host routes (agent, loops, schedules, sessions, settings, workspace, new, open-project)
  - `src/app/schedules.tsx` - Schedule entry point
  - `src/app/tmux-dashboard.tsx` - Tmux dashboard entry
  - `src/app/tmux-pane.tsx` / `tmux-pane-xterm.tsx` - Tmux pane views
  - `src/app/welcome.tsx` - Onboarding
- `src/hooks/` - Custom hooks
- `src/stores/` - Zustand state stores
  - `src/stores/tmux-agent-store.ts` - Selected tmux agent state
  - `src/stores/schedule-assistant-store.ts` - Schedule assistant thread state (session-only, keyed by serverId)
- `src/styles/` - Theme and style definitions
- `src/utils/` - Utility functions
- `src/constants/` - App constants
  - `src/constants/agent-commands.ts` - Slash-command definitions and filtering

**Notable Components**:
- `schedule-create-modal.tsx` / `schedule-edit-modal.tsx` — Schedule creation/editing modals
- `schedule-assistant/` — Schedule Assistant chat panel (message list, proposal card, composer) with `use-schedule-assist` / `use-assistant-thread` / `use-proposal-confirm` hooks
- `svg-preview.tsx` / `svg-preview.web.tsx` — SVG file preview (WebView for mobile, native for web)
- `mermaid-preview.tsx` / `mermaid-preview.web.tsx` — Mermaid diagram rendering
- `ansi-text-renderer.tsx` / `ansi-text-line.tsx` — ANSI escape sequence rendering
- `error-boundary.tsx` — React error boundary

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
| `schedule/` | Scheduler (incl. `schedule/assist` RPC schemas) |
| `tmux/` | Tmux RPC schemas and types |

### 2.4 Shared & Utils

| Directory | Responsibility |
|-----------|---------------|
| `src/shared/` | Shared types and constants |
| `src/utils/` | Utility functions |

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
    ├── loop/            # Loop automation engine (engine, store, types)
    ├── llm/             # OpenAI-compatible chat completion client (schedule assistant)
    ├── memory/          # Session memory: TurnRecorder / bridge / filebackend / redact
    ├── memorysetup/     # Assembles recorder+redactor+bridge from MemoryConfig
    ├── metrics/         # Metrics
    ├── pidlock/         # PID lock
    ├── push/            # Push notifications
    ├── relayclient/     # Relay client
    ├── schedule/        # Cron-based schedule automation (executor, store, runner, assistant*)
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
- `session_tmux.go` - Tmux subprocess management, agent scanning, pane capture, key injection
- `session_schedule.go` - Schedule message handlers and session-bound schedule state
- `session_schedule_assist.go` - `schedule/assist` handler; per-session Assistant (NL schedule parse) built lazily via `sync.Once`
- `schedule_runner.go` - Schedule execution wiring
- `session_register_handlers.go` - WebSocket handler registration (routes tmux/schedule messages)
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
- **`internal/agent/base/turn_guard.go`** — `TurnGuard` prevents duplicate/inconsistent provider turn transitions
- **`internal/agent/errors.go`** — Typed sentinel errors for provider lifecycle failures
- **`internal/agent/stall_monitor.go`** — Agent stuck-loop detection and grace-period tightening

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

### 3.6 Tmux Subsystem

**File**: `internal/server/session_tmux.go`

Features:
- **Agent scanning**: Three-layer detection (command name, pane title unicode normalization, child process inspection)
- **Pane capture**: `tmux capture-pane -p -e -S {startLine}` with configurable scrollback
- **Key injection**: `tmux send-keys -t {paneId} {keys} [Enter]`
- **Status line**: `tmux display-message -p` for status-left, status-right, and window list
- Supported agents: claude, pi, kimi, kimi-cli, opencode, qodercli, cursor, codex

### 3.7 App-Bridge Tmux Modules

**Directory**: `app-bridge/src/server/tmux/`

| File | Responsibility |
|------|---------------|
| `rpc-schemas.ts` | Zod schemas for all tmux RPC messages (list_agents, capture_pane, send_keys, new_session, get_theme) |

**DaemonClient methods** (in `app-bridge/src/client/daemon-client.ts`):
- `tmuxListAgents(hostId)` — Discover AI agent panes across tmux sessions
- `tmuxCapturePane(hostId, paneId, startLine?)` — Capture pane content with ANSI codes
- `tmuxSendKeys(hostId, paneId, keys, sendEnter?)` — Send keystrokes to a tmux pane
- `tmuxNewSession(name, options?)` — Create a new tmux session with optional working directory and command
- `tmuxGetTheme(hostId, sessionId)` — Get tmux session theme colors (legacy, now uses terminal themes)

### 3.8 App Tmux Components

| Component | File | Responsibility |
|-----------|------|---------------|
| `TmuxDashboardScreen` | `screens/tmux-dashboard/tmux-dashboard-screen.tsx` | Dashboard showing aggregated tmux agents from all hosts |
| `TmuxPaneScreen` | `screens/tmux-pane-screen.tsx` | Full-screen pane content view with ANSI rendering and input |
| `tmux-agent-store` | `stores/tmux-agent-store.ts` | Zustand store for selected agent (serverId + paneId) |
| `useAggregatedTmuxAgents` | `hooks/use-tmux-agents.ts` | Parallel useQueries across all hosts for agent discovery |
| `useTmuxCapturePane` | `hooks/use-tmux-capture-pane.ts` | Polling useQuery for pane content with foreground awareness |
| `useTmuxNewSession` | `hooks/use-tmux-new-session.ts` | Create new tmux sessions from the dashboard |
| `useTmuxTheme` | `hooks/use-tmux-theme.ts` | Query for terminal theme colors |
| `useTmuxStatusLine` | `hooks/use-tmux-status-line.ts` | Parse and render tmux status line with ANSI colors |
| `useTmuxStatusLines` | `hooks/use-tmux-status-lines.ts` | Aggregate status lines from multiple hosts |
| `ansi-text-renderer` | `components/ansi-text-renderer.tsx` | ANSI escape sequence rendering component |
| `error-boundary` | `components/error-boundary.tsx` | React error boundary wrapping tmux screens |
| `terminal-themes` | `styles/terminal-themes.ts` | 5 terminal theme presets (`system`, `dark`, `light`, `bash`, `auto`) |
| `resolve-terminal-colors` | `utils/resolve-terminal-colors.ts` | Resolve effective terminal colors from theme + content + tmux theme |
| `detect-ansi-colors` | `utils/detect-ansi-colors.ts` | 256-color palette detection from ANSI content |

### 3.9 Schedule Assistant

**Directories**: `internal/schedule/` (assistant files), `internal/llm/`

Core files:
- `internal/llm/client.go` - OpenAI-compatible chat completion client (`POST {baseURL}/chat/completions`, Bearer auth, non-streaming, 60s timeout; sentinel errors `ErrLLMAuth` / `ErrLLMRateLimited`)
- `internal/schedule/assistant.go` - Orchestration: request guards, per-connection rate limit + single-flight, one validation retry, `nextRunAt` enrichment; never mutates the schedule store
- `internal/schedule/assistant_resolve.go` - Default provider/model resolution from `config.llmProviders` (first enabled provider with baseURL+apiKey; `isDefault` model else first)
- `internal/schedule/assistant_prompt.go` - System prompt (JSON-only contract) + context block (agents/schedules ≤50 each, ~8k cap)
- `internal/schedule/assistant_extract.go` - Fenced/balanced-brace JSON extraction, per-op schema + semantic validation

See [Schedule Assistant](schedule-assistant.md).

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
- `message_agent_inbound.go` - Inbound agent messages
- `message_agent_outbound.go` - Outbound agent messages
- `message_common.go` - Shared message types
- `message_editor.go` - Editor-related messages
- `message_loop.go` - Loop automation messages
- `message_schedule.go` - Schedule messages
- `message_solo_compat.go` - Solo compatibility messages
- `message_terminal_msg.go` - Terminal messages
- `message_tmux.go` - Tmux-related messages
- `message_worktree.go` - Worktree messages
- `statemachine.go` - State machine logic
- `stream_event.go` - Streaming event types
- `terminal.go` - Terminal type definitions
- `tool_call_detail.go` - Tool call detail structures

## 7. Highlight (Shared Syntax Highlighting)

**Directory**: `packages/highlight/`

**Tech Stack**: TypeScript

**Responsibilities**:
- Shared syntax highlighting library used by the app
- Lezer-based parser support for 14+ languages
- Color theme management

**Key Files**:
- `src/highlighter.ts` - Core highlighting logic
- `src/parsers.ts` - Lezer parser definitions
- `src/colors.ts` - Color palette
- `src/types.ts` - Type definitions
- `src/__tests__/` - Unit tests

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
