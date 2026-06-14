# Solo Loop Schedule：用大模型把 Schedule 进化成自治循环

> **文档类型**：产品/架构设计方案
> **日期**：2026-06-13
> **基线版本**：Solo v0.6.0
> **目标读者**：产品、后端、前端、CLI 开发者
> **依赖文档**：[Schedule Module Analysis](../analysis/app-bridge-schedule-module.md)、[Create Schedule Flow](../analysis/create-schedule-flow.md)

---

## 1. 背景与目标

### 1.1 当前 Schedule 的局限

Solo 已有一个完整的 Schedule 系统（`daemon/internal/schedule/`）：

- 支持 **cron** 和 **interval** 两种触发方式；
- 每次触发后创建/复用一个 Agent，执行一段 prompt；
- 执行结果记录到 `ScheduleRun`，然后等待下一次触发。

这套系统适合**定时值守任务**（例如每晚跑 lint、定时备份），但不够智能：

- 任务失败后不会自动重试或调整策略；
- 无法根据上一次执行结果决定下一步；
- 无法处理需要多轮迭代才能完成的目标（例如“修复所有测试失败”）。

### 1.2 目标：Schedule → Loop

引入 **Loop Schedule**：

> 一个由大模型驱动的自治循环。每次迭代执行一个 step，根据执行结果由 LLM 决策下一步，直到目标达成、失败、需要人工介入或达到最大迭代次数。

核心变化：

| 维度 | 传统 Schedule | Loop Schedule |
|---|---|---|
| 触发 | 基于时间 | 基于状态（上一步完成即触发下一步） |
| 决策 | 固定 prompt | LLM 根据上下文动态决策 |
| 反馈 | 只记录日志 | 反馈回循环，影响下一步 |
| 终止 | 时间/最大次数 | 目标完成、失败、人工确认、最大迭代 |
| 能力 | 单次任务 | 多 step 自治工作流 |

---

## 2. 核心概念

### 2.1 Loop Schedule 定义

```json
{
  "id": "loop-001",
  "type": "loop",
  "name": "Auto Fix Tests",
  "goal": "修复当前项目的所有测试失败，并确保 CI 通过。",
  "cadence": {
    "type": "loop",
    "maxIterations": 20,
    "pauseBetweenIterationsMs": 5000,
    "autoStart": true
  },
  "controller": {
    "provider": "claude",
    "model": "claude-opus-4-7",
    "modeId": "plan",
    "systemPrompt": "你是一个自治的软件开发循环控制器..."
  },
  "target": {
    "type": "agent",
    "agentId": "test-fix-agent",
    "config": {
      "provider": "kimi",
      "cwd": "~/work/backend",
      "approvalPolicy": "ask-for-dangerous"
    }
  },
  "tools": ["read", "write", "bash", "test", "git-diff", "ask-user"],
  "context": {
    "includeGitStatus": true,
    "includeTestOutput": true,
    "memoryEnabled": true
  },
  "status": "running",
  "currentIteration": 3,
  "steps": [...]
}
```

### 2.2 Loop 状态机

```
         ┌─────────────┐
         │   pending   │
         └──────┬──────┘
                │ 自动/手动启动
                ▼
         ┌─────────────┐
         │   running   │◄────────────────┐
         └──────┬──────┘                 │
                │ 执行 step               │
                ▼                        │
         ┌─────────────┐                 │
         │  evaluating │                 │
         │  （LLM 决策） │                 │
         └──────┬──────┘                 │
                │                        │
     ┌──────────┼──────────┐             │
     ▼          ▼          ▼             │
┌────────┐ ┌────────┐ ┌──────────┐      │
│ next   │ │ done   │ │ human    │      │
│ step   │ │        │ │ confirm  │      │
└────┬───┘ └───┬────┘ └────┬─────┘      │
     │         │           │            │
     └─────────┴───────────┘            │
               │                         │
               ▼                         │
         ┌─────────────┐                 │
         │paused/failed│                 │
         └─────────────┘                 │
                                         │
               用户确认 / 继续           │
               ──────────────────────────┘
```

状态：
- `pending`：已创建，等待启动
- `running`：正在执行 step
- `evaluating`：等待 LLM 决策下一步
- `paused`：需要人工确认或用户手动暂停
- `completed`：目标达成
- `failed`：失败或达到最大迭代次数

---

## 3. 架构设计

### 3.1 整体架构

