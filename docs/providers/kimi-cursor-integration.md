# Kimi & Cursor-Agent Provider Integration

> Analysis date: 2026-05-21
> Status update: 2026-06-01
> Scope: daemon (Go) + app-bridge (TS) + app (React Native)

---

## 状态总览

| Provider | 状态 | 集成方式 | 复杂度 |
|---------|------|---------|--------|
| **Kimi** | **✅ 已实现** | Wire mode (`kimi --wire`), JSON-RPC 2.0 stdio | — |
| **Cursor-Agent** | **❌ 未实现** | Print mode (`cursor agent --print --output-format stream-json`) | Medium |

---

## Kimi — 已实现

Kimi Wire mode provider 已完整实现，无需进一步工作。

**实现文件**:
- `daemon/internal/agent/provider_kimi.go` — 758 LOC, Wire mode, JSON-RPC 2.0 stdio, EventPump
- `daemon/internal/agent/provider_kimi_test.go` — 23 unit tests
- `daemon/internal/agent/provider_registry.go` — `kimi` ID/Label/Modes 注册
- `app-bridge/src/server/agent/provider-manifest.ts` — `KIMI_MODES` 定义
- `app/src/utils/provider-command-templates.ts` — `kimi --resume {sessionId}` 命令模板

**技术选型**: `kimi --wire`（JSON-RPC 2.0 over stdin/stdout）提供真正的双向流式传输，避免了 `--print --output-format stream-json` 的"整轮缓冲"问题。

---

## Cursor-Agent — 待实现

### 当前状态

- 项目中无 `cursor-agent` 定义、实现或图标
- `app/assets/images/editor-apps/cursor.png` 存在（编辑器图标，非 provider 图标）

### CLI 调研

```bash
$ cursor agent --help
Usage: cursor agent [options] [command] [prompt...]
```

**核心选项**:

| 选项 | 描述 |
|------|------|
| `-p, --print` | 非交互式 print 模式 |
| `--output-format <format>` | `text` / `json` / `stream-json` |
| `--stream-partial-output` | 增量文本流输出 |
| `--mode <mode>` | `plan` / `ask` |
| `--resume [chatId]` | 恢复会话 |
| `--model <model>` | `gpt-5`, `sonnet-4`, `sonnet-4-thinking` 等 |
| `-f, --force` / `--yolo` | 自动批准所有操作 |
| `--trust` | 信任当前 workspace（headless 模式必需） |
| `--workspace <path>` | 指定工作目录 |

**stream-json 输出格式**（来自文档和第三方 SDK）:

```jsonl
{"type": "start", "chatId": "abc123"}
{"type": "content", "delta": "Analyzing..."}
{"type": "tool_use", "tool": "read_file", "args": {"path": "main.py"}}
{"type": "content", "delta": "Found 5 functions..."}
{"type": "end", "result": "Analysis complete"}
```

> **注意**: 目前因网络/认证问题无法测试 `cursor agent --print` 的流式输出。实现基于官方文档和第三方 SDK（`cursor-cli`, `@nothumanwork/cursor-agents-sdk`）推断。

### 推荐方案：Print Stream-JSON 模式

**理由**:
1. Cursor 无公开 Wire/JSON-RPC/ACP 协议
2. `--print --output-format stream-json --stream-partial-output` 提供逐行 NDJSON 流
3. 架构最接近 Claude 的 stdio 模式，可复用 `base.EventPump` + translator 模式

**架构**:

```
┌─────────────────┐      stdout     ┌─────────────────────────┐
│                 │ <────────────── │                         │
│  Solo Daemon    │   NDJSON lines  │  cursor agent --trust   │
│  (provider_     │                 │  --print                │
│   cursor_agent) │                 │  --output-format        │
│                 │                 │   stream-json           │
└─────────────────┘                 └─────────────────────────┘
         │                                        │
         │ Translate Cursor events                │ Cursor Cloud / LLM
         v                                        v
┌─────────────────┐                 ┌─────────────────────────┐
│  Solo Event     │                 │  Cursor Services        │
│  Pipeline       │                 │                         │
└─────────────────┘                 └─────────────────────────┘
```

