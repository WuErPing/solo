# Tmux Pane Content Loading

> Dashboard-driven tmux agent discovery, pane content capture with ANSI rendering, periodic polling, and keystroke injection — all via the standard Client → App-Bridge → Relay → Daemon pipeline.

- Status: **Implemented**
- Author: Andy
- Created: 2026-06-05

## 1. Overview

This document describes the end-to-end flow for loading tmux pane content into the Solo mobile/web app. The system supports:

- **Agent detection**: Scanning all connected hosts for AI agent panes (Claude, OpenCode, Qoder, Pi, Cursor, Kimi, etc.)
- **Pane capture**: Streaming tmux pane content with ANSI escape sequences preserved
- **Live polling**: Automatic refresh while the pane screen is visible and the app is in the foreground
- **Key injection**: Sending keystrokes to a remote tmux pane
- **New session creation**: Create new tmux sessions directly from the dashboard
- **Non-agent pane display**: Browse and interact with non-agent tmux panes (shells, editors, etc.)
- **Command history**: Track and display recent commands sent to coding agents
- **Terminal themes**: User-selected theme presets (system, dark, light, bash, auto) for consistent terminal appearance
- **ANSI rendering**: Full ANSI color support in pane content and dashboard status lines
- **Window list**: Tmux window information displayed in dashboard status line

All tmux operations are proxied through the existing WebSocket session infrastructure (Client ↔ App-Bridge ↔ Relay ↔ Daemon) using correlated request/response messages.

## 2. Overall Architecture

### 2.1 Four-Layer Data Flow

```
┌─────────────────────────────────────────────────────────────────────┐
│  Layer 1: App (React Native)                                        │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────────┐  │
│  │ TmuxDashboard   │  │ TmuxPaneScreen  │  │  useAppVisible      │  │
│  │   (agent list)  │  │  (content view) │  │  (foreground hook)  │  │
│  └────────┬────────┘  └────────┬────────┘  └─────────────────────┘  │
│           │                    │                                      │
│  ┌────────▼────────────────────▼────────┐                            │
│  │      Zustand: tmux-agent-store       │                            │
│  │   (selected serverId + paneId)       │                            │
│  └────────┬─────────────────────────────┘                            │
└───────────┼──────────────────────────────────────────────────────────┘
            │
            ▼
┌─────────────────────────────────────────────────────────────────────┐
│  Layer 2: App-Bridge (TypeScript)                                   │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────────┐  │
│  │ useAggregated   │  │ useTmuxCapture  │  │  useTmuxTheme       │  │
│  │   TmuxAgents    │  │     Pane        │  │                     │  │
│  └────────┬────────┘  └────────┬────────┘  └──────────┬──────────┘  │
│           │                    │                       │              │
│  ┌────────▼────────────────────▼───────────────────────▼──────────┐  │
│  │              DaemonClient (WebSocket RPC)                      │  │
│  │  tmuxListAgents()  tmuxCapturePane()  tmuxSendKeys()          │  │
│  │  tmuxGetTheme()                                                │  │
│  └────────┬────────────────────────────────────────────────────────┘  │
└───────────┼──────────────────────────────────────────────────────────┘
            │  WebSocket (correlated session request / response)
            ▼
┌─────────────────────────────────────────────────────────────────────┐
│  Layer 3: Network Transport                                         │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │                    Relay Server                                │  │
│  │         (forward encrypted WS frames)                        │  │
│  └─────────────────────────┬─────────────────────────────────────┘  │
└────────────────────────────┼────────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────────┐
│  Layer 4: Daemon (Go)                                               │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────────┐  │
│  │  Session WS     │  │  scanTmuxAgents │  │ captureTmuxPane     │  │
│  │   Handlers      │  │   (3 layers)    │  │  (-p -e -S -200)    │  │
│  └────────┬────────┘  └────────┬────────┘  └──────────┬──────────┘  │
│           │                    │                       │              │
│  ┌────────▼────────────────────▼───────────────────────▼──────────┐  │
│  │                     tmux subprocess                             │  │
│  │  list-panes  │  capture-pane  │  send-keys  │  new-session  │  display-message │  │
│  └─────────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
```

### 2.2 Component Responsibilities

