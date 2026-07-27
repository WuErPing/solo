# Coding Agent Hook 系统对比分析

> 日期: 2026-07-26
> 目的: 分析 Claude Code、Codex、Qoder、Kimi Code、OpenCode、Pi、Cursor 七个编码代理的 Hook 机制，为 Solo 平台 Hook 支持设计提供参考。

---

## 1. 总览对比

| 维度 | Claude Code | Codex | Qoder | Kimi Code | OpenCode | Pi | Cursor |
|------|-------------|-------|-------|-----------|----------|-----|--------|
| Hook 事件数 | ~30 | 6 | 21 | 13 | 40+ (plugin) | 30+ (extension) | 21 |
| 配置格式 | JSON | JSON | JSON | TOML | TypeScript 插件 | TypeScript 扩展 | JSON |
| 配置位置 | `~/.claude/settings.json` + `.claude/settings.json` | `~/.codex/hooks.json` + `.codex/hooks.json` | `~/.qoder/settings.json` + `.qoder/settings.json` | `~/.kimi-code/config.toml` + `.kimi-code/local.toml` | `.opencode/plugins/*.ts` + `opencode.json` | `~/.pi/agent/extensions/*.ts` + `.pi/extensions/*.ts` | `~/.cursor/hooks.json` + `.cursor/hooks.json` |
| Handler 类型 | command / http / mcp_tool / prompt / agent | command | command / http / prompt / agent | command | TypeScript 代码 | TypeScript 代码 | command / prompt |
| 输入传递 | stdin JSON | stdin JSON | stdin JSON | stdin JSON | 函数参数 (event, ctx) | 函数参数 (event, ctx) | stdin JSON |
| 阻断机制 | exit 2 / JSON decision | exit 2 / JSON decision | exit 2 / JSON decision | exit 2 / JSON hookSpecificOutput | throw Error | return { block: true } | exit 2 / JSON permission |
| Matcher | 正则 + `if` glob | 正则 | 正则 + `if` glob | 正则 | 代码条件判断 | 代码条件判断 | 正则 |
| 默认启用 | 是 | 否 (feature flag) | 是 | 是 (Beta) | 是 | 是 | 是 (Beta) |
| 异步 Hook | 支持 (`async: true`) | 不支持 | 支持 (`async` + `asyncRewake`) | 不支持 | N/A (代码控制) | N/A (代码控制) | 不支持 |
| LLM Hook | 支持 (`prompt` / `agent` 类型) | 不支持 | 支持 (`prompt` / `agent` 类型) | 不支持 | N/A | N/A | 支持 (`prompt` 类型) |

---

## 2. Hook 事件对比矩阵

### 2.1 会话生命周期

| 事件 | Claude Code | Codex | Qoder | Kimi Code | OpenCode | Pi | Cursor |
|------|:-----------:|:-----:|:-----:|:---------:|:--------:|:--:|:------:|
| SessionStart | ✅ | ✅ | ✅ | ✅ | ✅ `session.created` | ✅ `session_start` | ✅ `sessionStart` |
| SessionEnd | ✅ | ❌ | ✅ | ✅ | ✅ `session.deleted` | ✅ `session_shutdown` | ✅ `sessionEnd` |
| UserPromptSubmit | ✅ | ✅ | ✅ | ✅ | ❌ | ✅ `input` | ✅ `beforeSubmitPrompt` |
| Stop (Agent 完成) | ✅ | ✅ | ✅ | ✅ | ✅ `session.idle` | ✅ `agent_end` | ✅ `stop` |
| StopFailure | ✅ | ❌ | ✅ | ✅ | ✅ `session.error` | ❌ | ❌ |
| ConfigChange | ✅ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ |
| CwdChanged | ✅ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ |
| InstructionsLoaded | ✅ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ |
| workspaceOpen | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ |

### 2.2 工具调用

