# Session Memory — Implementation Spec

> 实现规格：将用户输入与 agent 返回按 turn 粒度持久化为项目级 Markdown，为复盘、检索、agent 长期记忆奠基。

- 上游设计：[`docs/architecture/session-memory-persistence.md`](../architecture/session-memory-persistence.md)
- 状态：**Implemented — Phase 1 + 生产化加固**
- 作者：Andy
- 创建日期：2026-05-28
- 最后修订：2026-05-29（P1 路径收敛 + P2 流式合并 + P3 隔离 + P4 回归）
- 阶段：Phase 1（FileTurnRecorder + SafeBridge）

## Revision Log

| 日期 | 修订 |
|---|---|
| 2026-05-28 | M1–M6 初版（TurnRecorder / FileTurnRecorder / Redactor / Bridge / MemoryConfig / wiring）|
| 2026-05-29 | **P1** 路径收敛：`<projectRoot>/.solo/memory/...` → **`~/.solo/memory/...`**（固定 SoloHome 下）；`Enabled` 改为 `*bool`，nil 即启用（opt-out）；删除 AutoGitignore（不再写入项目目录）；`RecordTurn` 去掉 `projectRoot` 入参 |
| 2026-05-29 | **P2** 流式合并：`bridge.OnAssistantChunk` 累积，`turn_completed` / `turn_failed` / `turn_canceled` 时落盘；`SafeBridge.Close` 在 shutdown 刷未闭合 buffer |
| 2026-05-29 | **P3** 隔离：`bridge.SafeBridge` 包裹 panic recovery + 失败计数 + 熔断（默认 3 连败 / 30s cooldown）|
| 2026-05-29 | **P4** 全 daemon 16 包 `-race` 回归通过；`golangci-lint` 0 新增告警 |

## Implementation Status

| Milestone | Scope | Status |
|---|---|---|
| M1 | `TurnRecorder` 接口 + `Turn` 数据结构 + 单元测试 | ✅ |
| M2 | `FileTurnRecorder`（async channel writer、目录/文件布局、`sessions.jsonl`）| ✅ |
| M3 | `Redactor`（regex + env + multi + `BuildRedactor`）| ✅ |
| M4 | `Bridge`（turn builder + redactor + seq/parent chain）+ `Session` hook 注入 | ✅ |
| M5 | `MemoryConfig` + daemon wiring (`memorysetup`) + auto `.gitignore` | ✅ |
| M6 | E2E 测试（config → redactor → bridge → recorder → disk）+ doc sync | ✅ |

Phase 1 实现位于 `daemon/internal/memory*` + `daemon/internal/memorysetup` + `daemon/internal/config/memoryconfig.go` + `daemon/internal/server/memorybridge*.go`。

## Where data lives

Turn 文件写在 **`~/.solo/memory/sessions/{YYYY-MM}/{sessionID}/turns/{seq:04d}-{role}.md`**（固定在 SoloHome 下，多项目共享一个 memory 目录）。`sessions.jsonl` 也在同目录。项目根目录**不再**被 memory 功能写入任何文件。

### 默认行为

- **Enabled**：`*bool`，**nil 即启用**（opt-out 模型）。在 `~/.solo/config.json` 显式写 `"memory": {"enabled": false}` 关闭。Build 失败时：nil（auto 模式）仅 warn 并跳过；显式 `true` 视为 fatal。
- **Backend**：默认 `"file"`，预留 `"sqlite"` / `"middleware"`。
- **Root**：默认 `"memory"`，相对 SoloHome 解析 → `~/.solo/memory`。
- **QueueSize**：默认 1024；**Overflow**：默认 `"block"`（`"error"` 可选）。
- **RetentionDays**：默认 90（后端实现负责裁剪）。

### 流式输出合并

服务端一次 assistant 响应通常会产生多个 streaming timeline 事件（`assistant_message` 增量）。**Bridge 在内存中累积这些 chunk，等到 `turn_completed` / `turn_failed` / `turn_canceled` 时才一次性落盘为单个 `assistant` turn 文件**。因此每个逻辑 turn 严格对应一个 `user.md` + 一个 `assistant.md`，不会因流式事件而膨胀为数十个文件。若 daemon 关停时 turn 未结束，`SafeBridge.Close` 会把每个 agent 的在途 buffer 各自刷成一个 turn。