| Layer | Component | Responsibility |
|---|---|---|
| App | `TmuxDashboardScreen` | Displays aggregated agent list, non-agent panes, command history, and new session creation from all connected hosts |
| App | `TmuxPaneScreen` | Renders captured pane content with ANSI colors, handles input |
| App | `tmux-agent-store` | Zustand store holding selected `serverId` + `paneId` |
| App-Bridge | `useAggregatedTmuxAgents` | Parallel `useQueries` across all hosts for agent discovery |
| App-Bridge | `useTmuxCapturePane` | Polling `useQuery` for pane content with foreground awareness |
| App-Bridge | `useTmuxNewSession` | Hook for creating new tmux sessions with optional working directory and command |
| App-Bridge | `useTmuxTheme` | One-shot query for tmux session theme colors |
| App-Bridge | `DaemonClient` | WebSocket RPC client exposing typed tmux methods |
| Daemon | `session_register_handlers.go` | Routes tmux messages to handler functions |
| Daemon | `session_tmux.go` | Executes tmux subprocesses and parses output |

## 3. Agent List Scanning Flow

### 3.1 Dashboard Entry

When the user navigates to **Tmux Dashboard**, the app calls `useAggregatedTmuxAgents`, which fires parallel `useQueries` — one per connected host — to discover AI agent panes.

```
TmuxDashboardScreen
        │
        ▼
useAggregatedTmuxAgents (useQueries per host)
        │
        ├──► DaemonClient.tmuxListAgents(hostA)
        ├──► DaemonClient.tmuxListAgents(hostB)
        └──► DaemonClient.tmuxListAgents(hostC)
                 │
                 ▼
        scanTmuxAgents() on each Daemon
                 │
                 ▼
        tmux list-panes -a -F "..."
                 │
                 ▼
        parseTmuxPaneLines() ──► 3-layer detection
                 │
                 ▼
        Return []TmuxAgentInfo
```

### 3.2 Three-Layer Detection Rules

The daemon parses every tmux pane via `parseTmuxPaneLines()`. Detection proceeds through three layers until a match is found.

| Layer | Source | Logic | Example |
|---|---|---|---|
| **Layer 1** | `pane_current_command` | Exact match against known agent binaries | `claude`, `opencode`, `qodercli`, `pi`, `cursor`, `kimi`, `kimi-cli` |
| **Layer 2** | `pane_title` | Extract agent name from title, Unicode → ASCII normalization | `π` → `pi` |
| **Layer 3** | Child processes | Recursive check via `pgrep -P {panePid}` + `ps -o comm=` | Agent spawned as child of shell |

```go
// daemon/internal/server/session_tmux.go
// scanTmuxAgents executes:
tmux list-panes -a -F "#{pane_id}|#{pane_index}|#{pane_pid}|#{pane_current_command}|#{session_name}|#{window_name}|#{pane_current_path}|#{pane_title}"
```

The output is line-split and parsed into `TmuxAgentInfo` structs. Only panes that match at least one layer are returned.

### 3.3 React Query Configuration (Agent List)

| Parameter | Value | Rationale |
|---|---|---|
| `staleTime` | `30_000` ms | Agent panes change infrequently; reduce server load |
| `retry` | `1` | One retry on transient network errors |

## 4. Pane Content Capture Flow

### 4.1 Core Loading Sequence

When the user selects an agent pane, the app stores `(serverId, paneId)` in `tmux-agent-store` and navigates to `TmuxPaneScreen`. The screen mounts `useTmuxCapturePane`, which begins polling.

```
User taps agent pane
        │
        ▼
tmux-agent-store.setSelection(serverId, paneId)
        │
        ▼
Router → TmuxPaneScreen
        │
        ▼
useTmuxCapturePane(paneId)
        │
        ├── staleTime: 5s ──► cache hit? return cached
        │
        └── miss ──► DaemonClient.tmuxCapturePane(paneId)
                          │
                          ▼
                   captureTmuxPane(paneID)
                          │
                          ▼
                   tmux capture-pane -t {paneId} -p -e -S -200
                          │
                          ▼
                   Return content string (with ANSI codes)
```

### 4.2 Capture Command Parameters

```
tmux capture-pane -t {paneId} -p -e -S {startLine}
```

