# Solo Network · Data · State — Architecture Deep Dive

> **Audience**: Solo core developers, architects reviewing end-to-end correctness.
> **Last revised**: 2026-06-09
> **Related**: [network-architecture.md](network-architecture.md), [data-flow.md](data-flow.md), [timeline-design.md](timeline-design.md), [session-memory-persistence.md](session-memory-persistence.md)

This document ties together three subsystems that are usually documented in isolation:

1. **Network layer** — how bytes travel between App, Relay, and Daemon.
2. **Data layer** — how a `user_message` becomes a persisted turn on disk and a timeline row in memory.
3. **State layer** — how clients and the server agree on "what is the current conversation" across reconnects, multi-device sync, and provider failover.

It is written as a *synthesis* of the existing architecture docs plus the current source code. Read it after the per-subsystem docs; it fills the seams between them.

---

## 1. Layered Reference Model

```
┌──────────────────────────────────────────────────────────────────┐
│ App  (Expo/React Native)        Zustand · react-query · Context  │  STATE
├──────────────────────────────────────────────────────────────────┤
│ App-Bridge  (TS)     DaemonClient · Transports · E2EE · Codecs   │  DATA
├──────────────────────────────────────────────────────────────────┤
│ Network   WebSocket / WSS · Relay control+data · Pairing Link    │  NETWORK
├──────────────────────────────────────────────────────────────────┤
│ Daemon    (Go)   Server · Session · AgentManager · Providers     │  DATA+STATE
├──────────────────────────────────────────────────────────────────┤
│ Persistence   TimelineStore · Memory · Workspace · Config        │  DATA
└──────────────────────────────────────────────────────────────────┘
```

Every vertical request (e.g. "send a message") crosses **all six layers twice** (outbound + inbound). Bugs usually live at the handoff:

| Handoff | Failure mode |
|---|---|
| App → App-Bridge | React state lag; optimistic UI drifts from server truth |
| App-Bridge → Network | Transport disposed race; E2EE nonce reuse on reconnect |
| Network → Daemon | Hello timeout, CORS rejection, wrong relay endpoint |
| Daemon → Provider | Stream stalls, missing `user_message` echo, tool-call idempotency |
| Provider → Persistence | Duplicate timeline rows, out-of-order turn recording |
| Persistence → App | Bootstrap tail race, epoch mismatch on warm resume |

---

## 2. Network Layer — Recap of the Key Invariants

(Full details: [network-architecture.md](network-architecture.md).)

| Invariant | Where enforced |
|---|---|
| Daemon only listens `127.0.0.1:17612` | `daemon/internal/server/daemon.go` |
| Relay only listens `127.0.0.1:8081` in production | systemd unit + `relay-go` `HOST` env |
| Public traffic **must** use `solo.up2ai.top:443` (WSS) | Nginx reverse proxy; firewall drops raw `:8081` |
| Protocol version negotiation happens in `hello` (<15 s) | `protocol.HelloTimeoutMs` |
| Grace period on disconnect is 90 s | `protocol.SessionDisconnectGraceMs` |
| E2EE is end-to-end between App ↔ Daemon; Relay only forwards ciphertext | `app-bridge/src/relay/e2ee.ts`, `daemon/internal/relayclient/e2ee.go` |

### 2.1 Connection taxonomy

| Mode | Transport class | Crypto | Typical client |
|---|---|---|---|
| **Local direct** | `WebSocketTransport` (`ws://127.0.0.1:17612/ws`) | none (localhost) | Web dev, CLI |
| **Relay, no E2EE** | `WebSocketTransport` over WSS to `solo.up2ai.top:443` | TLS only | Trusted internal tests |
| **Relay + E2EE** | `RelayE2EETransport` (control + data sockets) | TLS + X25519 + XSalsa20-Poly1305 | Mobile, public web |

The **Pairing Link** (`https://solo.up2ai.top/#offer={base64url(ConnectionOfferV2)}`) is the *bootstrap secret* that tells the client: relay endpoint + daemon public key + serverId. Everything else follows from it.

---

## 3. Data Layer — From User Input to Persisted Turn

### 3.1 The canonical message path

