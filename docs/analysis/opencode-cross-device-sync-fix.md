# OpenCode 跨客户端同步修复记录

> Date: 2026-06-08
> Scope: `daemon/internal/agent/provider_opencode_session.go`, `daemon/internal/agent/timeline.go`, `protocol/stream_event.go`
> Symptom: Web 发送消息后，App 看不到 `user_message` prompt；Web 刷新后出现重复的 assistant response

---

## 1. 现象

### 问题 A：非发消息端看不到 prompt
- 打开 Solo App，进入某个 agent
- 打开 Solo Web，进入同一个 agent
- **在 Web 端发送** `who are you`
- App 端能看到 assistant 回复，但**看不到 `who are you` 这条 prompt**

### 问题 B：后连接客户端看到重复回复
- Web 端已经聊了几句
- 刷新 Web 页面（重新建立 WebSocket Session）
- timeline hydrate 后，assistant response **出现两条重复内容**

---

## 2. 根因

### 2.1 OpenCode 不实时返回 `user_message`

对比各 provider 的 `Run()` 实现：

| Provider | `Run()` 开始时是否 emit `user_message` |
|---|---|
| mock | ✅ |
| claude | ✅ |
| kimi | ✅ |
| pi | ✅ |
| **opencode** | **❌ 没有** |

OpenCode 的 SSE 流只返回 assistant 侧事件，不会把 user prompt 带回来。`user_message` 只在 `StreamHistory()`（从 opencode server 拉全量消息历史）时才会被补进 timeline store。

这导致：
- Web 端自己发消息时有 optimistic update，能看到 prompt（**客户端本地**）
- App 端只收到 assistant SSE 事件，**永远收不到 `user_message` 实时事件**
- 如果 App 之前已经 `fetch_agent_timeline` 过，`historyPrimed = true`，后续 hydrate 是 no-op，prompt 彻底丢失

### 2.2 `AppendFromHistory` 去重只查最后一行

```go
func (s *InMemoryTimelineStore) Append(agentID string, item TimelineItem) TimelineItem {
    if len(state.Rows) > 0 {
        last := state.Rows[len(state.Rows)-1]
        if timelineItemsEqual(last.Item, item) {
            return last
        }
    }
    // append new row
}
```

live 事件与历史事件可能**交错**：

```
Live:    user_message(1) → assistant_message(2)
History: user_message(1) → assistant_message(2)
```

history 的 `user_message(1)` 进来时，最后一行已经是 live 的 `assistant_message(2)`，不匹配，被插入。接着 history 的 `assistant_message(2)` 进来，最后一行是刚插的 history `user_message(1)`，也不匹配，再插一条。

结果：Web 刷新时 timeline 中出现重复的 assistant response。

---

## 3. 修复方案

### 修复 1：opencode `Run()` / `StartTurn()` 主动 emit `user_message`

在启动 SSE 消费之前，先把 user prompt 作为 `TimelineStreamEvent` 广播出去：

```go
if text != "" {
    s.notifySubscribers(AgentStreamEvent{
        Event: protocol.TimelineStreamEvent{
            Item:     TimelineItem{Type: "user_message", Text: text, MessageID: messageID},
            Provider: opencodeProviderName,
            TurnID:   turnID,
        },
        Timestamp: time.Now(),
    })
}
```

效果：
- 所有在线 client（包括 app）实时收到 prompt
- timeline store 立即写入，不再依赖后续 `StreamHistory`
- `MessageID` 帮助 client 端 optimistic 去重

### 修复 2：`AppendFromHistory` 全表扫描去重

```go
func (s *InMemoryTimelineStore) AppendFromHistory(agentID string, item interface{}) {
    // ... convert to TimelineItem ...
    s.mu.Lock()
    defer s.mu.Unlock()

    state := s.getOrCreateStateLocked(agentID)
    for i := len(state.Rows) - 1; i >= 0; i-- {
        if timelineItemsEqual(state.Rows[i].Item, ti) {
            return
        }
    }
    s.appendLocked(state, agentID, ti)
}
```

- `Append`（live 路径）保持原语义：只检查最后一行，性能不变
- `AppendFromHistory`（历史回填路径）扫描所有 row，避免插入与 live 事件交错的重复项

### 修复 3：`session_closed` 使用独立事件类型

之前 opencode `Close()` 把 `session_closed` 包进 `TimelineStreamEvent{Item: {Type: "session_closed"}}`，导致 app 端 Zod parse 失败，WebSocket 消息被丢弃。

修复：
- `protocol/stream_event.go` 新增 `SessionClosedStreamEvent`
- opencode `Close()` 改为 emit 顶层 `session_closed`
- `session_agent_stream.go` 增加对应 case 转发

---

## 4. 回归测试

新增 `daemon/internal/agent/timeline_test.go`：

| Test | 覆盖点 |
|---|---|
| `TestAppendFromHistory_DeduplicatesNonAdjacentLiveEvents` | history hydration 不插入与 live 事件非相邻重复的项 |
| `TestAppendFromHistory_DeduplicatesDuplicateHistoryEntries` | history batch 内部重复去重 |
| `TestAppend_DoesNotDeduplicateNonAdjacentLiveDuplicates` | `Append()` 仍允许非相邻的相同内容（如用户两次发 "hi"） |
| `session-stream-reducers.test.ts` 新增 case | bootstrap tail init 保留 `user_message` |
| `protocol/stream_event_test.go` | `SessionClosedStreamEvent` marshal/unmarshal round-trip |

---

## 5. 验证清单

- [x] `go test -short -race ./protocol/... ./daemon/... ./cli/... ./relay-go/...` 通过
- [x] `cd app-bridge && npm test` 通过
- [x] `cd app && npm test -- session-stream-reducers` 通过
- [x] `make darwin` 构建成功
- [x] daemon 重新启动（端口 17612）

---

## 6. 后续工作

1. **Provider Contract 文档化**：所有新增 provider 必须在 `Run()` / `StartTurn()` 开头 emit `user_message`，已在 `docs/architecture/timeline-design.md` §11.5 记录。
2. **E2E 覆盖**：补充 web + app 同时在线的 E2E，验证跨端发送消息后两端 timeline 一致且无重复。
3. **类型化事件推进**：`protocol/stream_event.go` 仍然是 `interface{}` 类型池，长期建议按 `go-provider-type-erasure-analysis.md` 的 D→B 方案推进 tagged union。
