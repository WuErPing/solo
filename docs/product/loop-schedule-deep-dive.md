# Solo Loop Schedule 深入分析

> **文档类型**：技术深度分析
> **日期**：2026-06-13
> **基线版本**：Solo v0.6.0
> **关联文档**：[Loop Schedule Design](loop-schedule-design.md)
> **目标读者**：后端/架构/核心开发者

---

## 1. 设计目标与边界

### 1.1 要解决的核心问题

Loop Schedule 不是简单地把多个 schedule 串起来，而是让系统能够：

1. **根据执行反馈动态调整下一步**；
2. **在无人值守时安全地执行多轮操作**；
3. **在关键决策点让人类保持控制**；
4. **失败后能恢复而不是从头开始**。

### 1.2 明确不做的事

- 不做通用 DAG 工作流引擎（那是另一个更大的问题域）。
- 不做跨机器分布式执行（Solo 仍是本地优先）。
- 不替代人类对代码 final review 和 merge 的责任。

### 1.3 成功标准

- "fix all tests" 类任务能在 5–10 轮内自主完成率达到 70%+。
- loop 崩溃后能从最后一次 step 恢复。
- 危险操作 100% 经过配置的策略审批。

---

## 2. Loop Controller 深度设计

### 2.1 Controller 不是 Agent

Loop Controller 与 Solo Agent 的关键区别：

| | Solo Agent | Loop Controller |
|---|---|---|
| 目标 | 完成一次用户 prompt | 管理多轮循环的决策 |
| 输出 | 流式文本/代码/思考 | 结构化决策（tool call） |
| 生命周期 | 一次会话 | 跨越多个 step |
| 工具 | provider 原生工具 | Loop 层定义的工具 |
| 状态 | session timeline | loop state store |

Controller 调用 LLM 时，应该**禁止自由生成文本**，必须输出结构化 JSON/tool call。这可以通过 system prompt + 强制输出格式实现。

### 2.2 Controller 输入上下文

上下文必须足够让 LLM 决策，但不能过长导致 token 爆炸。建议控制在 8K–16K tokens 内。

```go
type ControllerContext struct {
    Meta LoopMeta
    Goal string
    CurrentIteration int
    MaxIterations int
    LastSteps []LoopStepSummary // 最近 N 步，默认 10
    Workspace WorkspaceSnapshot
    Tools []ToolDefinition
    Policy ApprovalPolicy
}

type LoopStepSummary struct {
    Index int
    Type string
    Status string
    Input map[string]interface{}
    Output *string
    Error *string
    DurationMs int64
}

type WorkspaceSnapshot struct {
    Cwd string
    GitBranch string
    GitDirty bool
    RecentFiles []string
    TestSummary *TestSummary
    LastAgentOutput *string
}
```

### 2.3 Prompt 模板

```markdown
You are Loop Controller for Solo, an autonomous software development assistant.

## Goal
{{.Goal}}

## Constraints
- Max iterations: {{.MaxIterations}}
- Current iteration: {{.CurrentIteration}}
- Approval policy: {{.Policy}}
- Available tools: {{.Tools}}

## Workspace
{{.Workspace}}

## Recent Steps
{{.LastSteps}}

## Instructions
Decide the next action. You must respond with a JSON object matching one of these schemas:

1. next_step
2. human_confirm
3. completed
4. failed

Do not output explanations outside the JSON.
```

### 2.4 决策输出 Schema

```json
{
  "$schema": "https://solo.sh/schemas/loop-decision-v1.json",
  "oneOf": [
    {
      "type": "object",
      "properties": {
        "decision": { "const": "next_step" },
        "step": { "$ref": "#/definitions/Step" },
        "reasoning": { "type": "string" }
      },
      "required": ["decision", "step"]
    },
    {
      "type": "object",
      "properties": {
        "decision": { "const": "human_confirm" },
        "request": {
          "type": "object",
          "properties": {
            "message": { "type": "string" },
            "options": { "type": "array", "items": { "type": "string" } }
          }
        }
      }
    },
    {
      "type": "object",
      "properties": {
        "decision": { "const": "completed" },
        "summary": { "type": "string" }
      }
    },
    {
      "type": "object",
      "properties": {
        "decision": { "const": "failed" },
        "reason": { "type": "string" }
      }
    }
  ]
}
```