| 事件 | Claude Code | Codex | Qoder | Kimi Code | OpenCode | Pi | Cursor |
|------|:-----------:|:-----:|:-----:|:---------:|:--------:|:--:|:------:|
| PreToolUse | ✅ | ✅ | ✅ | ✅ | ✅ `tool.execute.before` | ✅ `tool_call` | ✅ `preToolUse` |
| PostToolUse | ✅ | ✅ | ✅ | ✅ | ✅ `tool.execute.after` | ✅ `tool_result` | ✅ `postToolUse` |
| PostToolUseFailure | ✅ | ❌ | ✅ | ✅ | ❌ | ❌ | ✅ `postToolUseFailure` |
| PermissionRequest | ✅ | ✅ | ✅ | ❌ | ✅ `permission.ask` | ❌ | ❌ |
| PermissionDenied | ✅ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ |
| PostToolBatch | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| tool.definition | ❌ | ❌ | ❌ | ❌ | ✅ | ❌ | ❌ |
| beforeShellExecution | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ |
| afterShellExecution | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ |
| beforeMCPExecution | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ |
| afterMCPExecution | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ |
| beforeReadFile | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ |
| afterFileEdit | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ |

### 2.3 子代理 & 任务

| 事件 | Claude Code | Codex | Qoder | Kimi Code | OpenCode | Pi | Cursor |
|------|:-----------:|:-----:|:-----:|:---------:|:--------:|:--:|:------:|
| SubagentStart | ✅ | ❌ | ✅ | ✅ | ❌ | ❌ | ✅ `subagentStart` |
| SubagentStop | ✅ | ❌ | ✅ | ✅ | ❌ | ❌ | ✅ `subagentStop` |
| TaskCreated | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| TaskCompleted | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| TeammateIdle | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |

### 2.4 上下文压缩

| 事件 | Claude Code | Codex | Qoder | Kimi Code | OpenCode | Pi | Cursor |
|------|:-----------:|:-----:|:-----:|:---------:|:--------:|:--:|:------:|
| PreCompact | ✅ | ❌ | ✅ | ✅ | ✅ `experimental.session.compacting` | ✅ `session_before_compact` | ✅ `preCompact` |
| PostCompact | ✅ | ❌ | ✅ | ✅ | ✅ `session.compacted` | ✅ `session_compact` | ❌ |

### 2.5 通知 & 文件 & 其他

| 事件 | Claude Code | Codex | Qoder | Kimi Code | OpenCode | Pi | Cursor |
|------|:-----------:|:-----:|:-----:|:---------:|:--------:|:--:|:------:|
| Notification | ✅ | ❌ | ✅ | ✅ | ❌ | ❌ | ❌ |
| FileChanged | ✅ | ❌ | ✅ | ❌ | ✅ `file.watcher.updated` | ❌ | ❌ |
| WorktreeCreate | ✅ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ |
| WorktreeRemove | ✅ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ |
| Elicitation (MCP) | ✅ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ |
| ElicitationResult | ✅ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ |
| MessageDisplay | ✅ | ❌ | ❌ | ❌ | ✅ `message.part.updated` | ✅ `message_update` | ✅ `afterAgentResponse` |
| afterAgentThought | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ |
| shell.env | ❌ | ❌ | ❌ | ❌ | ✅ | ❌ | ❌ |
| chat.params | ❌ | ❌ | ❌ | ❌ | ✅ | ❌ | ❌ |
| chat.system.transform | ❌ | ❌ | ❌ | ❌ | ✅ | ✅ `before_agent_start` | ❌ |
| Provider HTTP 拦截 | ❌ | ❌ | ❌ | ❌ | ✅ `chat.headers` | ✅ `before_provider_request` | ❌ |
| context (消息过滤) | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ `context` | ❌ |

---

## 3. 各代理详细分析

### 3.1 Claude Code

**架构**: 声明式 JSON 配置 + 多类型 Handler

**配置位置** (优先级从高到低):
1. Organization managed policy
2. `.claude/settings.local.json` (项目本地, gitignored)
3. `.claude/settings.json` (项目共享)
4. `~/.claude/settings.json` (用户全局)

**配置格式**:
```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "if": "Bash(git *)",
        "hooks": [
          {
            "type": "command",
            "command": "$CLAUDE_PROJECT_DIR/.claude/hooks/guard.sh",
            "args": ["--strict"],
            "timeout": 10,
            "statusMessage": "Running security check...",
            "async": false
          }
        ]
      }
    ]
  }
}
```

**Handler 类型**:
| 类型 | 用途 | 默认超时 |
|------|------|----------|
| `command` | 执行 shell 命令 | 600s |
| `http` | POST 到 URL | 30s |
| `mcp_tool` | 调用 MCP 服务器工具 | - |
| `prompt` | 单轮 LLM 评估 → `{ok, reason}` | 30s |
| `agent` | 多轮子代理验证 (实验性) | - |

