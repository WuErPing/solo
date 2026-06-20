# Solo Loop Schedule 实现规范（v1.0）

> **文档类型**：产品/架构实现规范
> **日期**：2026-06-20
> **基线版本**：Solo v0.6.3
> **目标读者**：后端、前端、CLI 开发者
> **关联文档**：
> - [Loop Schedule Design](loop-schedule-design.md)
> - [Loop Schedule Deep Dive](loop-schedule-deep-dive.md)
> - [Solo 2026 产品/技术路线图](roadmap-2026.md)
> - [Solo Roadmap Architecture Mapping](../analysis/solo-roadmap-architecture-mapping.md)
> - [Schedule Module Analysis](../analysis/app-bridge-schedule-module.md)
> - [Create Schedule Flow](../analysis/create-schedule-flow.md)

---

## 执行摘要

本规范定义 **Loop Schedule** 从 Solo v0.6.3 到路线图目标的实现路径。核心决策：

1. **把 Loop 作为 `StoredSchedule` 的高级类型**（`type: "loop"`），复用 `schedule.Store` 的持久化、恢复、暂停/恢复机制。
2. **引入 LLM 驱动的 `LoopController`**，输出结构化决策（`next_step` / `human_confirm` / `completed` / `failed`）。
3. **抽象 `StepExecutor` 注册表**，支持 `agent` / `bash` / `test` / `read` / `write` / `git` / `ask-user` 等 step 类型。
4. **渐进式迁移现有独立 `loop` 模块**：先复用其 RPC/UI，再逐步把持久化目标切到 `schedule.Store`，最终移除独立 `loop.Store/Engine`。

---

## 1. 当前状态分析

### 1.1 现有 Schedule 系统

- `daemon/internal/schedule/store.go`：`StoredSchedule` 的内存 + JSON 持久化，路径 `~/.solo/schedules.json`。
- `daemon/internal/schedule/executor.go`：30s tick，调用 `Runner.Run(schedule)`。
- `daemon/internal/server/schedule_runner.go`：`daemonRunner` 实现 `Runner`，支持 `Target.Type = "agent" | "new-agent"`。
- `protocol/message_schedule.go`：`StoredSchedule`、`ScheduleCadence`、`ScheduleTarget`、`ScheduleAgentConfig`。

### 1.2 现有独立 Loop 系统

- `daemon/internal/loop/store.go`：独立内存 + JSON 持久化，路径 `~/.solo/loops.json`。
- `daemon/internal/loop/engine.go`：worker/verifier 模式循环：
  - 每轮创建一个 worker agent，发送固定 prompt；
  - 运行 verifier agent（可选）和 verify checks（shell 命令）；
  - 如果通过则成功，否则进入下一轮直到 `MaxIterations`。
- `protocol/message_loop.go`：`LoopRecord`、`LoopIterationRecord`、`LoopLogEntry`。
- `app-bridge/src/server/loop/rpc-schemas.ts`：loop CRUD RPC。
- `app/src/hooks/use-loops.ts`、`use-loop-inspect.ts`、`use-loop-mutations.ts`：React Query hooks。
- `app/src/screens/loops-screen.tsx`、`loop-detail-screen.tsx`、`loop-create-screen.tsx`：UI 已存在。

### 1.3 现状与目标的差距

| 目标能力 | 现状 | 差距 |
|---------|------|------|
| LLM 动态决策下一步 | worker/verifier 固定流程 | 缺少 `LoopController` |
| 多种 step 类型 | 只有 agent + verify check | 缺少 bash/test/read/write/git/ask-user 抽象 |
| 与 Schedule 统一 | loop 独立存储/执行 | 需要合并到 `schedule.Store` |
| 人工确认门控 | 无 | 需要 `HumanConfirmGate` |
| 状态机 | 简单 running/succeeded/failed | 需要 pending/running/evaluating/paused/completed/failed |
| 崩溃恢复 | daemon 启动不自动恢复 loop | 需要恢复机制 |

---

## 2. 目标架构

### 2.1 高层架构