### 主流程隔离（SafeBridge）

`bridge.SafeBridge` 包裹在 `Bridge` 外层，daemon 主 session 循环只接触 SafeBridge：

- **Panic recovery**：任意 hook（`OnUserTurn` / `OnAssistantTurn` / `OnAssistantChunk` / `OnAssistantTurnEnd` / `OnSystemTurn`）内部的 panic 都会被 `recover()` 吞掉，仅 slog.Warn 一条；绝不传到 session。
- **熔断**：连续失败达到 `FailureThreshold`（默认 3）即开断，`FailureCooldown`（默认 30s）内所有调用直接 short-circuit；cooldown 过后允许一次 probe 调用，成功即闭断。
- **Idempotent Close**：多次调用只把 `Close` 透传到 inner 一次，daemon 可以安全 `defer` 多次。
- **nil inner**：传入 `nil` inner 时所有 hook 都是无操作，便于测试与 feature flag 关闭路径。

主流程隔离的硬保证由 `TestSafeBridge_MainFlowNotBlocked` 测试守护：100 次 panic 注入下，调用方 goroutine 必须在 2s 内正常返回。

## 1. 概述

Solo 当前会话数据仅驻留在内存中，关闭 daemon 即丢失。本规格在 **`daemon/internal/server` session 层** 注入对称 hook（`OnUserTurn` / `OnAssistantTurn` + streaming `OnAssistantChunk` / `OnAssistantTurnEnd`），把每个 turn 异步写入 `~/.solo/memory/` 下的 markdown 文件。整条链路由 `SafeBridge` 包裹，确保 recorder 故障不影响主 session。实现严格遵循 `TurnRecorder` 接口，便于后续切换到 SQLite 或向量记忆中间件。

## 2. 目标

| ID | 目标 | 度量 |
|---|---|---|
| G1 | 记录 100% 的 user/assistant turn（无遗漏） | 在 e2e 测试中，turn 数 = 会话实际消息数 |
| G2 | 主流程零阻塞 | 写盘路径完全异步，agent 循环延迟 < 1ms（P99） |
| G3 | 写盘失败不影响会话 | 失败仅记 log + metric，主流程无感知 |
| G4 | 平滑迁移到 DB / middleware | `TurnRecorder` 接口稳定，新实现可热插拔 |
| G5 | 敏感信息不入盘 | `.env` 片段、API key 模式 100% 脱敏 |

## 3. 非目标

- 不实现 RAG / 向量检索（阶段二+）
- 不记录 subagent / tool call 内部轨迹
- 不引入新的外部 Go 模块（阶段一）
- 不实现跨项目记忆聚合
- 不实现 CLI 查询子命令（独立 spec）

## 4. 范围

### 4.1 阶段一（本规格）

- `TurnRecorder` 接口 + `Turn` 数据结构
- `FileTurnRecorder` 实现（异步 channel writer、ULID、YAML frontmatter）
- `daemon/internal/server` 注入两处 hook
- 配置项（`memory.*`）
- 脱敏规则引擎（可复用现有 `security` 模块）
- 自动 `.gitignore` 建议
- 单元测试 + 集成测试

### 4.2 后续阶段（不在本规格）

- SQLite + FTS5 后端
- mem0 / Letta / 自建向量库适配
- `solo memory search | export | prune` CLI 子命令
- 跨项目全局记忆存储

## 5. 详细需求

### FR-1 Turn 捕获

- 每次 session 收到并校验完 inbound `UserMessage` 后，在派发 agent 前触发 `OnUserTurn`。
- 每次 agent 输出 finalize 后、推送客户端前触发 `OnAssistantTurn`。
- 每次触发产生一个 `Turn`，不合并、不拆分。
- 同一会话内 turn 严格按时序编号（`0001-`、`0002-`...）。

### FR-2 数据结构

