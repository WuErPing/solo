# PRD: Product + Task Unification

> **Date**: 2026-07-31
> **Status**: Draft
> **Priority**: High
> **Supersedes**: [product-layer-evolution.md](product-layer-evolution.md) 中的 P2-P5（统一 Task、Board、Inbox、Trigger 部分）
> **Requires**: ADR-002 (Product-Task Unification architecture decision)

---

## 1. Problem Statement

Solo 当前有 6 个用户可感知概念：

| 概念 | 用户需要理解的内容 |
|------|-------------------|
| Project | 代码仓库绑定 |
| Workspace | tmux 执行环境、tab 管理 |
| Agent | provider/model/mode 配置、生命周期状态 |
| Loop | 迭代 + 验证 + 模板 + 实例 |
| Schedule | cron/interval + target 类型 + run 历史 |
| Tmux Pane | agent 检测、实时流、key injection |

用户必须理解这 6 个概念及其交叉关系才能有效使用产品。核心矛盾：

- Loop 和 Schedule 都是 "让 agent 做事"，但用户需要学两套心智模型
- Agent 是执行器，但被暴露为一等管理对象（用户不关心 agent 本身，关心事有没有做完）
- Workspace 和 Project 边界模糊（一个 project 可以有多个 workspace？一个 workspace 对应一个 tmux session？）
- Tmux Pane 是底层实现细节，但用户被迫直接管理

**目标**：收敛到 2 个用户概念 — **Product**（我在做什么）和 **Task**（要完成什么事）。其他一切降为内部实现。

---

## 2. Target User Mental Model

### Before

```
"我要在 Project X 里开一个 Workspace，创建一个 Loop，
 配置 Agent 的 provider 和 model，设定 verify prompt，
 然后去 Tmux Pane 里看它跑。另外还有个 Schedule 每天跑一次。"
```

### After

```
"Product: Solo App"
  ├── Task: 修复登录 bug         (manual, running → 看它跑)
  ├── Task: 重构 auth 模块       (iterate ×5, 验证通过才停)
  ├── Task: 每日依赖审计         (cron 09:00, 自动跑)
  ├── Task: 写 release notes     (manual, queued)
  └── Task: 调试性能问题         (interactive, 我自己在操控)
```

用户只需回答两个问题：
1. **我在做什么产品？** → Product
2. **这个产品下有什么事要做？** → Task

---

## 3. Concept Reduction

| 现有概念 | 归约到 | 理由 |
|----------|--------|------|
| Project | **Product** | 语义更准确 — 不只是 repo，是 "我在做的东西" |
| Workspace | Product 的运行态 | tmux session 是 Product 的执行环境，不是独立概念 |
| Loop | Task(execution.mode=iterate) | 迭代验证是执行策略 |
| Schedule | Task(trigger.type=cron) | 定时触发是触发方式 |
| Agent | Task 的内部执行器 | 用户不需要直接 "管理 agent" |
| Tmux Pane | Task 的实时视图 | 执行时的窗口，不是管理对象 |

---

## 4. Data Model

### 4.1 Product

```go
type Product struct {
    ID        string        `json:"id"`
    Name      string        `json:"name"`
    Avatar    string        `json:"avatar,omitempty"` // emoji
    Repos     []string      `json:"repos"`            // 关联代码仓库绝对路径
    Config    ProductConfig `json:"config"`
    CreatedAt time.Time     `json:"createdAt"`
    UpdatedAt time.Time     `json:"updatedAt"`
}

type ProductConfig struct {
    DefaultProfileID string   `json:"defaultProfileId,omitempty"`
    Skills           []string `json:"skills,omitempty"`     // 产品级 skills，所有 task 继承
    VerifyChecks     []string `json:"verifyChecks,omitempty"` // 产品级验证基线 (e.g., "make ci")
    Env              map[string]string `json:"env,omitempty"` // 注入执行环境的变量
}
```

**持久化**：`~/.solo/products.json`

**与现有 Project 的关系**：Product 吸收 Project 的 repo 绑定职责。一个 Product 可关联多个 repo（monorepo 场景或前后端分离场景）。

### 4.2 Task