```
┌─────────────────────────────────────────────────────────────┐
│                         Solo App / CLI                       │
│  Schedule Dashboard · Loop Detail · Step Timeline · Confirm  │
└─────────────────────────────┬───────────────────────────────┘
                              │ WebSocket / CLI
┌─────────────────────────────▼───────────────────────────────┐
│                         Solo Daemon                          │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐   │
│  │              Loop Controller (LLM)                   │   │
│  │  · Decide next step / human_confirm / completed      │   │
│  │  · Structured JSON / tool_use output                 │   │
│  └────────────────────────┬─────────────────────────────┘   │
│                           │ calls provider via AgentManager  │
│  ┌────────────────────────▼─────────────────────────────┐   │
│  │              Loop Runner                             │   │
│  │  implements schedule.Runner                          │   │
│  │  · load loop state from schedule.Store               │   │
│  │  · run Loop Engine state machine                     │   │
│  └────────────────────────┬─────────────────────────────┘   │
│                           │                                  │
│  ┌────────────────────────▼─────────────────────────────┐   │
│  │              Loop Engine                             │   │
│  │  · state machine                                     │   │
│  │  · context builder                                   │   │
│  │  · human confirm gate                                │   │
│  │  · budget / max iterations guard                     │   │
│  └────────────────────────┬─────────────────────────────┘   │
│                           │ executes steps                   │
│  ┌────────────────────────▼─────────────────────────────┐   │
│  │              Step Executor Registry                  │   │
│  │  · agent · bash · test · read · write · git · ask    │   │
│  └────────────────────────┬─────────────────────────────┘   │
│                           │                                  │
│  ┌────────────────────────▼─────────────────────────────┐   │
│  │  Existing Infrastructure                             │   │
│  │  · AgentManager  · Workspace  · Terminal             │   │
│  │  · Schedule Store · Memory · Push · Tmux             │   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 关键设计原则

1. **Schedule-first**：Loop 是 `StoredSchedule` 的一种类型，不是并列系统。
2. **协议优先**：所有新类型先进入 `protocol/`，再镜像到 `app-bridge/src/shared/messages.ts`。
3. **Step 可扩展**：新增 step 类型只需实现 `StepExecutor` 并注册。
4. **幂等执行**：每个 step 有唯一 ID，崩溃后可安全重试或跳过。
5. **渐进迁移**：保留现有 loop RPC/UI 直到新系统稳定，再统一替换。

---

## 3. 协议变更

### 3.1 扩展 `StoredSchedule`

在 `protocol/message_schedule.go` 中扩展：

```go
type StoredSchedule struct {
    // 现有字段保持不变...
    ID        string
    Name      *string
    Prompt    string
    Cadence   ScheduleCadence
    Target    ScheduleTarget
    Status    string            // "active" | "paused" | "completed" | "failed"
    CreatedAt string
    UpdatedAt string
    NextRunAt *string
    LastRunAt *string
    PausedAt  *string
    ExpiresAt *string
    MaxRuns   *int
    Runs      []ScheduleRun

    // 新增 Loop 字段
    Type            string                `json:"type,omitempty"` // "schedule" | "loop"
    Goal            string                `json:"goal,omitempty"`
    Controller      *LoopControllerConfig `json:"controller,omitempty"`
    Tools           []string              `json:"tools,omitempty"`
    CurrentIteration int                  `json:"currentIteration,omitempty"`
    Steps           []LoopStep            `json:"steps,omitempty"`
    PendingHumanConfirm *HumanConfirmRequest `json:"pendingHumanConfirm,omitempty"`
    Budget          *LoopBudget           `json:"budget,omitempty"`
}
```

### 3.2 新增 Loop 类型

```go
// protocol/message_loop.go（扩展现有文件）

type LoopControllerConfig struct {
    Provider         string  `json:"provider"`
    Model            *string `json:"model,omitempty"`
    ModeID           *string `json:"modeId,omitempty"`
    SystemPrompt     string  `json:"systemPrompt,omitempty"`
    MaxIterations    int     `json:"maxIterations"`
    PauseBetweenIterationsMs int `json:"pauseBetweenIterationsMs"`
}

type LoopStep struct {
    ID        string                 `json:"id"`
    Type      string                 `json:"type"` // agent | bash | test | read | write | git | ask-user | wait | terminate
    Input     map[string]interface{} `json:"input"`
    Output    *string                `json:"output,omitempty"`
    Status    string                 `json:"status"` // pending | running | succeeded | failed | skipped
    StartedAt *string                `json:"startedAt,omitempty"`
    EndedAt   *string                `json:"endedAt,omitempty"`
    Error     *string                `json:"error,omitempty"`
    Reasoning *string                `json:"reasoning,omitempty"` // Controller 决策理由
}

type HumanConfirmRequest struct {
    StepID  string   `json:"stepId"`
    Message string   `json:"message"`
    Options []string `json:"options"` // ["approve", "skip", "abort"]
    TimeoutSec int   `json:"timeoutSec"`
}

