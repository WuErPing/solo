# Tmux Discovery & Refresh Mechanism Analysis

Date: 2026-07-24
Scope: daemon (`daemon/internal/server/`), app-bridge (`app-bridge/src/`), app (`app/src/`)
Status: Findings verified against source; fixes for #1–#5 implemented (see "Resolution").

---

## 1. Discovery Pipeline (发现机制)

### 1.1 Daemon — request-driven only, no background scanner

All discovery happens synchronously inside the `tmux/list_agents` request handler.
There is **no periodic scanner/collector loop** in the daemon (only WebSocket ping
tickers exist: `session.go:451-461`).

- Handler registration: `daemon/internal/server/session_register_handlers.go:121-128`
  registers 7 handlers: `tmux/list_agents`, `tmux/capture_pane`, `tmux/send_keys`,
  `tmux/new_session`, `tmux/kill_session`, `tmux/delete_command_history`,
  `tmux/status_line`.
- `tmux/get_theme` has protocol registration (`protocol/message.go:188`) and zod
  schemas (`app-bridge/src/server/tmux/rpc-schemas.ts:106-119`) but **no daemon
  handler** — dead path.
- Agent-name set: `s.cfg.GetTmuxAgentNames()` (built-ins at
  `daemon/internal/config/config.go:38-40`: `claude, opencode, qodercli, pi, cursor,
  kimi, kimi-cli, codex`, merged with user config).

### 1.2 tmux CLI commands executed per `list_agents` request

| Command | Purpose | Where |
|---|---|---|
| `tmux list-panes -a -F "#{pane_id}\|…\|#{window_activity}"` | pane inventory (5s timeout) | `session_tmux_scan.go:197-208` |
| `tmux capture-pane -t %N -p -e -J -S -50` | title/prompt extraction, per agent pane | `session_tmux_scan.go:226-239` |
| `tmux capture-pane … -S -10` | activity hash, per agent pane | `session_tmux_scan.go:144` |
| `ps -eo pid,ppid,args=` | process snapshot (2s timeout) | `session_tmux_scan.go:359-384` |
| `tmux capture-pane -S {start}` + `display-message #{pane_width}` | capture requests, with ANSI-aware rewrap | `session_tmux_pane.go:55-86` |
| `show-options` + `display-message`×2 + `list-windows` | status line (5 subprocess calls) | `session_tmux_session.go:49-106` |

### 1.3 Agent detection — actually 4 layers

`parseTmuxPaneLines`, `session_tmux_scan.go:256-345` (docs claim 3):

1. `pane_current_command` exact/version-suffix match — `matchAgentCommand` `:38-58`.
2. Pane-title match for non-shell commands, Unicode→ASCII normalization (π→pi) —
   `agentNameFromTitle` `:90-107`.
3. Descendant-process DFS over a `ps` snapshot, incl. argv token matching for
   wrappers (`node /path/kimi`) — `descendantProcesses` `:388-403`,
   `argsContainsAgentName` `:412-423`.
4. Title-only match when command is a shell → marks `status="exited"`.

### 1.4 Content-hash change detection — two mechanisms

- **Per-request dedup**: `handleTmuxCapturePane` computes a 16-char SHA-256 prefix
  (`computeContentHash`, `session_tmux_pane.go:50-53`) and returns `changed=false`
  with empty content when the client's `LastContentHash` matches
  (`session_tmux.go:132-139`). Concurrent identical captures coalesced via
  process-wide singleflight (`capturePaneFlight`, `session_tmux.go:12,114-125`).
- **Per-scan activity detection**: `detectAgentActivity` (`session_tmux_scan.go:127-171`)
  compares consecutive scan hashes stored in **per-WS-Session** maps
  `paneContentHashes` / `paneLastContentChange` (`session.go:71-73,178`) to derive
  busy/idle. `filterWindowActivity` (`:175-195`) suppresses <3s status-bar redraw noise.

### 1.5 App-bridge — pure request/response, zero push

- RPC client: `app-bridge/src/client/terminal-rpc.ts:551-643` (`tmuxListAgents` 10s
  timeout, `tmuxCapturePane`, `tmuxSendKeys`, `tmuxStatusLine`, `tmuxNewSession`,
  `tmuxKillSession`, `tmuxDeleteCommandHistory`).
- **No WS push/subscription events exist for tmux** — contrast with the Solo-agent
  subsystem which broadcasts `agent_update`
  (`daemon/internal/server/session_agent.go:47-59`). Tmux is 100% pull.

### 1.6 App-side consumption

- `app/src/hooks/use-tmux-agents.ts:66-217` — `useQueries` per host, key
  `["tmux-agents", serverId]`; adaptive `refetchInterval` gated on app visibility;
  no `staleTime` override → inherits global `staleTime: Infinity`
  (`app/src/query/query-client.ts:6`).
