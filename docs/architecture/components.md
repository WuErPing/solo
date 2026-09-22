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
  - `src/screens/usage/` - Usage / quota dashboard screens
  - `src/screens/workspace/` - Workspace management screens
- `src/components/` - Reusable components
- `src/app/` - Expo Router routes
  - `src/app/h/[serverId]/` - Per-host routes (agent, dashboard, loops, schedules, sessions, settings, tmux-dashboard, usage, workspace, new, open-project)
  - `src/app/schedules.tsx` - Schedule entry point
  - `src/app/usage.tsx` - Usage dashboard entry
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
| `usage/` | Usage quota RPC schemas (`usage/quota/list`) |
| `version/` | Daemon version RPC schemas (`list_daemon_versions_request`, `switch_daemon_version_request`) |

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
    ├── httpx/           # Shared HTTP clients with sane timeouts
    ├── loop/            # Loop automation engine (engine, store, types, templates, instance grouping)
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
- `session_tmux.go` - Tmux message handlers, split across `session_tmux_scan.go` (agent scanning), `session_tmux_pane.go` (pane capture), `session_tmux_session.go` (key injection, session creation, status line); `tmux_watcher.go` pushes `tmux/pane_changed` notifications
- `session_schedule.go` - Schedule message handlers and session-bound schedule state
- `session_schedule_assist.go` - `schedule/assist` handler; per-session Assistant (NL schedule parse) built lazily via `sync.Once`
- `session_usage.go` - `usage/quota/list` handler; process-wide quota cache (60s TTL, singleflight) over the shared `usage` module
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
- Enabled by default; note: the `config.json` opt-out (`"memory": {"enabled": false}`) is **not currently wired** — `PersistedConfig` only has `daemon`/`app` keys, so memory cannot be disabled via the config file today (tracked as [DEBT-001](../debt/debt-001-memory-config-opt-out.md), target v0.13.0)

Core structure:
- `recorder.go` - `TurnRecorder` stable interface (Phase 1 implemented as `filebackend`)
- `filebackend/` - Async channel writer + directory layout + `sessions.jsonl`
- `redact/` - Pre-write redaction (regex / env / multi, includes OpenAI/GitHub/Anthropic/AWS default patterns)
- `bridge/` - Session→turn bridge: seq/parent chain, streaming chunk merging; `SafeBridge` provides panic recovery + circuit breaker
- `internal/server/memorybridge.go` / `memory_wiring.go` - Session scheduler layer hook injection

See [Session Memory Persistence](session-memory-persistence.md).

### 3.6 Tmux Subsystem

**Files**: `internal/server/session_tmux*.go` + `internal/server/tmux_watcher.go`

Features:
- **Agent scanning** (`session_tmux_scan.go`): Three-layer detection (command name, pane title unicode normalization, child process inspection)
- **Pane capture** (`session_tmux_pane.go`): `tmux capture-pane -t {paneId} -p -e -J -S {startLine}` with configurable scrollback (optional `-C {cols}` width cropping)
- **Key injection** (`session_tmux_session.go`): `tmux send-keys -t {paneId} {keys} [Enter]`
- **Status line** (`session_tmux_session.go`): `tmux show-options -gv` + `display-message -p` for status-left/status-right, `list-windows` for the window list
- **Push refresh** (`tmux_watcher.go`): server-level pane-activity poller broadcasting `tmux/pane_changed`
- Supported agents (built-in, `config.builtInTmuxAgentNames`): claude, opencode, qodercli, pi, cursor, kimi, kimi-cli, codex

### 3.7 App-Bridge Tmux Modules

**Directory**: `app-bridge/src/server/tmux/`

| File | Responsibility |
|------|---------------|
| `rpc-schemas.ts` | Zod schemas for all tmux RPC messages (list_agents, capture_pane, send_keys, get_theme, status_line, new_session, kill_session, delete_command_history, pane_changed) |

**Tmux RPC methods** (canonical location: `TerminalRpc` in `app-bridge/src/client/terminal-rpc.ts`, reached via `client.terminal.*`; the flat `DaemonClient.tmux*` wrappers still exist but are deprecated). Each app↔daemon connection is per-host, so methods take no `hostId`:
- `tmuxListAgents()` — Discover AI agent panes (also returns other panes, command history, input history)
- `tmuxCapturePane(paneId, startLine?, lastContentHash?, cols?)` — Capture pane content with ANSI codes (hash-based skip when unchanged)
- `tmuxSendKeys(paneId, keys, sendEnter?)` — Send keystrokes to a tmux pane
- `tmuxStatusLine(sessionId)` — Get parsed status line segments (`tmux/status_line`)
- `tmuxNewSession(name, options?)` — Create a new tmux session with optional working directory and command
- `tmuxKillSession(sessionName)` — Kill a tmux session
- `tmuxDeleteCommandHistory(launchCmd)` — Delete recorded command history for a launch command

Note: the `tmux/get_theme` types remain in `protocol/message_tmux.go` and `rpc-schemas.ts` for backward compatibility, but the daemon no longer registers a handler and no client method exists (see [tmux-pane-content-loading.md](tmux-pane-content-loading.md) §8.4).

### 3.8 App Tmux Components

