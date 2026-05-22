# Kimi & Cursor-Agent Provider 集成分析与解决方案

> 分析日期：2026-05-21
> 仓库：/Users/wuerping/code/wuerping/solo
> 分析范围：daemon (Go) + app-bridge (TS) + app (React Native)
>
> **Status update (2026-05-22)**: Kimi Wire mode provider is now **fully implemented** (`provider_kimi.go`, 758 LOC, 23 tests).
> Cursor-Agent remains planned.

---

## 一、执行摘要

| Provider | 当前状态 | 推荐集成方式 | 复杂度 |
|---------|---------|------------|--------|
| **Kimi** | **✅ 已实现** (JSON-RPC 2.0 stdio, EventPump) | **Wire 模式** (`kimi --wire`) | 中高 |
| **Cursor-Agent** | 完全缺失 | **Print 模式** (`cursor agent --print --output-format stream-json`) | 中等 |

---

## 二、现状深度分析

### 2.1 Kimi — 有定义，无实现

#### 已存在的前端定义

- `daemon/internal/agent/provider_registry.go` — `BuiltinProviderDefinitions()` 中包含 `kimi` 的 ID/Label/Modes
- `app-bridge/src/server/agent/provider-manifest.ts` — `KIMI_MODES` 和 `AGENT_PROVIDER_DEFINITIONS` 已定义
- `app/src/utils/provider-command-templates.ts` — `kimi --resume {sessionId}` 命令模板

#### 缺失的后端实现

- ❌ `daemon/internal/server/daemon.go` 中**未注册** Kimi 的 `AgentClient`
- ❌ `daemon/internal/agent/` 下**没有** `provider_kimi.go`
- ❌ `app/src/components/provider-icons.ts` 和 `icons/` 目录**没有** Kimi 图标

#### CLI 调研结果

```bash
$ kimi --version
kimi-cli version: 1.43.0
agent spec versions: 1
wire protocol: 1.10
```

`kimi` CLI 已安装于 `/Users/wuerping/.local/bin/kimi`。

**Print 模式测试：**

```bash
$ kimi --print --output-format stream-json --prompt "hi"
{"role":"assistant","content":[{"type":"think","think":"...","encrypted":null},{"type":"text","text":"Hi there! ..."}]}
To resume this session: kimi -r <session-id>
```

⚠️ **关键发现**：`--print --output-format stream-json` 输出**完整 JSON（非流式）**，每个 turn 结束后才输出一行 JSON，UI 需要等待整个 turn 完成才能显示内容，体验差。

✅ **最佳集成点**：`kimi --wire` 提供 **JSON-RPC 2.0 over stdin/stdout** 的 Wire 协议，支持真正的双向流式通信。

#### Wire 协议核心特性

Wire 是 Kimi Code CLI 的低级通信协议，专为外部程序集成设计。

**协议基础：**
- 传输：逐行 JSON-RPC 2.0 via stdin/stdout
- 版本：1.10
- 方向：双向（客户端 request ↔ 服务端 event/request）

**客户端请求方法：**

| 方法 | 方向 | 类型 | 说明 |
|------|------|------|------|
| `initialize` | Client → Agent | Request | 握手，协商协议版本、注册外部工具 |
| `prompt` | Client → Agent | Request | 发送用户输入，启动 agent turn |
| `cancel` | Client → Agent | Request | 取消当前 turn |
| `set_plan_mode` | Client → Agent | Request | 设置 plan 模式开关 |
| `steer` | Client → Agent | Request | 向运行中的 turn 注入追加输入 |
| `replay` | Client → Agent | Request | 触发历史回放 |

**服务端推送通知：**

| 方法 | 方向 | 类型 | 说明 |
|------|------|------|------|
| `event` | Agent → Client | Notification | 流式事件（无需响应） |
| `request` | Agent → Client | Request | 权限/工具调用请求（必须响应） |

**Event 类型（关键）：**

| Event | 说明 | Solo 映射 |
|-------|------|----------|
| `TurnBegin` | Turn 开始 | `thread_started` |
| `ContentPart(text)` | 文本片段 | `timeline(assistant_message)` |
| `ContentPart(think)` | 思考片段 | `timeline(reasoning)` |
| `ToolCall` | 工具调用 | `timeline(tool_call)` |
| `ToolResult` | 工具执行结果 | `timeline(tool_call completed)` |
| `ApprovalResponse` | 审批完成 | — |
| `TurnEnd` | Turn 结束 | `turn_completed` |
| `StepBegin` | Step 开始 | — |
| `StepRetry` | Step 重试 | — |
| `CompactionBegin/End` | 上下文压缩 | `timeline(compaction)` |
| `StatusUpdate` | 状态更新 | — |