```
User types "fix the bug"
  │
  ▼
App (Zustand)                  optimistic user_message
  │
  ▼
App-Bridge.sendAgentMessage()  JSON-RPC: send_agent_message_request
  │
  ▼ (WSS, maybe E2EE-wrapped)
Daemon server/session.go       SessionInboundMessage registry decode
  │
  ▼
session.handleSendAgentMessage
  ├── memoryBridge.OnUserTurn        → async TurnRecorder.RecordTurn (user.md)
  ├── agentMgr.Run / StartTurn       → provider emits user_message TimelineStreamEvent
  ├── m.emit(user_message)           → broadcast to every subscribed Session
  └── sendAgentStream(event)         → push to this client (live)
                                           │
                                           ▼
                                     App receives stream event
                                     Seq Gate → Head buffer → Tail flush
```

### 3.2 Two parallel writes, one logical turn

A single user prompt produces writes to **two** stores:

| Store | Location | Shape | Purpose |
|---|---|---|---|
| **TimelineStore** | in-memory, Daemon process | `TimelineRow { Item, Epoch, Seq }` | live multi-device sync, UI rendering |
| **Memory (TurnRecorder)** | `~/.solo/memory/sessions/{YYYY-MM-DD}/{sid}/turns/{seq}-{role}.md` | Markdown + YAML frontmatter | long-term recall, future vector/RAG |

They are **not** coordinated transactionally. Timeline is hot + volatile; Memory is cold + durable. On daemon crash, the Memory file survives; the Timeline must be rebuilt from provider history (`StreamHistory()` → `AppendFromHistory()`).

### 3.3 Provider contract: `user_message` is mandatory

Every provider **must** emit a `user_message` `TimelineStreamEvent` at the start of `Run()`/`StartTurn()`. Rationale: multi-device sync. The sending client's optimistic update is local-only; other clients rely on the live stream.

| Provider | Emits `user_message` | Notes |
|---|---|---|
| claude | ✅ | before stream |
| kimi | ✅ | before stream |
| pi | ✅ | before stream |
| opencode | ✅ (Run + StartTurn) | OpenCode SSE doesn't echo the prompt; Solo synthesizes it |
| mock | ✅ | tests |

### 3.4 Idempotent append — the multi-device dedup trick

`m.emit()` is synchronous: it walks the subscriber list sequentially. If N Sessions are online, `TimelineStore.Append()` fires N times for the same event. The fix is last-row equality check:

```go
// daemon/internal/agent/timeline.go
if len(state.Rows) > 0 {
    last := state.Rows[len(state.Rows)-1]
    if timelineItemsEqual(last.Item, item) {
        return last   // no duplicate
    }
}
```

Equality is **type-specific**:

| Stream item type | Equality key |
|---|---|
| `user_message` | `MessageID` (preferred) else `Text` |
| `assistant_message` / `reasoning` | `Text` |
| `tool_call` | `CallID + Status` |

`AppendFromHistory()` (used during bootstrap) uses a stronger backwards-scan over *all* rows because history items may arrive interleaved with live items.

---

## 4. State Layer — Agreeing on "The Current Conversation"

### 4.1 Head / Tail double-buffer

The client splits streaming state into:

- **Tail** — committed conversation history, low churn, canonical
- **Head** — currently streaming chunk, high churn, ephemeral

On `turn_completed` / `turn_failed` / `turn_canceled` or event-type switch, Head is flushed to Tail. Completed Markdown code blocks are also promoted early (`promoteCompletedAssistantBlocks`) to keep Head small.

### 4.2 Seq Gate — the single entry rule

Every event carries `{epoch, seq}`. The gate classifies:

| Class | Action |
|---|---|
| `init` | set cursor |
| `accept` (`seq == endSeq+1`) | apply, advance cursor |
| `drop_stale` (`seq ≤ endSeq`) | silent discard |
| `drop_epoch` (epoch mismatch) | silent discard |
| `gap` (`seq > endSeq+1`) | reject + trigger `catch_up` |

The cursor `{epoch, startSeq, endSeq}` is the **single source of truth** for client sync position.

### 4.3 Bootstrap policy