| Flag | Meaning |
|---|---|
| `-p` | Print captured content to stdout instead of pasting to a buffer |
| `-e` | Preserve ANSI escape sequences (colors, styles) |
| `-S {startLine}` | Start capture from this line (negative = scrollback from bottom; e.g. `-S -200` = last 200 lines) |

**Default**: `-200` (last 200 lines). When the user scrolls up and requests more history, the app sends progressively larger negative values (`-400`, `-600`, … up to `-5000`).

### 4.3 React Query Configuration (Pane Capture)

| Parameter | Value | Rationale |
|---|---|---|
| `staleTime` | `5_000` ms | Content changes actively; 5s keeps it reasonably fresh |
| `refetchInterval` | `5_000` ms | Poll every 5s while subscribed and foreground |
| `placeholderData` | `keepPreviousData` | Prevent flicker on refetch; show old content until new arrives |
| `retry` | `1` | One retry on transient failures |

### 4.4 Rendering Pipeline

Captured content flows through ANSI parsing before display:

```
Raw content (ANSI string)
        │
        ▼
parseAnsi() ──► structured spans with color/style
        │
        ▼
AnsiTextContent component
        │
        ▼
detectColorsFromAnsi() ──► extract content colors
        │
        ▼
resolveTerminalColors(theme, contentColors, tmuxTheme)
        │
        ▼
Final rendered terminal view
```

## 5. Lazy History Loading

When a user scrolls toward the top of the pane content, the app automatically loads older history lines in increments of 200.

### 5.1 Scroll-Driven Loading

```
User scrolls toward top of ScrollView
        │
        ▼
handleScroll() ──► contentOffset.y < 20?
        │
        ├── YES ──► loadMoreHistory()
        │                │
        │                ▼
        │       scrollbackLines += 200
        │                │
        │                ▼
        │       queryKey changes ──► React Query refetch
        │                │
        │                ▼
        │       DaemonClient.tmuxCapturePane(paneId, -scrollbackLines)
        │                │
        │                ▼
        │       tmux capture-pane -S -400 (or -600, -800…)
        │
        └── NO  ──► do nothing
```

### 5.2 Scrollback Limits

| Parameter | Value |
|---|---|
| Initial scrollback | `200` lines |
| Increment per load | `200` lines |
| Maximum scrollback | `5_000` lines |
| Trigger threshold | `contentOffset.y < 20` |

### 5.3 Scrollback Reset

When the user switches to a different pane (`paneId` changes), `scrollbackLines` resets to the default `200` automatically.

## 6. Automatic Polling Mechanism

### 6.1 Foreground Awareness

Polling is gated by `useAppVisible`. When the app moves to the background, the `refetchInterval` is effectively suspended (the query stops refetching). When the app returns to the foreground, polling resumes immediately.

```
App State
    │
    ├── Foreground ──► refetchInterval: 5000ms (active)
    │
    └── Background ──► pause refetch (no network, no CPU)
         │
         └── Return Foreground ──► immediate stale-while-revalidate check
```

### 6.2 Data Freshness Strategy

| Strategy | Implementation |
|---|---|
| Stale-while-revalidate | `staleTime: 5000` + `placeholderData: keepPreviousData` — user always sees content; background refresh is invisible |
| Pause on background | `useAppVisible` disables interval when app not focused |
| Quick retry | `retry: 1` handles transient WS blips without user-visible error |
| Timeout guard | `loadTimedOut` at 8s shows "Pane content too large or unavailable" |

### 6.3 Auto Refresh Toggle

Users can disable automatic polling via an **"Auto" toggle** in the header. Default is **on**.

| State | Behavior |
|---|---|
| **Auto ON** (default) | `refetchInterval: 5000ms` active; new content auto-scrolls to bottom |
| **Auto OFF** | Polling stops; auto-scroll disabled; a **"Refresh"** button appears in the key row for manual refresh |

This prevents the pane from jumping to the latest output while the user is scrolling up to read history.

## 7. Dashboard Status Line

The Tmux Dashboard displays a status line for each tmux session, rendered with ANSI color support.

### 7.1 ANSI Text Rendering in Status Line

Status line segments (`status-left`, `status-right`, window list) are parsed and rendered with proper ANSI color support via the `ansi-text-renderer.tsx` component. This preserves the visual styling intended by the tmux configuration.

### 7.2 Window List Display