**Request 类型（需要客户端响应）：**

| Request | 说明 | Solo 映射 |
|---------|------|----------|
| `ApprovalRequest` | 操作审批请求 | `permission_requested` |
| `ToolCallRequest` | 外部工具调用 | —（如注册外部工具） |
| `QuestionRequest` | 结构化问题（AskUserQuestion） | — |
| `HookRequest` | Hook 处理请求 | — |

**错误码：**

| Code | 说明 |
|------|------|
| `-32000` | Turn 进行中 / 不支持的操作 |
| `-32001` | LLM 未配置 |
| `-32002` | 指定 LLM 不支持 |
| `-32003` | LLM 服务错误 |
| `-32700` | JSON 格式错误 |
| `-32601` | 方法不存在 |

**示例交互：**

```json
// 1. 客户端发送 initialize
{"jsonrpc":"2.0","method":"initialize","id":"1","params":{"protocol_version":"1.10","client":{"name":"solo","version":"0.1.0"},"capabilities":{"supports_question":true}}}

// 2. 服务端响应
{"jsonrpc":"2.0","id":"1","result":{"protocol_version":"1.10","server":{"name":"Kimi Code CLI","version":"1.43.0"},"slash_commands":[...],"capabilities":{"supports_question":true}}}

// 3. 客户端发送 prompt
{"jsonrpc":"2.0","method":"prompt","id":"2","params":{"user_input":"Hello"}}

// 4. 服务端推送 event（流式）
{"jsonrpc":"2.0","method":"event","params":{"type":"TurnBegin","payload":{"user_input":"Hello"}}}
{"jsonrpc":"2.0","method":"event","params":{"type":"ContentPart","payload":{"type":"text","text":"Hi"}}}
{"jsonrpc":"2.0","method":"event","params":{"type":"ContentPart","payload":{"type":"text","text":" there!"}}}
{"jsonrpc":"2.0","method":"event","params":{"type":"TurnEnd","payload":{}}}

// 5. prompt 请求最终响应
{"jsonrpc":"2.0","id":"2","result":{"status":"finished"}}
```

**审批请求交互：**

```json
// 服务端发送 ApprovalRequest
{"jsonrpc":"2.0","method":"request","id":"req-1","params":{"type":"ApprovalRequest","payload":{"id":"approval-1","tool_call_id":"tc-1","sender":"Shell","action":"run shell command","description":"Run command `ls`","display":[]}}}

// 客户端响应
{"jsonrpc":"2.0","id":"req-1","result":{"request_id":"approval-1","response":"approve"}}
```

---

### 2.2 Cursor-Agent — 完全缺失

#### 现状

- 项目中没有任何 `cursor-agent` 的定义、实现或图标
- 存在 `app/assets/images/editor-apps/cursor.png`（编辑器应用图标，非 provider 图标）

#### CLI 调研结果

```bash
$ cursor agent --help
Usage: cursor agent [options] [command] [prompt...]

Start the Cursor Agent
```

`cursor` CLI 已安装于 `/Users/wuerping/.local/bin/cursor`。

**核心选项：**

| 选项 | 说明 |
|------|------|
| `-p, --print` | 非交互式打印模式 |
| `--output-format <format>` | `text` / `json` / `stream-json` |
| `--stream-partial-output` | 流式输出文本增量 |
| `--mode <mode>` | `plan` / `ask` |
| `--plan` | plan 模式简写 |
| `--resume [chatId]` | 恢复会话 |
| `--continue` | 继续上一个会话 |
| `--model <model>` | `gpt-5`, `sonnet-4`, `sonnet-4-thinking` 等 |
| `-f, --force` / `--yolo` | 自动批准所有操作 |
| `--trust` | 信任当前工作区（headless 必需） |
| `--workspace <path>` | 指定工作目录 |
| `--sandbox <mode>` | 沙盒模式开关 |

**stream-json 输出格式（据文档和第三方 SDK）：**