```
┌─────────────────────────────────────────────────────────────┐
│                         Solo App / CLI                       │
│  Loop Dashboard · Step Timeline · Human Confirm UI           │
└─────────────────────────────┬───────────────────────────────┘
                              │ WebSocket
┌─────────────────────────────▼───────────────────────────────┐
│                         Solo Daemon                          │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐   │
│  │              Loop Controller                         │   │
│  │  (LLM-driven decision engine)                        │   │
│  └────────────────────────┬─────────────────────────────┘   │
│                           │ calls LLM via provider          │
│  ┌────────────────────────▼─────────────────────────────┐   │
│  │              Loop Engine                             │   │
│  │  · step scheduler    · state machine                 │   │
│  │  · context manager   · human confirm gate            │   │
│  └────────────────────────┬─────────────────────────────┘   │
│                           │ executes steps                  │
│  ┌────────────────────────▼─────────────────────────────┐   │
│  │  Existing Infrastructure                             │   │
│  │  · Agent Manager  · Workspace  · Terminal            │   │
│  │  · Schedule Store · Memory     · Tmux Dashboard      │   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

### 3.2 新增模块

新增 `daemon/internal/loop/`：

```
daemon/internal/loop/
├── controller.go        # LLM 决策控制器
├── engine.go            # 循环执行引擎
├── store.go             # Loop 状态持久化
├── step.go              # Step 定义与执行
├── context.go           # 循环上下文构建
├── tool.go              # Loop 可用工具注册表
├── human_confirm.go     # 人工确认门控
└── loop.go              # 公共类型与状态机
```

### 3.3 与现有 Schedule 的关系

两种集成策略：

**策略 A：Schedule 扩展（推荐）**
- 在现有 `StoredSchedule` 上新增 `type: "loop"`。
- `Cadence.Type` 新增 `"loop"`。
- 复用 `ScheduleStore` 的持久化，但执行路径走新的 `LoopEngine`。
- 优点：UI/CLI 改动小，向后兼容。

**策略 B：独立 Loop 模块**
- 完全新建 `LoopSchedule` 类型和 `loop.Store`。
- 与 Schedule 并行存在。
- 优点：概念清晰，不污染 Schedule。
- 缺点：两套持久化、两套 UI。

**推荐策略 A**：把 Loop 视为 Schedule 的高级形态，在 `type: "loop"` 时走 Loop 执行路径。

---

## 4. Loop Controller（LLM 决策）

### 4.1 职责

Loop Controller 是一个大模型调用层，负责：

1. 接收当前循环上下文（goal、历史 steps、当前状态、可用工具）；
2. 输出下一步决策：执行哪个 step、使用什么参数、是否需要人工确认、是否完成。

### 4.2 输入上下文

```go
type LoopContext struct {
    Goal            string
    CurrentState    string
    Iteration       int
    MaxIterations   int
    History         []LoopStep
    WorkspaceStatus WorkspaceSnapshot
    AvailableTools  []ToolDefinition
    HumanPending    *HumanConfirmRequest
}

type LoopStep struct {
    ID          string
    Type        string // "agent", "bash", "test", "read", "write", "git", "ask-user"
    Input       map[string]interface{}
    Output      *string
    Status      string // "succeeded" | "failed" | "skipped"
    StartedAt   string
    EndedAt     *string
    Error       *string
}
```

### 4.3 输出决策

使用 function calling / tool use 让 LLM 输出结构化决策：

```json
{
  "decision": "next_step",
  "step": {
    "type": "agent",
    "input": {
      "prompt": "根据测试失败信息，修复 auth_test.go 中的 JWT 验证逻辑。",
      "modeId": "default"
    }
  },
  "reasoning": "上一步测试显示 3 个失败，都与 JWT 验证有关。先让 agent 修复该文件。"
}
```

或：

```json
{
  "decision": "completed",
  "summary": "所有测试已通过，无需进一步修改。"
}
```

或：

```json
{
  "decision": "human_confirm",
  "request": {
    "message": "我计划删除 migrations/ 目录下的旧文件，是否继续？",
    "options": ["approve", "skip", "abort"]
  }
}
```

### 4.4 Provider 适配

Loop Controller 本身就是 Solo 的一个 Agent 调用方：

```go
type Controller struct {
    provider string        // "claude" / "kimi" / "opencode"
    model    string
    client   agent.AgentClient
}

