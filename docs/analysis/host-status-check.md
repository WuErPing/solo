# `/settings/hosts/` Host Status Check Logic Analysis

## 1. Page Structure

| File | Responsibility |
|------|------|
| `app/src/app/settings/hosts/[serverId].tsx` | Route entry, corresponds to `/settings/hosts/:serverId` |
| `app/src/screens/settings/host-page.tsx` | Page UI, displays status badges, connection list, action buttons |
| `app/src/screens/settings-screen.tsx` | Layout container, sidebar lists all hosts |

---

## 2. Core Status Check: Probe Cycle

Status check logic is concentrated in **`app/src/runtime/host-runtime.ts`**'s `HostRuntimeController`, which uses a **periodic polling + real-time ping** mechanism.

### 2.1 Probe Timing Strategy

```typescript
const PROBE_TICK_MS = 2_000;        // Base polling interval 2s
const PROBE_STEADY_MS = 10_000;     // Stable probe interval for online connections
const PROBE_MAX_BACKOFF_MS = 30_000; // Maximum backoff interval
```

For **inactive connections**, the probe interval increases based on time since "first seen":
- `< 10s` → probe every **2s**
- `< 30s` → probe every **5s**
- `< 60s` → probe every **10s**
- `≥ 60s` → probe every **30s** (maximum backoff)

For **currently active online connections** → fixed probe every **10s**.

### 2.2 Single Probe Flow (`runProbeCycle`)

1. **Filter** connections that need probing this cycle (based on last probe time and the interval strategy above)
2. Set probe status of connections to be probed to **`pending`**
3. **In parallel**, for each connection:
   - If the connection is already the current active online connection, **reuse** the existing `DaemonClient`
   - Otherwise, call `connectToDaemon()` to **create a new test connection**
   - **Verify serverId matches** (prevents connecting to the wrong host)
   - Call `client.ping({ timeoutMs: 5000 })` to measure **RTT**
   - Success → set status to **`available`**, record `latencyMs`
   - Failure → set status to **`unavailable`**
4. After all probes complete, call `finalizeProbeCycle()` for **connection decision**

### 2.3 Connection Test Underlying (`test-daemon-connection.ts`)

```typescript
// connectToDaemon → connectAndProbe
// 1. Create DaemonClient
// 2. Establish WebSocket connection
// 3. Wait for serverInfo message (contains serverId, hostname, version)
// 4. Return { client, serverId, hostname }
```

Timeout settings:
- **Relay connection**: 10s
- **Direct connection**: 6s

Supported 4 connection types:

| Type | Transport |
|------|----------|
| `directTcp` | WebSocket `ws://host:port` |
| `directSocket` | Unix Domain Socket |
| `directPipe` | Named Pipe |
| `relay` | WebSocket via Relay + E2EE |

---

## 3. Connection Selection and Switching Logic

After probing is complete, processing follows this priority:

### 3.1 No Active Connection
Select the connection with the **lowest latency** from all `available` connections as the active connection.

### 3.2 Active Connection Fails
If the current active connection probe result is `unavailable`, immediately switch to the next best available connection.

### 3.3 Adaptive Switching
If another connection's latency **continuously outperforms** the current connection by ≥**40ms**, and **consecutive 3 probes** all meet the condition, automatically switch to the faster connection.

```typescript
const ADAPTIVE_SWITCH_THRESHOLD_MS = 40;
const ADAPTIVE_SWITCH_CONSECUTIVE_PROBES = 3;
```

### 3.4 Optimal Connection Algorithm (`connection-selection.ts`)

Simple iteration over all `available` connections, selecting the one with the smallest `latencyMs`.

---

## 4. State Machine

`HostRuntimeConnectionStatus` has 5 states:

```typescript
"idle" | "connecting" | "online" | "offline" | "error"
```

Transition paths:
- `booting` → `connecting` → `online` / `offline` / `error`
- Trigger events: `select_connection`, `client_state`, `connect_failed`, `no_connections`, `stopped`

---

## 5. UI Display (`host-page.tsx`)

### 5.1 Top Status Badges (Identity Badges)
- **Status capsule**: colored dot + text (Online / Connecting / Offline / Error / Idle)
  - `success` (online) → green
  - `warning` (connecting/offline) → amber
  - `error` → red
  - `muted` (idle) → gray
- **Connection type badge**: "Relay" / "Local" / TCP endpoint
- **Version badge**: e.g. `v1.2.3`

### 5.2 Error Message
When status is `error`, display `snapshot.lastError`.

### 5.3 Connection List (ConnectionsSection)
Each connection as a row, showing:
- Connection label (e.g. `TCP (localhost:17612)`, `Relay (endpoint)`, `Local (/path)`)
- **Latency**:
  - `"... "` → probing (`pending`)
  - `"Timeout"` → unavailable (`unavailable`)
  - `"123ms"` → available, showing RTT
- **Remove** button

### 5.4 Action Area (DaemonSection)
- **Restart daemon**: only available when host is online, calls `daemonClient.restartServer()`
- **Inject Solo tools**: MCP injection toggle

