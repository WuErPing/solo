# Session Memory Persistence Design

> Automatically persist user input and agent output as project-level markdown documents, providing a foundational data layer for future memory, retrieval, and review.

- Status: **Implemented — Phase 1** (see implementation at [`../product/session-memory-spec.md`](../product/session-memory-spec.md))
- Author: Andy
- Created: 2026-05-28
- Last revised: 2026-05-29

> This document is the design source. **Implementation has converged on some decisions**, most notably the storage location changed from `<project-root>/.solo/memory/` to fixed **`~/.solo/memory/`** (under SoloHome, shared across projects). Sections §4, §5, §7 below are synchronized. Authoritative implementation spec is [`session-memory-spec.md`](../product/session-memory-spec.md).

## 1. Goals

- Record **user input** and **agent output** in every session, without loss or omission
- Store in **Markdown + YAML frontmatter** format, human-readable and tool-parseable
- Bound to the project (stored in the project directory), facilitating version control and team collaboration
- Clean abstraction, enabling smooth future migration to **database** or **agent memory middleware** (e.g., mem0, Letta, vector stores)

## 2. Non-Goals

- Vector retrieval / RAG implementation is out of scope for this document
- No fine-grained tracking of internal agent subagent or tool call traces (only the externally visible final turn is recorded)
- No new external dependencies introduced (Phase 1)

## 3. Hook Location (Core Decision)

Recommended hook point: **session message dispatch layer in `daemon/internal/server`**.

### 3.1 Why the Session Layer

| Candidate Layer | Assessment |
|---|---|
| `cli/internal/client` | Can only see local terminal input, cannot see App / Relay sources ❌ |
| `protocol` | Byte/frame level, lacks business semantics ❌ |
| `daemon/internal/agent` | Internal concurrent tool calls, complex subagent, messages not consolidated ❌ |
| **`daemon/internal/server` session** | **Mandatory path for all endpoints (CLI / App / Relay) messages, already structured `UserMessage` / `AgentMessage`** ✅ |

### 3.2 Symmetric Hook Points

| Event | Trigger Timing | Content Recorded |
|---|---|---|
| `OnUserTurn` | After session receives and validates inbound message, before dispatching to agent | User raw text, attachments, source channel (cli / app / relay) |
| `OnAssistantTurn` | After agent output is finalized, before pushing to client | Final text, tool uses, token usage, finish reason |

### 3.3 Integration Form

Reuse existing event/observer mechanisms (referencing event pipelines in `metrics` and `push` modules), avoiding hardcoding persistence logic into the main flow:

- Session only responsible for **emitting events**
- Recorder acts as an async **subscriber** consuming events
- Failures, delays, and retries do not affect the main flow

## 4. Abstraction Layer (Reserved for Migration)

### 4.1 Core Interface

```go
// daemon/internal/memory/recorder.go
package memory

// NOTE: the implemented contract is slightly tighter than this sketch —
// see docs/product/session-memory-spec.md (FR-3) for the final shape,
// which also drops projectRoot (turns now live under SoloHome) and adds
// a Flush method for graceful shutdown.
type TurnRecorder interface {
    RecordTurn(ctx context.Context, sessionID string, turn Turn) error
    Flush(ctx context.Context) error
    Close() error
}

type Turn struct {
    ID        string         // ULID，有序且全局唯一
    SessionID string
    Role      string         // "user" | "assistant" | "system"
    Ts        time.Time
    Source    string         // "cli" | "app" | "relay"
    Content   string         // markdown body
    Metadata  map[string]any // tokens, tool_calls, model, attachments, etc.
    ParentID  string         // 上一 turn ID，用于重建对话链
}
```

### 4.2 实现演进路径

| 阶段 | 实现 | 适用场景 |
|---|---|---|
| 1 | `FileTurnRecorder`（markdown 文件） | 单机、调试、人工复盘 |
| 2 | `SQLiteTurnRecorder`（`~/.solo/memory.db`，带 FTS5） | 检索、统计、跨项目查询 |
| 3 | `MemoryMiddlewareRecorder`（对接 mem0 / Letta / 自建向量库） | agent 长期记忆、语义检索 |

三者可**多写并存**，而非替换。只要 `TurnRecorder` 接口不变，迁移只是换实现 + 一次性导入。

## 5. 目录与文件设计

### 5.1 目录结构