**实现要点**:
1. **进程启动**: `cursor agent --trust --print --output-format stream-json --stream-partial-output --workspace <cwd> --resume <id> <prompt>`
2. **读取事件**: 逐行 NDJSON from stdout
3. **事件映射**:

| Cursor Event | Solo Event |
|-------------|-----------|
| `start` | `thread_started` |
| `content(delta)` | `timeline(assistant_message)` (增量累积) |
| `thinking` / `reasoning` | `timeline(reasoning)` |
| `tool_use` | `timeline(tool_call)` |
| `tool_result` | `timeline(tool_call completed)` |
| `permission_request` | `permission_requested` |
| `end` | `turn_completed` |
| `error` | `turn_failed` |

4. **中断**: 发送 SIGINT
5. **Trust 处理**: 必须始终包含 `--trust` 以避免 headless 模式交互式确认挂起

### 实现计划

| # | 文件 | 操作 | 描述 |
|---|------|------|------|
| 1 | `daemon/internal/agent/provider_cursor_agent.go` | 新建 | `CursorAgentClient` + `cursorSession` + `cursorTranslator` |
| 2 | `daemon/internal/server/daemon.go` | 修改 | 注册 `NewCursorAgentClient` |
| 3 | `daemon/internal/agent/provider_cursor_agent_test.go` | 新建 | 单元测试 |
| 4 | `app-bridge/src/server/agent/provider-manifest.ts` | 修改 | 添加 `CURSOR_AGENT_MODES` |
| 5 | `daemon/internal/agent/provider_registry.go` | 修改 | 添加 `cursor-agent` 定义 |
| 6 | `app/src/utils/provider-command-templates.ts` | 修改 | 添加 resume 模板 |
| 7 | `app/src/components/icons/cursor-agent-icon.tsx` | 新建 | Cursor Logo SVG 组件 |
| 8 | `app/src/components/provider-icons.ts` | 修改 | 添加 `cursor-agent` 映射 |

**预估工作量**: ~500 LOC, 1-2 天

### 风险

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| 输出格式未验证 | stream-json 字段可能与文档不同 | 宽松解析（忽略未知字段），保留 debug 日志 |
| Trust 要求 | 缺少 `--trust` 导致 headless 挂起 | 强制添加 `--trust`；预检测并返回明确错误 |
| 认证问题 | `cursor agent` 需要登录或 API key | `IsAvailable()` 中运行 `cursor agent status` 检测 |
| 网络依赖 | Cursor 调用云服务，可能超时 | 合理超时（35min watchdog），错误时输出 `turn_failed` |
| 模型名称变化 | `--model` 支持列表可能变化 | `ListModels()` 优先动态查询，失败时回退静态列表 |

---

## Provider 架构参考

每个 Provider 需实现两个 Go 接口：

**AgentClient** (Provider 级): `Provider()`, `IsAvailable()`, `CreateSession()`, `ResumeSession()`, `ListModels()`, `ListModes()`, `ListClientCommands()`

**AgentSession** (Session 级): `Run()`, `StartTurn()`, `Subscribe()`, `Interrupt()`, `Close()`, `RespondPermission()`, `SetMode()`, `SetModel()`, `StreamHistory()` 等

**已有参考实现**:
- Claude (`provider_claude.go`): stdio, `--print --output-format stream-json`, 逐行 SDK message 解析
- OpenCode (`provider_opencode.go`): HTTP server, `/session` API + SSE `/global/event`
- Kimi (`provider_kimi.go`): Wire mode, JSON-RPC 2.0 stdio, EventPump
- Mock (`provider_mock.go`): 内存测试用

**关键基础设施**:
- `base.BaseSession` — 公共会话状态管理
- `base.ProcessManager` — 子进程生命周期
- `base.ChannelDispatcher` — 事件分发
- `base.EventPump` — 阻塞/后台事件泵（逐行 stdout 读取，调用 translator + terminal detector）

---

## CLI 命令参考

### Cursor Agent

```bash
# Headless 模式启动
cursor agent --trust --print --output-format stream-json --stream-partial-output "prompt"

# 指定 workspace
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

---

## 相关文档

- [Kimi Wire vs ACP](kimi-wire-vs-acp.md) — Kimi 协议选型决策
- [Provider Type Erasure Analysis](../analysis/go-provider-type-erasure-analysis.md) — `interface{}` 增长问题诊断