### 2.5 Provider 适配细节

不同 provider 输出结构化数据的方式不同：

| Provider | 方式 | 说明 |
|---|---|---|
| Claude | `tool_use` / `tool_result` | 最稳定，强制 schema 遵循能力强 |
| Kimi (Wire) | function calling | 需测试 schema 遵循 |
| OpenCode | SSE stream tool call | 需从 stream 中提取 |
| OpenAI/Codex | function calling | 标准 |

Controller 应该抽象一个统一接口：

```go
type DecisionProvider interface {
    Decide(ctx context.Context, prompt string, schema DecisionSchema) (*LoopDecision, error)
}
```

内部再适配到各 provider 的原生调用方式。

### 2.6 决策失败回退

LLM 可能输出无效 JSON 或选择不存在的工具。处理策略：

1. **JSON 解析失败**：重试 1 次，并在 prompt 中强调格式；仍失败则 `failed`。
2. **工具不存在**：回退到 `human_confirm`，把问题抛给用户。
3. **连续 3 次重复相同 step**：自动暂停，避免死循环。
4. **幻觉 completed**：执行验证 step（如跑测试）确认后再真正结束。

---

## 3. Step Executor 深度设计

### 3.1 Step 接口

```go
package loop

// StepType identifies the kind of step.
type StepType string

const (
    StepTypeAgent   StepType = "agent"
    StepTypeBash    StepType = "bash"
    StepTypeTest    StepType = "test"
    StepTypeRead    StepType = "read"
    StepTypeWrite   StepType = "write"
    StepTypeGit     StepType = "git"
    StepTypeAskUser StepType = "ask-user"
    StepTypeWait    StepType = "wait"
    StepTypeTerminate StepType = "terminate"
)

// StepDefinition is what the Controller outputs.
type StepDefinition struct {
    Type StepType
    Input map[string]interface{}
}

// StepResult is what the Executor returns.
type StepResult struct {
    Status  string // succeeded | failed | skipped | timeout
    Output  *string
    Error   *string
    Metrics StepMetrics
}

type StepMetrics struct {
    DurationMs int64
    TokensIn   int64
    TokensOut  int64
}

// StepExecutor runs one step.
type StepExecutor interface {
    Execute(ctx context.Context, step StepDefinition, env *StepEnv) (StepResult, error)
    CanExecute(stepType StepType) bool
}
```

### 3.2 Agent Step 详解

Agent step 是 Loop 最重要的 step，它把任务交给 Solo Agent 执行。

```go
type AgentStepInput struct {
    Prompt       string
    Provider     string  // 可选，覆盖 loop 默认
    Model        string  // 可选
    ModeID       string  // 可选
    Cwd          string  // 可选，默认 loop cwd
    AutoApprove  bool    // 是否自动批准该 agent 的工具调用
    TimeoutMin   int     // 默认 10
}
```

执行流程：

1. 调用 `agent.Manager.CreateAgent()` 创建临时 agent（或复用 loop 专用 agent）。
2. 发送 prompt。
3. 等待 agent 完成或超时。
4. 收集最终输出（不是流式，是最终结果）。
5. 如果 agent 需要人类确认，根据 loop 的 `approvalPolicy` 决定是自动批准、暂停 loop 还是拒绝。

**关键问题**：Agent 内部可能产生多轮 tool call。Loop 不应把每一轮都暴露为 Loop step，而是把一次 Agent step 视为一个黑盒单元。

### 3.3 Bash Step 详解

```go
type BashStepInput struct {
    Command string
    Cwd     string
    TimeoutSec int
    Env     map[string]string
}
```

安全策略：
- 默认只允许读取/测试类命令。
- 写入/删除/git push 等命令需要 `approvalPolicy` 允许。
- 命令在 `cwd` 下执行，不能跨越项目目录。
- 支持超时，默认 60s，最大 10min。

### 3.4 Test Step 详解

Test step 是 Bash step 的特化，但增加了结果解析：

