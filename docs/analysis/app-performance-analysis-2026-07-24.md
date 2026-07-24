# App Performance Analysis

**Date:** 2026-07-24
**Status:** Analysis Complete
**Priority:** High (UX / Performance)

---

## Executive Summary

The Solo app codebase is unusually well-optimized: batched stream reducers (48ms flush), rAF-coalesced activity timestamps, adaptive polling with content-hash dedup, `@tanstack/react-virtual` on web, UI-thread Reanimated worklets, custom `React.memo` comparators on message components, and bounded send queues with backpressure. All 7 findings from this audit have been resolved (2026-07-24).

| Severity | Count | Status |
|----------|-------|--------|
| High | 1 | Resolved — dead code removed |
| Medium | 4 | Resolved — all fixes applied |
| Low | 2 | Resolved — all fixes applied |

---

## Analysis

### 1. [HIGH] ~~`assistant_chunk` streaming updates are per-chunk, unbatched~~ — RESOLVED

**File:** `app/src/contexts/session-event-handlers.ts` (~lines 350–354)

**Resolution (2026-07-24):** Investigation revealed this entire code path was **dead code**. The daemon never sends `assistant_chunk` WS messages (no Go code references this message type), and `currentAssistantMessage` state was never read by any component. The handler, store state, store action, and context wiring have been removed entirely.

---

### 2. [MEDIUM] ~~Global React Query `gcTime: Infinity`~~ — RESOLVED

**File:** `app/src/query/query-client.ts`

**Resolution (2026-07-24):** Changed `gcTime` from `Infinity` to `5 * 60 * 1000` (5 minutes). Inactive query cache entries are now garbage-collected, preventing unbounded memory growth in long sessions. `staleTime: Infinity` retained (data refreshed via WS push).

---

### 3. [MEDIUM] ~~Terminal persistent 250ms fit interval~~ — RESOLVED

**File:** `app/src/terminal/runtime/terminal-emulator-runtime.ts`

**Resolution (2026-07-24):** Removed the redundant 250ms `setInterval`. The existing `ResizeObserver` on `input.root`/`input.host` already fires `fitAndEmitResize` on actual container size changes, and `window.resize` / `visualViewport.resize` / font-ready handlers cover the remaining cases.

---

### 4. [MEDIUM] ~~Fixed-interval polling~~ — RESOLVED

**Files:** `app/src/hooks/use-loop-inspect.ts`, `app/src/hooks/use-tmux-agents.ts`

**Resolution (2026-07-24):** Both hooks now use `computeAdaptiveInterval` (500ms active / 1s warm / 5s idle based on `dataUpdatedAt`) gated on `useAppVisible()`, matching the proven pattern from `use-tmux-capture-pane`. Polling pauses entirely when the app is hidden.

---

### 5. [MEDIUM] ~~Native FlatList config~~ — RESOLVED

**File:** `app/src/components/stream-strategy-native.tsx`

**Resolution (2026-07-24):** Enabled `removeClippedSubviews` (unmounts offscreen cells, reducing memory + layout cost) and reduced `initialNumToRender`/`maxToRenderPerBatch` from 40 to 20.

---

### 6. [LOW] ~~Per-message Zod validation~~ — RESOLVED

**File:** `app-bridge/src/client/connection-manager.ts`

**Resolution (2026-07-24):** Added a fast path for `agent_stream` messages: a lightweight structural check on the type discriminants (`type === "session"` + `message.type === "agent_stream"`) bypasses full Zod `safeParse`. All other message types still go through full validation.

---

### 7. [LOW] ~~Sidebar context resize cascade~~ — RESOLVED

**Files:** `app/src/contexts/sidebar-animation-context.tsx`, `app/src/contexts/explorer-sidebar-animation-context.tsx`

**Resolution (2026-07-24):** `windowWidth` is now `Math.round(rawWindowWidth)`, preventing sub-pixel resize events from producing a new context value and triggering re-render cascades during continuous desktop window drags.

---

## Confirmed Well-Optimized (No Action Needed)