```go
type Turn struct {
    ID        string         `yaml:"id"`        // ULID（26 字符）
    SessionID string         `yaml:"sessionId"`
    Seq       uint64         `yaml:"seq"`       // 会话内单调递增序号
    Role      TurnRole       `yaml:"role"`      // "user" | "assistant" | "system"
    Ts        time.Time      `yaml:"ts"`        // UTC，RFC3339
    Source    TurnSource     `yaml:"source"`    // "cli" | "app" | "relay"
    Content   string         `yaml:"-"`         // markdown body，不进 frontmatter
    Metadata  TurnMetadata   `yaml:"metadata,omitempty"`
    ParentID  string         `yaml:"parent,omitempty"`
}

type TurnRole   string
type TurnSource string

type TurnMetadata struct {
    Model        string            `yaml:"model,omitempty"`
    Tokens       *TokenUsage       `yaml:"tokens,omitempty"`
    ToolCalls    []string          `yaml:"toolCalls,omitempty"`
    FinishReason string            `yaml:"finishReason,omitempty"`
    Attachments  []AttachmentRef   `yaml:"attachments,omitempty"`
    Extra        map[string]any    `yaml:"extra,omitempty"`
}

type TokenUsage struct {
    Prompt     int `yaml:"prompt"`
    Completion int `yaml:"completion"`
}

type AttachmentRef struct {
    Name string `yaml:"name"`
    Kind string `yaml:"kind"` // "image" | "file"
    Size int    `yaml:"size"`
}
```

### FR-3 `TurnRecorder` 接口

```go
type TurnRecorder interface {
    // RecordTurn 异步入队一个 turn。
    // 实现必须线程安全、可并发调用。
    // 返回 nil 表示入队成功（不代表落盘成功）；
    // 返回 error 表示入队失败（如 channel 已满、recorder 已关闭）。
    RecordTurn(ctx context.Context, sessionID string, turn Turn) error

    // Flush 同步等待所有排队 turn 落盘，用于测试和关停。
    Flush(ctx context.Context) error

    // Close 刷盘并释放资源；调用后任何 RecordTurn 必须返回 ErrClosed。
    Close() error
}

var ErrClosed = errors.New("turn recorder closed")
```

### FR-4 `FileTurnRecorder` 行为

- 内部启动 **1 个 writer goroutine**，从 channel 消费 `Turn`。
- channel 容量默认 **1024**，满时按配置策略处理（`block` 默认 / `error` 可选）。
- 写盘路径：`~/.solo/memory/sessions/{YYYY-MM}/{sessionID}/turns/{seq:04d}-{role}.md`（固定在 SoloHome 下）
  - 例：`~/.solo/memory/sessions/2026-05/01J.../turns/0003-user.md`
- 每写一个 turn，更新 `~/.solo/memory/sessions.jsonl`（追加一行 JSON）。
- 文件存在时**不覆盖**（ULID + seq 天然唯一）。
- 首次写入某 session 时按需创建目录，权限 `0o755`；文件权限 `0o644`。

### FR-5 Turn 文件格式

```markdown
---
id: 01J5XQ8K9P...
sessionId: 01J5XQ...
seq: 3
role: assistant
ts: 2026-05-28T10:23:45Z
source: cli
metadata:
  model: solo-v1
  tokens:
    prompt: 1234
    completion: 567
  toolCalls: [Read, Bash]
  finishReason: stop
parent: 01J5XQ8K7M...
---

<agent 输出原文，保留 markdown>
```

- frontmatter 使用 `gopkg.in/yaml.v3`（已存在于项目）
- body 部分不转义、不加工，原样写出
- 结尾**不加**额外空行（保持原文）

### FR-6 会话索引 `sessions.jsonl`

每行一个 session 摘要，**首次写入 turn 时追加一次**（同一 session 后续 turn 不再更新行）。索引文件位置：`~/.solo/memory/sessions.jsonl`。

```jsonl
{"id":"01J5XQ...","startedAt":"2026-05-28T10:20:00Z","turnsCount":1,"source":"cli"}
```