**输入** (stdin JSON):
```json
{
  "session_id": "abc123",
  "transcript_path": "/path/to/transcript",
  "cwd": "/Users/me/project",
  "hook_event_name": "PreToolUse",
  "permission_mode": "default",
  "tool_name": "Bash",
  "tool_input": { "command": "rm -rf build/" }
}
```

**环境变量**: `CLAUDE_PROJECT_DIR`, `CLAUDE_PLUGIN_ROOT`, `CLAUDE_ENV_FILE`, `CLAUDE_CODE_REMOTE`, `CLAUDE_TOOL_INPUT_FILE_PATH`

**输出协议**:
| Exit Code | 行为 |
|-----------|------|
| 0 | 成功; stdout 解析为 JSON 指令 |
| 2 | 阻断; stderr 反馈给模型 |
| 其他 | 非阻断错误; stderr 展示给用户 |

**PreToolUse JSON 输出**:
```json
{
  "hookSpecificOutput": {
    "permissionDecision": "allow|deny|ask|defer",
    "permissionDecisionReason": "原因",
    "updatedInput": { "command": "修改后的命令" },
    "additionalContext": "注入的额外上下文"
  }
}
```

**Matcher 语法**:
- 省略 / `""` / `"*"` → 匹配所有
- `"Bash"` → 精确匹配
- `"Edit|Write"` → 正则交替
- `"mcp__server__tool"` → MCP 工具
- `if` 字段: `"Bash(git *)"` → glob 匹配工具参数

**关键特性**:
- 支持修改工具输入 (`updatedInput`)
- 支持 `defer` 将决策传递给下一个 Hook
- 支持异步执行 (`async: true`)
- 支持 `watchPaths` 文件监控
- `disableAllHooks` / `allowManagedHooksOnly` 全局开关

---

### 3.2 Codex (OpenAI)

**架构**: 声明式 JSON 配置, 仅 command Handler, 实验性功能

**前置条件**: 需在 `~/.codex/config.toml` 启用:
```toml
[features]
codex_hooks = true
```

**配置位置**:
- `~/.codex/hooks.json` (用户全局)
- `<repo>/.codex/hooks.json` (项目级, 需信任仓库)

**配置格式**:
```json
{
  "PreToolUse": [
    {
      "matcher": "Bash",
      "hooks": [
        { "type": "command", "command": "node .codex/hooks/guard.mjs", "timeout": 600 }
      ]
    }
  ]
}
```

**仅支持 6 个事件**: `SessionStart`, `UserPromptSubmit`, `PreToolUse`, `PermissionRequest`, `PostToolUse`, `Stop`

**输入** (stdin JSON, snake_case):
```json
{
  "hook_event_name": "PreToolUse",
  "session_id": "abc-123",
  "cwd": "/path/to/project",
  "tool_name": "Bash",
  "tool_input": { "command": "rm -rf /tmp/build" }
}
```

**输出协议**:
| Exit Code | 行为 |
|-----------|------|
| 0 | 允许 (stdout 可选 JSON) |
| 2 | 阻断 |

**阻断输出** (camelCase):
```json
{ "decision": "block", "reason": "Destructive command prevented." }
```

**限制**:
- 仅 `command` 类型 (无 http/prompt/agent)
- 不能修改工具输入/输出, 只能 block/allow
- PreToolUse 不拦截 WebSearch 等非 shell 工具
- 企业管控: `requirements.toml` 中 `allow_managed_hooks_only = true`

---

### 3.3 Qoder

**架构**: 声明式 JSON 配置 + 多类型 Handler (与 Claude Code 高度相似)

**配置位置**:
- `~/.qoder/settings.json` (用户全局)
- `${project}/.qoder/settings.json` (项目共享)
- `${project}/.qoder/settings.local.json` (项目本地, gitignored)

**配置格式**:
```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "~/.qoder/hooks/guard.sh",
            "timeout": 600,
            "if": "Bash(git *)",
            "async": false,
            "asyncRewake": true,
            "rewakeMessage": "Background check failed",
            "once": false,
            "statusMessage": "Checking..."
          }
        ]
      }
    ]
  }
}
```

