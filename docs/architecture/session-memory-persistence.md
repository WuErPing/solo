# Session Memory Persistence Design

> 自动将用户输入与 agent 返回持久化为项目级 markdown 文档，为后续记忆、检索、复盘提供基础数据层。

- 状态：Draft
- 作者：Andy
- 创建日期：2026-05-28

## 1. 目标

- 记录每一次会话中的 **用户输入** 与 **agent 输出**，不丢不漏
- 存储格式为 **Markdown + YAML frontmatter**，对人可读、对工具可解析
- 与项目绑定（存储在项目目录下），便于版本控制与团队协作
- 抽象清晰，未来可平滑迁移到 **数据库** 或 **agent memory 中间件**（如 mem0、Letta、向量库）

## 2. 非目标

- 不在本文范围内实现向量检索 / RAG
- 不处理 agent 内部 subagent、tool call 的细粒度轨迹（仅记录对外可见的最终 turn）
- 不引入新的外部依赖（阶段一）

## 3. 挂接位置（核心决策）

推荐挂接点：**`daemon/internal/server` 的 session 消息调度层**。

### 3.1 为什么是 session 层

| 候选层 | 评估 |
|---|---|
| `cli/internal/client` | 仅能看到本地终端输入，看不到 App / Relay 来源 ❌ |
| `protocol` | 字节/帧级，缺乏业务语义 ❌ |
| `daemon/internal/agent` | 内部并发 tool call、subagent 复杂，消息未归并 ❌ |
| **`daemon/internal/server` session** | **所有端（CLI / App / Relay）消息必经之路，已是结构化 `UserMessage` / `AgentMessage`** ✅ |

### 3.2 对称 hook 点

| 事件 | 触发时机 | 记录内容 |
|---|---|---|
| `OnUserTurn` | session 接收并校验完 inbound 消息后、派发给 agent 前 | 用户原文、attachments、来源通道 (cli / app / relay) |
| `OnAssistantTurn` | agent 输出 finalize 后、推送给客户端前 | 最终文本、tool uses、token usage、finish reason |

### 3.3 集成形态

复用现有事件/观察者机制（参考 `metrics`、`push` 模块的事件管道），避免将持久化逻辑硬塞进主流程：

- session 仅负责 **emit 事件**
- recorder 作为 **subscriber** 异步消费
- 失败、延迟、重试均不影响主流程

## 4. 抽象层（为迁移预留）

### 4.1 核心接口

```go
// daemon/internal/memory/recorder.go
package memory

type TurnRecorder interface {
    RecordTurn(ctx context.Context, projectRoot string, sessionID string, turn Turn) error
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
<project-root>/
└── .solo/
    └── memory/
        ├── sessions.jsonl              # 会话级索引（轻量，便于检索）
        └── sessions/
            └── {YYYY-MM}/{session-id}/
                ├── session.md          # 完整会话拼接视图（按需生成）
                └── turns/
                    ├── 0001-user.md
                    ├── 0002-assistant.md
                    └── 0003-user.md
```

### 5.2 设计要点

- **复用 `.solo/`**：项目约定目录，降低心智负担；写入前自动补全 `.gitignore` 建议
- **`YYYY-MM/{session-id}/`** 分桶：避免单目录文件爆炸，便于按时间归档与清理
- **序号前缀 `0001-`**：保留天然顺序，比时间戳排序更稳
- **一个 turn 一个文件**（而非整个 session 一个文件）：
  - 追加安全、并发无锁
  - 利于增量备份
  - 为后续 RAG chunking 天然切分
- **frontmatter 用 YAML**：迁移到 DB 时直接映射为列
- **`sessions.jsonl`** 做索引：字段包含 `id`、`title`、`started_at`、`turns_count`、`tags`；DB 化后这一层可移除

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
| 脱敏 | 写入前过滤 `.env` 内容、API key 模式（复用现有 `security` 规则） |
| 轮转/清理 | 保留策略（如 90 天 / 项目配额）内置于 `FileTurnRecorder` |
| 配置开关 | `daemon/internal/config` 新增 `memory.enabled`、`memory.backend`、`memory.retention_days` |

## 7. 配置示例

```yaml
# ~/.solo/config.yaml 或项目级 .solo/config.yaml
memory:
  enabled: true
  backend: file            # file | sqlite | middleware
  retention_days: 90
  redact:
    - env_files: true
    - api_keys: true
  file:
    root: .solo/memory     # 相对于项目根
    flush_interval_ms: 500
```

## 8. 迁移路径

```
FileTurnRecorder (now)
        │
        ├──► SQLiteTurnRecorder：读旧 .solo/memory/ 一次性导入
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

- 是否需要在 session 层记录 system prompt？（占 token 但信息量低）
- `session.md` 是懒生成（按需）还是每次 turn 后追加？
- 跨项目的"全局记忆"是否需要独立存储位置（如 `~/.solo/global-memory/`）？
- 多 agent 并发时，parent 链如何准确重建？