The status line includes tmux window information (e.g., `0:claude*`), showing the window index, name, and active indicator. This is parsed from the tmux `display-message` output and rendered alongside the status-left and status-right segments.

### 7.3 Status Line Hooks

| Hook | File | Responsibility |
|---|---|---|
| `useTmuxStatusLine` | `use-tmux-status-line.ts` | Parse and render a single tmux session's status line |
| `useTmuxStatusLines` | `use-tmux-status-lines.ts` | Aggregate status lines from multiple hosts for the dashboard |

## 8. Terminal Themes

The tmux pane rendering uses user-selected terminal themes instead of extracting colors from the host tmux session. This decouples the app's appearance from the host's tmux configuration.

### 8.1 Theme System Overview

```
User opens Settings
        │
        ▼
Terminal Theme Picker
        │
        ├── System (default, follows OS theme)
        ├── Dark
        ├── Light
        ├── Bash (terminal-native colors)
        └── Auto (auto-detect from content)
        │
        ▼
Selected theme stored in app settings
        │
        ▼
TmuxPaneScreen uses theme for rendering
        │
        ├── Background/foreground colors
        ├── ANSI color mapping (16 colors + 256 palette)
        └── Status line colors
```

### 8.2 Theme Integration with ANSI Rendering

The rendering pipeline merges the selected terminal theme with content-detected ANSI colors:

```
Selected terminal theme (base colors)
        │
        ▼
parseAnsi() ──► structured spans with color/style
        │
        ▼
detectColorsFromAnsi() ──► extract content-specific colors
        │
        ▼
resolveTerminalColors(theme, contentColors)
        │
        ▼
Final rendered terminal view
```

### 8.3 256-Color Palette Detection

The `detect-ansi-colors.ts` utility detects 256-color ANSI sequences and maps them to the terminal theme's color palette. This enables faithful rendering of terminal applications that use extended color codes.

### 8.4 Removed: Host Tmux Theme Extraction

Previously, theme colors were fetched from the host tmux session via `tmux show-options -gv`. This approach was removed because:

- Host tmux configuration varies widely and may not match the app's design
- Theme extraction added latency to the pane loading flow
- User-selected themes provide consistent, predictable appearance

The `TmuxThemeColors` struct and `tmux/get_theme` RPC message remain defined in the protocol for backward compatibility but are no longer used by the frontend.

## 9. New Session Creation

Users can create new tmux sessions directly from the dashboard without switching to a terminal.

### 9.1 Flow

```
User taps "New" button in dashboard header
        │
        ▼
TextInput appears (session name, optional working dir / command)
        │
        ▼
User enters name + taps "Create"
        │
        ▼
useTmuxNewSession.createSession(serverId, { name, workingDir?, command? })
        │
        ▼
DaemonClient.tmuxNewSession(name, { workingDir?, command? })
        │
        ▼
tmux new-session -d -s {name} [-c {workingDir}] [{command}]
        │
        ▼
Success → close input, refresh agent list
Error → display error message inline
```

### 9.2 Daemon Implementation

```go
// daemon/internal/server/session_tmux.go
func createTmuxSession(name string, workingDir *string, command *string) error {
    args := []string{"new-session", "-d", "-s", name}
    if workingDir != nil {
        args = append(args, "-c", *workingDir)
    }
    if command != nil {
        args = append(args, *command)
    }
    // exec with 10s timeout
}
```

### 9.3 Error Handling

| Scenario | Behavior |
|---|---|
| Duplicate session name | tmux returns error, displayed inline in the input row |
| Invalid working directory | tmux returns error, displayed inline |
| Network timeout | 10-second client-side timeout via `sendCorrelatedSessionRequest` |

## 10. Non-Agent Pane Display

The dashboard shows non-agent tmux panes (shells, editors, etc.) in a separate "Other Panes" tab, grouped by command name.

### 10.1 Pane Grouping

Non-agent panes are grouped by `currentCmd` (e.g., `bash`, `vim`, `htop`). Users can filter by command name using a dropdown selector.

### 10.2 Interaction

Non-agent panes support the same interaction as agent panes:
- Tap to view pane content (navigates to TmuxPaneScreen)
- Session badge showing session name, window, and pane ID
- Working directory display

## 11. Command History

The dashboard tracks commands sent to coding agents and displays them in a "History" tab.

### 11.1 Data Source