type LoopBudget struct {
    MaxControllerTokens int64   `json:"maxControllerTokens"`
    MaxAgentTokens      int64   `json:"maxAgentTokens"`
    MaxTotalCost        float64 `json:"maxTotalCost"`
}
```

### 3.3 新增 Schedule 子类型请求/响应

保留现有 `schedule/create`、`schedule/list`、`schedule/update` 等 RPC，但在创建时支持 `type: "loop"`：

```go
// ScheduleCreateRequest 已存在，其 Prompt/Cadence/Target 对 loop 依然可用：
// - Prompt 可复用为 Goal（过渡期兼容）
// - Cadence 对 loop 可选；loop 默认立即启动
// - Target 对 loop 提供 cwd / provider / model 默认值
```

新增显式 loop 请求（与现有 `loop/run` 兼容）：

```go
type ScheduleLoopCreateRequest struct {
    Type            string
    RequestID       string
    Name            *string
    Goal            string
    Cadence         *ScheduleCadence // 可选：定时触发 loop
    Target          ScheduleTarget   // cwd / provider / model
    Controller      *LoopControllerConfig
    Tools           []string
    Budget          *LoopBudget
    ApprovalPolicy  string // auto | dangerous-only | every-step
}
```

### 3.4 App-Bridge 镜像

在 `app-bridge/src/shared/messages.ts` 和 `app-bridge/src/server/schedule/rpc-schemas.ts` 中同步新增：

- `StoredScheduleSchema` 增加 `type`、`goal`、`controller`、`tools`、`currentIteration`、`steps`、`pendingHumanConfirm`、`budget` 字段。
- 新增 `LoopControllerConfigSchema`、`LoopStepSchema`、`HumanConfirmRequestSchema`、`LoopBudgetSchema`。

---

## 4. Daemon 模块设计

### 4.1 模块位置

```
daemon/internal/loop/
├── types.go              # LoopStatus, StepType, LoopBudget 等常量
├── runner.go             # 实现 schedule.Runner
├── engine.go             # 状态机、循环执行
├── controller.go         # LLM 决策控制器
├── controller_provider.go # provider 适配层
├── context.go            # LoopContext 构建
├── step.go               # StepExecutor 接口与注册表
├── steps/
│   ├── agent_step.go     # agent step
│   ├── bash_step.go      # bash step
│   ├── test_step.go      # test step
│   ├── read_step.go      # read step
│   ├── write_step.go     # write step
│   ├── git_step.go       # git step
│   └── ask_user_step.go  # ask-user step
├── human_confirm.go      # 人工确认门控
├── policy.go             # approval policy 与危险操作判定
├── recovery.go           # 崩溃恢复
├── metrics.go            # Prometheus metrics
└── loop_test.go          # 单元/集成测试
```

### 4.2 `LoopRunner`：实现 `schedule.Runner`

```go
package loop

import (
    "context"
    "github.com/WuErPing/solo/daemon/internal/schedule"
    "github.com/WuErPing/solo/protocol"
)

// LoopRunner executes schedules with Type == "loop".
type LoopRunner struct {
    engine *Engine
}

func NewLoopRunner(engine *Engine) schedule.Runner {
    return &LoopRunner{engine: engine}
}

func (r *LoopRunner) Run(sched protocol.StoredSchedule) schedule.RunResult {
    ctx := context.Background()
    err := r.engine.Start(ctx, sched.ID)
    if err != nil {
        return schedule.RunResult{Error: strPtr(err.Error())}
    }
    return schedule.RunResult{}
}

func strPtr(s string) *string { return &s }
```

注册方式：在 `daemon/internal/server/schedule_runner.go` 或 `daemon.go` 中：

```go
loopEngine := loop.NewEngine(scheduleStore, agentManager, ...)
loopRunner := loop.NewLoopRunner(loopEngine)

executor := schedule.NewExecutor(scheduleStore, compositeRunner, ...)

// compositeRunner 根据 schedule.Type 分发：
//   "schedule" -> daemonRunner
//   "loop"     -> loopRunner
```

### 4.3 `LoopEngine`：状态机

```go
type Engine struct {
    store          *schedule.Store
    agentManager   *agent.AgentManager
    controller     *Controller
    stepRegistry   *StepRegistry
    confirmGate    *HumanConfirmGate
    policy         *Policy
    logger         *slog.Logger
}

func (e *Engine) Start(ctx context.Context, scheduleID string) error {
    sched, ok := e.store.Get(scheduleID)
    if !ok {
        return fmt.Errorf("loop schedule not found")
    }
    if sched.Type != "loop" {
        return fmt.Errorf("not a loop schedule")
    }

    go e.run(ctx, scheduleID)
    return nil
}