On connect, `deriveInitialTimelineRequest` chooses:

| Has cursor? | Request | Reasoning |
|---|---|---|
| No | `direction: tail` (last N) | cold start |
| Yes | `direction: after(cursor)` | warm resume |

**The bootstrap tail race** (warm client + in-flight live stream): `deriveBootstrapTailTimelinePolicy` returns `replace: true` for the first tail response *during init* while stashing a `catchUpCursor` so subsequent live events fill the gap. Without this, the client would either lose mid-bootstrap events or double-append them.

### 4.4 Batched reducer queue

Events are coalesced on a 48 ms `setTimeout` window (~20 fps). Rationale: a streaming assistant can emit 50+ chunks/sec; batching collapses them into one React re-render per frame.

### 4.5 App-resume recovery

`handleAppResumed`:

1. If away > `HISTORY_STALE_AFTER_MS` (60 s), bump history-sync generation.
2. If a cursor exists, `fetchAgentTimeline(after, cursor)` to catch up.

---

## 5. Cross-Cutting: End-to-End Flows

### 5.1 Send a message (multi-device, E2EE)

```
Web (sender)                      Relay                        Daemon                   Mobile (observer)
     │                              │                            │                            │
     │─ optimistic user_message ─   │                            │                            │
     │                              │                            │                            │
     │─ send_agent_message ──────►  │─ forward (cipher) ──────► │                            │
     │                              │                            │─ OnUserTurn (Memory) ─     │
     │                              │                            │─ provider emits            │
     │                              │                            │   user_message             │
     │                              │                            │─ m.emit() ─────────────────┤
     │                              │                            │                            │
     │                              │◄──── stream (cipher) ─────┤                            │
     │◄─ stream event (live) ────── │                            │                            │
     │   Seq Gate → Head → Tail     │─ forward (cipher) ────────────────────────────────────►│
     │                              │                            │                            │  Seq Gate → Head → Tail
     │                              │                            │─ OnAssistantTurn ─         │
     │                              │                            │   (async, on flush)        │
     │                              │                            │                            │
     │─ turn_completed ───────────► │ ──────────────────────────────────────────────────────►│
     │   Head → Tail (flush)        │                            │   Head → Tail (flush)      │
```

### 5.2 Reconnect after 30 s offline

```
Mobile (offline 30s)                Relay                        Daemon
     │                               │                            │
     │ (comes back)                  │                            │
     │─ WebSocket reconnect ───────► │                            │
     │─ hello ─────────────────────► │─ validate ────────────────►│
     │                               │                            │  (grace period 90s, still alive)
     │                               │                            │
     │─ fetchAgentTimeline(after) ──►│──────────────────────────►│
     │                               │◄────────────── gap rows ──│
     │◄──── batched entries ──────── │                            │
     │   processTimelineResponse     │                            │
     │   → acceptIncremental         │                            │
     │   → applyStreamEvent          │                            │
```

### 5.3 Daemon restart (epoch boundary)

Server increments epoch; client's next event fails the Seq Gate (`drop_epoch`). The reducer resets the cursor and re-bootstraps via `tail`. Head is discarded.

---

## 6. Failure Modes & Their Layer of Resolution

| Failure | Detected at | Resolved at |
|---|---|---|
| Dropped WebSocket | TLS/WS layer | App-Bridge transport reconnect + exponential backoff |
| Relay unreachable | TCP timeout | App shows "Host offline"; user re-pairs |
| Hello timeout (>15 s) | Daemon | Daemon closes with `4001` (`WSCloseHelloTimeout`) |
| CORS rejection | Daemon | HTTP 403 on upgrade; check `daemon.cors.origins` |
| E2EE handshake fail | App-Bridge | Transport surfaces error; user re-pairs |
| Stale seq after resume | App reducer (Seq Gate `gap`) | Auto `catch_up` RPC |
| Epoch mismatch | App reducer (`drop_epoch`) | Auto re-bootstrap with `tail` |
| Provider stuck (>2 min no events or ≥6 repeats/10) | `StallMonitor` | `interruptFn` → `CancelAgentRun` → `turn_failed` |
| Duplicate `user_message` across N Sessions | `TimelineStore.Append` | Last-row equality short-circuit |
| Turn record write failure | Memory module | async subscriber; main flow unaffected |
| Bootstrap race (live events during tail fetch) | `deriveBootstrapTailTimelinePolicy` | `replace: true` + `catchUpCursor` |