```go
type TestStepInput struct {
    Command string // e.g. "go test ./..."
    Parser  string // "go", "jest", "pytest", "generic"
    Cwd     string
}

type TestResult struct {
    Passed  bool
    Summary string
    Failures []TestFailure
}
```

解析器：
- `go`：解析 `go test` 输出，提取失败包和测试名。
- `jest`：解析 Jest JSON 输出。
- `pytest`：解析 pytest 输出。
- `generic`：只判断 exit code。

### 3.5 Read / Write Step 详解

```go
type ReadStepInput struct {
    Path string
}

type WriteStepInput struct {
    Path    string
    Content string
    Mode    string // overwrite | append
}
```

安全策略：
- Path 必须位于 loop 的 cwd 下（解析 symlink 后检查）。
- Write step 默认需要审批，除非 `approvalPolicy: auto`。

### 3.6 Git Step 详解

```go
type GitStepInput struct {
    Subcommand string // "status", "diff", "commit", "push"
    Args       []string
}
```

- `status`/`diff`：只读，无需审批。
- `commit`：需要审批。
- `push`：需要审批，且不能 push 到 main/master。

### 3.7 Ask-User Step

```go
type AskUserStepInput struct {
    Question string
    Options  []string // 空表示自由输入
}
```

执行后 loop 进入 `paused` 状态，等待用户响应。用户响应后作为下一步的上下文。

---

## 4. 状态机详细设计

### 4.1 状态转换图

```
                    ┌─────────┐
         ┌─────────│ pending │─────────┐
         │ start() └────┬────┘         │ edit/save
         │              │               │
         ▼              ▼               ▼
    ┌─────────┐    ┌─────────┐    ┌─────────┐
    │ running │───▶│evaluating│    │ pending │
    └────┬────┘    └────┬────┘    └─────────┘
         │              │
         │ step done    │ LLM decision
         │              │
         │    ┌─────────┼─────────┐
         │    ▼         ▼         ▼
         │ ┌───────┐ ┌───────┐ ┌─────────┐
         └─│ next  │ │ done  │ │ human   │
           │ step  │ │       │ │ confirm │
           └───┬───┘ └───┬───┘ └────┬────┘
               │         │          │
               │         ▼          ▼
               │    ┌─────────┐  ┌─────────┐
               └────│ running │  │ paused  │
                    └─────────┘  └────┬────┘
                                       │ user respond
                                       ▼
                                  ┌─────────┐
                                  │ running │
                                  └─────────┘
```

### 4.2 状态转换规则

| From | To | Trigger | 说明 |
|---|---|---|---|
| pending | running | `Start()` | 启动 loop |
| pending | pending | `Update()` | 编辑保存 |
| running | evaluating | step 完成 | 等待 LLM 决策 |
| evaluating | running | decision=next_step | 执行下一步 |
| evaluating | completed | decision=completed | 目标达成 |
| evaluating | paused | decision=human_confirm | 等待用户 |
| evaluating | failed | decision=failed 或异常 | 失败 |
| running | paused | `Pause()` 或 agent 需要审批 | 用户暂停 |
| paused | running | `Resume()` 或用户响应 | 继续 |
| any | failed | 达到 maxIterations / 连续错误 | 失败 |

### 4.3 幂等性

每个 step 都有唯一 ID。执行 step 前先检查该 step 是否已执行过，避免重复执行：

```go
func (e *Engine) runStepWithIdempotency(loopID, stepID string, step StepDefinition) {
    if existing := e.store.GetStep(loopID, stepID); existing != nil {
        return existing.Result
    }
    result := e.executor.Execute(...)
    e.store.SaveStep(loopID, stepID, result)
    return result
}
```

---

## 5. 持久化与恢复

### 5.1 持久化内容

每个 loop 保存为一个 JSON 文件：`~/.solo/loops/{id}.json`

```json
{
  "id": "loop-001",
  "type": "loop",
  "status": "running",
  "currentIteration": 3,
  "goal": "修复所有测试失败",
  "controller": { "provider": "claude", "model": "opus-4-7" },
  "steps": [
    {
      "id": "step-1",
      "type": "test",
      "status": "succeeded",
      "input": { "command": "go test ./..." },
      "output": "FAIL: 3 tests failed...",
      "startedAt": "2026-06-13T10:00:00Z",
      "endedAt": "2026-06-13T10:00:05Z"
    },
    {
      "id": "step-2",
      "type": "agent",
      "status": "succeeded",
      "input": { "prompt": "修复 auth_test.go 中的 JWT 验证" },
      "output": "已修复...",
      // ...
    }
  ],
  "pendingHumanConfirm": null,
  "createdAt": "2026-06-13T09:55:00Z",
  "updatedAt": "2026-06-13T10:05:00Z"
}
```