func (e *Engine) run(ctx context.Context, scheduleID string) {
    defer func() {
        if r := recover(); r != nil {
            e.logger.Error("loop panic recovered", "scheduleId", scheduleID, "panic", r)
            _ = e.transition(scheduleID, LoopStatusFailed, strPtr(fmt.Sprintf("panic: %v", r)))
        }
    }()

    for {
        sched, ok := e.store.Get(scheduleID)
        if !ok {
            return
        }

        switch sched.Status {
        case string(LoopStatusPending):
            e.transition(scheduleID, LoopStatusRunning, nil)

        case string(LoopStatusRunning):
            // 执行下一步
            if err := e.runNextStep(ctx, scheduleID); err != nil {
                e.transition(scheduleID, LoopStatusFailed, strPtr(err.Error()))
                return
            }
            // 执行完后进入 evaluating
            e.transition(scheduleID, LoopStatusEvaluating, nil)

        case string(LoopStatusEvaluating):
            decision, err := e.controller.Decide(ctx, e.buildContext(scheduleID))
            if err != nil {
                e.transition(scheduleID, LoopStatusFailed, strPtr(fmt.Sprintf("controller error: %v", err)))
                return
            }
            if err := e.applyDecision(ctx, scheduleID, decision); err != nil {
                e.transition(scheduleID, LoopStatusFailed, strPtr(err.Error()))
                return
            }

        case string(LoopStatusPaused):
            // 等待 HumanConfirmGate 回调或 Resume 请求
            return

        case string(LoopStatusCompleted), string(LoopStatusFailed):
            return

        default:
            return
        }
    }
}
```

状态定义（`daemon/internal/loop/types.go`）：

```go
type LoopStatus string

const (
    LoopStatusPending     LoopStatus = "pending"
    LoopStatusRunning     LoopStatus = "running"
    LoopStatusEvaluating  LoopStatus = "evaluating"
    LoopStatusPaused      LoopStatus = "paused"
    LoopStatusCompleted   LoopStatus = "completed"
    LoopStatusFailed      LoopStatus = "failed"
)
```

### 4.4 `Controller`：LLM 决策

```go
type Controller struct {
    agentManager *agent.AgentManager
    provider     string
    model        *string
    modeID       *string
    systemPrompt string
    maxIterations int
}

type DecisionType string

const (
    DecisionNextStep     DecisionType = "next_step"
    DecisionHumanConfirm DecisionType = "human_confirm"
    DecisionCompleted    DecisionType = "completed"
    DecisionFailed       DecisionType = "failed"
)

type LoopDecision struct {
    Type      DecisionType
    Step      *LoopStep        // for next_step
    Request   *HumanConfirmRequest // for human_confirm
    Summary   *string          // for completed
    Reason    *string          // for failed
    Reasoning string
}

func (c *Controller) Decide(ctx context.Context, loopCtx *LoopContext) (*LoopDecision, error) {
    // 1. 渲染 system prompt + context
    // 2. 通过 AgentManager 创建临时 controller agent 或复用 ProviderClient
    // 3. 要求结构化输出（function calling 或 JSON schema）
    // 4. 解析并校验决策
}
```

**Controller Provider 适配**：

```go
type DecisionProvider interface {
    Decide(ctx context.Context, systemPrompt string, userPrompt string, schema DecisionSchema) (*LoopDecision, error)
}

// 实现：
// - claudeDecisionProvider：使用 tool_use，强制 schema
// - openaiDecisionProvider：使用 function calling
// - kimiDecisionProvider：function calling
```

### 4.5 `StepExecutor` 注册表

```go
type StepExecutor interface {
    Type() string
    Execute(ctx context.Context, step LoopStep, env *StepEnv) (StepResult, error)
}

type StepEnv struct {
    ScheduleID   string
    Cwd          string
    AgentManager *agent.AgentManager
    TerminalMgr  *terminal.Manager
    Workspace    *workspace.Registry
    GitService   workspace.WorkspaceGitService
    PushNotifier *push.Notifier
    Logger       *slog.Logger
}

type StepResult struct {
    Status  string // succeeded | failed | skipped | timeout
    Output  *string
    Error   *string
    Metrics StepMetrics
}

type StepRegistry struct {
    executors map[string]StepExecutor
}