```go
type Task struct {
    ID        string     `json:"id"`
    ProductID string     `json:"productId"`
    Title     string     `json:"title"`
    Prompt    string     `json:"prompt"`
    Status    TaskStatus `json:"status"`

    Execution Execution  `json:"execution"`
    Trigger   Trigger    `json:"trigger"`

    ProfileID string     `json:"profileId,omitempty"` // 覆盖 product default
    Skills    []string   `json:"skills,omitempty"`    // task 级追加

    Runs      []Run      `json:"runs"`
    CreatedAt time.Time  `json:"createdAt"`
    UpdatedAt time.Time  `json:"updatedAt"`
}

type TaskStatus string
const (
    TaskQueued    TaskStatus = "queued"
    TaskRunning   TaskStatus = "running"
    TaskDone      TaskStatus = "done"
    TaskFailed    TaskStatus = "failed"
    TaskPaused    TaskStatus = "paused"
)
```

### 4.3 Execution（怎么跑）

```go
type Execution struct {
    Mode          string   `json:"mode"` // "once" | "iterate" | "interactive"
    MaxIterations int      `json:"maxIterations,omitempty"` // iterate mode
    MaxTimeMs     int64    `json:"maxTimeMs,omitempty"`
    SleepMs       int      `json:"sleepMs,omitempty"`       // iterate 间隔
    VerifyPrompt  string   `json:"verifyPrompt,omitempty"`  // LLM 语义验证
    VerifyChecks  []string `json:"verifyChecks,omitempty"`  // shell 验证（覆盖 product 级）
}
```

| Mode | 行为 | 对应现有概念 |
|------|------|-------------|
| `once` | 给 prompt，agent 跑完即结束 | Schedule 的 new-agent target |
| `iterate` | prompt + 验证，循环直到通过或达上限 | Loop |
| `interactive` | 无 prompt，用户实时操控 agent | 当前 Workspace/Agent 体验 |

### 4.4 Trigger（什么时候跑）

```go
type Trigger struct {
    Type  string `json:"type"` // "manual" | "cron" | "webhook" | "on_complete"
    Cron  string `json:"cron,omitempty"`
    Event string `json:"event,omitempty"` // for on_complete: "task:done:{taskID-glob}"
}
```

| Type | 行为 | 对应现有概念 |
|------|------|-------------|
| `manual` | 用户点击 Run 或创建时立即执行 | Loop 手动 run |
| `cron` | 按 cron 表达式定时触发 | Schedule |
| `webhook` | daemon HTTP endpoint 触发 | 新增 |
| `on_complete` | 另一个 task 完成后触发 | 新增（task chain） |

### 4.5 Run（执行记录）

```go
type Run struct {
    ID         string     `json:"id"`
    Status     string     `json:"status"` // running|done|failed|stopped
    AgentID    string     `json:"agentId"`
    Iterations []Iteration `json:"iterations,omitempty"`
    StartedAt  time.Time  `json:"startedAt"`
    FinishedAt *time.Time `json:"finishedAt,omitempty"`
    Error      string     `json:"error,omitempty"`
}

type Iteration struct {
    Index      int    `json:"index"`
    AgentID    string `json:"agentId"`
    VerifyPass bool   `json:"verifyPass"`
    Reason     string `json:"reason,omitempty"`
}
```

### 4.6 AgentProfile（执行器配置）

```go
type AgentProfile struct {
    ID          string        `json:"id"`
    Name        string        `json:"name"`
    Avatar      string        `json:"avatar,omitempty"`
    Description string        `json:"description,omitempty"`
    Template    AgentTemplate `json:"template"` // provider, model, systemPrompt, mcp...
    Skills      []string      `json:"skills,omitempty"`
    Concurrency int           `json:"concurrency,omitempty"` // 0=unlimited
    CreatedAt   time.Time     `json:"createdAt"`
    UpdatedAt   time.Time     `json:"updatedAt"`
}
```

**持久化**：`~/.solo/profiles.json`

### 4.7 Skill

```
~/.solo/skills/
├── go-conventions.md
├── deploy-checklist.md
├── code-review.md
└── assets/
    └── pr-template.md
```

文件系统即库。Skill ID = 文件名（去 `.md`）。无 JSON 索引。

### 4.8 Notification

```go
type Notification struct {
    ID        string    `json:"id"`
    Type      string    `json:"type"` // task_done|task_failed|attention|verify_failed
    TaskID    string    `json:"taskId,omitempty"`
    ProductID string    `json:"productId,omitempty"`
    Title     string    `json:"title"`
    Body      string    `json:"body"`
    Read      bool      `json:"read"`
    CreatedAt time.Time `json:"createdAt"`
}
```

**持久化**：`~/.solo/notifications.json`（FIFO，保留最近 200 条）

---

## 5. Resolution Rules

Task 创建 agent 时的 resolve 链：