func (c *Controller) Decide(ctx context.Context, loopCtx LoopContext) (*LoopDecision, error) {
    // 1. 构建 system prompt + 用户上下文
    // 2. 通过 AgentClient 发送消息
    // 3. 解析返回的 tool_call 或结构化输出
}
```

- 复用 `daemon/internal/agent` 的 ProviderClient。
- 复用 `SerializableConfig` 的 `systemPrompt`、`model`、`provider` 等字段。
- 不需要新增 Provider，只需要新增一个使用现有 Provider 的 Controller。

---

## 5. Step 执行器

### 5.1 Step 类型

| Type | 说明 | 复用模块 |
|---|---|---|
| `agent` | 调用 Solo Agent 执行 prompt | `agent.Manager` |
| `bash` | 执行 shell 命令 | `terminal` |
| `test` | 运行测试并解析结果 | `terminal` + 自定义 parser |
| `read` | 读取文件 | `workspace` |
| `write` | 写入文件 | `workspace` |
| `git` | git diff / status / commit | `workspace` |
| `ask-user` | 向用户提问 | WebSocket push |
| `wait` | 等待一段时间 | time.Sleep |
| `terminate` | 结束循环 | LoopEngine |

### 5.2 Step 执行流程

```go
func (e *Engine) executeStep(step LoopStep) (LoopStepResult, error) {
    switch step.Type {
    case "agent":
        return e.runAgentStep(step)
    case "bash":
        return e.runBashStep(step)
    case "test":
        return e.runTestStep(step)
    case "read":
        return e.runReadStep(step)
    // ...
    }
}
```

### 5.3 结果回传

每个 step 执行后，结果会被：
1. 记录到 `LoopStep`；
2. 写入循环上下文；
3. 触发 Controller 进行下一轮决策。

---

## 6. 持久化设计

### 6.1 Loop 记录

复用 `protocol.StoredSchedule`，新增字段：

```go
type StoredSchedule struct {
    // 现有字段...
    Type            string          `json:"type,omitempty"` // "schedule" | "loop"
    Goal            string          `json:"goal,omitempty"`
    Controller      *LoopControllerConfig `json:"controller,omitempty"`
    CurrentIteration int            `json:"currentIteration,omitempty"`
    Steps           []LoopStep      `json:"steps,omitempty"`
}
```

### 6.2 状态保存策略

- 每次 step 开始前保存状态，防止崩溃后丢失进度。
- 保存到 `~/.solo/schedules/loops/{id}.json` 或合并到现有 schedule JSON。
- 支持从崩溃中恢复：`Engine.Start()` 时扫描未完成的 loop，询问用户是否继续。

---

## 7. 人工确认门控

### 7.1 触发条件

LLM 输出 `decision: "human_confirm"` 时，循环暂停，向用户推送确认请求。

### 7.2 推送渠道

- App 端：Push 通知 + Loop Dashboard 弹窗。
- CLI 端：命令行阻塞等待输入。
- 超时策略：默认 30 分钟无响应则暂停 loop。

### 7.3 审批粒度

可配置：
- `approvalPolicy: auto`：LLM 可执行所有 step，无需确认。
- `approvalPolicy: dangerous-only`：危险操作（删除、push、执行任意命令）需要确认。
- `approvalPolicy: every-step`：每个 step 都需要确认。

---

## 8. 安全与约束

1. **沙箱**：bash/test step 默认在 cwd 下运行，不跨越项目边界。
2. **最大迭代次数**：防止无限循环，默认 20 次。
3. **Token/成本上限**：单次 loop 设置最大 token 消耗，超限暂停。
4. **不可变目标**：Loop 启动后 goal 不可修改，防止 LLM 偏离初衷。
5. **审计日志**：所有 step 和 LLM 决策记录到 memory，支持回溯。

---

## 9. CLI / App 界面

### 9.1 CLI

```bash
# 创建 loop
solo loop create "修复所有测试失败" --provider claude --model opus-4-7 --cwd ~/work/backend

# 启动/暂停/恢复
solo loop ls
solo loop start <id>
solo loop pause <id>
solo loop resume <id>
solo loop abort <id>

# 查看进度
solo loop logs <id>
solo loop status <id>

# 确认待审批操作
solo loop confirm <id> --action approve
```

### 9.2 App

- **Loop Dashboard**：所有 loop 列表、状态、进度。
- **Loop Detail**：step 时间线、每次 LLM 决策理由、输出日志。
- **Human Confirm Sheet**：底部弹窗确认危险操作。
- **Create Loop**：从 Schedule Create 升级，新增 "Loop Mode" 选项。

---

## 10. 实施路线图

### Phase 1：Loop 基础（2–3 周）

1. 新增 `daemon/internal/loop/` 模块。
2. 扩展 `protocol.StoredSchedule` 支持 `type: "loop"`。
3. 实现 Loop Engine 状态机和 step 执行器（agent/bash/test/read/write）。
4. 实现 Loop Controller，复用现有 ProviderClient。
5. CLI：`solo loop create/start/pause/resume/abort/logs`。

### Phase 2：LLM 决策优化（1–2 周）

1. 优化 Controller prompt，提升决策准确率。
2. 支持 function calling/tool use 输出结构化决策。
3. 添加常见 loop 模板（fix-tests、refactor、update-deps）。

### Phase 3：人工确认与移动端（1–2 周）

1. Human confirm gate。
2. App Loop Dashboard 和 confirm sheet。
3. Push 通知。

### Phase 4：与 Schedule 深度融合（1–2 周）

1. 在 App Schedule Dashboard 中支持 "Convert to Loop"。
2. Schedule 失败时自动建议转换为 Loop。
3. Loop 可设置定时触发（例如每晚启动一次 fix-tests loop）。

---

## 11. 与产品方向的关系

该方案直接对应 [Feature Directions 2026](feature-directions-2026.md) 中的：

- **2.3 任务编排与 Loop 工作流**
- **3.2 测试驱动自动化（Auto Test/Fix Loop）**
- **2.2 长期记忆与项目知识库**（Loop 历史可作为 project memory）

同时它也为 **Provider Hub / CC-Switch 迁移** 提供了上层使用场景：Loop Controller 可以消费 Provider Hub 的路由决策，选择最优 provider 执行控制任务。

---

## 参考文档

- [Feature Directions 2026](feature-directions-2026.md)
- [Provider Hub / CC-Switch Migration Design](agent-profile-switch-export-design.md)
- [Schedule Module Analysis](../analysis/app-bridge-schedule-module.md)
- [Create Schedule Flow](../analysis/create-schedule-flow.md)