func (r *StepRegistry) Register(ex StepExecutor)
func (r *StepRegistry) Execute(ctx context.Context, stepType string, step LoopStep, env *StepEnv) (StepResult, error)
```

### 4.6 Step 详细设计

#### 4.6.1 Agent Step

```go
type AgentStepInput struct {
    Prompt      string  `json:"prompt"`
    Provider    *string `json:"provider,omitempty"`
    Model       *string `json:"model,omitempty"`
    ModeID      *string `json:"modeId,omitempty"`
    TimeoutMin  int     `json:"timeoutMin,omitempty"`
    AutoApprove bool    `json:"autoApprove,omitempty"`
}
```

执行流程：

1. 用 `AgentManager.CreateAgent` 创建临时 agent（标签 `source=loop,scheduleId=xxx,stepId=xxx`）。
2. 发送 prompt。
3. 等待 agent 完成或超时（复用现有 `waitForAgent` 模式）。
4. 收集最终 assistant message 作为 output。
5. agent 内部 tool 调用的审批遵循 loop 的 `approvalPolicy`。

注意：不要把 agent 内部每一轮 tool call 都暴露为 Loop step；一次 Agent step 是黑盒单元。

#### 4.6.2 Bash Step

```go
type BashStepInput struct {
    Command    string            `json:"command"`
    Cwd        *string           `json:"cwd,omitempty"`
    TimeoutSec int               `json:"timeoutSec,omitempty"`
    Env        map[string]string `json:"env,omitempty"`
}
```

执行：

- 默认使用 `terminal` 的 PTY 执行，或直接用 `exec.CommandContext`。
- 目录隔离：`cwd` 必须位于 loop 的 project root 下。
- 命令解析后检查危险操作（`rm -rf`, `sudo`, `curl | sh`, `git push` 等）。
- 超时默认 60s，最大 600s。

#### 4.6.3 Test Step

```go
type TestStepInput struct {
    Command string `json:"command"` // e.g. "go test ./..."
    Parser  string `json:"parser"`  // go | jest | pytest | generic
    Cwd     *string `json:"cwd,omitempty"`
}

type TestResult struct {
    Passed   bool           `json:"passed"`
    Summary  string         `json:"summary"`
    Failures []TestFailure  `json:"failures"`
}

type TestFailure struct {
    File    string `json:"file"`
    Test    string `json:"test"`
    Message string `json:"message"`
}
```

解析器：

- `go`：解析 `go test -json` 输出。
- `jest`：解析 `jest --json` 输出。
- `pytest`：解析 `pytest -v` 输出。
- `generic`：仅判断 exit code。

Test step 继承 Bash step 的安全策略，但默认属于“读/验证类”，在 `dangerous-only` 策略下自动执行。

#### 4.6.4 Read / Write Step

```go
type ReadStepInput struct {
    Path string `json:"path"`
}

type WriteStepInput struct {
    Path    string `json:"path"`
    Content string `json:"content"`
    Mode    string `json:"mode,omitempty"` // overwrite | append
}
```

- Path 解析为绝对路径后必须位于 project root 下（防止 symlink escape）。
- Write step 在 `dangerous-only` 策略下需要确认。

#### 4.6.5 Git Step

```go
type GitStepInput struct {
    Subcommand string   `json:"subcommand"` // status | diff | log | commit | push
    Args       []string `json:"args,omitempty"`
}
```

- `status` / `diff` / `log`：只读，自动执行。
- `commit` / `push`：需要确认；push 到 main/master 默认禁止。
- 通过 `workspace.GitService` 执行。

#### 4.6.6 Ask-User Step

```go
type AskUserStepInput struct {
    Question string   `json:"question"`
    Options  []string `json:"options,omitempty"` // 空表示自由输入
}
```

执行后 loop 进入 `paused`，通过 WebSocket push `loop/human_confirm_request` 给所有连接客户端，并触发 Push 通知。

### 4.7 人工确认门控

```go
type HumanConfirmGate struct {
    pending map[string]*pendingConfirm // scheduleID -> confirm
}

type pendingConfirm struct {
    StepID   string
    Options  []string
    Response chan string
}