```jsonl
{"type": "start", "chatId": "abc123"}
{"type": "content", "delta": "Analyzing..."}
{"type": "tool_use", "tool": "read_file", "args": {"path": "main.py"}}
{"type": "content", "delta": "Found 5 functions..."}
{"type": "end", "result": "Analysis complete"}
```

⚠️ **注意**：当前因网络/认证原因，未能实际测试 `cursor agent --print` 的流式输出。实现基于官方文档和第三方 SDK（`cursor-cli`, `@nothumanwork/cursor-agents-sdk`）推断。

---

## 三、Solo Provider 技术架构回顾

每个 Provider 需实现两个 Go 接口：

### 3.1 AgentClient（Provider 级别）

```go
type AgentClient interface {
    Provider() string
    IsAvailable(ctx context.Context) error
    CreateSession(ctx context.Context, config *protocol.AgentSessionConfig) (AgentSession, error)
    ResumeSession(ctx context.Context, handle *protocol.AgentPersistenceHandle) (AgentSession, error)
    ListModels(ctx context.Context, cwd string) ([]protocol.AgentModelDefinition, error)
    ListModes(ctx context.Context, cwd string) ([]protocol.AgentMode, error)
    ListClientCommands(ctx context.Context, cwd string) ([]protocol.AgentSlashCommand, error)
}
```

### 3.2 AgentSession（Session 级别）

```go
type AgentSession interface {
    Run(ctx context.Context, text string, images []protocol.ImageAttachment, attachments []protocol.AgentAttachment) (*AgentRunResult, error)
    StartTurn(ctx context.Context, text string, images []protocol.ImageAttachment, attachments []protocol.AgentAttachment) (<-chan AgentStreamEvent, error)
    Subscribe() <-chan AgentStreamEvent
    Interrupt(ctx context.Context) error
    Close() error
    RespondPermission(requestID string, response protocol.AgentPermissionResponse) error
    GetRuntimeInfo(ctx context.Context) (*protocol.AgentRuntimeInfo, error)
    GetAvailableModes(ctx context.Context) ([]protocol.AgentMode, error)
    GetCurrentMode(ctx context.Context) (*string, error)
    SetMode(modeID string) error
    SetModel(modelID string) error
    SetThinkingOption(optionID string) error
    DescribePersistence() *protocol.AgentPersistenceHandle
    GetPendingPermissions() []interface{}
    ListCommands(ctx context.Context) ([]protocol.AgentSlashCommand, error)
    StreamHistory(ctx context.Context) ([]AgentStreamEvent, error)
}
```

### 3.3 现有参考实现

| Provider | 文件 | 模式 | 特点 |
|---------|------|------|------|
| Claude | `provider_claude.go` | stdio | 启动 `claude --print --output-format stream-json`，逐行解析 SDK 消息，自定义 translator + terminal detector |
| OpenCode | `provider_opencode.go` | HTTP server | 启动本地 server，`/session` API + SSE `/global/event`，支持 reasoning/thinking |
| Mock | `provider_mock.go` | 内存 | 测试用，模拟事件流 |

**关键基础设施：**
- `base.BaseSession` — 通用 session 状态管理（sessionID、mode、model、cancelFn）
- `base.ProcessManager` — 子进程生命周期管理（Start/Stop/Interrupt/Kill/DrainStderr/WaitForExit）
- `base.ChannelDispatcher` — 事件分发（订阅/广播）
- `base.EventPump` — 阻塞/后台事件泵（逐行读取 stdout，调用 translator + terminal detector）

---

## 四、推荐方案

### 4.1 Kimi — Wire 模式（强烈推荐）

**理由：**
1. Wire 协议是 Kimi 官方为"嵌入其他应用"设计的协议，文档完整
2. 真正的双向流式通信，支持增量内容、工具调用、权限请求
3. 与 Claude 的 stdio 模式架构最相似（逐行读取 stdout，翻译为内部事件）
4. 支持 session 持久化（`--session` / `--continue`）
5. 避免 Print 模式的"整 turn 缓冲"问题

**架构图：**