```
~/.solo/                                 # SoloHome（固定，多项目共享）
└── memory/                              # 由 config.memory.root 控制，默认 "memory"
    ├── sessions.jsonl                   # 会话级索引（轻量，便于检索）
    └── sessions/
        └── {YYYY-MM-DD}/{session-id}/
            └── turns/
                ├── 0001-user.md
                ├── 0002-assistant.md
                └── 0003-user.md
```

### 5.2 设计要点

- **固定在 `~/.solo/` 下**：所有项目共享一个 memory 目录，避免污染项目工作区，也不再需要写入项目级 `.gitignore`
- **`YYYY-MM-DD/{session-id}/`** 分桶：避免单目录文件爆炸，便于按时间归档与清理
- **序号前缀 `{seq:04d}-`**（如 `0001-`）：保留天然顺序，比时间戳排序更稳
- **一个 turn 一个文件**（而非整个 session 一个文件）：
  - 追加安全、并发无锁
  - 利于增量备份
  - 为后续 RAG chunking 天然切分
- **frontmatter 用 YAML**：迁移到 DB 时直接映射为列
- **`sessions.jsonl`** 做索引：字段包含 `id`、`title`、`startedAt`、`turnsCount`；DB 化后这一层可移除

### 5.3 turn 文件格式

```markdown
---
id: turn_01J...
role: assistant
ts: 2026-05-28T10:23:45Z
source: cli
model: solo-v1
tokens:
  prompt: 1234
  completion: 567
tool_calls: [Read, Bash]
parent: turn_01H...
---

<agent 输出正文，保留原始 markdown>
```

## 6. 关键工程细节

| 关注点 | 方案 |
|---|---|
| 异步写入 | turn 通过 channel 投递给后台 writer goroutine，绝不阻塞 agent 循环 |
| 失败隔离 | 写盘失败仅打 log + metric，不影响会话主流程 |
| 幂等键 | turn ID 使用 ULID（有序 + 全局唯一），重放不产生重复 |
| 脱敏 | 写入前过滤 `.env` 内容、API key 模式（`redact` 包：regex / env / multi，内置 OpenAI/GitHub/Anthropic/AWS 模式）|
| 轮转/清理 | 保留策略（`retention_days`，默认 90）内置于 `FileTurnRecorder` |
| 配置开关 | `config.MemoryConfig`：`enabled`（`*bool`，nil/缺省即开启，opt-out）、`backend`、`root`、`retention_days`、`queue_size`、`overflow`、`redact.*`、`safe.*`（熔断阈值/冷却）|
| 故障隔离 | `bridge.SafeBridge` 包裹：panic recovery + 连续失败计数熔断（默认 3 连败 / 30s 冷却），异常永不波及会话主流程 |

## 7. 配置示例

存储在 `~/.solo/config.json`（**默认开启，无需配置**；以下仅示意 opt-out 与可调旋钮）：

```json
{
  "memory": {
    "enabled": false,
    "backend": "file",
    "root": "memory",
    "retention_days": 90,
    "queue_size": 1024,
    "overflow": "block"
  }
}
```

## 8. 迁移路径

```
FileTurnRecorder (now)
        │
        ├──► SQLiteTurnRecorder：读旧 `~/.solo/memory/` 一次性导入
        │
        └──► MemoryMiddlewareRecorder：暴露 TurnRecorder 接口给插件
```

只要 `TurnRecorder` 接口稳定，迁移工作 = 新实现 + 一次 import 脚本，对上层零侵入。

## 9. 实施建议

**阶段一（建议 1-2 天）**：

1. 定义 `TurnRecorder` 接口与 `Turn` 结构
2. 实现 `FileTurnRecorder`（含异步 writer、脱敏、轮转）
3. 在 session 层接入 `OnUserTurn` / `OnAssistantTurn` 两处 hook
4. 配置项与 `.gitignore` 模板

**后续迭代**：

- SQLite 后端 + 全文索引
- 中间件适配层（mem0 / Letta）
- CLI 子命令：`solo memory search / export / prune`

## 10. 开放问题

- 是否需要在 session 层记录 system prompt？（占 token 但信息量低）—— 仍开放
- 多 agent 并发时，parent 链如何准确重建？—— 部分解决：`bridge` 维护 session-seq/parentID 链，复杂 subagent 场景待 Phase 2 验证

**已收敛**：
- `session.md` 懒生成？→ Phase 1 **不生成** session.md，只写 per-turn 文件
- 跨项目"全局记忆"独立位置？→ 已收敛为固定 `~/.solo/memory/`（SoloHome 下，天然全局共享）