---

## 6. Data Flow Summary

```
┌─────────────────┐     periodic probe     ┌─────────────────────┐
│ HostPage (UI)   │ ◄─── snapshot ─────────│ HostRuntimeController │
│                 │                        │  - runProbeCycle()    │
│ - Status badges │                        │  - ping() RTT         │
│ - Connection    │                        │  - Connection         │
│   list          │                        │    selection/switch   │
│ - Action        │                        └──────────┬──────────┘
│   buttons       │                                   │
└─────────────────┘                                   ▼
                                          ┌─────────────────────┐
                                          │ DaemonClient        │
                                          │  - connect()        │
                                          │  - ping()           │
                                          │  - WebSocket        │
                                          │    transport        │
                                          └─────────────────────┘
```

---

## Key Conclusions

1. **Not passively waiting for connection state changes, but actively polling**: one tick every 2s, dynamically adjusting probe frequency based on connection "age".
2. **Multi-connection parallel probing**: a host can have multiple connection methods (TCP + Relay + Local), and the system probes all connections simultaneously.
3. **Intelligent routing**: defaults to selecting the lowest latency, and supports adaptive switching to faster connections (requires 3 consecutive confirmations to avoid flapping).
4. **serverId verification**: each probe verifies the returned `serverId` matches, preventing connection to the wrong host.
5. **Connection reuse**: when probing the current active connection, reuses the existing client, avoiding repeated connection establishment.

---

## 7. State Persistence Mechanism Conflict Analysis

### 7.1 Problem Description

**Expected logic**: After polling determines a connection is `available`/`online`, the state should **persist until the next probe cycle**, and should not **immediately** become `error`/`offline` due to underlying network glitches.

**Actual mechanism**: Once the underlying WebSocket connection disconnects, `connectionStatus` will **immediately** drop from `online` to `offline` or `error` in real-time, **completely bypassing the next probe cycle's judgment**.

### 7.2 Root Cause of Conflict: Two Independent State Update Channels

| Channel | Trigger Method | Update Target | Frequency |
|------|---------|---------|------|
| **Probe Cycle** | Periodic polling `runProbeCycle()` | `probeByConnectionId` (latency status per connection) | 2s ~ 30s |
| **Connection Status Subscription** | Real-time push `subscribeConnectionStatus()` | `connectionStatus` (host overall online status) | **Immediate** |

The problem lies in the second channel. When `HostRuntimeController` switches to an active connection, it registers a **real-time callback**:

```typescript
// app/src/runtime/host-runtime.ts:1112
this.unsubscribeClientStatus = client.subscribeConnectionStatus((state) => {
  this.applyConnectionEvent({
    type: "client_state",
    state,
    lastError: client.lastError,
  });
  // ...immediately update snapshot
});
```

When DaemonClient's underlying WebSocket disconnects, it immediately pushes `state.status === "disconnected"`, and the state machine transitions accordingly:

```typescript
// app/src/runtime/host-runtime.ts:292-320
function resolveConnectionStateResult(...) {
  const disconnectedReason =
    event.state.status === "disconnected" ? (event.state.reason ?? null) : null;
  const reason = disconnectedReason ?? event.lastError ?? null;
  if (!reason || reason === "client_closed") {
    return { tag: "offline", ... };   // ← no reason or client closed intentionally → offline
  }
  return { tag: "error", message: reason, ... };  // ← has reason → error
}
```

This means:
- Network glitch, TCP timeout, WebSocket close, or any other reason causing underlying disconnection
- The "Online" badge on UI will change to "Offline" or "Error" within **milliseconds**
- Even if the network recovers after 500ms, the user has already seen a status flicker

### 7.3 Additional Aggravating Factor: Auto-reconnect Disabled

```typescript
// app/src/runtime/host-runtime.ts:458
reconnect: { enabled: false }
```

`HostRuntimeController` **disables auto-reconnect** when creating DaemonClient. Therefore, once the underlying connection disconnects, it will not automatically recover, and can only wait for the next `probe cycle` (up to 30s) to attempt re-establishing the connection or switching connections.

### 7.4 Probe Cycle's Recovery Capability

`finalizeProbeCycle` **does** make connection decisions after probing completes, including:
- Switching to another available connection when the active connection is unavailable
- Entering `offline` when all connections are unavailable

But this happens **after probe execution**. Network glitches between probes have no mechanism to "mask" or "delay" this state change.

### 7.5 Suggested Adjustment Directions

To achieve "persist state after polling determination until the next judgment", consider:

1. **Decouple real-time status subscription from direct `connectionStatus` impact**: let `client_state` only be used for internal marking, not immediately drive state machine transitions.
2. **Grant `connectionStatus` change authority exclusively to probe cycle**: `client_state`'s `disconnected` event only affects connection selection during the next probe, rather than immediately dropping the status.
3. **Introduce a "grace period"**: after receiving `disconnected`, don't immediately change to `error`, but first enter a brief `connecting` buffer state, waiting for probe cycle or retry confirmation.