---

## 7. Source Map — Where to Look

| Concern | File |
|---|---|
| Protocol constants | `protocol/protocol.go` |
| Message registry (inbound/outbound) | `protocol/message*.go`, `init()` in `message.go` |
| Stream event union (typed) | `protocol/stream_event.go` |
| Tool-call detail structs | `protocol/tool_call_detail.go` |
| Daemon WS server | `daemon/internal/server/daemon.go`, `session.go` |
| Inbound handler dispatch | `daemon/internal/server/handler_registry.go`, `session_register_handlers.go` |
| Agent lifecycle + broadcast | `daemon/internal/agent/manager.go` |
| Provider implementations | `daemon/internal/agent/provider_{claude,kimi,opencode,pi,mock}.go` |
| Timeline store + dedup | `daemon/internal/agent/timeline.go` |
| Memory hook wiring | `daemon/internal/server/memory_wiring.go`, `memorybridge.go` |
| Memory module | `daemon/internal/memory/` (TurnRecorder, FileTurnRecorder, Redactor, Bridge) |
| Relay client (Daemon side) | `daemon/internal/relayclient/client.go`, `e2ee.go` |
| Relay server | `relay-go/internal/relay/{server,session,control,buffer}.go` |
| App-Bridge DaemonClient | `app-bridge/src/client/daemon-client.ts` |
| WebSocket transport | `app-bridge/src/client/daemon-client-websocket-transport.ts` |
| Relay + E2EE transport | `app-bridge/src/client/daemon-client-relay-e2ee-transport.ts` |
| E2EE crypto | `app-bridge/src/relay/{e2ee,encrypted-channel,crypto}.ts` |
| Connection offer schema | `app-bridge/src/shared/connection-offer.ts` |
| Timeline reducers | `app/src/contexts/session-timeline-*.ts`, `session-stream-reducers.ts` |
| Zustand session store | `app/src/stores/session-store.ts` |

---

## 8. Design Highlights

1. **Two stores, two tempos.** Timeline (hot, in-memory) and Memory (cold, on-disk) serve different consumers and tolerate independent failure.
2. **Synchronous fan-out, idempotent append.** `m.emit()` is deliberately synchronous so last-row dedup catches every duplicate before the next subscriber fires.
3. **Seq Gate as the single entry point.** All timeline mutations pass through one classifier — making correctness arguments local and testable.
4. **Bootstrap tail race is explicit.** Instead of hoping init and live streams don't collide, the policy *assumes* they do and stashes a catch-up cursor.
5. **Pairing Link as the bootstrap secret.** One URL carries relay endpoint + daemon public key + serverId; everything else derives from it.
6. **48 ms batching.** Chosen to align with ~20 fps rendering, not with any protocol constant — changing it requires no server coordination.
7. **Grace > timeout.** The 90 s disconnect grace (`SessionDisconnectGraceMs`) is longer than most mobile tunnel timeouts, so a brief network loss doesn't orphan a Session.
8. **Provider contract is a checklist, not a type.** `user_message` emission is enforced by code review and integration tests, not by the `AgentClient` interface. A future refactor should promote it to a typed pre-hook.

---

## 9. Open Questions / Future Work

- **Vector recall over Memory** — Phase 1 records turns; Phase 2 should expose a retrieval interface without breaking the TurnRecorder contract.
- **Stronger provider contract** — lift `user_message` emission into `AgentManager` so providers can't forget.
- **Persistent TimelineStore** — current in-memory store rebuilds from provider history on restart; a SQLite backend would skip the replay.
- **E2EE key rotation** — daemon keypair at `~/.solo/daemon-keypair.json` is long-lived; rotation ceremony is manual.
- **Cursor-aware Relay buffering** — the Relay currently buffers the last 200 frames per session regardless of epoch; smarter buffering could drop stale epochs early.
