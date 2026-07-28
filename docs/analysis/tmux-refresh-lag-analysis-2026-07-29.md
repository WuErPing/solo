# Tmux Pane Refresh Lag Analysis

Date: 2026-07-29
Scope: app (`app/src/hooks/use-tmux-capture-pane.ts`, `app/src/screens/tmux-pane-xterm-screen.tsx`), daemon (`daemon/internal/server/session_tmux.go`)
Status: P0 fix applied; P1-P3 recorded for future work.

---

## Symptom

Tmux pane content refresh feels laggy / choppy, especially during active agent output.

## Root Cause

Tmux pane content is **100% pull-based polling** with no push mechanism (contrast
with the live terminal's 5ms-coalesced PTY stream). The adaptive polling tiers in
`use-tmux-capture-pane.ts:16-28` cap the active refresh rate at **2 fps (500ms)**,
far below the ~15-24 fps needed for smooth text scrolling.

### Polling tiers (before fix)

| Phase | Trigger | Interval | Effective fps |
|---|---|---|---|
| Active | content changed within 2s | 500ms | 2 |
| Warm | changed 2-10s ago | 1000ms | 1 |
| Idle | stable >10s | 5000ms | 0.2 |

### Aggravating factors

1. **Aggressive ramp-down** — output pauses of just 2s drop the poller from 500ms
   to 1000ms; 10s drops it to 5s. Agent output is bursty (thinking, tool calls),
   so the poller frequently downshifts right before the next burst arrives.

2. **Full-grid rewrite per snapshot** — `tmux-pane-xterm-screen.tsx:509` writes
   `\x1b[3J\x1b[H${snapshot}\x1b[J` on every change: clear scrollback, home cursor,
   repaint entire grid, clear trailing cells. At 2 fps this produces visible flicker.

3. **Round-trip latency stacks on the interval** — each poll traverses
   React Query -> WebSocket -> Relay -> Daemon -> `tmux capture-pane` subprocess
   (5s timeout) -> SHA-256 hash -> return path -> React Query dedup -> xterm write.
   Best-case overhead is 50-200ms on top of the 500ms timer, yielding an effective
   update interval of 550-700ms.

4. **No push channel** — the daemon already detects content changes via SHA-256
   hash comparison (`session_tmux.go:132-139`) but never notifies the client.
   The Solo-agent subsystem has `agent_update` push events
   (`session_agent.go:47-59`); tmux has none.

## Data flow (polling path)

```
React Query refetchInterval (adaptive 500ms/1s/5s)
  -> useTmuxCapturePane hook
    -> withLiveTmuxClient(serverId, c => c.tmuxCapturePane(...))
      -> TerminalRpc.tmuxCapturePane()  [app-bridge/src/client/terminal-rpc.ts:562-580]
        -> ConnectionManager.sendCorrelatedSessionRequest()
          -> WebSocket JSON (type: "tmux/capture_pane")
            -> Relay Server
              -> Daemon handler  [daemon/internal/server/session_tmux.go:105-143]
                -> singleflight coalescing
                -> tmux capture-pane -t {paneId} -p -e -J -S {startLine}
                -> SHA-256 content hash comparison
                -> Response: {content, changed, contentHash, paneCols}
  -> React Query dedup (data-reference identity via WeakMap)
    -> snapshotText prop -> TerminalEmulator -> xterm.js write
```

## Fix applied (P0)

`app/src/hooks/use-tmux-capture-pane.ts`:

| Constant | Before | After | Rationale |
|---|---|---|---|
| `ACTIVE_POLL_INTERVAL` | 500ms | **150ms** | ~7 fps, noticeably smoother while keeping relay/daemon load reasonable |
| `ACTIVE_PHASE_MS` | 2,000ms | **5,000ms** | keeps the poller in the active tier through typical agent think/tool-call pauses |

Warm and idle tiers unchanged (1000ms / 5000ms) — they only engage after 5s / 10s
of true inactivity, where battery savings matter more than responsiveness.

## Remaining work

| Priority | Approach | Scope | Effect |
|---|---|---|---|
| P1 | Daemon push notification on tmux content change (WS event -> immediate client refetch) | `session_tmux.go`, `terminal-rpc.ts`, hook | eliminates polling latency; change arrives instantly |
| P2 | Incremental xterm rendering (diff changed lines instead of full `\x1b[3J` clear+rewrite) | `terminal-emulator-runtime.ts` | removes flicker, reduces repaint cost |
| P3 | Stream tmux pane output like the live terminal (PTY subscription + 5ms coalescing) | architecture-level | fundamental fix, 60fps-class experience |