func (g *HumanConfirmGate) Request(scheduleID string, req HumanConfirmRequest) (string, error)
func (g *HumanConfirmGate) Respond(scheduleID string, stepID string, choice string) error
```

**审批策略**：

- `auto`：所有 step 自动执行，无需确认。
- `dangerous-only`：只对 write / bash-rm / git-push 等危险操作请求确认。
- `every-step`：每个 step 都请求确认。

---

## 5. 持久化与恢复

### 5.1 持久化内容

Loop schedule 使用 `schedule.Store` 的 `~/.solo/schedules.json`，每个记录为：

```json
{
  "id": "loop-001",
  "type": "loop",
  "name": "Auto Fix Tests",
  "status": "running",
  "goal": "修复当前项目所有测试失败",
  "cadence": { "type": "cron", "expression": "0 2 * * *", "timezone": "Asia/Shanghai" },
  "target": { "type": "new-agent", "config": { "provider": "claude", "cwd": "~/work/backend" } },
  "controller": {
    "provider": "claude",
    "model": "claude-opus-4-7",
    "maxIterations": 20,
    "pauseBetweenIterationsMs": 5000
  },
  "tools": ["agent", "bash", "test", "read", "write", "git", "ask-user"],
  "currentIteration": 3,
  "steps": [
    {
      "id": "step-1",
      "type": "test",
      "status": "succeeded",
      "input": { "command": "go test ./...", "parser": "go" },
      "output": "FAIL: auth_test.go:42",
      "startedAt": "2026-06-20T02:00:00Z",
      "endedAt": "2026-06-20T02:00:05Z",
      "reasoning": "先跑测试了解当前状态"
    },
    {
      "id": "step-2",
      "type": "agent",
      "status": "succeeded",
      "input": { "prompt": "修复 auth_test.go 中的 JWT 验证" },
      "output": "已修改 jwt.go...",
      "reasoning": "测试失败集中在 JWT 验证，交给 agent 修复"
    }
  ],
  "pendingHumanConfirm": null,
  "budget": { "maxControllerTokens": 100000, "maxAgentTokens": 500000, "maxTotalCost": 5.0 },
  "createdAt": "2026-06-20T01:55:00Z",
  "updatedAt": "2026-06-20T02:05:00Z",
  "nextRunAt": "2026-06-21T02:00:00Z"
}
```

### 5.2 持久化时机

- 每次状态变更（pending → running → evaluating → paused → completed/failed）。
- 每个 step 开始前和结束后。
- Controller 决策后。
- 使用 `schedule.Store.saveLocked()` 的原子 temp+rename。

### 5.3 崩溃恢复

Daemon 启动时，`schedule.Executor` 或 `LoopEngine.Recover()` 扫描所有 `type: "loop"` 且状态为 `running` / `evaluating` / `paused` 的记录：

```go
func (e *Engine) Recover() {
    for _, sched := range e.store.List() {
        if sched.Type != "loop" {
            continue
        }
        switch LoopStatus(sched.Status) {
        case LoopStatusRunning:
            // 检查最后一步状态
            last := lastStep(sched)
            if last != nil && last.Status == string(StepStatusRunning) {
                // 该 step 可能已执行或未完成；根据 step 类型决定是否重试
                e.retryOrContinue(sched.ID, last)
            } else {
                e.Start(context.Background(), sched.ID)
            }
        case LoopStatusEvaluating:
            // 重新执行一次 Controller.Decide
            e.Start(context.Background(), sched.ID)
        case LoopStatusPaused:
            // 保持暂停，通知用户
            e.notifyRecovered(sched.ID)
        }
    }
}
```

### 5.4 Step 幂等性

每个 step ID 使用 `uuid` 或 `{scheduleID}-{iteration}-{seq}`。执行前检查：

```go
func (e *Engine) executeStepWithIdempotency(scheduleID string, step LoopStep, env *StepEnv) StepResult {
    sched, _ := e.store.Get(scheduleID)
    for _, s := range sched.Steps {
        if s.ID == step.ID && s.Status == string(StepStatusSucceeded) {
            return StepResult{Status: string(StepStatusSucceeded), Output: s.Output}
        }
    }
    return e.stepRegistry.Execute(...)
}
```

---

## 6. 与现有模块集成

### 6.1 AgentManager

- Controller 使用 `AgentManager` 创建临时 agent 进行决策。
- Agent step 使用 `AgentManager` 创建 worker agent。
- 标签统一：`source=loop`, `scheduleId=xxx`, `stepId=xxx`。

### 6.2 Terminal

- Bash step 和 Test step 使用 `daemon/internal/terminal/` 的 PTY 或直接执行能力。
- 输出截断：单 step 输出超过 64KB 时截断并提示。

### 6.3 Workspace / Git

- Read/Write/Git step 使用 `workspace.ProjectRegistry` 解析 project root。
- Git step 使用 `WorkspaceGitService`。

### 6.4 Memory

- Loop 中每个 agent step 产生的 turn 自动进入 session memory。
- 未来 Project Memory 可以把 loop 历史作为项目知识索引。

### 6.5 Push 通知

- Loop 进入 `paused`（human_confirm）时触发 Push 通知。
- Loop `completed` / `failed` 时触发通知。

---

## 7. App-Bridge 与 App 变更

### 7.1 App-Bridge

1. 更新 `app-bridge/src/shared/messages.ts`：
   - `StoredScheduleSchema` 增加 loop 字段。
   - 新增 `LoopControllerConfigSchema`、`LoopStepSchema`、`HumanConfirmRequestSchema`。

2. 保留现有 `app-bridge/src/server/loop/rpc-schemas.ts` 以兼容旧 loop，但新增 `schedule/loop` 相关 schema。

### 7.2 App UI

| 页面 | 变更 |
|------|------|
| **Schedule Dashboard** | 显示 loop 类型卡片，带 `LOOP` badge |
| **Schedule Create** | 新增 "Loop Mode" 开关 |
| **Loop Detail** | 用 step timeline 替换现有 iteration list；显示 Controller reasoning |
| **Human Confirm Sheet** | 新增底部弹窗，支持 approve/skip/abort |
| **Push 通知** | loop 暂停/完成/失败时显示 |

**新增 hooks**：

- `useLoopSteps(scheduleId)`：查询 step timeline
- `useLoopHumanConfirm(scheduleId)`：发送确认响应

---

## 8. CLI 变更

保留现有 `solo loop` 命令，但底层逐步切换为 schedule-based loop：

```bash
# 创建 loop（底层生成 StoredSchedule with type="loop"）
solo loop create "修复所有测试失败" --provider claude --model opus-4-7 --cwd ~/work/backend