```
┌─────────────────┐      stdin      ┌─────────────────┐
│                 │ ──────────────> │                 │
│  Solo Daemon    │   JSON-RPC req  │  kimi --wire    │
│  (provider_kimi)│                 │  (Wire server)  │
│                 │ <────────────── │                 │
│                 │   stdout        │                 │
│                 │   JSON-RPC      │                 │
│                 │   event/request │                 │
└─────────────────┘                 └─────────────────┘
         │                                    │
         │ Translate Wire events              │ LLM / Tools
         │ to AgentStreamEvent                │
         v                                    v
┌─────────────────┐                 ┌─────────────────┐
│  Solo Event     │                 │  Moonshot AI    │
│  Pipeline       │                 │  Kimi API       │
│  (timeline,     │                 │                 │
│   coalescer)    │                 │                 │
└─────────────────┘                 └─────────────────┘
```

**实现要点：**

1. **进程启动**：`kimi --wire --work-dir <cwd> --session <id>`（或 `--continue`）
2. **握手**：通过 stdin 发送 `initialize` 请求，协商 `protocol_version: "1.10"`
3. **启动 Turn**：通过 stdin 发送 `prompt` 请求
4. **读取事件**：逐行读取 stdout，解析 JSON-RPC
   - `method: "event"` → 翻译为 `AgentStreamEvent`，推入 dispatcher
   - `method: "request"` → 处理 `ApprovalRequest`，通过 stdin 写回 response
5. **事件翻译映射**：

| Wire Event | Solo Event |
|-----------|-----------|
| `TurnBegin` | `thread_started` |
| `ContentPart(text)` | `timeline(assistant_message)` |
| `ContentPart(think)` | `timeline(reasoning)` |
| `ToolCall` | `timeline(tool_call)` |
| `ToolResult` | `timeline(tool_call completed)` |
| `ApprovalRequest` | `permission_requested` |
| `CompactionBegin` | `timeline(compaction loading)` |
| `CompactionEnd` | `timeline(compaction completed)` |
| `TurnEnd` | `turn_completed` |
| `StepRetry` | `timeline(error)` |

6. **权限响应**：对于 `ApprovalRequest`，在 `RespondPermission()` 中通过 stdin 写回 JSON-RPC response
7. **中断**：发送 `cancel` 请求（JSON-RPC），或发送 SIGINT

### 4.2 Cursor-Agent — Print Stream-JSON 模式

**理由：**
1. Cursor 没有公开的 Wire/JSON-RPC/ACP 协议
2. `--print --output-format stream-json --stream-partial-output` 提供逐行 NDJSON 流
3. 架构上与 Claude 的 stdio 模式最接近，可复用 `base.EventPump` + translator 模式

**架构图：**

```
┌─────────────────┐      stdout     ┌─────────────────────────┐
│                 │ <────────────── │                         │
│  Solo Daemon    │   NDJSON lines  │  cursor agent --trust   │
│  (provider_     │                 │  --print                │
│   cursor_agent) │                 │  --output-format        │
│                 │                 │   stream-json           │
│                 │                 │  --stream-partial-output│
└─────────────────┘                 └─────────────────────────┘
         │                                        │
         │ Translate Cursor events                │ Cursor Cloud / LLM
         │ to AgentStreamEvent                    │
         v                                        v
┌─────────────────┐                 ┌─────────────────────────┐
│  Solo Event     │                 │  Cursor Services        │
│  Pipeline       │                 │                         │
└─────────────────┘                 └─────────────────────────┘
```

**实现要点：**

1. **进程启动**：`cursor agent --trust --print --output-format stream-json --stream-partial-output --workspace <cwd> --resume <id> <prompt>`
2. **读取事件**：逐行读取 stdout 的 NDJSON
3. **事件翻译映射（推测，需实测验证）**：

| Cursor Event | Solo Event |
|-------------|-----------|
| `start` | `thread_started` |
| `content(delta)` | `timeline(assistant_message)`（增量累积） |
| `thinking` / `reasoning` | `timeline(reasoning)` |
| `tool_use` | `timeline(tool_call)` |
| `tool_result` | `timeline(tool_call completed)` |
| `permission_request` | `permission_requested` |
| `end` | `turn_completed` |
| `error` | `turn_failed` |

4. **中断**：发送 SIGINT 到进程
5. **Trust 处理**：必须始终携带 `--trust`，避免 headless 模式下的交互式确认挂起

---

## 五、实施计划

### Phase 1: Kimi Provider（优先，预计 2-3 天）

