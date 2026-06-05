# `Transport not connected (status: disposed)` 深度分析

> 问题：app tmux 窗口反复出现 `Transport not connected (status: disposed)`。
> 日期：2026-06-05
> 范围：`app-bridge/src/client/daemon-client.ts`、`app/src/runtime/host-runtime.ts`、`app/src/hooks/use-tmux-*.ts`、`app/src/screens/tmux-pane-screen.tsx`

## 一、错误抛出点（仅 4 处，全部在 `daemon-client.ts`）

| 位置 | 含义 |
|---|---|
| `daemon-client.ts:1099` | `sendSessionMessage`（fire-and-forget）发送时发现状态非 connected |
| `daemon-client.ts:1117` | `sendBinaryFrame` 同上 |
| `daemon-client.ts:1162` | **`sendSessionMessageOrThrow`** 发送 RPC 请求时状态非 connected/connecting |
| `daemon-client.ts:1307` | 等待响应时超时 |

tmux 的所有 RPC（`tmuxCapturePane` / `tmuxSendKeys` / `tmuxListAgents` / `tmuxGetTheme`）走 `sendCorrelatedSessionRequest` → `sendSessionMessageOrThrow`，所以报错位置是 `:1162`，触发条件：**`connectionState.status === "disposed"`**。

## 二、`disposed` 状态的来源（唯一入口）

```ts
// daemon-client.ts:966-994
async close() {
  ...
  this.disposeTransport(1000, "Client closed");
  this.clearWaiters(new Error("Daemon client closed"));   // 拒掉所有 in-flight waiter
  this.rejectPendingSendQueue(new Error("Daemon client closed"));
  ...
  this.updateConnectionState(
    { status: "disposed" },
    { event: "DISPOSE", reason: "Client closed", reasonCode: "disposed" },
  );
}
```

`close()` 是 `disposed` 的**唯一**赋值入口。`disposed` 是终态，不可逆（`daemon-client.ts:781/802/967` 都直接 return 或 throw，没有任何出口转回 `connecting`）。

## 三、谁在调用 `close()`（网络路径上的触发链）

`host-runtime.ts` 是 `DaemonClient` 生命周期的主人。关键调用：

| 位置 | 触发时机 |
|---|---|
| `:586` `stop()` | Host 被移除 |
| `:867/903` `runProbeCycle` | probe 失败，或新 client 未被激活 → 关掉临时 probe client |
| **`:1032` `disposePreviousActiveClient`** | **`switchToConnection` 激活新连接时，把旧 activeClient `close()`** ← 嫌疑最大 |
| `:991/1094` | switch 被更高 version 抢占，临时 client 被销毁 |
| `:1570` | `activateConnection` 失败兜底 |

注意 `reconnect: { enabled: false }`（`host-runtime.ts:461`），所以 `DaemonClient` 一旦 transport 断开就 `disconnected` → `offline`，不会自重连，只能等 probe 周期把它替换。

## 四、Probe 循环 —— 每 2 秒一次的"夺命轮询"

```ts
// host-runtime.ts:667-909 runProbeCycle
PROBE_TICK_MS = 2_000                          // 每 2s 启动一轮
ADAPTIVE_SWITCH_THRESHOLD_MS = 40              // 延迟差 ≥40ms 就考虑切换
ADAPTIVE_SWITCH_CONSECUTIVE_PROBES = 3         // 连续 3 轮命中就切换
```

每轮对每个 connection：

1. 若已是 active+online → 复用现有 client
2. 否则 `connectToDaemon(...)` **新建一个 DaemonClient**，连接成功后调 `ping({ timeoutMs: 5000 })`
3. 结束后：**`if (connectedClient && shouldCloseClient) await connectedClient.close()`**（`:903`）

`finalizeProbeCycle` 的切换逻辑（`:746-836`）：

```
无 active connection        → switchToConnection(best)
active probe = unavailable  → switchToConnection(best)   ← 关键！一次 unavailable 就换
connectionStatus !== online → switchToConnection(fastest)
另一条连接连续 3 次快 ≥40ms → switchToConnection(fastest)
```

切换内部：

```ts
// :1066-1098 switchToConnection
const requestVersion = ++this.switchRequestVersion;
await this.disposePreviousActiveClient();   // 旧 client.close() → disposed
...
this.activeClient = client;                 // 快照更新 client 引用
```

## 五、竞态路径（"disposed" 反复出现的真实链路）

```
T0   UI: useTmuxCapturePane → queryFn 开始
       liveClient = store.getClient(serverId) → ClientA (status=connected) ✓
       await liveClient.tmuxCapturePane(paneId)
       → sendSessionMessageOrThrow (status=connected) → waiter 入队

T1   Probe 周期触发（每 2s）
       探测 ClientA 所在连接 → ping 超时 / transport 抖动
       probeByConnectionId.set(id, { status: "unavailable" })

T2   finalizeProbeCycle:  active probe = unavailable
       → selectBestConnection → 选另一条连接（relay/socket/pipe）
       → switchToConnection(otherId)
         → disposePreviousActiveClient()
           → ClientA.close()
             → status := "disposed"          ← 状态机终态
             → clearWaiters("Daemon client closed")
             → rejectPendingSendQueue(...)

T3   UI waiter 被 reject（"Daemon client closed"）
       或 sendSessionMessageOrThrow 直接 throw
       "Transport not connected (status: disposed)"
       → useQuery error → tmux pane 显示错误

T4   store 快照更新为新 ClientB，UI 重新渲染，下一轮 queryFn 拿 ClientB → 成功
```