# 也可以走 schedule 路径
solo schedule create --type loop --goal "..." --controller-provider claude --tools agent,bash,test

# 状态/日志
solo loop status <id>
solo loop logs <id>

# 控制
solo loop start <id>
solo loop pause <id>
solo loop resume <id>
solo loop abort <id>

# 确认
solo loop confirm <id> --action approve
```

---

## 9. 从现有 Loop 模块迁移

### 9.1 迁移策略

采用**双轨运行 → 数据迁移 → 旧模块下线**三阶段：

**Phase 1：双轨运行（2 周）**

- 新建 `daemon/internal/loop/` 下的 Runner/Engine/Controller/Step 模块。
- 在 `schedule.Executor` 中注册 `LoopRunner`，处理 `type: "loop"` 的 schedule。
- 旧 `loop.Store/Engine` 继续服务现有 `loop/*` RPC。
- 新增 `schedule/loop/create` RPC，UI 的新建 loop 走新路径。

**Phase 2：数据迁移（1 周）**

- 启动时读取 `~/.solo/loops.json`。
- 把每个 `LoopRecord` 转换为 `StoredSchedule{Type:"loop"}`，写入 `~/.solo/schedules.json`。
- 删除或重命名 `~/.solo/loops.json` 为 `loops.json.migrated`。

**Phase 3：旧模块下线（1 周）**

- 现有 `loop/*` RPC 改为对新 schedule store 的兼容层。
- 移除 `daemon/internal/loop/store.go` 和 `engine.go` 中旧的 worker/verifier 逻辑。
- 保留 `protocol/message_loop.go` 中的类型以兼容旧客户端，标记 deprecated。

### 9.2 数据映射

| 旧 LoopRecord | 新 StoredSchedule |
|--------------|-------------------|
| `Prompt` | `Goal` |
| `Provider` / `Model` | `Controller.Provider` / `Controller.Model` |
| `MaxIterations` | `Controller.MaxIterations` |
| `SleepMs` | `Controller.PauseBetweenIterationsMs` |
| `VerifyChecks` | 转换为初始 steps（test step） |
| `Iterations[]` | 转换为 `Steps[]`（agent step + test step） |
| `Status` | `Status`（running/succeeded/failed/stopped） |

---

## 10. 安全与成本控制

### 10.1 目录隔离

```go
func sanitizePath(projectRoot, requested string) (string, error) {
    abs, err := filepath.EvalSymlinks(filepath.Join(projectRoot, requested))
    if err != nil {
        return "", err
    }
    rootAbs, _ := filepath.EvalSymlinks(projectRoot)
    if !strings.HasPrefix(abs, rootAbs+string(filepath.Separator)) && abs != rootAbs {
        return "", fmt.Errorf("path escapes workspace")
    }
    return abs, nil
}
```

### 10.2 危险操作判定

```go
var dangerousPatterns = []string{
    `\brm\s+-rf\s+/`,
    `\bsudo\b`,
    `\bcurl\s+.*\|\s*sh`,
    `\bwget\s+.*\|\s*sh`,
    `\bgit\s+push\s+origin\s+(main|master)`,
}
```

### 10.3 成本门控

每次 Controller/Agent 调用后累加 token/成本，超过 `LoopBudget` 任意上限时：

1. 记录错误日志。
2. 自动暂停 loop。
3. 推送通知：“Loop 因超出预算已暂停”。

---

## 11. 可观测性

### 11.1 Prometheus Metrics

```
solo_loop_total
solo_loop_status{status="pending|running|evaluating|paused|completed|failed"}
solo_loop_iterations_total{schedule_id="..."}
solo_loop_step_duration_seconds{step_type="agent|bash|test|..."}
solo_loop_controller_decisions_total{decision="next_step|human_confirm|completed|failed"}
solo_loop_controller_tokens_total
solo_loop_agent_tokens_total
solo_loop_human_confirm_pending_total
```

### 11.2 日志

- 每个 loop 的完整日志写入 `~/.solo/schedules/loops/{scheduleID}.jsonl`（可选）。
- Controller 的 prompt、raw response、parsed decision 均记录 debug 日志。

---

## 12. 测试策略

### 12.1 单元测试

- Controller prompt 渲染。
- 决策 JSON 解析与失败回退。
- Step executor：bash parser、test parser、path sanitize。
- 状态机转换。
- HumanConfirmGate 行为。

### 12.2 集成测试

- 使用 Mock Provider 模拟 Controller 决策，跑完整 loop。
- 测试崩溃恢复：daemon kill 后重启，loop 从最后 step 恢复。
- 测试 human_confirm：loop 暂停后用户响应再继续。

### 12.3 E2E 测试

- 创建一个真实 loop（如 "create hello.go and run go test"），验证自主完成。
- 危险操作触发 human confirm sheet。

---

## 13. 实施阶段

| 阶段 | 时长 | 任务 | 成功标准 |
|------|------|------|----------|
| **P1** | 1 周 | 扩展 protocol；新建 loop Runner/Engine 骨架；注册到 schedule.Executor | `type:"loop"` schedule 能被识别并启动 |
| **P2** | 1 周 | 实现 Controller（Claude 适配）与决策 schema | Controller 输出有效 next_step/completed/failed |
| **P3** | 1 周 | 实现 agent/bash/test/read/write step | 跑通 "create hello.go" loop |
| **P4** | 1 周 | 实现 git/ask-user step、HumanConfirmGate、Push 通知 | 危险操作触发确认，移动端可响应 |
| **P5** | 1 周 | 崩溃恢复、预算控制、metrics | kill daemon 后 loop 可恢复 |
| **P6** | 1 周 | 旧 loop 数据迁移、UI 切换到 schedule-based loop | 旧 loops 在新 dashboard 可见 |

---

## 14. 风险与缓解

| 风险 | 影响 | 缓解 |
|------|------|------|
| LLM Controller 决策不稳定 | 高 | Prompt 工程 + JSON schema + function calling + 解析失败重试 |
| Loop 进入死循环 | 高 | maxIterations、early stop（连续无进展）、预算上限 |
| Step 执行中 daemon 崩溃 | 中 | step 级持久化 + 幂等执行 + 恢复时重试或跳过 |
| 旧 loop 数据迁移失败 | 中 | 备份 `loops.json`；写一次性迁移脚本；灰度测试 |
| 人工确认响应超时 | 中 | 默认 30 分钟超时后暂停；用户可配置 |
| Agent step 内部 tool 调用审批与 loop 策略冲突 | 中 | Loop policy 作为兜底；agent 内部审批仍生效 |

---

## 15. 参考实现接口草图

### 15.1 LoopRunner

```go
func NewLoopRunner(engine *Engine) schedule.Runner
```

### 15.2 Engine

```go
func NewEngine(
    store *schedule.Store,
    agentManager *agent.AgentManager,
    controller *Controller,
    registry *StepRegistry,
    gate *HumanConfirmGate,
    logger *slog.Logger,
) *Engine

func (e *Engine) Start(ctx context.Context, scheduleID string) error
func (e *Engine) Recover()
func (e *Engine) Pause(scheduleID string) error
func (e *Engine) Resume(scheduleID string) error
func (e *Engine) Abort(scheduleID string) error
func (e *Engine) RespondHumanConfirm(scheduleID string, stepID string, choice string) error
```

### 15.3 Controller

```go
func NewController(agentManager *agent.AgentManager, config LoopControllerConfig) *Controller
func (c *Controller) Decide(ctx context.Context, loopCtx *LoopContext) (*LoopDecision, error)
```

### 15.4 StepRegistry

```go
func NewStepRegistry() *StepRegistry
func (r *StepRegistry) Register(ex StepExecutor)
func (r *StepRegistry) Execute(ctx context.Context, stepType string, step LoopStep, env *StepEnv) (StepResult, error)
```

---

## 参考文档

- [Loop Schedule Design](loop-schedule-design.md)
- [Loop Schedule Deep Dive](loop-schedule-deep-dive.md)
- [Solo 2026 产品/技术路线图](roadmap-2026.md)
- [Solo Roadmap Architecture Mapping](../analysis/solo-roadmap-architecture-mapping.md)
- [Schedule Module Analysis](../analysis/app-bridge-schedule-module.md)
- [Create Schedule Flow](../analysis/create-schedule-flow.md)