- `app/src/hooks/use-tmux-capture-pane.ts` — adaptive tiers 500ms/1s/5s (2s/10s
  phase thresholds, `:16-28`); change detection via data-reference identity tracked
  in a WeakMap keyed by the RQ query object (`:35-48`); lazy history 200→5000 lines
  (`:181-184`); foreground-edge refetch (`:172-179`); `staleTime: 5000`.
- `app/src/hooks/use-tmux-status-lines.ts:34-58` — per-session status line,
  `staleTime: 10000`, **no refetchInterval, no invalidation anywhere**.
- Dashboard `app/src/screens/tmux-dashboard/tmux-dashboard-screen.tsx` —
  pull-to-refresh (`:644-646`), mount refresh (`:374-376`), `refreshAll()` after
  mutations (`:389,447-449,463-465,477-479`).
- Pane screens poll via `useTmuxCapturePane`; manual `refetch()` after send-keys.
  Xterm screen pushes polled snapshots into `TerminalEmulator` (`:236`) which
  clears + rewrites the whole grid on change — snapshot-polled, **not streamed**.
- Sidebar: `app/src/components/left-sidebar.tsx:213` runs `useTmuxProjectCounts`
  → `useAggregatedTmuxAgents` even when the sidebar renders `null` (`:1035-1037`).

---

## 2. Refresh / Trigger Inventory (刷新机制)

Global: `refetchOnMount/OnReconnect/OnWindowFocus` all disabled
(`app/src/query/query-client.ts:8-10`). No WS push events for tmux anywhere.

| # | Data | Mechanism | Interval / timing | Where |
|---|---|---|---|---|
| 1 | agents+panes+history (per host) | RQ `refetchInterval`, adaptive by data identity | 500ms (≤2s) / 1s (≤10s) / 5s; app-visible only | `use-tmux-agents.ts:83-86` |
| 2 | agents | `refreshAll()` → invalidate on dashboard mount | once per mount | `tmux-dashboard-screen.tsx:374-376` |
| 3 | agents | Pull-to-refresh | manual | `tmux-dashboard-screen.tsx:644-646` |
| 4 | agents | `refreshAll()` after new/kill/delete/run | post-mutation | `tmux-dashboard-screen.tsx:389,448,464,478` |
| 5 | pane content | RQ `refetchInterval`, adaptive; gated on enabled+visible+autoRefresh | 500ms/1s/5s | `use-tmux-capture-pane.ts:118-121` |
| 6 | pane content | Daemon content-hash dedup (`changed=false` → empty payload) | every poll | `session_tmux.go:132-139` |
| 7 | pane content | `refetch()` after send-keys | post-mutation | `tmux-pane-screen.tsx:173-176` |
| 8 | pane content | Manual refresh button | manual | `tmux-pane-screen.tsx:423-437` |
| 9 | pane content | Foreground-edge refetch (hidden→visible) | once per transition | `use-tmux-capture-pane.ts:172-179` |
| 10 | pane history | `loadMoreHistory()` → queryKey change | scroll <20px from top | `use-tmux-capture-pane.ts:181-184` |
| 11 | status lines | Initial fetch only | effectively once per app run | `use-tmux-status-lines.ts:41-58` |
| 12 | loop inspect | RQ `refetchInterval`, adaptive (same helper) | 500ms/1s/5s | `use-loop-inspect.ts:50-53` |

---

## 3. Issues Found

### #1 (P0, correctness) — Wrong-server mutation on multi-host
`handleCloseSession`, `handleRunCommand`, `handleDeleteCommand`,
`handleCreateSession` all use `firstConnectedServerId`
(`tmux-dashboard-screen.tsx:378-479`) instead of the target agent/session's own
`serverId`. On multi-host setups, killing a session displayed for host B sends
`tmux/kill_session` to host A. **Verified.**

### #2 (P1, performance) — `ps` snapshot not shared per scan
`findAgentDescendant` re-runs full `ps -eo pid,ppid,args=` on every call
(`session_tmux_scan.go:431-449`): once per pane failing layers 1–2, plus up to 2
more per agent via `extractAgentLaunchCmd` (`:463-471`). Dozens of panes → dozens
of full process snapshots per request, amplified by the 500ms poll tier.
**Verified.**

### #3 (P1, performance) — Agents polling runs app-wide
`LeftSidebar` executes `useTmuxProjectCounts` unconditionally
(`left-sidebar.tsx:213`); polling is gated only on app visibility, never on
sidebar open state or screen focus. Continuous per-host `list_agents` scans (with
all their subprocesses) run whenever the app is foregrounded, even if the user
never opens tmux surfaces.

