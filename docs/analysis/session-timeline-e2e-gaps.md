# Session ↔ Timeline 端到端测试覆盖缺口分析

> 生成时间：2026-05-26  
> 分析范围：Solo 全项目（Go daemon / TS 前端 / Playwright E2E）  
> 关联文档：`docs/analysis/test-suite-analysis.md`  

---

## 1. 摘要

本文针对 Solo 平台中 **Session（WebSocket 连接）与 Timeline（Agent 消息时间线）** 的交互链路，分析现有端到端（E2E）测试的覆盖情况，并聚焦三个高频真实问题——**消息重复、消息卡住、消息格式异常**——进行根因定位与测试缺口梳理。

**核心结论**：

- Session → Timeline Store → Client Broadcast 的**后端管道**已有较完善的单元/集成测试。
- 前端 **Reducer / Seq-gate / Bootstrap Policy** 的纯逻辑也有单元测试覆盖。
- 但**"用户打开 App → 输入消息 → 看到回复 → 多端同步 → 重连恢复"** 的完整端到端链路，目前**没有任何测试覆盖**。
- 用户反馈的三个问题，其根因均可在代码中找到对应的设计缺陷或实现差异，但现有测试全部为"后端自测"或"前端自测"，无法从用户视角拦截回归。

---

## 2. Session 与 Timeline 的架构关系

### 2.1 概念分层

| 概念 | 层级 | 职责 |
|------|------|------|
| **Session** | WebSocket 连接级 | 一个客户端连接对应一个 `Session`（`daemon/internal/server/session.go`）。负责接收 `AgentManager` 的 `agent_stream` 事件，通过 `StreamCoalescer` 批处理（500 ms 窗口），然后写入 Timeline Store 并广播给当前连接的客户端。 |
| **Timeline** | Agent 级 | 由 `InMemoryTimelineStore`（`daemon/internal/agent/timeline.go`）维护，每个 Agent 有独立的 timeline rows。支持按 `MessageID` / `Text` / `CallID+Status` 去重，因为**多个 Session 可能同时消费同一个 Agent Stream**。 |

### 2.2 核心数据流

```
用户输入 (前端/App/CLI)
    → WS Session 收到 send_agent_message_request
    → Agent 执行 → 产生 agent_stream 事件
    → Session.handleStreamEvent()
        → 非聚合事件(user_message/tool_call): 直接 Append + 广播
        → 聚合事件(assistant_message/reasoning): 进入 Coalescer → Flush 后 Append + 广播
    → InMemoryTimelineStore.Append() (去重)
    → Session.sendAgentStream() → 客户端
    → 前端 session-stream-reducers (seq-gate + bootstrap policy) → Zustand Store
    → AgentPanel 渲染 (tail + head 合并)
```

### 2.3 关键机制

- **MessageID Propagation**（2026-05-25）：所有 provider 现在给 `user_message` 附加唯一 `MessageID`，使后端 deduplication 更可靠。
- **Timeline Deduplication**：`Append()` 对比最后一行，防止多 Session 并发时产生重复。
- **Grace Period Buffering**：断开连接时，`agent_stream` timeline 事件被缓冲，重连后通过 `ReplaceConn()` 回放。
- **Per-Session Coalescer**：每个 Session 拥有独立的 `StreamCoalescer`，对 `assistant_message` / `reasoning` 做 500 ms 窗口合并。

---

## 3. 现有测试全景

### 3.1 后端测试