| Area | Implementation |
|------|---------------|
| Message rendering | `React.memo` with custom comparators; markdown split into memoized blocks; shimmer on UI thread with `cancelAnimation` cleanup |
| Session store | `subscribeWithSelector` + `fast-deep-equal` identity preservation; `clearSession` cleans up all per-agent Maps |
| Stream batching | 48ms batched flush + rAF-coalesced timestamps (`session-stream-reducers.ts`, `last-activity-coalescer/`) |
| Tmux scrollback | 200→5000 lazy loading, adaptive polling, content-hash dedup, `keepPreviousData` |
| Web virtualization | `@tanstack/react-virtual` with partial-virtualization threshold (100) and mounted-recent window (50) |
| WS transport | Bounded send queue + waiters, exponential-backoff reconnect with jitter, binary-frame fast path |
| Startup | Fully async bootstrap with `AbortSignal` cancellation |
| Animations | UI-thread Reanimated worklets throughout; memoized values; extensive `useCallback` |
| Code splitting | Dynamic imports for platform-specific stores and lazy `confirm-dialog` |

---

## Recommendations (Priority Order)

All items resolved (2026-07-24):

1. ~~**Batch `assistant_chunk`**~~ — Done: dead code removed; daemon never sends this message type.
2. ~~**Set finite `gcTime`**~~ — Done: `gcTime: 5 * 60 * 1000` in `query-client.ts`.
3. ~~**Replace terminal fit interval**~~ — Done: removed redundant 250ms interval; ResizeObserver covers it.
4. ~~**Adaptive polling**~~ — Done: `use-loop-inspect` and `use-tmux-agents` now use `computeAdaptiveInterval` + `useAppVisible`.
5. ~~**Tune native FlatList**~~ — Done: `removeClippedSubviews` enabled, `initialNumToRender`/`maxToRenderPerBatch` reduced to 20.
6. ~~**Fast-path Zod validation**~~ — Done: `agent_stream` bypasses full `safeParse` via type-discriminant check.
7. ~~**Debounce sidebar context width**~~ — Done: `Math.round(rawWindowWidth)` in both sidebar animation providers.

---

## Related Files

- `app/src/contexts/session-event-handlers.ts`
- `app/src/contexts/session-stream-reducers.ts`
- `app/src/query/query-client.ts`
- `app/src/terminal/runtime/terminal-emulator-runtime.ts`
- `app/src/hooks/use-loop-inspect.ts`
- `app/src/hooks/use-tmux-agents.ts`
- `app/src/hooks/use-tmux-capture-pane.ts` (reference pattern)
- `app/src/components/stream-strategy-native.tsx`
- `app/src/components/stream-strategy-web.tsx`
- `app-bridge/src/client/connection-manager.ts`
- `app/src/contexts/sidebar-animation-context.tsx`

---

# Round 2: Deep Cost-Distribution Analysis (2026-07-24, follow-up)

Static code-path analysis across five dimensions (message rendering, streaming update path, terminal/polling, store/context fan-out, memory growth). Percentages are estimates from code-path complexity, not runtime profiling.

## Cost Distribution by Scenario

| Scenario | Dominant cost sources (estimated share) |
|----------|------------------------------------------|
| **Active streaming** (heaviest) | Reducer string work ~35% · per-flush full-history dedup ~20% · live-block markdown re-parse ~20% · E2EE decrypt (relay only) ~10-15% · selector fan-out ~10% · bridge dispatch ~5% |
| **Foreground idle** | tmux flat-500ms/host polling (largest) · host-runtime 2s probe · xterm cursor blink · 15s heartbeat |
| **Background** | React Query pollers correctly paused; residual: host-runtime 2s probe + 15s heartbeat |
| **Memory growth** | `agentStreamTail` 20-100 MB/h (overwhelming) · legacy `session.messages` 5-20 MB/h dead weight · xterm transient · tmux query-cache variants |

## Round-2 Findings

### 8. [HIGH] Reducer string work is quadratic in response length

**Files:** `app/src/types/stream.ts:256,302` (concat), `:838` (block split)