**Handler 类型**:
| 类型 | 用途 | 默认超时 |
|------|------|----------|
| `command` | 执行 shell 命令 | 600s |
| `http` | POST JSON 到 URL | 30s |
| `prompt` | 单轮 LLM 评估 | 30s |
| `agent` | 子代理验证 (需调用 StructuredOutput) | 60s |

**输入** (stdin JSON):
```json
{
  "session_id": "...",
  "transcript_path": "...",
  "cwd": "/path/to/project",
  "hook_event_name": "PreToolUse",
  "permission_mode": "default",
  "agent_id": "...",
  "agent_type": "...",
  "tool_name": "Bash",
  "tool_input": { "command": "..." },
  "tool_use_id": "..."
}
```

**环境变量**: `QODER_PROJECT_DIR`, `QODER_PLUGIN_ROOT`, `QODER_PLUGIN_DATA`

**输出协议**: 与 Claude Code 相同 (exit 0/2/other)

**独有特性**:
- `asyncRewake`: 异步 Hook 完成后唤醒 Agent 并注入结果
- `rewakeMessage` / `rewakeSummary`: 自定义唤醒消息
- `once: true`: Hook 仅执行一次后自动移除
- `PostToolUse` 支持 `updatedToolOutput` 修改工具输出
- `PermissionDenied` 支持 `retry: true` 重试
- 冲突决策: 最严格者优先 (deny > allow)
- IDE 插件仅支持 5 个事件: `UserPromptSubmit`, `PreToolUse`, `PostToolUse`, `PostToolUseFailure`, `Stop`

---

### 3.4 Kimi Code (Moonshot AI)

**架构**: 声明式 TOML 配置, 仅 command Handler, Beta 阶段

**配置位置**:
- `~/.kimi-code/config.toml` (用户全局, 可通过 `KIMI_CODE_HOME` 覆盖)
- `<project>/.kimi-code/local.toml` (项目级)

**配置格式**:
```toml
[[hooks]]
event = "PreToolUse"
matcher = "Bash"
command = "node ~/.kimi-code/hooks/check-bash.mjs"
timeout = 5

[[hooks]]
event = "PostToolUse"
matcher = "Write|Edit"
command = "~/.kimi-code/hooks/auto-format.sh"

[[hooks]]
event = "Notification"
matcher = "task\\.completed"
command = "terminal-notifier -title 'Kimi' -message 'Task done'"
```

**字段**: `event` (必填), `command` (必填), `matcher` (可选正则), `timeout` (1-600s, 默认 30)

**13 个事件**:
- 可阻断: `PreToolUse`, `UserPromptSubmit`, `Stop`
- 观察型: `PostToolUse`, `PostToolUseFailure`, `StopFailure`, `SessionStart`, `SessionEnd`, `SubagentStart`, `SubagentStop`, `PreCompact`, `PostCompact`, `Notification`

**输入** (stdin JSON):
```json
{
  "hook_event_name": "PreToolUse",
  "session_id": "...",
  "cwd": "/path/to/project",
  "tool_name": "Bash",
  "tool_input": { "command": "..." },
  "tool_call_id": "..."
}
```

**输出协议**:
| Exit Code | 行为 |
|-----------|------|
| 0 | 允许; stdout 追加到 Agent 上下文 |
| 2 | 阻断; stderr 反馈给 LLM |
| 其他 | Fail-open; stderr 记录日志但不影响流程 |

**JSON 阻断** (exit 0 时也可阻断):
```json
{
  "hookSpecificOutput": {
    "permissionDecision": "deny",
    "permissionDecisionReason": "Background tasks still running"
  }
}
```

**设计原则**:
- Fail-open: 超时/崩溃不阻断工作流
- 并行执行: 同事件多 Hook 并发运行
- 去重: 相同命令自动去重
- 防循环: Stop Hook 限制重触发次数
- 查看活跃 Hook: Shell 模式下 `/hooks` 命令

---

### 3.5 OpenCode (SST)

**架构**: TypeScript 插件系统 (无独立 hooks 配置)

**配置位置**:
- `.opencode/plugins/*.ts` (项目级, 自动加载)
- `~/.config/opencode/plugins/*.ts` (全局, 自动加载)
- `opencode.json` 中 `"plugin": ["@my-org/plugin"]` (npm 包)

**配置解析顺序**: Remote → Global → Project → Custom (env) → Managed (admin)