Command history is derived from the pane content capture — the daemon parses recent terminal output to extract command entries.

### 11.2 Display

Each history entry shows the command text and timestamp. Tapping an entry navigates to the corresponding agent pane.

## 12. Slash-Command Filtering

When the user types `/` in the `TmuxPaneScreen` input field, the app offers context-aware slash commands for the selected agent. The command list is defined in `app/src/constants/agent-commands.ts` and filtered by `filterSlashCommands(agentName, input)`.

```ts
// app/src/constants/agent-commands.ts
export interface AgentCommand {
  label: string;
  command: string;
}

export const AGENT_COMMANDS: Record<string, AgentCommand[]> = {
  claude: [
    { label: "compact", command: "/compact" },
    { label: "clear",   command: "/clear" },
    { label: "help",    command: "/help" },
    // ...
  ],
};

export function filterSlashCommands(
  agentName: string,
  input: string,
): AgentCommand[] {
  if (!input.startsWith("/")) return [];
  const query = input.slice(1).toLowerCase();
  const commands = AGENT_COMMANDS[agentName];
  if (!commands) return [];
  if (!query) return commands;
  return commands.filter((c) => c.label.startsWith(query));
}
```

Tapping a suggested command inserts its `command` string into the input field, ready to be sent via `tmuxSendKeys`.

## 13. Keystroke Interaction Flow

Users can type into an input field and send keystrokes to the remote tmux pane.

```
User types text + taps Send (or Enter)
        │
        ▼
TmuxPaneScreen.onSendKeys(keys, sendEnter)
        │
        ▼
DaemonClient.tmuxSendKeys(paneId, keys, sendEnter)
        │
        ▼
sendKeysToTmuxPane(paneID, keys, sendEnter)
        │
        ▼
tmux send-keys -t {paneId} {keys} [Enter]
        │
        ▼
Return success / error
        │
        ▼
On error: setSendError(msg) → auto-clear after 2s
```

### 10.1 Send-Keys Command

```
tmux send-keys -t {paneId} {keys} [Enter]
```

The `sendEnter` boolean appends a literal `Enter` key to the sequence, useful for submitting commands.

## 14. Protocol Message Definitions

### 11.1 Go (protocol/message_tmux.go)

```go
// Agent metadata
type TmuxAgentInfo struct {
    SessionName string `json:"sessionName"`
    WindowName  string `json:"windowName"`
    PaneID      string `json:"paneId"`
    PaneIndex   int    `json:"paneIndex"`
    PanePID     int    `json:"panePid"`
    AgentName   string `json:"agentName"`
    CurrentCmd  string `json:"currentCmd"`
    WorkingDir  string `json:"workingDir"`
}

// List agents
type TmuxListAgentsRequest  struct { Type string; RequestID string }
type TmuxListAgentsResponse struct { Type string; Payload TmuxListAgentsResponsePayload }
type TmuxListAgentsResponsePayload struct {
    RequestID string          `json:"requestId"`
    Agents    []TmuxAgentInfo `json:"agents"`
    Error     *string         `json:"error"`
}

// Capture pane
type TmuxCapturePaneRequest  struct { Type string; PaneID string; StartLine *int `json:"startLine,omitempty"`; RequestID string }
type TmuxCapturePaneResponse struct { Type string; Payload TmuxCapturePaneResponsePayload }
type TmuxCapturePaneResponsePayload struct {
    RequestID string  `json:"requestId"`
    Content   string  `json:"content"`
    Error     *string `json:"error"`
}

// Send keys
type TmuxSendKeysRequest  struct { Type string; PaneID string; Keys string; SendEnter *bool; RequestID string }
type TmuxSendKeysResponse struct { Type string; Payload TmuxSendKeysResponsePayload }
type TmuxSendKeysResponsePayload struct {
    RequestID string  `json:"requestId"`
    Error     *string `json:"error"`
}

// New session
type TmuxNewSessionRequest struct {
    Type       string  `json:"type"`
    Name       string  `json:"name"`
    WorkingDir *string `json:"workingDir,omitempty"`
    Command    *string `json:"command,omitempty"`
    RequestID  string  `json:"requestId"`
}
type TmuxNewSessionResponse struct { Type string; Payload TmuxNewSessionResponsePayload }
type TmuxNewSessionResponsePayload struct {
    RequestID   string  `json:"requestId"`
    SessionName string  `json:"sessionName"`
    Error       *string `json:"error"`
}

// Theme colors
type TmuxThemeColors struct {
    Background            string `json:"background"`
    Foreground            string `json:"foreground"`
    PaneActiveBorder      string `json:"paneActiveBorder,omitempty"`
    PaneInactiveBorder    string `json:"paneInactiveBorder,omitempty"`
    StatusBackground      string `json:"statusBackground,omitempty"`
    StatusForeground      string `json:"statusForeground,omitempty"`
    MessageBackground     string `json:"messageBackground,omitempty"`
    MessageForeground     string `json:"messageForeground,omitempty"`
    WindowStatusCurrentBg string `json:"windowStatusCurrentBg,omitempty"`
    WindowStatusCurrentFg string `json:"windowStatusCurrentFg,omitempty"`
}

type TmuxGetThemeRequest  struct { Type string; SessionID string; RequestID string }
type TmuxGetThemeResponse struct { Type string; Payload TmuxGetThemeResponsePayload }
type TmuxGetThemeResponsePayload struct {
    RequestID string          `json:"requestId"`
    Theme     TmuxThemeColors `json:"theme"`
    Error     *string         `json:"error"`
}
```

