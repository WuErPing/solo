# `/settings/hosts/` Host 状态检查逻辑分析

## 1. 页面结构

| 文件 | 职责 |
|------|------|
| `app/src/app/settings/hosts/[serverId].tsx` | 路由入口，对应 `/settings/hosts/:serverId` |
| `app/src/screens/settings/host-page.tsx` | 页面 UI，展示状态徽章、连接列表、操作按钮 |
| `app/src/screens/settings-screen.tsx` | 布局容器，侧边栏列出所有 host |

---

## 2. 核心状态检查：Probe Cycle（探测周期）

状态检查逻辑集中在 **`app/src/runtime/host-runtime.ts`** 的 `HostRuntimeController` 中，是一个**定时轮询 + 实时 ping** 的机制。

### 2.1 探测定时策略

```typescript
const PROBE_TICK_MS = 2_000;        // 基础轮询间隔 2s
const PROBE_STEADY_MS = 10_000;     // 在线连接的稳定探测间隔
const PROBE_MAX_BACKOFF_MS = 30_000; // 最大退避间隔
```

对于**非活跃连接**，探测间隔随"首次看到"的时间递增：
- `< 10s` → 每 **2s** 探测
- `< 30s` → 每 **5s** 探测
- `< 60s` → 每 **10s** 探测
- `≥ 60s` → 每 **30s** 探测（最大退避）

对于**当前活跃的在线连接** → 固定每 **10s** 探测一次。

### 2.2 单次探测流程 (`runProbeCycle`)

1. **筛选**本次需要探测的连接（根据上次探测时间和上述间隔策略）
2. 将待探测连接的 probe 状态设为 **`pending`**
3. **并行**对每个连接执行：
   - 如果该连接已经是当前活跃的在线连接，**复用**现有 `DaemonClient`
   - 否则，调用 `connectToDaemon()` **新建一个测试连接**
   - **验证 serverId 是否匹配**（防止连到错误的 host）
   - 调用 `client.ping({ timeoutMs: 5000 })` 测量 **RTT**
   - 成功 → 状态设为 **`available`**，记录 `latencyMs`
   - 失败 → 状态设为 **`unavailable`**
4. 所有探测完成后，调用 `finalizeProbeCycle()` 进行**连接决策**

### 2.3 连接测试底层 (`test-daemon-connection.ts`)

```typescript
// connectToDaemon → connectAndProbe
// 1. 创建 DaemonClient
// 2. 建立 WebSocket 连接
// 3. 等待 serverInfo 消息（含 serverId、hostname、version）
// 4. 返回 { client, serverId, hostname }
```

超时设置：
- **Relay 连接**: 10s
- **Direct 连接**: 6s

支持的 4 种连接类型：

| 类型 | 传输方式 |
|------|----------|
| `directTcp` | WebSocket `ws://host:port` |
| `directSocket` | Unix Domain Socket |
| `directPipe` | Named Pipe |
| `relay` | WebSocket via Relay + E2EE |

---

## 3. 连接选择与切换逻辑

探测完成后，按以下优先级处理：

### 3.1 无活跃连接时
从所有 `available` 的连接中选择**延迟最低**的作为活跃连接。

### 3.2 活跃连接失效时
如果当前活跃连接探测结果为 `unavailable`，立即切换到次优的可用连接。

### 3.3 自适应切换（Adaptive Switching）
如果存在另一个连接的延迟**持续优于**当前连接 ≥**40ms**，且**连续 3 次探测**都满足条件，则自动切换到更快连接。

```typescript
const ADAPTIVE_SWITCH_THRESHOLD_MS = 40;
const ADAPTIVE_SWITCH_CONSECUTIVE_PROBES = 3;
```

### 3.4 最优连接算法 (`connection-selection.ts`)

简单遍历所有 `available` 的连接，选择 `latencyMs` 最小的。

---

## 4. 状态机

`HostRuntimeConnectionStatus` 有 5 种状态：

```typescript
"idle" | "connecting" | "online" | "offline" | "error"
```

转换路径：
- `booting` → `connecting` → `online` / `offline` / `error`
- 触发事件：`select_connection`, `client_state`, `connect_failed`, `no_connections`, `stopped`

---

## 5. UI 展示 (`host-page.tsx`)

### 5.1 顶部状态徽章 (Identity Badges)
- **状态胶囊**：彩色圆点 + 文字（Online / Connecting / Offline / Error / Idle）
  - `success` (online) → 绿色
  - `warning` (connecting/offline) → 琥珀色
  - `error` → 红色
  - `muted` (idle) → 灰色
- **连接类型徽章**："Relay" / "Local" / TCP 端点
- **版本徽章**：如 `v1.2.3`

### 5.2 错误信息
当状态为 `error` 时，展示 `snapshot.lastError`。

### 5.3 连接列表 (ConnectionsSection)
每个连接一行，显示：
- 连接标签（如 `TCP (localhost:17612)`、`Relay (endpoint)`、`Local (/path)`）
- **延迟**：
  - `"... "` → 探测中 (`pending`)
  - `"Timeout"` → 不可用 (`unavailable`)
  - `"123ms"` → 可用，显示 RTT
- **Remove** 按钮