```
Task.ProfileID (or Product.Config.DefaultProfileID)
  → load AgentProfile
    → merge Profile.Template + Task-level overrides
      → merge Skills: global + Product.Config.Skills + Profile.Skills + Task.Skills
        → inject into AgentSessionConfig
          → daemon creates agent session
```

VerifyChecks resolve：
```
Task.Execution.VerifyChecks (if non-empty)
  → else Product.Config.VerifyChecks
    → else none
```

---

## 6. RPC Surface

### Product

```
product/list     → []Product
product/get      → Product
product/create   → Product
product/update   → Product
product/delete   → {id}
```

### Task

```
task/create      → Task
task/list        → []Task  (filter: productId, status, trigger.type)
task/get         → Task (full, with runs)
task/run         → {taskId}  (manual trigger / re-run)
task/stop        → {taskId}
task/update      → Task
task/delete      → {taskId}
task/logs        → streaming (SSE/WS)
```

### Profile

```
profile/list     → []AgentProfile
profile/get      → AgentProfile
profile/create   → AgentProfile
profile/update   → AgentProfile
profile/delete   → {id}
```

### Skill

```
skill/list       → []SkillInfo{ID, Title, Size, ModifiedAt}
skill/read       → {ID, Content}
skill/write      → {ID, Content}
skill/delete     → {ID}
```

### Notification

```
notification/list        → []Notification (unread first, paginated)
notification/read        → {id}
notification/read-all    → {}
notification/unread-count → {count}
```

### Interactive Session（escape hatch）

```
session/open     → {productId, profileId?} → SessionInfo{agentId, paneId}
session/send     → {agentId, input}  (key injection)
session/stream   → {agentId} → streaming ANSI output
session/close    → {agentId}
```

Interactive session 在数据层表现为 `Task(mode=interactive, trigger=manual)`，但 RPC 独立以保留低延迟操控体验。