字段：`id` / `startedAt` / `turnsCount` / `source`。

### FR-7 Hook 集成

- `daemon/internal/server` 新增 `MemoryBridge` 接口，由 session 直接调用：
  - `OnUserTurn(sessionID, agentID, content)`
  - `OnAssistantTurn(sessionID, agentID, content)`（one-shot，attention 等非流式场景）
  - `OnAssistantChunk(agentID, sessionID, fragment)`（流式累积）
  - `OnAssistantTurnEnd(agentID, sessionID)`（`turn_completed` / `turn_failed` / `turn_canceled` 时 flush）
  - `OnSystemTurn(sessionID, agentID, content)`
  - `Close() error`（shutdown 时刷未闭合 buffer）
- 实现位于 `daemon/internal/memory/bridge.Bridge`，由 `bridge.SafeBridge` 包裹后注入 session，确保 recorder 故障不回传到 session。
- Hook 内部做四件事：
  1. 将 inbound/outbound 消息映射为 `Turn`
  2. 调用 `Redactor.Redact(turn.Content)`
  3. 维护 session 内 `seq` / `parentID` 链
  4. `recorder.RecordTurn(ctx, sess.ID, turn)`
- Hook 错误路径：仅 `slog.Warn` + 失败计数，**绝不**把错误回传到 session；连续失败触发熔断。

### FR-8 脱敏（Redactor）

```go
type Redactor interface {
    Redact(content string) string
}
```

阶段一实现：
- `RegexRedactor`：可配置正则列表（默认含 `sk-[A-Za-z0-9]{32,}`、`ghp_[A-Za-z0-9]{36}`、`AKIA[0-9A-Z]{16}` 等常见 token 模式）
- `EnvFileRedactor`：识别 `KEY=value` 形式且 key 在已知敏感名列表（`*_KEY`, `*_SECRET`, `*_TOKEN`, `PASSWORD`, `DATABASE_URL`...）中，整行替换为 `[redacted: KEY]`
- 组合为 `MultiRedactor`，按顺序应用

脱敏后的文本使用 `[redacted:<reason>]` 占位，便于审计。

### FR-9 配置

在 `daemon/internal/config` 新增：

```go
type MemoryConfig struct {
    Enabled       *bool             `yaml:"enabled,omitempty" json:"enabled,omitempty"` // nil == enabled (opt-out)
    Backend       string            `yaml:"backend" json:"backend"`                     // "file" | "sqlite" | "middleware"
    RetentionDays int               `yaml:"retention_days" json:"retention_days"`       // default 90
    QueueSize     int               `yaml:"queue_size" json:"queue_size"`               // default 1024
    Overflow      string            `yaml:"overflow" json:"overflow"`                   // "block" (default) | "error"
    Root          string            `yaml:"root" json:"root"`                           // default "memory" → ~/.solo/memory
    Redact        RedactConfig      `yaml:"redact" json:"redact"`
    Safe          SafeBridgeConfig  `yaml:"safe" json:"safe"`
    SoloHome      string            `yaml:"-" json:"-"` // runtime-only, set by daemon
}

// IsEnabled reports whether the feature should run. nil or true → enabled.
func (c MemoryConfig) IsEnabled() bool { return c.Enabled == nil || *c.Enabled }

type RedactConfig struct {
    EnvFiles      bool     `yaml:"env_files" json:"env_files"`
    APIKeys       bool     `yaml:"api_keys" json:"api_keys"`
    CustomRegexes []string `yaml:"custom_regexes" json:"custom_regexes"`
    SensitiveKeys []string `yaml:"sensitive_keys" json:"sensitive_keys"`
}

type SafeBridgeConfig struct {
    FailureThreshold int           `yaml:"failure_threshold" json:"failure_threshold"` // default 3
    FailureCooldown  time.Duration `yaml:"failure_cooldown" json:"failure_cooldown"`   // default 30s
}
```

`~/.solo/config.json` 例：

```json
{
  "memory": {
    "enabled": false
  }
}
```