### #4 (P2, freshness) — Status lines fetch-once, go stale immediately
`use-tmux-status-lines.ts` has no `refetchInterval`; global mount/focus refetch is
disabled; nothing invalidates `tmuxStatusLineQueryKey`. A status-right containing
a clock or live pane title freezes after first fetch.

### #5 (P2, freshness/leak) — Incomplete invalidation after mutations
- Kill-session invalidates only `tmux-agents` (`tmux-dashboard-screen.tsx:463-465`):
  the killed session's `tmux-status-line` and its panes' `tmux-capture-pane`
  caches survive until `gcTime` (5 min).
- Daemon-side, `paneContentHashes`/`paneLastContentChange` entries are deleted only
  for panes still present and detected as `exited` (`session_tmux_scan.go:133-136`)
  — panes that vanish (killed) leak map entries for the WS session lifetime.

### #6 (P3, architecture — recorded, not fixed)
Per-connection activity state (`paneContentHashes` on WS `Session`): busy/idle
resets on every reconnect; N clients trigger N full scans — no daemon-wide shared
scan cache (singleflight only coalesces identical capture requests). Requires a
daemon-level scan cache redesign; deferred.

### #7 (P3, docs) — Documentation drift
`docs/architecture/tmux-pane-content-loading.md`: claims 3-layer detection (impl:
4), omits `#{window_activity}` in the format string, says agent list
`staleTime: 30_000` (impl: inherits global `Infinity`), says fixed
`refetchInterval: 5000ms` (impl: adaptive 500/1000/5000), lists capture flags
without `-J`/rewrap, references non-existent `app/src/hooks/use-tmux-theme.ts`.
`docs/analysis/tmux-pane-analysis.md`: says adaptive "200ms/1s/5s" (impl: 500ms
tier); its "no incremental mechanism" claim is outdated (content-hash dedup +
singleflight exist). This report is the current source of truth.

---

## 4. Resolution

All fixes implemented and verified on 2026-07-24.

| # | Fix | Files |
|---|---|---|
| #1 | Mutation handlers resolve the target item's own `serverId`; `AgentCommandEntry` now carries `serverId`/`serverLabel`; manual new-session keeps the first-connected-host default | `app/src/hooks/use-tmux-agents.ts`, `app/src/screens/tmux-dashboard/tmux-dashboard-screen.tsx` |
| #2 | One lazily-loaded `ps` snapshot per `parseTmuxPaneLines` scan, threaded through layer-3 detection and launch-cmd extraction (was: 1 full `ps` per lookup) | `daemon/internal/server/session_tmux_scan.go` |
| #3 | `useAggregatedTmuxAgents({ enabled })` option; sidebar passes `isCompactLayout \|\| isOpen` (RQ merges observers per queryKey, so an open dashboard keeps polling) | `app/src/hooks/use-tmux-agents.ts`, `app/src/hooks/use-tmux-project-counts.ts`, `app/src/components/left-sidebar.tsx` |
| #4 | Status lines get adaptive `refetchInterval` (500ms/1s/5s by data-change recency, app-visible only) | `app/src/hooks/use-tmux-status-lines.ts` |
| #5 | App: kill-session `removeQueries` for the dead session's status-line + capture-pane caches (new `tmuxCapturePanePanePrefix` helper). Daemon: new `prunePaneActivityState` drops hash-map entries for vanished panes once per scan | `tmux-dashboard-screen.tsx`, `app/src/hooks/use-tmux-capture-pane.ts`, `daemon/internal/server/session_tmux_scan.go`, `session_tmux.go` |
| #6 | Deferred — needs daemon-level shared scan cache | — |
| #7 | Superseded by this document | — |

Also fixed en passant: close/delete buttons on dashboard cards now use `testID` +
`accessibilityLabel` (RN Web forwards them; the previous `data-testid` was dropped
by RN Web and unreachable to tests and screen readers).

### Tests added

- `daemon/internal/server/session_tmux_test.go`: `TestPrunePaneActivityState`
  (vanished-pane pruning keeps live entries), `TestParseTmuxPaneLinesSharesSingleProcessSnapshot`
  (3 layer-3 panes → exactly 1 `ps` call). Existing `findAgentDescendant` /
  `extractAgentLaunchCmd` tests updated to the new snapshot-parameter signature.
- `app/src/screens/tmux-dashboard/tmux-dashboard-screen.test.tsx`: 3 multi-host
  regression tests — kill / delete / run target the owning item's `serverId`, not
  the first connected host. Existing renders wrapped in `QueryClientProvider`.

### Verification results

- daemon: `go build ./...` clean; `go test -short -race ./...` all pass;
  `golangci-lint run ./internal/server/` 0 issues.
- app: `tsc --noEmit` clean; `expo lint` clean; `npx vitest run` 265 files /
  1944 tests all pass (1941 baseline + 3 new).