- `appendAssistantMessage` does `` `${last.text}${chunk}` `` per chunk — a full-length copy each time; a response of length M at chunk size c costs O(M²/c). A 100 KB response in 1 KB chunks ≈ ~10 MB of char copying.
- `promoteCompletedAssistantBlocks` runs `splitMarkdownBlocks(activeItem.text)` over the **entire accumulated text on every chunk**, even when fewer than 2 blocks exist. Per flush with k deltas: O(k·n).

### 9. [HIGH] `tailContentSet` rebuilds over full history every flush

**File:** `app/src/components/agent-stream-render-model.ts:165-170`

Every 48ms flush builds `` `${item.kind}:${item.text}` `` strings for **every history item** into a `Set` — O(total history bytes) allocation + hashing 20×/second, purely to dedup head vs tail. WeakMap caches exist for ordering/split but not for this Set.

### 10. [HIGH] Agent removal leaks stream tail/head

**File:** `app/src/contexts/session-agent-sync.ts:520-569`

The remove path deletes `agents`, `pendingPermissions`, `queuedMessages`, `agentTimelineCursor` — but never `agentStreamTail`, `agentStreamHead`, `agentHistorySyncGeneration`, `initializingAgents`, or `fileExplorer`. Deleting/archiving an agent strands its entire stream (potentially tens of MB) until whole-session clear.

### 11. [MEDIUM] Adaptive polling never downshifts (5s idle tier unreachable)

**File:** `app/src/hooks/use-tmux-capture-pane.ts:22-28` (`computeAdaptiveInterval`)

React Query v5 bumps `dataUpdatedAt` to `Date.now()` on **every successful fetch** (even when structural sharing keeps data identical). Since the minimum interval (500ms) < `ACTIVE_PHASE_MS` (2s), `elapsed` never exceeds 2s on the success path — all three "adaptive" pollers (`use-tmux-capture-pane`, `use-tmux-agents`, `use-loop-inspect`) run at flat 500ms whenever visible. Impact amplified because `useAggregatedTmuxAgents` is consumed by the always-mounted left sidebar (`use-tmux-project-counts.ts:54`), so per-host 500ms polling runs effectively the whole time the app is visible.

### 12. [MEDIUM] Markdown style object rebuilt every render (withUnistyles)

**File:** `app/src/components/message.tsx:182-184`

`ThemedMarkdown = withUnistyles(Markdown)` calls `createMarkdownStyles(theme)` (~60-key object) on every render with no memoization, so each live-block render per flush also pays `getStyle`/`StyleSheet.create` + `new AstRenderer` inside react-native-markdown-display — defeating the block-level memo.

### 13. [MEDIUM] Legacy `session.messages` accumulator is dead weight

**File:** `app/src/contexts/session-event-handlers.ts:268-342`

Every `activity_log` WS event appends to `session.messages: MessageEntry[]` (including tool args/result) — unbounded, duplicating stream data, with no remaining UI readers. 5-20 MB/h of pure waste.

### 14. [LOW] Wasted UTF-8 encode per text frame

**File:** `app-bridge/src/client/connection-manager.ts:919`

`asUint8Array(rawData)` runs an O(S) UTF-8 encode + S-byte allocation on every message solely to sniff binary terminal frames; text frames then get JSON.parsed anyway. After the Zod fast-path this is the largest avoidable per-message bridge cost.

### 15. [LOW] Draft persistence serializes the whole store per keystroke

**File:** `app/src/stores/draft-store.ts:495-496,647-649`

`saveDraftInput` fires per keystroke → `persist` middleware JSON-serializes the entire store + AsyncStorage write per keystroke (no `partialize`, no debounce); every save also schedules an attachment-GC scan of all agents' stream tails/heads.

### 16. [LOW] Relay E2EE chain: 8-9 O(S) sync passes per message

**Files:** `app-bridge/src/relay/crypto.ts:133-152`, `app-bridge/src/relay/base64.ts:7-18`

Base64 decode chain makes 2 regex copies + a redundant extra copy; decrypt does nonce/ciphertext slice copies + pure-JS Poly1305/XSalsa20 + output copies — all synchronous on the JS thread. Several× slower on Hermes than V8.

## Memory Growth Detail (heavy use, per hour)