**插件格式**:
```typescript
import { type Plugin, tool } from "@opencode-ai/plugin"

export const MyPlugin: Plugin = async (ctx) => {
  // ctx.project, ctx.directory, ctx.worktree, ctx.client, ctx.$
  return {
    "tool.execute.before": async (input, output) => {
      if (output.args?.filePath?.includes(".env")) {
        throw new Error("Blocked: sensitive file")
      }
    },
    "tool.execute.after": async (input, output) => {
      if (input.tool === "write") {
        await ctx.$`prettier --write ${output.args.filePath}`
      }
    },
    "shell.env": async (input, output) => {
      output.env.MY_VAR = "value"
    },
    "experimental.chat.system.transform": async (input, output) => {
      output.system.push("Always follow TDD.")
    },
    event: async ({ event }) => {
      if (event.type === "session.idle") { /* notify */ }
    },
  }
}
```

**核心 Hook 点**:
| 类别 | Hook | 能力 |
|------|------|------|
| 工具 | `tool.execute.before` | 阻断 (throw) / 修改参数 (mutate output.args) |
| 工具 | `tool.execute.after` | 修改输出 (mutate output) |
| 工具 | `tool.definition` | 修改工具 schema |
| 会话 | `session.created/idle/compacted/deleted/error/status/updated` | 观察 |
| LLM | `chat.params` / `chat.message` / `chat.headers` | 修改 LLM 请求 |
| LLM | `experimental.chat.system.transform` | 注入系统提示 |
| 文件 | `file.edited` / `file.watcher.updated` | 观察 |
| 权限 | `permission.ask/asked/replied` | 拦截权限提示 |
| 命令 | `command.executed` / `command.execute.before` | 拦截斜杠命令 |
| 配置 | `config` | 动态注入命令/代理/MCP |
| Shell | `shell.env` | 注入环境变量 |
| 通用 | `event` | 订阅所有事件 |

**阻断方式**: `throw new Error("reason")` → 错误信息传递给 LLM

**Matcher**: 无声明式 matcher, 通过代码条件判断:
```typescript
if (input.tool === "bash" && output.args.command.includes("rm -rf")) { ... }
```

**独有能力**:
- 注册自定义工具 (`tool` 属性 + Zod schema)
- 动态注入 MCP 服务器、命令、代理 (`config` hook)
- Bun shell 内置 (`ctx.$`)
- 修改 LLM 参数/headers/系统提示

---

### 3.6 Pi (Earendil Works)

**架构**: TypeScript 扩展系统 (统一了早期 hooks + custom tools)

**配置位置**:
- `~/.pi/agent/extensions/*.ts` (全局自动发现)
- `.pi/extensions/*.ts` (项目级, 需信任)
- `.pi/settings.json` 或 `~/.pi/agent/settings.json` 中 `"extensions": [...]`
- `package.json` 中 `"pi": { "extensions": [...] }`
- CLI: `pi -e ./my-extension.ts`

**扩展格式**:
```typescript
import type { ExtensionAPI } from "@earendil-works/pi";

export default function (pi: ExtensionAPI) {
  pi.on("tool_call", async (event, ctx) => {
    if (event.toolName === "bash" && event.input.command.includes("rm -rf")) {
      return { block: true, reason: "Destructive command blocked" };
    }
  });

  pi.on("tool_result", async (event) => {
    // 修改/脱敏工具输出
    return { content: redactedContent };
  });

  pi.on("before_agent_start", async (event, ctx) => {
    return { systemPrompt: event.systemPrompt + "\nAlways use TDD." };
  });

  pi.on("context", async (event) => {
    return { messages: filteredMessages };
  });
}
```

**核心事件**:
| 类别 | 事件 | 能力 |
|------|------|------|
| 工具 | `tool_call` | 阻断 / 修改输入 |
| 工具 | `tool_result` | 修改输出 / 脱敏 |
| 工具 | `tool_execution_start/update/end` | 观察 |
| 代理 | `before_agent_start` | 注入系统提示 / 消息 |
| 代理 | `agent_start/end/settled` | 观察 |
| 轮次 | `turn_start/end` | 观察 |
| 消息 | `message_start/update/end` | 观察 / 替换最终消息 |
| 上下文 | `context` | 过滤/修改发送给 LLM 的消息 |
| 会话 | `session_start/info_changed/shutdown` | 观察 |
| 会话 | `session_before_switch/fork/compact` | 取消操作 |
| Provider | `before_provider_headers/request` | 修改 HTTP 请求 |
| Provider | `after_provider_response` | 修改响应 |
| 输入 | `user_bash` / `input` | 拦截/转换用户输入 |
| 模型 | `model_select` / `thinking_level_select` | 观察 |