### 11.2 TypeScript Zod (app-bridge/src/server/tmux/rpc-schemas.ts)

```typescript
export const TmuxAgentInfoSchema = z.object({
  sessionName: z.string(),
  windowName: z.string(),
  paneId: z.string(),
  paneIndex: z.number().int(),
  panePid: z.number().int(),
  agentName: z.string(),
  currentCmd: z.string(),
  workingDir: z.string(),
});

export const TmuxListAgentsRequestSchema = z.object({
  type: z.literal("tmux/list_agents"),
  requestId: z.string(),
});

export const TmuxListAgentsResponseSchema = z.object({
  type: z.literal("tmux/list_agents/response"),
  payload: z.object({
    requestId: z.string(),
    agents: z.array(TmuxAgentInfoSchema),
    error: z.string().nullable(),
  }),
});

export const TmuxCapturePaneRequestSchema = z.object({
  type: z.literal("tmux/capture_pane"),
  paneId: z.string(),
  startLine: z.number().int().optional(),
  requestId: z.string(),
});

export const TmuxCapturePaneResponseSchema = z.object({
  type: z.literal("tmux/capture_pane/response"),
  payload: z.object({
    requestId: z.string(),
    content: z.string(),
    error: z.string().nullable(),
  }),
});

export const TmuxSendKeysRequestSchema = z.object({
  type: z.literal("tmux/send_keys"),
  paneId: z.string(),
  keys: z.string(),
  sendEnter: z.boolean().optional(),
  requestId: z.string(),
});

export const TmuxSendKeysResponseSchema = z.object({
  type: z.literal("tmux/send_keys/response"),
  payload: z.object({
    requestId: z.string(),
    error: z.string().nullable(),
  }),
});

export const TmuxNewSessionRequestSchema = z.object({
  type: z.literal("tmux/new_session"),
  name: z.string(),
  workingDir: z.string().optional(),
  command: z.string().optional(),
  requestId: z.string(),
});

export const TmuxNewSessionResponseSchema = z.object({
  type: z.literal("tmux/new_session/response"),
  payload: z.object({
    requestId: z.string(),
    sessionName: z.string(),
    error: z.string().nullable(),
  }),
});

export const TmuxThemeColorsSchema = z.object({
  background: z.string(),
  foreground: z.string(),
  paneActiveBorder: z.string().optional(),
  paneInactiveBorder: z.string().optional(),
  statusBackground: z.string().optional(),
  statusForeground: z.string().optional(),
  messageBackground: z.string().optional(),
  messageForeground: z.string().optional(),
  windowStatusCurrentBg: z.string().optional(),
  windowStatusCurrentFg: z.string().optional(),
});

export const TmuxGetThemeRequestSchema = z.object({
  type: z.literal("tmux/get_theme"),
  sessionId: z.string(),
  requestId: z.string(),
});

export const TmuxGetThemeResponseSchema = z.object({
  type: z.literal("tmux/get_theme/response"),
  payload: z.object({
    requestId: z.string(),
    theme: TmuxThemeColorsSchema,
    error: z.string().nullable(),
  }),
});
```

