# Solo Product Layer Evolution — Absorbing Multica Abstractions

> **Date**: 2026-07-30
> **Status**: Draft
> **Direction**: B — Solo 吸收 Multica 产品层抽象，保持 terminal-native 体验
> **Reference**: [Multica](https://github.com/multica-ai/multica) 产品分析

---

## 1. Background & Motivation

### Solo 现状

Solo 有三个独立工作原语：Agent（原子 session）、Loop（迭代验证）、Schedule（定时触发）。它们各自创建 agent，无共同抽象、无统一视图、无知识沉淀机制。

| 痛点 | 表现 |
|------|------|
| 心智模型割裂 | 用户需分别理解 Loop 和 Schedule 两套概念、两套 RPC、两个 screen |
| 无知识复用 | System prompt 是裸 string，团队规范/检查清单无法沉淀为可复用资产 |
| Agent 无身份 | 每次创建 agent 都要重新配置 provider/model/prompt，无 "前端专家" 一键复用 |
| 无全局视图 | 无 "现在所有 agent 在做什么" 的聚合看板 |
| 被动感知缺失 | Agent 完成/失败/阻塞时用户必须主动查看 |
| 触发方式单一 | Schedule 仅支持 cron/interval，无 webhook、无 task 链 |

### Multica 可借鉴的产品抽象

| 抽象 | Multica 实现 | Solo 适配方向 |
|------|-------------|---------------|
| Task（统一工作对象） | Issue → Task → Agent session | Task 统一 Loop/Schedule |
| Skills | Markdown + 附件，provider-aware 注入 | 文件系统即库，daemon 注入 |
| Agent Profile | 多态 actor，有名字/头像/能力 | 轻量 preset（profiles.json） |
| Board | Kanban + List，drag-and-drop | Dashboard 升级为 Task Board |
| Inbox | 分级通知，实时推送 | 单用户简化版（unread/read） |
| Autopilot | Cron + Webhook + 并发策略 | Trigger 泛化 |

### 不引入的 Multica 概念

| 概念 | 原因 |
|------|------|
| Squads | 单用户产品，无团队分派场景 |
| 完整 Issue Tracker | Solo 不是项目管理工具 |
| Slack/飞书集成 | 交互面是 app + tmux，IM 集成 ROI 低 |
| 多 Workspace / RBAC | 单用户单 daemon |
| Composio / MCP overlay | AgentSessionConfig.McpServers 已覆盖 |

---

## 2. Feature Specifications

### 2.1 Skills（可复用知识注入）— P0

#### 目标

将团队知识（代码规范、部署清单、review 标准）沉淀为 Markdown 文档，agent 启动时自动注入，提升产出质量。

#### 数据模型

```
~/.solo/skills/
├── go-conventions.md
├── deploy-checklist.md
├── code-review.md
└── assets/
    └── pr-template.md
```

无数据库、无 JSON 索引 — 文件系统即库。Skill ID = 文件名（去 `.md`）。

#### Protocol 变更

```go
// protocol/message_common.go
type AgentSessionConfig struct {
    // ... existing fields ...
    Skills []string `json:"skills,omitempty"` // skill IDs to inject
}
```

#### Daemon 行为

1. Agent session 创建时，解析 `Skills` 列表。
2. 读取 `~/.solo/skills/{id}.md`（+ `assets/` 下同名目录的附件）。
3. 按 provider 注入：
   - Claude: 写入 `{cwd}/.claude/skills/{id}.md`
   - Codex: 写入 `$CODEX_HOME/skills/{id}.md`
   - 其他: 追加到 system prompt 尾部（`---\n## Skill: {id}\n{content}`）
4. 注入失败（文件不存在）→ 警告日志，不阻塞 session 创建。

#### RPC

```
skill/list   → []SkillInfo{ID, Name(first heading), Size, ModifiedAt}
skill/read   → {ID, Content}
skill/write  → {ID, Content} (create or update)
skill/delete → {ID}
```

#### App UI

- Settings → Skills 管理页（列表 + Markdown 编辑）
- Agent 创建 / Loop 创建 / Schedule 创建时，多选 skill 绑定

#### 验收标准

- [ ] 创建 skill 后，绑定该 skill 的 agent session 启动时 cwd 下出现对应文件
- [ ] Claude provider 使用 `.claude/skills/` 路径
- [ ] 不存在的 skill ID 不阻塞 agent 创建
- [ ] `skill/list` 返回按 ModifiedAt 倒序的列表

---

### 2.2 AgentProfile（Agent 身份化）— P1

#### 目标

用户可定义命名的 agent 配置 preset（"Frontend Expert"、"Go Backend"），一键复用。

#### 数据模型

```go
// protocol/message_profile.go
type AgentProfile struct {
    ID          string         `json:"id"`
    Name        string         `json:"name"`
    Avatar      string         `json:"avatar,omitempty"` // emoji
    Description string         `json:"description,omitempty"`
    Template    AgentTemplate  `json:"template"`
    Skills      []string       `json:"skills,omitempty"`
    Concurrency int            `json:"concurrency,omitempty"` // max parallel tasks, 0=unlimited
    CreatedAt   time.Time      `json:"createdAt"`
    UpdatedAt   time.Time      `json:"updatedAt"`
}
```

持久化：`~/.solo/profiles.json`（与 loops.json / schedules.json 同级）。

#### 行为

- Task/Loop/Schedule 的 AgentTemplate 字段可替换为 `ProfileID string`。
- 创建 agent 时 resolve：`ProfileID` → load profile → merge template + skills → 得到完整 AgentSessionConfig。
- Profile 不改变 AgentClient/AgentSession 接口 — 纯上层 preset。

#### RPC

```
profile/list   → []AgentProfile
profile/get    → AgentProfile
profile/create → AgentProfile
profile/update → AgentProfile
profile/delete → {ID}
```

#### App UI

- 新 tab 或 Settings 子页：Profile 列表（avatar + name + provider badge）
- 创建 agent / task 时：先选 profile（或 "custom"），再微调
- Workspace 中 agent tab 显示 profile avatar

#### 验收标准

- [ ] 创建 profile 后，通过 ProfileID 创建 agent，provider/model/systemPrompt/skills 均正确
- [ ] 删除 profile 不影响已运行的 agent session
- [ ] Profile 的 Skills 字段与 AgentSessionConfig.Skills 合并（profile skills + 显式 skills 取并集）

---

### 2.3 统一 Task 模型 — P2

#### 目标

Loop 和 Schedule 合并为 Task 的执行策略，用户只需理解一个概念。

#### 数据模型

```go
// protocol/message_task.go
type Task struct {
    ID          string        `json:"id"`
    Name        string        `json:"name"`
    Prompt      string        `json:"prompt"`
    Cwd         string        `json:"cwd"`
    ProfileID   string        `json:"profileId,omitempty"`
    Template    *AgentTemplate `json:"template,omitempty"` // overrides profile
    Skills      []string      `json:"skills,omitempty"`

    Strategy    TaskStrategy  `json:"strategy"`
    Status      TaskStatus    `json:"status"` // queued|running|succeeded|failed|stopped
    Runs        []TaskRun     `json:"runs"`
    CreatedAt   time.Time     `json:"createdAt"`
    UpdatedAt   time.Time     `json:"updatedAt"`
}

type TaskStrategy struct {
    Type     string          `json:"type"` // "manual" | "loop" | "schedule"
    Loop     *LoopStrategy   `json:"loop,omitempty"`
    Schedule *ScheduleStrategy `json:"schedule,omitempty"`
}

type LoopStrategy struct {
    MaxIterations int    `json:"maxIterations"`
    MaxTimeMs     int64  `json:"maxTimeMs,omitempty"`
    SleepMs       int    `json:"sleepMs,omitempty"`
    VerifyPrompt  string `json:"verifyPrompt,omitempty"`
    VerifyChecks  []string `json:"verifyChecks,omitempty"`
}

type ScheduleStrategy struct {
    Cron      string     `json:"cron,omitempty"`
    Interval  string     `json:"interval,omitempty"`
    MaxRuns   int        `json:"maxRuns,omitempty"`
    ExpiresAt *time.Time `json:"expiresAt,omitempty"`
}

type TaskRun struct {
    ID         string     `json:"id"`
    Status     string     `json:"status"`
    Iterations []Iteration `json:"iterations,omitempty"` // for loop strategy
    AgentIDs   []string   `json:"agentIds"`
    StartedAt  time.Time  `json:"startedAt"`
    FinishedAt *time.Time `json:"finishedAt,omitempty"`
    Error      string     `json:"error,omitempty"`
}
```

#### 引擎合并

- 现有 `loop.Engine` 和 `schedule.Executor` 合并为 `task.Engine`。
- `task.Engine` 内部按 strategy.Type 分派到 `runLoop()` / `runSchedule()` / `runManual()`。
- 共享：agent 创建（resolve profile → template → config）、run 记录、状态机、日志。

#### 兼容迁移

| 阶段 | 行为 |
|------|------|
| v0.12.x | 引入 Task RPC；Loop/Schedule RPC 保留，内部代理到 Task engine |
| v0.13.x | App 切换到 Task Board UI；Loop/Schedule screen 降级为 "advanced" |
| v0.14.x | 移除 Loop/Schedule 独立 RPC；旧数据自动迁移为 Task |

数据迁移：`loops.json` → Task(strategy=loop)，`schedules.json` → Task(strategy=schedule)。

#### RPC

```
task/create  → Task
task/list    → []Task (filter: status, strategy, profileId)
task/inspect → Task (full, with runs)
task/run     → {taskID} (manual trigger)
task/stop    → {taskID}
task/update  → Task
task/delete  → {taskID}
task/logs    → streaming log lines
```

#### 验收标准

- [ ] 现有 Loop 和 Schedule 数据无损迁移为 Task
- [ ] `task/list` 返回所有原 loop + schedule 记录
- [ ] 通过 `task/create` + strategy=loop 创建的 task 行为与原 loop/run 一致
- [ ] 旧 RPC（loop/run, schedule/create）在 v0.12.x 仍可用
- [ ] App Task Board 显示所有 task，按 status 分列

---

### 2.4 Task Board UI — P3

#### 目标

Dashboard 升级为全局 Task Board，替代独立的 Loops/Schedules screen。

#### 布局

```
┌─────────────────────────────────────────────────────┐
│ [+ New Task]          Filter: [All ▾] [Profile ▾]  │
├────────────┬────────────────┬───────────────────────┤
│ Queued (3) │ Running (2)    │ Completed (12)        │
│ ┌────────┐ │ ┌────────────┐ │ ┌──────────────────┐ │
│ │ Task A │ │ │ Task B     │ │ │ Task D           │ │
│ │ 🎨 FE  │ │ │ 🔧 BE     │ │ │ loop · 5 iters ✓ │ │
│ │ manual │ │ │ loop 3/5   │ │ │ 2h ago           │ │
│ └────────┘ │ │ ████░ 60%  │ │ └──────────────────┘ │
│ ┌────────┐ │ └────────────┘ │                       │
│ │ Task C │ │ ┌────────────┐ │                       │
│ │ 📋 QA  │ │ │ Task E     │ │                       │
│ │ cron   │ │ │ schedule   │ │                       │
│ └────────┘ │ │ next: 14:00│ │                       │
│            │ └────────────┘ │                       │
└────────────┴────────────────┴───────────────────────┘
```

#### 交互

- 点击 task card → Task Detail（runs 历史、实时 log、验证结果、关联 agent sessions）
- 长按 / 右键 → 快捷操作（run、stop、edit、duplicate）
- 下拉刷新 + WebSocket 实时状态推送（复用现有 timeline 机制）
- "Running" 列的 task card 可点击 "View Pane" 跳转到 tmux pane 实时视图

#### 与 Tmux 视图的关系

- Board = "what"（做什么、进度如何）
- Tmux Pane = "how"（agent 实际在终端里做什么）
- 两者通过 agent session ID 关联

---

### 2.5 Inbox（事件通知）— P4

#### 目标

Agent 完成/失败/阻塞时主动通知用户，无需轮询。

#### 数据模型

```go
type Notification struct {
    ID        string    `json:"id"`
    Type      string    `json:"type"` // task_succeeded|task_failed|agent_attention|verify_failed
    TaskID    string    `json:"taskId,omitempty"`
    AgentID   string    `json:"agentId,omitempty"`
    Title     string    `json:"title"`
    Body      string    `json:"body"`
    Read      bool      `json:"read"`
    CreatedAt time.Time `json:"createdAt"`
}
```

持久化：`~/.solo/notifications.json`（保留最近 200 条，FIFO 淘汰）。

#### 触发点

| 事件 | 通知 |
|------|------|
| Task run 成功 | "Task B completed (3 iterations)" |
| Task run 失败 | "Task B failed: timeout after 5min" |
| 验证不通过（loop） | "Task B iteration 3: verify failed — 2 checks red" |
| Agent 请求权限 | "Claude needs permission: write to /etc/hosts" |
| Schedule 触发 | "Daily triage started" (optional, off by default) |

#### RPC

```
notification/list  → []Notification (paginated, unread first)
notification/read  → {id} (mark read)
notification/read-all → {}
notification/unread-count → {count}
```

#### App UI

- Tab bar 新增 Inbox icon + badge（unread count）
- 列表页：按时间倒序，未读加粗，左滑 mark-read
- 点击 → 跳转到关联 Task Detail 或 Agent Session

#### Push 扩展

现有 Expo push 基础设施不变，payload 增加 `type` + `taskId` 字段，app 收到后 refetch notification/list。

---

### 2.6 Trigger 泛化（Autopilot 升级）— P5

#### 目标

Schedule 触发方式从 cron-only 扩展为 webhook + task-chain。

#### 数据模型变更

```go
type ScheduleStrategy struct {
    Triggers []Trigger  `json:"triggers"` // 替代单一 Cron/Interval
    MaxRuns  int        `json:"maxRuns,omitempty"`
    ExpiresAt *time.Time `json:"expiresAt,omitempty"`
}

type Trigger struct {
    Type  string `json:"type"`  // "cron" | "interval" | "webhook" | "on_task_complete"
    Cron  string `json:"cron,omitempty"`
    Every string `json:"every,omitempty"`
    Event string `json:"event,omitempty"` // for on_task_complete: "task:succeeded:{taskID-glob}"
}
```

#### Webhook 触发

- Daemon 在现有 HTTP server（:17612）上增加 `POST /api/trigger/{taskID}`。
- 请求体可选（作为 prompt 附加 context）。
- 鉴权：`Authorization: Bearer {daemon-local-token}`（复用现有 API token）。
- 触发后等同于 `task/run`。

#### Task Chain（on_task_complete）

- Task A 完成后，engine 检查是否有其他 task 的 trigger 匹配 `task:succeeded:{A.ID}`。
- 匹配 → 将 B 加入执行队列。
- 防环：chain 深度上限 10；已 running 的 task 不重复触发。

#### 并发策略

```go
type ConcurrencyPolicy string // "skip" | "queue" | "replace"
```

- `skip`: 上一次 run 未完成则跳过本次触发
- `queue`: 排队等待
- `replace`: 停止当前 run，启动新 run

---

## 3. Architecture Impact Summary

| 变更 | 影响层 | 风险 |
|------|--------|------|
| Skills 注入 | daemon/execenv | 低 — 新增 inject 步骤，不改现有流程 |
| AgentProfile | protocol + daemon/store | 低 — 新增 profiles.json，resolve 在创建时 |
| 统一 Task | protocol + daemon engine + app-bridge | 中 — Loop/Schedule engine 合并，需兼容迁移 |
| Task Board | app (frontend) | 低 — 消费新 RPC，不改后端 |
| Inbox | daemon + app + push | 低 — 新增独立模块 |
| Trigger 泛化 | daemon/scheduler + daemon/server | 中 — executor 扩展，webhook 增加 HTTP surface |

---

## 4. Implementation Priority

| Phase | Feature | 预估工期 | 前置依赖 |
|-------|---------|----------|----------|
| P0 | Skills | 3-4 days | None |
| P1 | AgentProfile | 2-3 days | Skills (optional) |
| P2 | 统一 Task 模型 | 5-7 days | P1 (profile resolve) |
| P3 | Task Board UI | 4-5 days | P2 (task RPC) |
| P4 | Inbox | 2-3 days | P2 (task events) |
| P5 | Trigger 泛化 | 3-4 days | P2 (task engine) |

P0-P1 独立交付，不改现有 RPC。P2 是架构核心，需单独 ADR。P3-P5 可在 P2 之后并行。

---

## 5. Success Metrics

| 指标 | 目标 |
|------|------|
| Agent 首次产出可用率（绑定 skill vs 未绑定） | +30% |
| 用户创建 agent 的平均配置时间（profile vs manual） | -70% |
| 日活 task 数（board 可见性带来的使用频率） | +50% |
| 用户主动轮询 agent 状态的频率（inbox 引入后） | -60% |

---

## 6. Open Questions

1. **Skill 版本控制**：是否需要 skill 历史版本？还是 git 管理 `~/.solo/skills/` 即可？
2. **Profile 共享**：未来是否支持 profile 导出/导入（跨设备）？
3. **Task 优先级**：多 task 同时 due 时的执行顺序？FIFO 还是用户可设优先级？
4. **Board 持久化视图**：用户自定义 filter/排序是否需要持久化？
5. **Webhook 安全**：daemon 暴露在 localhost，是否需要额外 HMAC 签名？