### 5.4 操作区 (DaemonSection)
- **Restart daemon**：仅在 host 在线时可用，调用 `daemonClient.restartServer()`
- **Inject Solo tools**：MCP 注入开关

---

## 6. 数据流总结

```
┌─────────────────┐     定时 probe     ┌─────────────────────┐
│ HostPage (UI)   │ ◄─── snapshot ─────│ HostRuntimeController │
│                 │                    │  - runProbeCycle()    │
│ - 状态徽章       │                    │  - ping() RTT         │
│ - 连接列表       │                    │  - 连接选择/切换       │
│ - 操作按钮       │                    └──────────┬──────────┘
└─────────────────┘                               │
                                                  ▼
                                        ┌─────────────────────┐
                                        │ DaemonClient        │
                                        │  - connect()        │
                                        │  - ping()           │
                                        │  - WebSocket 传输   │
                                        └─────────────────────┘
```

---

## 关键结论

1. **不是被动等待连接状态变化，而是主动轮询**：每 2s 一次 tick，根据连接"年龄"动态调整探测频率。
2. **多连接并行探测**：一个 host 可以有多个连接方式（TCP + Relay + Local），系统会同时探测所有连接。
3. **智能选路**：默认选延迟最低的，且支持自适应切换到更快连接（需连续 3 次确认，避免抖动）。
4. **serverId 验证**：每次探测都会验证返回的 `serverId` 是否匹配，防止连错 host。
5. **连接复用**：对当前活跃连接探测时复用已有 client，不会重复建连。

---

## 7. 状态保持机制冲突分析

### 7.1 问题描述

**期望逻辑**：轮询判定连接 `available`/`online` 后，状态应该**保持到下一次探测周期**，不应因底层网络闪断而**立刻**变成 `error`/`offline`。

**实际机制**：一旦底层 WebSocket 连接断开，`connectionStatus` 会**实时、立即**从 `online` 跌落到 `offline` 或 `error`，**完全绕过 probe cycle 的下一次判断**。

### 7.2 冲突根因：两条独立的状态更新通道

| 通道 | 触发方式 | 更新目标 | 频率 |
|------|---------|---------|------|
| **Probe Cycle** | 定时轮询 `runProbeCycle()` | `probeByConnectionId`（各连接的延迟状态） | 2s ~ 30s |
| **Connection Status 订阅** | 实时推送 `subscribeConnectionStatus()` | `connectionStatus`（host 整体在线状态） | **即时** |

问题出在第二条通道。当 `HostRuntimeController` 切换到某个活跃连接后，会注册一个**实时回调**：

```typescript
// app/src/runtime/host-runtime.ts:1112
this.unsubscribeClientStatus = client.subscribeConnectionStatus((state) => {
  this.applyConnectionEvent({
    type: "client_state",
    state,
    lastError: client.lastError,
  });
  // ...立即更新 snapshot
});
```

当 DaemonClient 的 WebSocket 底层断开时，会立即推送 `state.status === "disconnected"`，状态机随之转换：

```typescript
// app/src/runtime/host-runtime.ts:292-320
function resolveConnectionStateResult(...) {
  const disconnectedReason =
    event.state.status === "disconnected" ? (event.state.reason ?? null) : null;
  const reason = disconnectedReason ?? event.lastError ?? null;
  if (!reason || reason === "client_closed") {
    return { tag: "offline", ... };   // ← 无 reason 或客户端主动关闭 → offline
  }
  return { tag: "error", message: reason, ... };  // ← 有 reason → error
}
```

这意味着：
- 网络闪断、TCP 超时、WebSocket close 等任何原因导致底层断开
- UI 上的 "Online" 徽章会在**毫秒级**变成 "Offline" 或 "Error"
- 即使 500ms 后网络恢复，用户也已看到了一次状态跳动

### 7.3 额外加剧因素：关闭了自动重连

```typescript
// app/src/runtime/host-runtime.ts:458
reconnect: { enabled: false }
```

`HostRuntimeController` 在创建 DaemonClient 时**禁用了自动重连**。因此底层一旦断开，不会自动恢复，只能等待下一次 `probe cycle`（最长 30s）去尝试重新建立连接或切换连接。

### 7.4 Probe Cycle 的修复能力

`finalizeProbeCycle` **确实**会在探测完成后做连接决策，包括：
- 活跃连接不可用时切换到其他可用连接
- 所有连接不可用时进入 `offline`

但这发生在**探测执行后**。在两次探测之间的网络闪断，没有机制去"屏蔽"或"延迟"这个状态变化。

### 7.5 建议调整方向

若要实现"轮询判定后保持状态到下一次判断"，可以考虑：

1. **断开实时状态订阅对 `connectionStatus` 的直接影响**：让 `client_state` 仅用于内部标记，不立即驱动状态机跳转。
2. **由 probe cycle 独占 `connectionStatus` 的变更权**：`client_state` 的 `disconnected` 事件只影响下一次 probe 时的连接选择，而不是立即把状态打下来。
3. **引入"宽限期"（grace period）**：收到 `disconnected` 后不立即变 `error`，而是先进入一个短暂的 `connecting` 缓冲状态，等待 probe cycle 或重试确认。