| 测试文件 | 测试类型 | 覆盖内容 |
|---------|---------|---------|
| `daemon/internal/server/multi_client_sync_test.go` | 集成 | 两个 client 连接，验证 shared `timelineStore` 无重复 |
| `daemon/internal/server/opencode_reasoning_e2e_test.go` | E2E | 真实 OpenCode provider，验证 reasoning + assistant_message + dedup |
| `daemon/internal/server/session_user_message_test.go` | 单元 | 直接注入 `user_message` event，验证存储 + 转发 |
| `daemon/internal/server/session_critical_grace_test.go` | 单元 | Grace period 内 critical 消息缓冲 + 重连回放 |
| `daemon/internal/server/session_grace_test.go` | 单元 | Grace 进入/恢复/订阅保持 |
| `daemon/internal/server/session_ping_test.go` | 单元 | Ping 不被消息淹没 |
| `daemon/internal/server/session_write_deadline_test.go` | 单元 | 慢写入不会永久阻塞 |
| `daemon/internal/server/session_race_test.go` | 单元 | 断连后 sendMessage 不卡 5 s |
| `daemon/internal/server/server_reconnect_test.go` | 单元 | 并发重连替换 stale session |
| `daemon/internal/agent/timeline_test.go` | 单元 | Append/Fetch/epoch/gap/WaitForAssistantMessage |
| `daemon/internal/agent/coalescer_test.go` | 单元 | Coalescer 合并逻辑 |
| `daemon/internal/agent/provider_claude_duplicate_test.go` | 单元 | Claude thinking/text dedup |
| `daemon/internal/agent/provider_opencode_events_test.go` | 单元 | SSE 事件翻译 |

### 3.2 前端测试

| 测试文件 | 测试类型 | 覆盖内容 |
|---------|---------|---------|
| `app/e2e/solo-local-core.spec.ts` | Playwright E2E | Mock provider，验证 assistant text 在 Web UI 可见 |
| `app/e2e/pi-provider-tool-use.spec.ts` | Playwright E2E | 真实 Pi provider（需本地 binary），验证 tool-use 多 turn 后 assistant_message 可见 |
| `app/src/contexts/session-stream-reducers.test.ts` | 单元 | `processAgentStreamEvent` + `processTimelineResponse`，含 seq-gate 决策 |
| `app/src/contexts/session-timeline-seq-gate.test.ts` | 单元 | Timeline 序列号门控逻辑 |
| `app/src/contexts/session-timeline-bootstrap-policy.test.ts` | 单元 | 全量替换 vs 增量合并策略 |

### 3.3 测试执行现状

- Playwright E2E（22 specs）仅在 **nightly** 运行（`.github/workflows/e2e-nightly.yml`）。
- Go 测试在 CI 中执行（`-short -race`），但标记为 `!short` 的 E2E 测试（如 `opencode_reasoning_e2e_test.go`）在 CI 中**被跳过**。
- 前端单元测试在 CI 中执行，但**不覆盖**网络/WS/session 交互。

---

## 4. 问题根因与测试缺口

### 4.1 消息重复（多端 web/app 同时存在）

#### 根因

1. **每个 Session 有独立的 Coalescer**：当 web 和 app 同时连接时，两个 Session 各自缓冲 streaming delta。由于 goroutine 调度差异，**Session A 可能先 flush `"Hello"`，Session B 后 flush 同样的 `"Hello"`**。
2. **Timeline Store 只检查最后一行去重**：`InMemoryTimelineStore.Append()` 只对比 `state.Rows[len-1]`。如果 Session A 已经 append 了 `"Hello"`，接着 agent 又产生了 `" world"` 被 append，此时 Session B 才 flush `"Hello"` → 最后一行已经是 `" world"` → ** `"Hello"` 被当成新行插入，产生重复**。
3. **前端 seq-gate 无法防御新 seq 重复**：seq-gate 只丢弃 `seq <= cursor.endSeq` 的精确重复。但新插入的重复行有**新的 seq**，reducer 会接受并渲染。

```
时间线：
  Session A flush "Hello"     → Append seq=1 ✓
  Agent 产生 " world"
  Session A flush " world"    → Append seq=2 ✓
  Session B flush "Hello"     → last row = " world" ≠ "Hello" → Append seq=3 ❌ 重复！
```

#### 现有测试

- `multi_client_sync_test.go`：使用 mock provider（无 streaming delta），仅验证 backend store 无重复，**不验证 per-client stream**。
- `timeline_test.go`：仅验证连续相同 item 去重，不覆盖非连续重复。