### 5.2 持久化时机

- loop 状态变更时立即保存。
- 每个 step 开始前保存（防止执行中崩溃后重复执行）。
- step 完成后保存。
- 使用原子写入（temp + rename）。

### 5.3 崩溃恢复

daemon 启动时扫描 `~/.solo/loops/`：

```go
func (e *Engine) recoverLoops() {
    for _, loop := range e.store.ListUnfinished() {
        switch loop.Status {
        case "running":
            // 最后一步可能已执行或未完成
            last := loop.LastStep()
            if last.Status == "running" {
                // 重试最后一步
                e.retryLastStep(loop)
            } else {
                // 继续下一步决策
                e.continueLoop(loop)
            }
        case "paused":
            // 保持暂停，等待用户响应
            e.notifyUser(loop, "loop recovered, waiting for your response")
        }
    }
}
```

### 5.4 与 Schedule Store 的集成

推荐把 loop 数据合并到现有 schedule store：`~/.solo/schedules/{id}.json`，通过 `type: "loop"` 区分。这样：
- 无需新增目录；
- App 的 Schedule Dashboard 可以自然展示 loops；
- CLI `solo schedule list` 也能列出 loops（带类型标记）。

---

## 6. 与 Agent Manager 的集成

### 6.1 调用路径

```
Loop Engine
   │ CreateAgent(ScheduleAgentConfig)
   ▼
agent.Manager
   │
   ▼
ProviderClient (Claude/Kimi/OpenCode/Pi)
```

### 6.2 Agent 生命周期

- 每个 Agent step 创建一个新 Agent。
- Agent 完成后自动清理。
- 如果 Agent 进入需要人类确认的状态：
  - `approvalPolicy=auto`：Loop 自动批准（危险，需明确提示）。
  - `approvalPolicy=dangerous-only`：只自动批准读/测试类 tool，写/执行/推送类 tool 暂停 loop。
  - `approvalPolicy=every-step`：每个 tool 都暂停 loop。

### 6.3 输出收集

Agent step 不需要流式显示，只需要最终结果。可以通过：
- 监听 Agent 生命周期事件，等待 `idle` 或 `closed` 状态；
- 收集最后 N 条 assistant message 作为输出。

---

## 7. 工具调用协议

### 7.1 工具定义 Schema

Controller 需要知道有哪些工具可用，以及每个工具的参数 schema。

```go
var LoopTools = []ToolDefinition{
    {
        Name: "agent",
        Description: "Delegate a task to an AI coding agent",
        Parameters: ToolParameters{
            "prompt": ToolParam{Type: "string", Required: true},
            "provider": ToolParam{Type: "string", Required: false},
            "model": ToolParam{Type: "string", Required: false},
            "timeout_min": ToolParam{Type: "integer", Required: false, Default: 10},
        },
    },
    {
        Name: "bash",
        Description: "Run a shell command in the workspace",
        Parameters: ToolParameters{
            "command": ToolParam{Type: "string", Required: true},
            "timeout_sec": ToolParam{Type: "integer", Required: false, Default: 60},
        },
    },
    // ...
}
```

### 7.2 工具权限矩阵

| 工具 | auto | dangerous-only | every-step |
|---|---|---|---|
| agent (read/test) | 自动 | 自动 | 需确认 |
| agent (write/git) | 自动 | 需确认 | 需确认 |
| bash (ls/cat) | 自动 | 自动 | 需确认 |
| bash (rm/git push) | 需确认 | 需确认 | 需确认 |
| read | 自动 | 自动 | 需确认 |
| write | 自动 | 需确认 | 需确认 |
| test | 自动 | 自动 | 需确认 |
| git status/diff | 自动 | 自动 | 需确认 |
| git commit/push | 需确认 | 需确认 | 需确认 |