| # | 文件 | 操作 | 说明 |
|---|------|------|------|
| 1 | `daemon/internal/agent/provider_kimi.go` | **新建** | `KimiAgentClient` + `kimiSession` + `kimiWireTranslator` + `kimiWireTerminalDetector` |
| 2 | `daemon/internal/server/daemon.go` | **修改** | 添加 `registry.Register(agent.NewKimiAgentClient("", logger))` |
| 3 | `daemon/internal/agent/provider_kimi_test.go` | **新建** | 单元测试（mock stdin/stdout 的 Wire 交互） |

**`provider_kimi.go` 结构草案：**

```go
package agent

// KimiAgentClient 实现 AgentClient
type KimiAgentClient struct {
    binaryPath string
    logger     *slog.Logger
}

// kimiSession 实现 AgentSession
type kimiSession struct {
    mu sync.Mutex
    base       *base.BaseSession
    dispatcher *base.ChannelDispatcher
    process    processManager
    binaryPath string
    cmd        *exec.Cmd
    stdinPipe  io.WriteCloser
    stdoutPipe io.ReadCloser
    activeTurnID string
    // JSON-RPC 状态
    nextRequestID int
    pendingApprovals map[string]chan string // requestID -> response channel
}

// kimiWireTranslator 将 Wire 事件翻译为 AgentStreamEvent
type kimiWireTranslator struct {
    session *kimiSession
}

// kimiWireTerminalDetector 检测 turn 结束
type kimiWireTerminalDetector struct {
    session *kimiSession
}
```

### Phase 2: Cursor-Agent Provider（预计 1-2 天）

| # | 文件 | 操作 | 说明 |
|---|------|------|------|
| 4 | `daemon/internal/agent/provider_cursor_agent.go` | **新建** | `CursorAgentClient` + `cursorSession` + `cursorTranslator` + `cursorTerminalDetector` |
| 5 | `daemon/internal/server/daemon.go` | **修改** | 添加 `registry.Register(agent.NewCursorAgentClient("", logger))` |
| 6 | `daemon/internal/agent/provider_cursor_agent_test.go` | **新建** | 单元测试 |

### Phase 3: 前端定义与图标（预计 0.5 天）

| # | 文件 | 操作 | 说明 |
|---|------|------|------|
| 7 | `app-bridge/src/server/agent/provider-manifest.ts` | **修改** | 添加 `CURSOR_AGENT_MODES` 和 `cursor-agent` 到 `AGENT_PROVIDER_DEFINITIONS` |
| 8 | `daemon/internal/agent/provider_registry.go` | **修改** | 在 `BuiltinProviderDefinitions()` 中添加 `cursor-agent` |
| 9 | `app/src/utils/provider-command-templates.ts` | **修改** | 添加 `cursor-agent` resume 模板 |
| 10 | `app/src/components/icons/kimi-icon.tsx` | **新建** | Kimi Logo SVG React 组件 |
| 11 | `app/src/components/icons/cursor-agent-icon.tsx` | **新建** | Cursor Logo SVG React 组件 |
| 12 | `app/src/components/provider-icons.ts` | **修改** | 添加 `kimi` 和 `cursor-agent` 映射 |

### Phase 4: 集成测试与优化（预计 1-2 天）

| # | 任务 | 说明 |
|---|------|------|
| 13 | E2E 测试 | 通过 App 或 CLI 创建 Kimi/Cursor-Agent agent，验证完整生命周期 |
| 14 | 流式事件验证 | 确认 timeline 事件正确到达前端，无丢失或乱序 |
| 15 | 权限请求测试 | 验证 ApprovalRequest/permission_requested 端到端流程 |
| 16 | Session 恢复测试 | 验证 `--resume` / `--continue` 恢复现有会话 |
| 17 | 错误处理测试 | 验证 LLM 未配置、网络错误等场景的优雅降级 |

---

## 六、风险与注意事项