**阻断方式**: `return { block: true, reason: "..." }`

**Matcher**: 无声明式 matcher, 代码条件判断 + TypeScript 类型守卫:
```typescript
if (isToolCallEventType("bash", event)) { /* narrowed typing */ }
```

**独有能力**:
- 注册自定义工具 (`pi.registerTool`)
- 注册斜杠命令 (`pi.registerCommand`)
- 注册快捷键 (`pi.registerShortcut`)
- 注册 CLI flag (`pi.registerFlag`)
- 动态注册/注销 Provider (`pi.registerProvider`)
- 运行时切换工具集 (`pi.setActiveTools`)
- 注入消息 (`pi.sendMessage` / `pi.sendUserMessage`)
- 自定义 TUI 渲染器
- 社区适配层 `@hsingjui/pi-hooks` 提供 Claude Code 兼容的声明式 Hook

---

### 3.7 Cursor (Anysphere)

**架构**: 声明式 JSON 配置 + command/prompt Handler, Beta (Cursor 1.7+)

**配置位置** (优先级从高到低):
1. Enterprise — MDM 管控 (`/etc/cursor/hooks.json`)
2. Team — 云端通过 Dashboard 分发
3. Project — `<project>/.cursor/hooks.json`
4. User — `~/.cursor/hooks.json`

**配置格式**:
```json
{
  "version": 1,
  "hooks": {
    "beforeShellExecution": [
      {
        "type": "command",
        "command": "./block-git.sh",
        "matcher": "git",
        "timeout": 30,
        "loop_limit": 5,
        "failClosed": false
      }
    ],
    "preToolUse": [
      {
        "type": "prompt",
        "prompt": "Does this command look safe? Only allow read-only operations. $ARGUMENTS",
        "timeout": 15
      }
    ]
  }
}
```

**Handler 类型**:
| 类型 | 用途 | 说明 |
|------|------|------|
| `command` | 执行 shell 脚本 | stdin JSON 输入, stdout JSON 输出 |
| `prompt` | LLM 评估自然语言条件 | 支持 `$ARGUMENTS` 注入上下文, 返回 `{ok, reason}` |

**21 个事件**:

| 类别 | 事件 |
|------|------|
| 会话 | `sessionStart`, `sessionEnd`, `workspaceOpen` |
| 工具通用 | `preToolUse`, `postToolUse`, `postToolUseFailure` |
| Shell | `beforeShellExecution`, `afterShellExecution` |
| MCP | `beforeMCPExecution`, `afterMCPExecution` |
| 文件 | `beforeReadFile`, `afterFileEdit` |
| 子代理 | `subagentStart`, `subagentStop` |
| 提示 | `beforeSubmitPrompt` |
| 压缩 | `preCompact` |
| 停止 | `stop` |
| 响应 | `afterAgentResponse`, `afterAgentThought` |
| Tab | `beforeTabFileRead`, `afterTabFileEdit` |

**输入** (stdin JSON):
```json
{
  "conversation_id": "...",
  "generation_id": "...",
  "model": "...",
  "hook_event_name": "beforeShellExecution",
  "workspace_roots": ["/path/to/project"],
  "command": "git push origin main",
  "cwd": "/path/to/project"
}
```

**环境变量**: `CURSOR_PROJECT_DIR`, `CURSOR_VERSION`, `CURSOR_USER_EMAIL`, `CURSOR_TRANSCRIPT_PATH`, `CURSOR_CODE_REMOTE`

**输出协议**:

| Exit Code | 行为 |
|-----------|------|
| 0 | 成功; stdout JSON 被解析 |
| 2 | 阻断操作 |
| 其他 | Fail-open (除非 `failClosed: true`) |

**stdout JSON 输出**:
```json
{
  "permission": "allow|deny|ask",
  "user_message": "展示给用户的阻断原因",
  "agent_message": "展示给 AI Agent 的反馈",
  "updated_input": { "修改后的工具参数": "..." },
  "additional_context": "注入对话的额外上下文",
  "followup_message": "stop/subagentStop 后自动提交的后续提示"
}
```