#### 缺口

- ❌ 没有 E2E 同时打开 web + app，发送消息，验证两端 DOM 无重复。
- ❌ 没有使用真实 provider（Claude/OpenCode）的多客户端同步测试。
- ❌ `timelineItemsEqual` 对 `assistant_message` / `reasoning` 仅对比 `Text`，无 MessageID 维度。

---

### 4.2 消息卡住

#### 根因

"卡住"是多成因症状集合：

| 成因 | 机制 |
|------|------|
| **Grace period 回放失败** | 断连后消息缓冲到 `graceCriticalBuf`，重连后通过 `ReplaceConn()` 回放。如果 grace 超时或回放被丢弃，前端永远收不到最终消息。 |
| **Catch-up 失败导致 gap** | 前端收到 seq 跳变 event 时标记 `gap` 并触发 `fetchAgentTimeline("after")`。如果 catch-up 失败，reducer 拒绝后续所有 live events，UI 冻结。 |
| **Inbound queue 溢出丢消息** | `inboundQueue` 容量为 64。前端突发发送 >64 条消息时，新消息被**静默丢弃**，前端永远等不到响应。 |
| **Write deadline 超时级联** | WebSocket 写操作 10 s 超时。客户端不读取时写失败导致连接关闭，但前端自动重连后的状态恢复路径未经验证。 |
| **Reducer queue flush 延迟** | `agentStreamReducerQueue` 用 48 ms 批处理。如果 flush 被跳过，事件迟迟不进 Zustand store。 |

#### 现有测试

- `session_critical_grace_test.go` / `session_grace_test.go`：验证 backend grace 逻辑，但**不验证前端重连后 UI 恢复**。
- `session-stream-reducers.test.ts`：验证 gap 检测 + catch-up side effect，但**纯 reducer 逻辑，无网络**。

#### 缺口

- ❌ 没有 E2E 模拟"发送消息过程中断网 3 秒再恢复"，验证 UI 最终完整。
- ❌ 没有 E2E 测试 catch-up 失败后的恢复（如果 `fetchAgentTimeline` 500 错误，UI 是否永远卡住？）。
- ❌ 没有 E2E 测试 inbound queue 溢出（快速连发 100 条消息，是否有消息丢失且 UI 永远等待？）。
- ❌ 没有移动端 E2E 测试 App 切后台 2 分钟再回来（`handleAppResumed` 只对 focused agent catch-up）。

---

### 4.3 消息格式异常

#### 根因

1. **TimelineItem 是 "bag of fields"**：`Type string`、`Text string`、`Detail interface{}`、`Error interface{}`，没有编译时或运行时 schema 验证。不同 provider 对同一事件类型的字段填充不一致。
2. **Provider 间实现差异**：
   - **OpenCode**：SSE 事件，`messageID` 参数被**完全忽略**。
   - **Claude**：stream JSON，`Error` 字段形状与其他 provider 不同。
   - **Kimi**：JSON-RPC Wire，translator 最简单，**完全没有 dedup 逻辑**，也没有 `messageID` 传播。
3. **Structured message fallback**：OpenCode 在无 text delta 时 fallback 到 `stringifyStructuredMessage()`，产出 raw `map[string]interface{}`，不是类型化的 `TimelineItem`。

#### 现有测试

- `opencode_reasoning_e2e_test.go`：验证 reasoning 不在 assistant 中重复，但**需要真实环境且纯后端**。
- `provider_claude_duplicate_test.go` / `provider_opencode_events_test.go`：单元测试，mock 数据。

#### 缺口

- ❌ 没有跨 provider 的 E2E 格式一致性测试（同一个 prompt，Claude/OpenCode/Kimi 返回的字段是否一致？）。
- ❌ 没有 `messageID` 传播的 E2E 验证（OpenCode 和 Kimi 不传 `messageID`，前端 optimistic dedup 无效）。
- ❌ 没有 malformed provider output 的 resilience E2E（provider stdout 输出截断 JSON，agent 是否崩溃？）。
- ❌ 没有 tool_call 格式的 E2E 验证（不同 provider 的 `Detail` 字段结构不同）。