| Consumer | Growth/h | Issue |
|----------|----------|-------|
| `agentStreamTail` | **20-100 MB** | No cap; `ToolCallDetail` retains full shell output / file content / diffs; leaks on agent delete (#10) |
| Legacy `session.messages` | 5-20 MB | No UI readers (#13) |
| xterm buffers | 5-30 MB transient | 10k scrollback, ≤3 mounted tabs, freed on workspace switch — OK |
| tmux capture cache | 2-6 MB | `scrollbackLines` in query key → up to 25 depth variants coexist 5 min |
| `loop-inspect` logs | 1-5 MB | daemon-side `logs[]` uncapped |
| `fileExplorer` | 1-10 MB | Full file contents incl. base64 images, no eviction |

## Confirmed Well-Optimized (Round 2, No Action)

| Area | Evidence |
|------|----------|
| Store commit | O(1) per flush: identity guards, ~6-8 flat allocations (`session-store.ts:735-788`) |
| Re-render isolation | Exactly 2 components re-render per flush (`AgentStreamView` head + `AgentStreamSection` tail) |
| Selector hygiene | Zero whole-store subscriptions; all object/array selectors use `useShallow`/`useStoreWithEqualityFn` |
| Background gating | All React Query pollers stop when app hidden |
| Terminal writes | Serialized queue + xterm internal batching; WebGL renderer on desktop |
| Listeners | No accumulating subscriptions; bounded send queue (1000), waiters cleaned |
| xterm scrollback | Bounded (10k lines, LRU-capped 3 tabs) |

## Round-2 Recommendations (Priority Order)

All P0–P2 resolved (2026-07-24):

1. ~~**P0** — Reducer: skip `splitMarkdownBlocks` unless a block boundary can exist~~ — Done: `applyStreamEvent` now gates `promoteCompletedAssistantBlocks` on the incoming chunk containing `\n` (block boundaries require a blank line), eliminating the O(n) split per chunk. Verified: 32 stream tests pass. (#8)
2. ~~**P0** — Cache `tailContentSet` by tail reference~~ — Done: `tailContentSetCache` WeakMap keyed by raw tail ref (order-independent), built once per tail change instead of every 48ms flush. (#9)
3. ~~**P0** — Clean per-agent maps on agent removal~~ — Done: `agent_update` remove path now deletes `agentStreamTail`/`agentStreamHead`/`initializingAgents`/`fileExplorer` entries; `agent_deleted` handler also cleans `fileExplorer`. (#10)
4. ~~**P1** — Adaptive polling keyed on last-content-change~~ — Done: `computeAdaptiveQueryInterval` tracks data-reference identity per Query (WeakMap); structural sharing + daemon hash dedup keep the ref stable when unchanged, so warm/idle tiers are now reachable. Unit tests added. (#11)
5. ~~**P1** — Memoize markdown styles~~ — Done: `createMarkdownStyles` memoized per theme reference via WeakMap at the source (`markdown-styles.ts`), restoring stable `style` identity for the Markdown component's internal memoization. (#12)
6. ~~**P2** — Remove legacy `session.messages` accumulator~~ — Done: `activity_log`→`setMessages` handler, `setMessages` action, `messages` state field, `MessageEntry` type, and `applyToolResult/ErrorToMessages` helpers all removed (no readers existed). (#13)
7. ~~**P2** — Skip UTF-8 encode for string frames~~ — Done: `handleTransportMessage` skips `asUint8Array` when `rawData` is a string (binary frames never arrive as strings). (#14)
8. ~~**P2** — Draft store GC throttle~~ — Done: attachment GC now coalesces to at most one run per 2s (was: full stream scan + storage GC per keystroke). Persist debounce deliberately skipped: payload is small (drafts only; functions aren't serialized) and AsyncStorage writes are off-JS-thread on native — poor win/risk ratio. (#15)
9. **P3** — Relay base64/decrypt copy reduction (#16) — deferred (crypto-path change; needs dedicated verification).

### Round-2 verification

- `tsc --noEmit` clean: app + app-bridge
- ESLint clean on all touched files
- Full app suite: 265 files / 1941 tests pass (1 pre-existing fake-timer teardown warning, present on baseline)
- app-bridge suite: 178 tests pass