**Matcher 语法**:
| Hook 类型 | Matcher 目标 | 示例 |
|-----------|-------------|------|
| 工具 Hook | 工具名/类别 | `"Shell"`, `"Read"`, `"Write"`, `"MCP:tool_name"` |
| Shell Hook | 命令文本正则 | `"curl\|wget"`, `"git"` |
| 子代理 Hook | 子代理类型 | `"explore\|shell"` |

**独有特性**:
- `followup_message`: Agent 停止后自动提交后续提示 (链式循环)
- `loop_limit`: 限制自动重试次数
- `failClosed`: 脚本崩溃时阻断操作 (默认 fail-open)
- `afterAgentThought`: 拦截 Agent 思考过程
- 细粒度文件/Shell/MCP 分离事件 (不仅限于通用 PreToolUse)
- Cloud Agent 支持: 云端后台代理执行 project/team/enterprise Hook (排除 user 级和 prompt 类型)
- `permission: "ask"`: 弹出用户确认对话框

**其他扩展机制**:
- **Rules**: `.cursor/rules/*.mdc` (Markdown + YAML frontmatter, 支持 glob 匹配)
- **MCP**: `.cursor/mcp.json` (stdio / SSE / Streamable HTTP)
- **Custom Modes**: Settings UI 配置自定义模式
- **Skills**: `.cursor/skills/<name>/SKILL.md` 自定义命令

---

## 4. 输入/输出协议对比

### 4.1 输入传递方式

| 代理 | 方式 | 格式 |
|------|------|------|
| Claude Code | stdin | JSON (snake_case) |
| Codex | stdin | JSON (snake_case) |
| Qoder | stdin | JSON (snake_case) |
| Kimi Code | stdin | JSON (snake_case) |
| OpenCode | 函数参数 | TypeScript 对象 |
| Pi | 函数参数 | TypeScript 对象 |
| Cursor | stdin | JSON (snake_case) |

### 4.2 通用输入字段

| 字段 | Claude Code | Codex | Qoder | Kimi Code | Cursor |
|------|:-----------:|:-----:|:-----:|:---------:|:------:|
| `session_id` | ✅ | ✅ | ✅ | ✅ | ✅ (`conversation_id`) |
| `cwd` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `hook_event_name` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `transcript_path` | ✅ | ❌ | ✅ | ❌ | ✅ (`CURSOR_TRANSCRIPT_PATH` env) |
| `permission_mode` | ✅ | ❌ | ✅ | ❌ | ❌ |
| `agent_id` / `agent_type` | ❌ | ❌ | ✅ | ❌ | ❌ |
| `tool_name` | ✅ | ✅ | ✅ | ✅ | ✅ (事件名隐含) |
| `tool_input` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `tool_use_id` / `tool_call_id` | ❌ | ❌ | ✅ | ✅ | ❌ |
| `model` | ❌ | ❌ | ❌ | ❌ | ✅ |
| `workspace_roots` | ❌ | ❌ | ❌ | ❌ | ✅ |

### 4.3 输出/阻断协议

| 代理 | 阻断方式 | 修改输入 | 修改输出 | 注入上下文 |
|------|----------|----------|----------|------------|
| Claude Code | exit 2 / JSON `permissionDecision: "deny"` | ✅ `updatedInput` | ❌ | ✅ `additionalContext` / `systemMessage` |
| Codex | exit 2 / JSON `decision: "block"` | ❌ | ❌ | ✅ (UserPromptSubmit stdout) |
| Qoder | exit 2 / JSON `permissionDecision: "deny"` | ✅ `updatedInput` | ✅ `updatedToolOutput` | ✅ `additionalContext` / `systemMessage` |
| Kimi Code | exit 2 / JSON `permissionDecision: "deny"` | ❌ | ❌ | ✅ (exit 0 stdout) |
| OpenCode | `throw Error` | ✅ mutate `output.args` | ✅ mutate `output` | ✅ `output.system.push()` |
| Pi | `return { block: true }` | ✅ mutate `event.input` | ✅ `return { content }` | ✅ `return { systemPrompt }` |
| Cursor | exit 2 / JSON `permission: "deny"` | ✅ `updated_input` | ❌ | ✅ `additional_context` / `agent_message` |

---

## 5. 环境变量对比