Build 失败策略：
- `Enabled == nil`（auto）：`slog.Warn` + 跳过功能，daemon 启动不受影响。
- `Enabled == true`（显式）：fatal，daemon 拒绝启动。

默认值：
- `enabled`：`*bool`，nil 即启用（opt-out）
- `backend: "file"`
- `retention_days: 90`
- `queue_size: 1024`
- `overflow: "block"`（可选 `"error"`）
- `root: "memory"` → 实际路径 `~/.solo/memory`
- `safe.failure_threshold: 3`
- `safe.failure_cooldown: 30s`

### FR-11 生命周期

- Daemon 启动时：
  1. `cfg.Memory.SoloHome = cfg.SoloHome`
  2. 若 `cfg.Memory.IsEnabled()`，调 `memorysetup.Build(cfg.Memory)` 得到 `{Bridge, Recorder}`
  3. `Bridge` 由 `bridge.NewSafeBridge` 包裹（panic recovery + 熔断）
  4. 注入 `DaemonConfig.MemoryBridge` / `MemoryRecorder`，session 创建时 `SetMemoryBridge(safeBridge)`
- Daemon 关停时（SIGTERM/SIGINT，`Daemon.Stop`）：
  1. `safeBridge.Close()` —— 把每个 agent 在途 chunk buffer 各自刷成一个 turn
  2. `recorder.Flush(ctx)` —— 排空 channel 队列
  3. `recorder.Close()` —— 释放资源
  - 三步都是 best-effort，单步失败只 `slog.Warn`，不影响关停流程。
- 配置变更：阶段一**不支持**热加载，需重启 daemon。

## 6. 非功能性需求

### NFR-1 性能

- `RecordTurn` 入队延迟 P99 < 100 μs（在 10 万 turn/小时负载下）。
- 单 turn 写盘耗时 P99 < 5 ms（典型 SSD）。
- Daemon 常驻内存开销 < 10 MB（queue 满载时 < 20 MB）。

### NFR-2 可靠性

- 写盘失败不 panic；`persistTurn` / `maybeWriteSessionIndex` 内的所有 IO 错误都被静默吞掉（recorder 层不重试、不回传到 caller）。失败指标走 `slog.Warn`。
- 队列满时按 `cfg.Overflow` 行为：`"block"`（默认，受 `ctx` 取消控制）或 `"error"`（立即返回）。
- 任何 panic 在 `SafeBridge` 层被 `recover()` 吞掉，仅 `slog.Warn` 一条；连续失败 ≥ `FailureThreshold`（默认 3）触发熔断，`FailureCooldown`（默认 30s）内所有 hook 调用 short-circuit。
- Daemon 关停时 `safeBridge.Close()` / `recorder.Flush()` / `recorder.Close()` 三步都 best-effort，单步失败不影响关停流程。

### NFR-3 安全

- 所有写入走 `Redactor`，默认开启。
- 文件权限 `0o644`，目录 `0o755`；不创建 world-readable 之外的任何共享权限。
- 不在日志中打印 `turn.Content`（仅打印 `turn.ID` + `turn.Seq` + 字节数）。

### NFR-4 可观测性

暴露 metrics（复用 `daemon/internal/metrics`）：
- `memory.turns_recorded_total{role,source}` (counter)
- `memory.turns_written_total{role}` (counter)
- `memory.write_errors_total{reason}` (counter)
- `memory.queue_depth` (gauge)
- `memory.queue_overflows_total{policy}` (counter)
- `memory.write_duration_seconds` (histogram)
- `memory.flush_duration_seconds` (histogram)

### NFR-5 可测试性

- `TurnRecorder` 接口可注入 mock。
- `FileTurnRecorder` 接受 `fs.FS` 抽象（使用 `os` 默认，测试可替换）。
- 时间通过 `Clock` 接口注入（测试可固定）。

## 7. 数据流图