## 15. Error Handling and Edge Cases

| Scenario | Behavior |
|---|---|
| **Pane content too large / slow** | 8-second timeout triggers `loadTimedOut` → UI shows "Pane content too large or unavailable" |
| **Send-keys failure** | `sendError` state set → toast / inline error shown → auto-cleared after 2 seconds |
| **Transport disposed during request** | `withLiveTmuxClient` catches `disposed` error → automatic retry once after reconnecting |
| **Component crash** | `ErrorBoundary` wraps both `TmuxDashboardScreen` and `TmuxPaneScreen` |
| **Host disconnects** | Agent list query for that host fails gracefully; other hosts continue to display |
| **Pane closed while viewing** | Next capture request returns error; UI shows unavailable state |
| **Tmux operation failure** | User-friendly error messages with actionable recovery suggestions (e.g., "tmux not installed", "session not found") instead of raw error strings |
| **App backgrounded** | `useAppVisible` pauses `refetchInterval` → no polling, no battery drain |
| **Auto refresh off + user reading history** | Polling stops; no auto-scroll; manual "Refresh" button available in key row |
| **Race: old response after pane switch** | React Query key includes `paneId` → stale responses are ignored automatically |

## 16. Related Files

| File | Role |
|---|---|
| `app/src/app/tmux-dashboard.tsx` | Dashboard screen — entry point for agent discovery |
| `app/src/app/tmux-pane.tsx` | Pane screen — content rendering and input |
| `app/src/stores/tmux-agent-store.ts` | Zustand store for selected agent |
| `app/src/hooks/use-tmux-agents.ts` | `useAggregatedTmuxAgents` hook |
| `app/src/hooks/use-tmux-capture-pane.ts` | `useTmuxCapturePane` hook |
| `app/src/hooks/use-tmux-theme.ts` | `useTmuxTheme` hook |
| `app/src/hooks/use-tmux-status-line.ts` | `useTmuxStatusLine` hook — parse and render tmux status line |
| `app/src/hooks/use-tmux-status-lines.ts` | `useTmuxStatusLines` hook — aggregate status lines from multiple hosts |
| `app/src/hooks/use-tmux-new-session.ts` | `useTmuxNewSession` hook — create new tmux sessions from the dashboard |
| `app/src/styles/terminal-themes.ts` | 5 terminal theme presets (`system`, `dark`, `light`, `bash`, `auto`) |
| `app/src/components/ansi-text-renderer.tsx` / `ansi-text-line.tsx` | ANSI escape sequence rendering components |
| `app/src/components/error-boundary.tsx` | React error boundary for crash protection |
| `app/src/utils/resolve-terminal-colors.ts` | Resolve effective terminal colors from theme + content + tmux theme |
| `app/src/utils/detect-ansi-colors.ts` | 256-color palette detection from ANSI content |
| `app/src/utils/tmux-rpc.ts` | `withLiveTmuxClient` wrapper |
| `app/src/constants/agent-commands.ts` | Slash-command definitions and `filterSlashCommands` |
| `app-bridge/src/client/daemon-client.ts` | `DaemonClient` — `tmuxListAgents`, `tmuxCapturePane`, `tmuxSendKeys`, `tmuxNewSession`, `tmuxGetTheme` |
| `app-bridge/src/server/tmux/rpc-schemas.ts` | Zod schemas for all tmux RPC messages (including `TmuxNewSessionRequestSchema`, `TmuxNewSessionResponseSchema`) |
| `daemon/internal/server/session_register_handlers.go` | WebSocket handler registration (`tmux/list_agents`, `tmux/capture_pane`, `tmux/send_keys`, `tmux/new_session`, `tmux/get_theme`) |
| `daemon/internal/server/session_tmux.go` | Core tmux logic: `scanTmuxAgents`, `parseTmuxPaneLines`, `captureTmuxPane`, `sendKeysToTmuxPane`, `createTmuxSession`, `extractTmuxTheme` |
| `protocol/message_tmux.go` | Go struct definitions for tmux protocol messages |
| `docs/analysis/tmux-transport-disposed-race.md` | Transport disposed race condition analysis |
| `docs/architecture/data-flow.md` | General WebSocket data flow documentation |