| Component | File | Responsibility |
|-----------|------|---------------|
| `TmuxDashboardScreen` | `screens/tmux-dashboard/tmux-dashboard-screen.tsx` | Dashboard showing aggregated tmux agents from all hosts |
| `TmuxPaneScreen` | `screens/tmux-pane-screen.tsx` | Full-screen pane content view with ANSI rendering and input |
| `tmux-agent-store` | `stores/tmux-agent-store.ts` | Zustand store for selected agent (serverId + paneId) |
| `useAggregatedTmuxAgents` | `hooks/use-tmux-agents.ts` | Parallel useQueries across all hosts for agent discovery |
| `useTmuxCapturePane` | `hooks/use-tmux-capture-pane.ts` | Polling useQuery for pane content with foreground awareness |
| `useTmuxNewSession` | `hooks/use-tmux-new-session.ts` | Create new tmux sessions from the dashboard |
| `useTmuxStatusLines` | `hooks/use-tmux-status-lines.ts` | Aggregate status lines from multiple hosts (per-session queries via `client.terminal.tmuxStatusLine`) |
| `ansi-text-renderer` | `components/ansi-text-renderer.tsx` | ANSI escape sequence rendering component |
| `error-boundary` | `components/error-boundary.tsx` | React error boundary wrapping tmux screens |
| `terminal-themes` | `styles/terminal-themes.ts` | 5 terminal theme presets (`system`, `dark`, `light`, `bash`, `auto`) |
| `resolve-terminal-colors` | `utils/resolve-terminal-colors.ts` | Resolve effective terminal colors from theme preset + content-detected colors |
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

### 3.10 Usage Subsystem

**File**: `internal/server/session_usage.go` (reuses the top-level [`usage/`](#8-usage-usage-tracking-cli--module) module)

Features:
- Blank-imports the kimi / deepseek / qoder / xiaomimimo providers to register them
- Serves `usage/quota/list` RPC; responds with `usage/quota/list/response` (snapshots + per-provider errors + `cachedAt`)
- Process-wide cache with 60s TTL, `singleflight` coalescing of concurrent refreshes, 15s per-fetch timeout
- Missing `~/.solo/usage.json` or zero enabled providers is not an error (returns an empty snapshot set)

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
- `process_contract.go` - Daemon↔supervisor exit-code contract (`ExitCodeRestartRequested = 42`)
- `message.go` - Message types
- `message_agent_inbound.go` - Inbound agent messages
- `message_agent_outbound.go` - Outbound agent messages
- `message_common.go` - Shared message types
- `message_editor.go` - Editor-related messages
- `message_git.go` - Git operation messages
- `message_loop.go` - Loop automation messages
- `message_schedule.go` - Schedule messages
- `message_schedule_assist.go` - Schedule assistant (`schedule/assist`) messages
- `message_solo_compat.go` - Solo compatibility messages
- `message_terminal_msg.go` - Terminal messages
- `message_tmux.go` - Tmux-related messages
- `message_tmux_notify.go` - Tmux server-push notification (`tmux/pane_changed`)
- `message_usage.go` - Usage quota messages (`usage/quota/list` request/response, snapshots)
- `message_version.go` - Daemon version messages (`list_daemon_versions_request`, `switch_daemon_version_request`)
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

## 8. Usage (Usage Tracking CLI & Module)

**Directory**: `usage/`

**Tech Stack**: Go (standalone module, `spf13/cobra`)

**Responsibilities**:
- Fetch plan usage / quota / credit balance from multiple AI coding platforms
- Provide the `solo-usage` CLI (`init`, `fetch`, `providers`)
- Export `config` / `provider` packages reused by the daemon's usage subsystem

**Architecture**:

```
usage/
├── main.go              # Entry point (calls cmd.Execute)
├── cmd/                 # Cobra commands: root, init, fetch, providers (+ provider imports)
├── config/              # usage.json load + ${VAR} / ${file:/path} placeholder expansion
├── internal/output/     # Table / JSON rendering
└── provider/            # Provider interface, registry, and implementations
    ├── kimi/            # API key — weekly usage, per-window limits
    ├── deepseek/        # API key — remaining balance
    ├── qoder/           # Org OpenAPI (apiKey) or personal cookie
    └── xiaomimimo/      # Session cookie — Token Plan usage
```

**Config**: `~/.solo/usage.json` (created via `solo-usage init`, mode `0600`). Values support `${VAR}` env and `${file:/path}` file placeholders for rotating secrets. See [Configuration](../configuration.md#usagejson--usagequota-providers).

**Notes**: Darwin-only build target (`make darwin`); excluded from `make linux`. Included in the CI Go test matrix.

## 9. Supervisor (Daemon Watchdog)

**Directory**: `supervisor/`

**Tech Stack**: Go (standalone module)

**Responsibilities**:
- Spawn/respawn the daemon and enforce the exit-code contract: `42` = restart requested, `0` = clean stop, anything else = crash (backoff + circuit breaker)
- Crash fallback: blacklists a repeatedly failing build and rewrites the `~/.solo/versions/current` pointer to the last working version
- Owns `~/.solo/solo.pid` and `~/.solo/logs/daemon.log` when the daemon runs supervised

**Core Files**:
- `supervisor/main.go` - Entry point
- `supervisor/internal/supervisor/supervisor.go` - Supervisor logic (spawn loop, backoff, crash breaker)
- `supervisor/internal/supervisor/state.go` - State file writer (`~/.solo/supervisor-state.json`, observability only)
- `protocol/process_contract.go` - Shared exit-code contract (`ExitCodeRestartRequested = 42`)

See [Daemon Supervision](daemon-supervision.md), [ADR-003](../decisions/adr-003-supervisor-exit-code-restart-contract.md) and [ADR-005](../decisions/adr-005-supervisor-crash-fallback.md).

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