任何"刚拿到 client 引用 → 调 RPC → client 在 await 期间被切换掉"的请求都会报 disposed。tmux 场景特别容易命中：

- `useTmuxCapturePane` `refetchInterval: 5_000`（`:35`）—— 每 5 秒一次 RPC
- `useTmuxAgents` 同样的 polling
- `tmuxSendKeys` 在 `handleSend` 里 `getClient()` 拿到引用后才 `await tmuxSendKeys`（`tmux-pane-screen.tsx:108-117`）
- 这些 polling 和用户点击的时序与 probe 周期异步，撞车概率非零

## 六、为什么"仍然"存在（根因）

1. **切换太激进**：active probe 一次 `unavailable` 就换（`:770`），ping 5s 超时很容易误判。
2. **旧 client 立即处决**：`disposePreviousActiveClient` 直接 `close()`，没有任何 grace window 让 in-flight RPC 重发到新 client。
3. **UI 层引用过期**：`getClient()` 返回的是一个快照对象，await 期间就可能被替换。React Query 的 `queryFn` 不感知"client 换了"，只会把 disposed 当成 error 上报。
4. **`reconnect: false`**：旧 client 一旦 transport 抖动就是 `disconnected → offline`，probe 必须介入；没有"原地自愈"，放大了切换频率。
5. **`disposed` 不可复活**：状态机把 disposed 设计成终态（吸收态），无法回到 connecting。设计上没问题，但配合"频繁切换 + UI 缓存旧引用"就让错误表面化。

## 七、最小修复方向（按代价排序）

| 方案 | 改动 | 效果 |
|---|---|---|
| **A. UI 层 retry with live client** | `useTmuxCapturePane` / `useTmuxAgents` / `tmux-pane-screen.handleSend` 在 catch 到 `disposed` 时 `store.getClient()` 重新获取 client 并重试一次 | 最小侵入，立刻消除表面错误 |
| **B. probe 切换更稳健** | `unavailable` 要求连续 2 次才切换；active client 加 `graceMs`（如 10s 内不切走）；提高 `ADAPTIVE_SWITCH_THRESHOLD_MS` 到 100ms | 从源头减少切换次数 |
| **C. in-flight RPC 跨 client 重发** | `DaemonClient.sendRequest` 识别 close/disposed 时把 waiter 转给"下一个 active client"（需要 runtime 层协调） | 最彻底，但改动大 |
| **D. 给 active client 开 reconnect** | 在 `createDefaultDeps` 为 active client 设 `reconnect: { enabled: true, baseDelayMs: 500 }` | transport 抖动可自愈，减少 probe 介入 |

短期建议：先做 A（UI 层 disposed 自动 retry 一次），同时做 B 的"连续 N 次 unavailable 才切换"，基本可以消除这个错误。C 是架构层优化，留作后续。

---

## 八、实施记录（2026-06-05）

Plan A + Plan B 以 TDD 落地，详见 `/Users/wuerping/.qoder/plans/northern-dune-crane.md`。

### Plan B（源头）
- `host-runtime.ts`：新增 `consecutiveUnavailableThreshold`（生产默认 `2`，测试可配置），新增 `connectionConsecutiveUnavailable` 跟踪 map；active-probe unavailable 分支改为累计达阈值才切换；available 或 `disposePreviousActiveClient`/`stop()` 时复位计数；`ADAPTIVE_SWITCH_THRESHOLD_MS` 40 → 100。
- `host-runtime.test.ts`：新增 3 个测试（"single-cycle does not fail over (default)"、"fails over after two consecutive cycles"、"resets counter on recovery"）；现有 `fails over when the active client ping fails` 改为显式 opt-in `consecutiveUnavailableThreshold: 1`；`consecutive probes` / `transient spike` 两个测试的延迟差放大到 ≥ 100 ms，与新阈值匹配。

### Plan A（表面）
- 新增 `app/src/utils/tmux-rpc.ts`：共享 helper `withLiveTmuxClient(serverId, op)` —— 第一次调用抛"disposed"且 store 返回新 client 时自动重试一次，单点重试、不无限循环。
- 新增 `app/src/utils/tmux-rpc.test.ts`：8 个用例覆盖首次成功、disposed 重试、同实例不重试、store 无 client、非 disposed 错误不重试、首次无 client、首次已 disposed、mid-flight disposed 但错误消息通用等边界。
- `use-tmux-capture-pane.ts` + `use-tmux-agents.ts` 的 `queryFn` 改用 helper；`use-tmux-capture-pane.test.ts` 增加 1 个透明重试集成用例。
- `tmux-pane-screen.tsx` 的 `handleSend` / `sendKey` 改用 helper；对应 mock 扩展了 `getConnectionState`，现有 8 个 UI 用例全部通过。

### 验证
- 全量 `vitest run src/`：223 文件 / 1472 用例全部通过。
- `tsc --noEmit`：改动前 1030 个错误 → 改动后 1030 个错误（`tmux-rpc.ts` 0 个，所有新增/修改文件未引入新错误）。
- `expo lint --max-warnings 0`：改动引入 0 个新 warning（已有的 3 个 warning 来自未触碰文件，遗留）。