```
  User/App/Relay
        │
        ▼
┌─────────────────────────┐
│ session (server)        │
│   ├─ OnUserTurn ──────┐ │
│   └─ OnAssistantTurn ─┤ │
└─────────────────────────┼─┘
                          │
                          ▼
                  ┌──────────────┐
                  │ Hook bridge  │
                  │  - Map→Turn  │
                  │  - Redact    │
                  └──────┬───────┘
                         │
                         ▼
              ┌────────────────────┐
              │ TurnRecorder       │
              │  RecordTurn() ──┐  │
              └─────────────────┼──┘
                                │ channel (cap 1024)
                                ▼
                  ┌────────────────────────┐
                  │ writer goroutine       │
                  │  - mkdir -p            │
                  │  - write <seq>-<role>.md │
                  │  - append sessions.jsonl │
                  └──────────┬─────────────┘
                             ▼
                    <project>/.solo/memory/
```

## 8. 错误处理矩阵

| 场景 | 行为 | 可观测 |
|---|---|---|
| 入队时 channel 满 | 按 `overflow_policy` 处理 | `memory.queue_overflows_total{policy}` +1 |
| 目录创建失败 | 退避重试 3 次，仍失败则丢弃 + log | `memory.write_errors_total{reason="mkdir"}` |
| 文件写失败 | 退避重试 3 次，仍失败则丢弃 + log | `memory.write_errors_total{reason="write"}` |
| YAML 序列化失败 | 跳过本 turn + log（极罕见） | `memory.write_errors_total{reason="marshal"}` |
| `Close()` 超时 | 强退 + log 未刷盘数量 | `memory.flush_timeout_total` +1 |
| `RecordTurn` on closed | 返回 `ErrClosed` | — |
| 脱敏正则编译失败 | 启动时 fail-fast | daemon 启动失败 |

## 9. 测试策略

### 9.1 单元测试（`daemon/internal/memory`）

- `TestTurnRecorder_RecordTurn_Success`：1000 turn 顺序写入，校验文件数、内容、frontmatter。
- `TestTurnRecorder_RecordTurn_Concurrent`：10 goroutine 并发写 1000 turn，校验无丢失、无重复。
- `TestTurnRecorder_QueueOverflow_DropOldest`：queue_size=8，灌入 20 turn，校验最新 8 个落盘。
- `TestTurnRecorder_Flush`：写入后立刻 `Flush`，校验全部落盘。
- `TestTurnRecorder_Close_Idempotent`：重复 `Close` 不 panic。
- `TestTurnRecorder_RecordAfterClose`：返回 `ErrClosed`。
- `TestRedactor_ApiKeys`：覆盖 OpenAI / GitHub / AWS / Anthropic / 自定义 token。
- `TestRedactor_EnvFile`：覆盖常见敏感 key 名。
- `TestSessionIndex_AppendAndUpdate`：校验 `sessions.jsonl` 写入频次。

### 9.2 集成测试（`daemon/internal/server`）

- `TestSession_HooksFireOnBothTurns`：模拟一次完整会话，校验 `MockRecorder` 收到 2N 个 turn。
- `TestSession_HookErrorDoesNotAffectSession`：注入 fail-recorder，校验会话正常完成。

### 9.3 E2E 测试

- 启动真实 daemon + CLI，跑一次 `solo agent run`，断言 `.solo/memory/sessions/{YYYY-MM}/*/turns/` 存在 ≥2 个 turn 文件，frontmatter 可解析，内容非空。

## 10. 验收标准

| AC | 验证方式 |
|---|---|
| AC-1: 任意 Solo 会话结束后，对应 `.solo/memory/` 下 turn 文件数 = 会话 user + assistant 消息总数 | E2E 测试 |
| AC-2: 写盘期间 agent 响应延迟无可感知退化（P99 增量 < 1ms） | 基准测试 |
| AC-3: 写入 10 万 turn 不丢失、不重复 | 集成测试 |
| AC-4: 默认配置下，`.env` 内容、OpenAI/ GitHub token 不出现在任何 turn 文件 | 脱敏测试 + grep 扫描 |
| AC-5: `Close()` 后 daemon 退出干净（无悬挂 goroutine） | goroutine leak 检测（`goleak`） |
| AC-6: Daemon 重启后，旧 session 新 turn 仍可追加，序号连续 | 集成测试 |
| AC-7: 所有 metrics 在 `/metrics` 端点可见 | smoke test |
| AC-8: 单 turn 内容含 `[redacted:...]` 时，原文已被替换 | 脱敏单测 |