**Total**: ~25 RPCs（vs 当前 loop/* + schedule/* + agent/* 约 35+）

---

## 7. App UI Structure

### Navigation

```
Tab Bar: [Products] [Inbox] [Settings]
```

### Products Tab

```
Product List
├── Card: {avatar} {name} | {repo count} repos | {running tasks} running
└── Tap → Product Detail

Product Detail
├── Header: name, repo badges, [Edit] [New Task]
├── Task Board (horizontal scroll or segmented)
│   ├── Running (cards with live progress)
│   ├── Queued
│   └── Done / Failed (collapsible history)
├── Task Card
│   ├── Title, profile avatar, mode badge (once/iterate/interactive/cron)
│   ├── Progress: "iter 3/5" or "running 2m" or "next: 14:00"
│   └── Tap → Task Detail
└── Panes Strip (bottom, collapsible)
    └── Active task panes — tap to expand full-screen terminal view
```

### Task Detail

```
├── Title + Status badge
├── Prompt (collapsible)
├── Config: profile, skills, execution mode, trigger
├── [Run] [Stop] [Edit] [Duplicate] [Delete]
├── Runs History
│   ├── Run #3 — done, 2 iterations, 4m32s
│   ├── Run #2 — failed (verify: 2 checks red)
│   └── Run #1 — done, 1 iteration
├── Live View (when running)
│   └── Embedded terminal stream (xterm.js / ANSI render)
└── Verification Results (iterate mode)
    ├── Iteration 1: ✗ "missing error handling in handler"
    ├── Iteration 2: ✗ "test coverage below threshold"
    └── Iteration 3: ✓ all checks pass
```

### Inbox Tab

```
├── Unread badge on tab icon
├── List: {icon} {title} — {time ago}
│   ├── ✓ Task "修复登录 bug" completed (2m ago)
│   ├── ✗ Task "重构 auth" failed: timeout (1h ago)
│   └── ⚠ Claude needs permission: write /etc/hosts (3h ago)
└── Tap → navigate to Task Detail
```

### Settings Tab

```
├── Profiles (list + create/edit)
├── Skills (list + markdown editor)
├── Providers (LLM config, unchanged)
├── Host (daemon connection, unchanged)
└── About
```

### 砍掉的 Screen

| 现有 Screen | 处置 |
|-------------|------|
| sessions-screen | 合入 Product Detail → Panes Strip |
| loops-screen | 合入 Task Board (mode=iterate) |
| loop-create-screen | 合入 New Task flow (选 mode=iterate) |
| loop-detail-screen | 合入 Task Detail |
| schedules-screen | 合入 Task Board (trigger=cron) |
| schedule-detail-screen | 合入 Task Detail |
| tmux-dashboard-screen | 合入 Product Detail → Panes Strip |
| workspace-screen | 降级为 interactive session 的全屏终端视图 |
| projects-screen | 替换为 Products list |

---

## 8. Daemon Architecture Changes

### Module Mapping

| 现有模块 | 变化 |
|----------|------|
| `daemon/internal/agent/` | **不变** — 仍是执行层，被 task engine 调用 |
| `daemon/internal/loop/` | 合入 `daemon/internal/task/iterate.go` |
| `daemon/internal/schedule/` | 合入 `daemon/internal/task/trigger.go` |
| `daemon/internal/server/` | RPC handler 重构：product/*, task/*, profile/*, skill/*, notification/* |
| `daemon/internal/task/` | **新增** — 统一 engine |
| `daemon/internal/product/` | **新增** — product store |
| `daemon/internal/profile/` | **新增** — profile store |
| `daemon/internal/skill/` | **新增** — skill reader/injector |
| `daemon/internal/notification/` | **新增** — notification store + push trigger |

### Task Engine 内部结构

```go
// daemon/internal/task/engine.go
type Engine struct {
    store       *Store
    agentMgr    agent.Manager
    profiler    *profile.Store
    skiller     *skill.Injector
    notifier    *notification.Store
    triggerSched *TriggerScheduler  // cron/webhook/on_complete
}

func (e *Engine) Run(task *protocol.Task) (*protocol.Run, error) {
    cfg := e.resolveConfig(task)  // profile + skills + verify
    switch task.Execution.Mode {
    case "once":
        return e.runOnce(task, cfg)
    case "iterate":
        return e.runIterate(task, cfg)
    case "interactive":
        return e.openInteractive(task, cfg)
    }
}
```

### Trigger Scheduler

```go
// daemon/internal/task/trigger.go
type TriggerScheduler struct {
    ticker  *time.Ticker   // 30s tick for cron evaluation
    engine  *Engine
}

func (s *TriggerScheduler) Tick(now time.Time) {
    for _, task := range s.store.DueTasks(now) {
        go s.engine.Run(task)
    }
}

// Webhook: POST /api/trigger/{taskID}
// OnComplete: engine emits event → scheduler checks matching triggers
```

---

## 9. Migration Strategy

### Phase 1: Data Layer (v0.13.x)

- 引入 `daemon/internal/task/` engine，内部包装现有 loop.Engine + schedule.Executor
- 引入 `daemon/internal/product/` store
- 新增 RPC: product/*, task/*, profile/*, skill/*, notification/*
- **旧 RPC 保留**：loop/*, schedule/* 内部代理到 task engine
- 数据迁移工具：`loops.json` → Task(mode=iterate)，`schedules.json` → Task(trigger=cron)

### Phase 2: App 新 UI (v0.14.x)

- 新增 Products tab + Task Board + Task Detail + Inbox
- 旧 screen 保留但降级（Settings → "Legacy" 入口）
- 验证新 UI 覆盖所有旧功能

### Phase 3: 清理 (v0.15.x)

- 移除旧 screen（loops, schedules, sessions, tmux-dashboard, workspace）
- 移除旧 RPC（loop/*, schedule/*）
- 移除 `daemon/internal/loop/`, `daemon/internal/schedule/` 独立包
- protocol/ 中 message_loop.go, message_schedule.go 标记废弃

### 兼容性保证

| 数据 | 迁移方式 |
|------|----------|
| `~/.solo/loops.json` | 自动迁移为 products.json + tasks (mode=iterate) |
| `~/.solo/schedules.json` | 自动迁移为 tasks (trigger=cron) |
| `~/.solo/projects.json` | 重命名为 products.json，字段映射 |
| Agent sessions (in-memory) | 无持久化，重启后自然消失，无需迁移 |

---

## 10. Key Design Decisions

### D1: Interactive 模式的定位

**决策**：Interactive session 在数据层是 `Task(mode=interactive)`，但拥有独立 RPC（session/*）以保证低延迟。

**理由**：
- 语义统一（所有 "agent 在做事" 都是 task）
- 但 interactive 的交互模式（实时 key injection、ANSI stream）与 once/iterate 完全不同
- 独立 RPC 避免 task/run 的 request-response 模型对实时操控的延迟影响

### D2: Agent 不再是一等用户概念

**决策**：用户不直接 "create agent"。Agent 是 task 的内部执行器。

**Escape hatch**：
- `session/open` 本质上创建一个 mode=interactive 的 task + agent
- Settings 中仍可配置 provider/model（通过 Profile）
- 高级用户可通过 task prompt 完全控制 agent 行为

### D3: Product 极轻

**决策**：Product 最小形态 = 一个名字 + 一个 repo 路径。

**理由**：
- 单仓库单用户场景不需要复杂配置
- Config（skills, verify, env）全部 optional
- 避免 "必须先配置好 product 才能做事" 的摩擦

### D4: Tmux 回归执行层

**决策**：Tmux pane 不再是用户管理对象，而是 task run 的执行窗口。

**保留能力**：
- Product Detail 的 "Panes Strip" 展示所有 active task 的实时终端流
- 点击 pane 可全屏展开（当前 tmux-pane-xterm-screen 体验）
- Interactive session 的 key injection 不变

**砍掉**：
- 独立的 tmux-dashboard（多 pane 网格管理）
- 用户手动 "发现" / "管理" pane 的概念

### D5: 验证分层

**决策**：验证配置三层继承，task 级覆盖 product 级。

```
Product.Config.VerifyChecks  → 基线 (e.g., ["make ci", "make lint"])
Task.Execution.VerifyChecks  → 覆盖（非空时完全替代，不追加）
Task.Execution.VerifyPrompt  → LLM 语义验证（仅 iterate mode）
```

### D6: Skills 文件系统即库

**决策**：不引入数据库/JSON 索引。`~/.solo/skills/` 目录即库，文件名即 ID。

**理由**：
- 符合 Solo "daemon-local, file-based" 存储哲学
- 用户可直接 git 管理、编辑器编辑
- `skill/list` 通过 ReadDir 实现，性能足够（<100 files）

---

## 11. Success Metrics

| 指标 | 当前基线 | 目标 |
|------|----------|------|
| 新用户首次创建自动化任务的步骤数 | 7+ (project → workspace → loop → config → run → find pane) | 3 (product → task → run) |
| 用户需理解的概念数 | 6 | 2 |
| RPC 总数（用户可调用） | ~35 | ~25 |
| App screen 数量 | 15+ | 8 |
| 日活 task 创建数 | — | +50%（可见性 + 低摩擦） |
| 用户主动轮询 agent 状态频率 | — | -60%（inbox 推送） |

---

## 12. Risks & Mitigations

| 风险 | 概率 | 影响 | 缓解 |
|------|------|------|------|
| Interactive 体验在 "task" 语义下变得不自然 | 中 | 高 | 独立 session/* RPC + 全屏终端视图保持当前体验 |
| 迁移期两套 UI 并存增加维护成本 | 高 | 中 | 严格限定 Phase 1-2 为一个版本周期（4 周） |
| Product 概念对极简场景过重 | 低 | 低 | Product 可一键创建（仅 name + repo），零配置可用 |
| Task engine 合并引入回归 | 中 | 高 | Phase 1 旧 RPC 代理到新 engine，行为不变；E2E 覆盖 |
| 砍掉 tmux-dashboard 后多 pane 监控能力丢失 | 中 | 中 | Panes Strip 保留多 pane 缩略图 + 全屏展开 |

---

## 13. Open Questions

| # | 问题 | 选项 | 倾向 |
|---|------|------|------|
| 1 | Product 是否支持多 repo 的独立 cwd per task？ | A: task 级 cwd 覆盖 B: product 级统一 | A（灵活） |
| 2 | Interactive session 是否出现在 Task Board？ | A: 出现（mode=interactive）B: 仅在 Panes Strip | A（统一） |
| 3 | Task 优先级/排序？ | A: FIFO B: 用户手动排序 C: 优先级字段 | A（v1），C（后续） |
| 4 | Profile 是否跨 product 共享？ | A: 全局 B: per-product | A（全局，product 设 default） |
| 5 | 旧数据迁移失败时的 fallback？ | A: 阻塞启动 B: 跳过 + 告警 | B（不阻塞） |
| 6 | Webhook trigger 的鉴权方式？ | A: daemon local token B: HMAC signature | A（localhost 够用） |

---

## 14. Related Documents

- [Loop Schedule Spec](loop-schedule-spec.md) — 被本 PRD 的 Task 统一模型取代
- [ADR-001: Shared Agent Template](../../decisions/adr-001-shared-agent-template-for-loop-and-schedule.md) — 已铺路（AgentTemplate 统一）
- [Product Layer Evolution](product-layer-evolution.md) — 渐进方案（本 PRD 的 P0/P1 部分仍有效：Skills, Profile）
- [Roadmap 2026](../roadmap-2026.md) — 需更新以反映 Product+Task 方向