### 6.1 Kimi Wire 模式

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| **协议版本演进** | Wire 1.10 未来可能变更 | `initialize` 时明确声明 `protocol_version`，对不兼容版本拒绝连接并提示升级 |
| **ApprovalRequest 响应延迟** | 用户未及时响应导致 agent 挂起 | 设置合理的超时（默认 30s），超时后自动 reject；支持 `approve_for_session` |
| **JSON-RPC ID 冲突** | 并发 request/response 可能乱序 | 使用 UUID 或原子递增 ID，维护 `pendingRequests` map |
| **stdin 写入并发** | `RespondPermission` 与主循环同时写 stdin | 使用带锁的 writer，或所有写入通过单一 goroutine 串行化 |
| **进程崩溃检测** | `kimi --wire` 启动失败或运行时崩溃 | 复用 Claude provider 的 100ms 启动健康检查机制 |
| **会话目录冲突** | Solo 的 cwd 与 Kimi 的 `--work-dir` 不一致 | 始终将 `config.Cwd` 映射为 `--work-dir`，session ID 映射为 `--session` |

### 6.2 Cursor-Agent

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| **输出格式未实测** | stream-json 的字段名/结构可能与文档有偏差 | 实现时添加宽松的解析逻辑（未知字段忽略），预留调试日志开关 |
| **Trust 要求** | 缺少 `--trust` 时 headless 模式挂起等待用户输入 | 强制添加 `--trust` 参数；如仍需确认，前置检测并返回明确错误 |
| **认证问题** | `cursor agent` 需要登录或 `CURSOR_API_KEY` | `IsAvailable()` 中运行 `cursor agent status` 或轻量命令检测认证状态 |
| **网络依赖** | Cursor 调用云端服务，可能超时或失败 | 设置合理的进程超时（如 35min watchdog），错误时输出 `turn_failed` |
| **模型名称变动** | `--model` 支持的模型列表可能变更 | `ListModels()` 优先调用 `cursor agent models` 动态获取，失败时回退静态列表 |

### 6.3 通用

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| **流式 vs 完整输出** | 两个 provider 都需要翻译外部事件格式 | 建立统一的 translator 测试框架，验证每种事件类型的输出 |
| **模型列表** | 动态查询可能失败或慢 | 支持静态 fallback，缓存动态结果 |
| **历史记录** | `StreamHistory()` 实现复杂 | Phase 1 可先返回 `nil, nil`（非阻塞），后续迭代通过 CLI 的 replay/history 命令实现 |
| **图标版权** | 使用品牌 Logo 可能涉及版权问题 | 使用通用 `Bot` 图标作为占位，或确认品牌使用许可后再替换 |

---

## 七、工作量估算

| Phase | 内容 | 预估代码量 | 预估时间 |
|-------|------|-----------|---------|
| Phase 1 | Kimi Provider (Go) | ~800 行 | 2-3 天 |
| Phase 2 | Cursor-Agent Provider (Go) | ~500 行 | 1-2 天 |
| Phase 3 | 前端定义 + 图标 (TS/TSX) | ~200 行 | 0.5 天 |
| Phase 4 | 集成测试 + 调优 | — | 1-2 天 |
| **合计** | | **~1500 行** | **5-7.5 天** |

---

## 八、附录

### A. Kimi CLI 关键命令速查

```bash
# 启动 Wire server
kimi --wire --work-dir /path/to/project

# 指定 session
kimi --wire --session <session-id>

# 继续上一个 session
kimi --wire --continue

# 指定模型
kimi --wire --model k2

# Plan 模式
kimi --wire --plan

# YOLO 模式
kimi --wire --yolo

# 查看信息
kimi info
```

### B. Cursor Agent CLI 关键命令速查

```bash
# 启动 agent（headless）
cursor agent --trust --print --output-format stream-json --stream-partial-output "prompt"

# 指定工作区
cursor agent --trust --workspace /path/to/project --print ...

# 恢复会话
cursor agent --trust --resume <chatId> --print ...

# Plan 模式
cursor agent --trust --plan --print ...

# YOLO 模式
cursor agent --trust --yolo --print ...

# 列出模型
cursor agent models
```

### C. 相关文档链接

- [Kimi Wire Mode 文档](https://moonshotai.github.io/kimi-cli/en/customization/wire-mode.md)
- [Kimi Print Mode 文档](https://moonshotai.github.io/kimi-cli/en/customization/print-mode.md)
- [Cursor CLI 文档 (PraisonAI)](https://docs.praison.ai/docs/cli/cursor-cli)
- [Cursor Agents SDK (npm)](https://www.npmjs.com/package/@nothumanwork/cursor-agents-sdk)
- [Kimi CLI Issue #2179 — 增量 token deltas 功能请求](https://github.com/MoonshotAI/kimi-cli/issues/2179)