---

## 8. 安全沙箱

### 8.1 目录隔离

所有文件操作必须限制在 loop 的 `cwd` 内：

```go
func sanitizePath(cwd, requested string) (string, error) {
    abs, err := filepath.Abs(filepath.Join(cwd, requested))
    if err != nil {
        return "", err
    }
    if !strings.HasPrefix(abs, cwd) {
        return "", fmt.Errorf("path %q escapes workspace", requested)
    }
    return abs, nil
}
```

### 8.2 命令白名单

可配置命令白名单：

```json
{
  "allowedCommands": ["go", "npm", "yarn", "pnpm", "git", "ls", "cat", "grep"],
  "blockedCommands": ["rm -rf /", "sudo", "curl | sh"]
}
```

### 8.3 Git 保护

- 禁止直接 push 到 main/master（除非显式配置）。
- commit 前自动检查 diff，确保没有 secrets。
- 大删除操作需要额外确认。

---

## 9. 成本控制

### 9.1 Token 预算

```go
type LoopBudget struct {
    MaxControllerTokens int64   // Controller LLM 调用总 token 上限
    MaxAgentTokens      int64   // Agent step 总 token 上限
    MaxTotalCost        float64 // 预估总成本上限（USD）
}
```

每次调用后累加，超过则暂停 loop。

### 9.2 迭代次数

`maxIterations` 默认 20，可在创建 loop 时配置。

### 9.3  early stop

如果连续 3 次 step 没有产生有效进展（例如测试失败数未减少、输出相同），自动暂停。

---

## 10. 可观测性

### 10.1 日志

- 每个 loop 的完整 step 日志写入 `~/.solo/loops/logs/{id}.jsonl`。
- Controller 的每次决策记录 prompt、输出、reasoning。

### 10.2 Metrics

在现有 Prometheus metrics 基础上新增：

```
solo_loop_total
solo_loop_status{status="running|completed|failed|paused"}
solo_loop_iterations_total
solo_loop_step_duration_seconds{step_type="agent|bash|test|..."}
solo_loop_controller_tokens_total
solo_loop_agent_tokens_total
```

### 10.3 Tracing

每个 loop 作为一个 trace，每个 step 作为一个 span，Controller 决策也是一个 span。

---

## 11. 测试策略

### 11.1 单元测试

- Controller prompt 渲染。
- 决策 JSON 解析（含失败回退）。
- Step executor（尤其是 bash/test parser）。
- 状态机转换。
- 路径隔离沙箱。

### 11.2 集成测试

- 使用 Mock Provider 模拟 LLM 决策，验证完整 loop 流程。
- 测试崩溃恢复：kill daemon 后重启，loop 能从最后 step 恢复。
- 测试人工确认：LLM 输出 human_confirm 后，loop 正确暂停并推送。

### 11.3 E2E 测试

- 创建一个真实 loop（例如 "create a hello.go file"），验证自主完成。
- 危险操作触发确认弹窗。

---

## 12. MVP 建议

为了快速验证价值，MVP 可以只包含：

1. Loop type 支持（扩展 Schedule）。
2. 3 种 step：`agent`、`bash`、`test`。
3. 3 种决策：`next_step`、`completed`、`failed`。
4. 一个 Controller provider（Claude 或 Kimi）。
5. CLI：`solo loop create/start/status/logs`。
6. 无 App UI，仅 CLI。
7. 简单持久化和崩溃恢复。

预计 2 周可完成 MVP。

---

## 13. 与 Provider Hub 的协同

如果未来实现 [Provider Hub](agent-profile-switch-export-design.md)，Loop 可以：

- 由 Provider Hub 为 Controller 选择最优 provider/model；
- Loop 中不同 step 使用不同 provider（例如 plan 用 Claude，coding 用 Kimi）；
- 用量和成本统一汇总到 Provider Hub Usage Tracker。

---

## 参考文档

- [Loop Schedule Design](loop-schedule-design.md)
- [Feature Directions 2026](feature-directions-2026.md)
- [Schedule Module Analysis](../analysis/app-bridge-schedule-module.md)
- [Create Schedule Flow](../analysis/create-schedule-flow.md)