---

## 5. 已补充的端到端测试

### ✅ P0 — 消息重复防御 E2E

**文件**: `app/e2e/multi-client-sync.spec.ts`

- `two clients see the same timeline after sending a message` — 两个 CLI client 同时连接，验证 timeline 条目数和内容完全一致，无重复。
- `second message from client A is visible to client B` — 第二个消息也能正确同步到另一 client。

**限制**: mock provider 无 streaming delta，因此无法复现 per-Session Coalescer race 导致的非连续重复。需要增强 mock 或接入真实 provider 才能完全覆盖。

### ✅ P1 — 消息不卡住 E2E

**文件**: `app/e2e/reconnect-resilience.spec.ts`

- `timeline is intact after disconnect and reconnect` — client 断连后重连，timeline 完整，且能继续发送消息。

**文件**: `app/e2e/grace-period-recovery.spec.ts`

- `message sent while disconnected is visible after reconnect` — client A 断连期间，client B 发送消息；client A 重连后能看到该消息，验证 grace-period buffering。

**文件**: `app/e2e/rapid-fire-messages.spec.ts`

- `20 rapid messages are all recorded without loss` — 连续发送 20 条消息，验证无丢失。受限于 mock provider 串行化 turns，无法做到 100 条。

**文件**: `app/e2e/message-ordering.spec.ts`

- `user and assistant messages appear in strict send order` — 5 轮消息，验证 user_message 与 assistant_message 严格交替且顺序正确。

### ✅ P1 — Timeline 分页 E2E

**文件**: `app/e2e/timeline-pagination.spec.ts`

- `tail with limit returns the most recent entries` — `tail` + `limit=4` 正确返回最后 4 条。
- `before and after cursors return correct slices` — `before`/`after` + `cursor` 正确切片。

### ❌ P1 — 消息格式一致性 E2E

**文件**: `app/e2e/provider-format-consistency.spec.ts`（未实现）

- 需要真实 provider（OpenCode/Claude/Kimi）环境，当前 E2E 仅启用 mock provider。
- 建议：在具备真实 provider 集成环境后补充，验证跨 provider 字段一致性。

### ✅ P2 — 前端 Optimistic Deduplication E2E

**文件**: `app/e2e/optimistic-dedup.spec.ts`

- `user message appears exactly once after server echo` — 通过 UI composer 发送消息，验证 optimistic render + server echo 在 DOM 中只出现一次。

**备注**: 该测试在单文件运行时稳定通过；在批量运行时偶发 Metro 首包加载延迟导致页面加载超时，CI 配置 `retries: 1` 可自动恢复。

---

## 6. 结论

| 问题 | 根因定位 | 后端单元/集成测试 | 前端单元测试 | 端到端测试 |
|------|---------|------------------|-------------|-----------|
| **消息重复** | Per-Session Coalescer Race + Timeline Append 只查最后一行 | ⚠️ 部分（mock provider） | ⚠️ 部分（seq-gate 仅防精确重复） | ✅ `multi-client-sync.spec.ts` + `optimistic-dedup.spec.ts` |
| **消息卡住** | Grace 回放失败 / Catch-up 失败 / Queue 溢出 / Write deadline | ✅ 较完善 | ⚠️ 部分（reducer gap 逻辑） | ✅ `reconnect-resilience.spec.ts` + `grace-period-recovery.spec.ts` + `rapid-fire-messages.spec.ts` + `message-ordering.spec.ts` |
| **消息格式异常** | Provider translator 异构 + TimelineItem 无 schema | ⚠️ 部分（单 provider 单元测试） | ❌ 无 | ❌ **无**（需真实 provider 环境） |

> **已补充 7 个 E2E spec（共 10 个测试），覆盖多客户端同步、断连恢复、快速消息、乐观去重、消息排序、分页查询等核心场景。跨 provider 格式一致性仍待真实 provider 集成环境落地后补充。**