| 变量 | Claude Code | Qoder | Cursor | 用途 |
|------|:-----------:|:-----:|:------:|------|
| `*_PROJECT_DIR` | `CLAUDE_PROJECT_DIR` | `QODER_PROJECT_DIR` | `CURSOR_PROJECT_DIR` | 项目根目录 |
| `*_PLUGIN_ROOT` | `CLAUDE_PLUGIN_ROOT` | `QODER_PLUGIN_ROOT` | ❌ | 插件根目录 |
| `*_PLUGIN_DATA` | ❌ | `QODER_PLUGIN_DATA` | ❌ | 插件数据目录 |
| `*_ENV_FILE` | `CLAUDE_ENV_FILE` | ❌ | ❌ | 环境变量持久化文件 |
| `*_REMOTE` | `CLAUDE_CODE_REMOTE` | ❌ | `CURSOR_CODE_REMOTE` | 远程模式标识 |
| `*_TOOL_INPUT_FILE_PATH` | `CLAUDE_TOOL_INPUT_FILE_PATH` | ❌ | ❌ | 当前工具操作的文件路径 |
| `*_VERSION` | ❌ | ❌ | `CURSOR_VERSION` | 客户端版本 |
| `*_USER_EMAIL` | ❌ | ❌ | `CURSOR_USER_EMAIL` | 当前用户邮箱 |
| `*_TRANSCRIPT_PATH` | ❌ | ❌ | `CURSOR_TRANSCRIPT_PATH` | 对话记录路径 |

---

## 6. Solo 平台 Hook 设计建议

基于以上分析，Solo 若要支持跨代理 Hook，建议:

### 6.1 核心事件集 (最小公约数)

所有 7 个代理都支持或等价支持的事件:

| 事件 | 说明 | 优先级 |
|------|------|--------|
| `SessionStart` | 会话开始 | P0 |
| `PreToolUse` | 工具执行前 (可阻断) | P0 |
| `PostToolUse` | 工具执行后 | P0 |
| `Stop` | Agent 完成 (可阻断) | P0 |
| `UserPromptSubmit` | 用户提交提示 (可阻断) | P1 |
| `SessionEnd` | 会话结束 | P1 |
| `PreCompact` / `PostCompact` | 上下文压缩前后 | P2 |
| `SubagentStart` / `SubagentStop` | 子代理生命周期 | P2 |
| `Notification` | 通知 | P2 |

### 6.2 配置格式建议

采用 JSON 声明式 (兼容 Claude Code / Codex / Qoder / Kimi Code / Cursor 风格):

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "if": "Bash(git *)",
        "hooks": [
          {
            "type": "command",
            "command": "~/.solo/hooks/guard.sh",
            "timeout": 10
          }
        ]
      }
    ]
  }
}
```

### 6.3 输入/输出协议建议

- 输入: stdin JSON (snake_case), 包含 `session_id`, `cwd`, `hook_event_name`, `tool_name`, `tool_input`
- 输出: exit code (0=允许, 2=阻断) + stdout JSON
- 支持 `permissionDecision`, `updatedInput`, `additionalContext`

### 6.4 与现有 Provider 的映射

| Solo Provider | 对应代理 | Hook 集成方式 |
|---------------|----------|---------------|
| Claude | Claude Code | 原生支持, 直接透传 |
| Kimi | Kimi Code | 原生支持 (TOML → JSON 适配) |
| OpenCode | OpenCode | Plugin API 适配 |
| Pi | Pi | Extension API 适配 |
| Codex | Codex | 原生支持 (需 feature flag) |
| Cursor-Agent (计划中) | Cursor | 原生支持 (JSON hooks.json 透传) |

---

## 7. 参考来源

- [Claude Code Hooks 官方文档](https://code.claude.com/docs/en/hooks)
- [Codex CLI Hooks (GitHub)](https://github.com/openai/codex/blob/main/docs/config.md)
- [Qoder CLI Hooks 文档](https://docs.qoder.com/en/cli/hooks)
- [Kimi Code Hooks 文档](https://www.kimi.com/code/docs/en/kimi-code-cli/customization/hooks.html)
- [OpenCode Plugins 文档](https://opencode.ai/docs/plugins/)
- [Pi Extensions 文档](https://pi.dev/docs/latest/extensions)
- [Cursor Hooks 文档](https://cursor.com/docs/hooks)
- [Cursor Rules 文档](https://cursor.com/docs/rules)
