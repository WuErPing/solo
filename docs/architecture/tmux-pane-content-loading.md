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
- **Theme extraction**: Reading tmux session colors for faithful terminal rendering

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
│  │  list-panes  │  capture-pane  │  send-keys  │  show-options    │  │
│  └─────────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
```

### 2.2 Component Responsibilities

| Layer | Component | Responsibility |
|---|---|---|
| App | `TmuxDashboardScreen` | Displays aggregated agent list from all connected hosts |
| App | `TmuxPaneScreen` | Renders captured pane content with ANSI colors, handles input |
| App | `tmux-agent-store` | Zustand store holding selected `serverId` + `paneId` |
| App-Bridge | `useAggregatedTmuxAgents` | Parallel `useQueries` across all hosts for agent discovery |
| App-Bridge | `useTmuxCapturePane` | Polling `useQuery` for pane content with foreground awareness |
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
mergeTerminalColors(theme, contentColors, tmuxTheme)
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

## 7. Theme Color Extraction

Tmux theme colors are fetched independently from pane content via a separate query. This avoids re-fetching theme data on every 5s content poll.

```
TmuxPaneScreen mounts
        │
        ├──► useTmuxCapturePane(paneId)     (polls every 5s)
        │
        └──► useTmuxTheme(sessionId)        (staleTime: 30s)
                     │
                     ▼
             DaemonClient.tmuxGetTheme(sessionId)
                     │
                     ▼
             extractTmuxTheme(sessionID)
                     │
                     ▼
             tmux show-options -gv -t {sessionId} status-style
             tmux show-options -gv -t {sessionId} pane-border-style
             ...
                     │
                     ▼
             Return TmuxThemeColors
```

### 7.1 React Query Configuration (Theme)

| Parameter | Value | Rationale |
|---|---|---|
| `staleTime` | `30_000` ms | Theme rarely changes during a session |
| `retry` | `1` | One retry on transient errors |

## 7. Keystroke Interaction Flow

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

### 7.1 Send-Keys Command

```
tmux send-keys -t {paneId} {keys} [Enter]
```

The `sendEnter` boolean appends a literal `Enter` key to the sequence, useful for submitting commands.

## 8. Protocol Message Definitions

### 8.1 Go (protocol/message_tmux.go)

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

### 8.2 TypeScript Zod (app-bridge/src/server/tmux/rpc-schemas.ts)

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

## 9. Error Handling and Edge Cases

| Scenario | Behavior |
|---|---|
| **Pane content too large / slow** | 8-second timeout triggers `loadTimedOut` → UI shows "Pane content too large or unavailable" |
| **Send-keys failure** | `sendError` state set → toast / inline error shown → auto-cleared after 2 seconds |
| **Transport disposed during request** | `withLiveTmuxClient` catches `disposed` error → automatic retry once after reconnecting |
| **Component crash** | `ErrorBoundary` wraps both `TmuxDashboardScreen` and `TmuxPaneScreen` |
| **Host disconnects** | Agent list query for that host fails gracefully; other hosts continue to display |
| **Pane closed while viewing** | Next capture request returns error; UI shows unavailable state |
| **App backgrounded** | `useAppVisible` pauses `refetchInterval` → no polling, no battery drain |
| **Auto refresh off + user reading history** | Polling stops; no auto-scroll; manual "Refresh" button available in key row |
| **Race: old response after pane switch** | React Query key includes `paneId` → stale responses are ignored automatically |

## 10. Related Files

| File | Role |
|---|---|
| `app/src/app/tmux-dashboard.tsx` | Dashboard screen — entry point for agent discovery |
| `app/src/app/tmux-pane.tsx` | Pane screen — content rendering and input |
| `app/src/stores/tmux-agent-store.ts` | Zustand store for selected agent |
| `app/src/hooks/use-tmux-agents.ts` | `useAggregatedTmuxAgents` hook |
| `app/src/hooks/use-tmux-capture-pane.ts` | `useTmuxCapturePane` hook |
| `app/src/hooks/use-tmux-theme.ts` | `useTmuxTheme` hook |
| `app/src/utils/tmux-rpc.ts` | `withLiveTmuxClient` wrapper |
| `app-bridge/src/client/daemon-client.ts` | `DaemonClient` — `tmuxListAgents`, `tmuxCapturePane`, `tmuxSendKeys`, `tmuxGetTheme` |
| `app-bridge/src/server/tmux/rpc-schemas.ts` | Zod schemas for all tmux RPC messages |
| `daemon/internal/server/session_register_handlers.go` | WebSocket handler registration (`tmux/list_agents`, `tmux/capture_pane`, `tmux/send_keys`, `tmux/get_theme`) |
| `daemon/internal/server/session_tmux.go` | Core tmux logic: `scanTmuxAgents`, `parseTmuxPaneLines`, `captureTmuxPane`, `sendKeysToTmuxPane`, `extractTmuxTheme` |
| `protocol/message_tmux.go` | Go struct definitions for tmux protocol messages |
| `docs/analysis/tmux-transport-disposed-race.md` | Transport disposed race condition analysis |
| `docs/architecture/data-flow.md` | General WebSocket data flow documentation |