## 11. 迁移路径

阶段一完成后：

```
FileTurnRecorder  ─(接口不变)─►  SQLiteTurnRecorder
                                   │
                                   └─► 一次性导入脚本：
                                       扫描 .solo/memory/sessions/*/*/turns/*.md
                                       → 解析 frontmatter + body
                                       → 写入 sqlite + FTS5 索引
```

阶段二完成后：

```
SQLiteTurnRecorder ─(接口不变)─► MemoryMiddlewareRecorder
                                    │
                                    ├─► mem0 adapter
                                    ├─► Letta adapter
                                    └─► 自建向量库 adapter
```

`TurnRecorder` 是**稳定契约**，后端实现可独立演进、可并存（多写模式）。

## 12. 开放问题（Phase 1 已拍板）

| ID | 问题 | 决议 |
|---|---|---|
| Q1 | system prompt 是否入 turn？ | ✅ **入**：`bridge.OnSystemTurn` 提供 `role: "system"`，与 user/assistant 同链。实际是否触发由 server hook 决定（Phase 1 仅在 `sendAgentStream` 中对 `assistant_message` 触发，system prompt 触发点留到后续迭代）。|
| Q2 | `session.md`（拼接视图）是否阶段一实现？ | ✅ **延后**：Phase 1 只写 per-turn 文件，`session.md` 留到 SQLite 阶段由查询视图替代。|
| Q3 | 跨项目"全局记忆"存哪？ | ✅ **Phase 1 不实现**：独立 spec（建议 `~/.solo/global-memory/`）。|
| Q4 | 多 agent 并发时 parent 链如何重建？ | ✅ **已实现**：`bridge.Bridge` 内置 `sessionState` map + per-session `sync.Mutex`，在 assign→record→update 全程持锁，seq/ParentID 严格单调。|
| Q5 | 是否需要 CLI 侧也写入（独立于 daemon）？ | ✅ **否**：仅 daemon 层 `Session.handleSendAgentMessage` / `sendAgentStream` 触发，CLI 走 daemon WS 接口。|
| Q6 | 脱敏失败是否阻断写入？ | ⚠️ **Phase 1 偏差**：当前 `RegexRedactor` / `EnvFileRedactor` 在编译期校验正则（invalid regex → Build 失败），运行时 `ReplaceAllString` 不会失败；故无"脱敏运行时失败"路径，原文即安全文本。`[redacted:failed]` 占位符留到引入外部脱敏器（LLM-based / 远程服务）时再加。|

## 13. 依赖

- 内部：
  - `daemon/internal/server`（hook 注入）
  - `daemon/internal/config`（配置）
  - `daemon/internal/metrics`（指标）
  - `daemon/internal/security`（现有脱敏规则复用）
- 外部（已存在）：
  - `gopkg.in/yaml.v3`
  - `github.com/oklog/ulid/v2`（若项目已引入；否则改用 `github.com/google/uuid` + 序号）
- 不引入新的外部模块。

## 14. 里程碑

| Milestone | 交付物 | 估时 |
|---|---|---|
| M1 | `TurnRecorder` 接口 + `Turn` 数据结构 + 单元测试 | 0.5 天 |
| M2 | `FileTurnRecorder`（含异步 writer、目录/文件布局）+ 单元测试 | 1 天 |
| M3 | `Redactor`（regex + env）+ 单元测试 | 0.5 天 |
| M4 | session hook 集成 + 集成测试 | 0.5 天 |
| M5 | 配置项 + `.gitignore` 自动建议 + metrics | 0.5 天 |
| M6 | E2E 测试 + 文档（`docs/architecture/session-memory-persistence.md` 更新） | 0.5 天 |
| **合计** | — | **~3.5 天** |

---

**签名**：Spec 审阅通过后进入实现。建议先由架构/安全 reviewer 对 §5 / §6.3 / §12 拍板。
