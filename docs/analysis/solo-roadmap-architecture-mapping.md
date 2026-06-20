# Solo 现有功能与 2026 路线图演进的架构映射与设计方案

> **Date**: 2026-06-20
> **Status**: Analysis Complete
> **Priority**: High
> **Author**: AI Coding Assistant
> **关联文档**：
> - [Solo 2026 产品/技术路线图](../product/roadmap-2026.md)
> - [Provider Hub / CC-Switch Migration Design](../product/agent-profile-switch-export-design.md)
> - [Loop Schedule Design](../product/loop-schedule-design.md)
> - [Architecture First-Principles Review](architecture-first-principles-review-2026-06-18.md)
> - [Components](../architecture/components.md)

---

## 执行摘要

Solo 当前架构（v0.6.3）已经具备支撑 2026 路线图的三大支柱所需的**大部分基础设施**，但实现形态与路线图目标之间存在“能力初现、接口未统一”的差距：

| 路线图支柱 | 现状 | 主要缺口 |
|-----------|------|---------|
| **Provider Hub** | ProviderRegistry、CustomModels、MCP 注入已存在 | 缺少上游 Provider 抽象、Local API Proxy、Config Exporter、MCP 工具目录与 attestation |
| **Loop Schedule** | Schedule Store/Executor/Runner 已存在；Loop 模块已实现但独立 | 需要把 Loop 合并进 Schedule 的 `type:"loop"` 路径，补齐 LLM Controller 与 Step Executor |
| **Project Memory + Chat** | Session Memory、Workspace/Project Registry、AGENTS.md 已存在；Chat RPC 已定义 | 缺少项目级记忆索引、Chat UI、多 Agent 协调器 |

**核心结论**：Solo 不需要推倒重来，而是应该在现有模块边界上“长出新层”——在 `ProviderRegistry` 之上加 `providerhub`，在 `schedule.Store` 之内加 `type:"loop"`，在 `memory.Bridge` 之上加 `projectmemory.Indexer`，在 `app-bridge` Chat RPC 之上补 UI 与 Daemon 协调器。

---

## 1. 现状盘点：现有 Solo 能力如何支撑演进

### 1.1 Agent / Provider 层：天然的中枢底座

Solo 的 `daemon/internal/agent/` 已经实现了高度抽象的 provider 插件架构：

- **`AgentClient` 接口**（`manager.go:18`）：任何 coding agent 只需实现 Provider、IsAvailable、CreateSession、ListModels 等 8 个方法即可接入。
- **`AgentSession` 接口**（`manager.go:43`）：统一了 Run、StartTurn、Interrupt、SetModel/SetMode 等运行时语义。
- **`ProviderRegistry`**（`provider_registry.go`）：集中管理内置 provider 与 `CustomModels` 扩展。
- **`TurnGuard` / `StallMonitor`**：已经具备 agent 运行时的稳定性治理能力。
- **MCP 注入**：`AgentSessionConfig.McpServers` 与 OpenCode/Claude 的 MCP 传递已实现。

这意味着 **Provider Hub 的上层 provider 管理只需落在 `ProviderRegistry` 之前做一次“解析”**，不需要重写 agent 运行时。

### 1.2 Schedule / Loop 层：两套实现需要归一

- **`schedule.Store` + `Executor` + `Runner`**：已经实现 cron/every 触发、JSON 持久化、原子保存、执行历史记录。
- **`daemonRunner`**：已经会针对 schedule 创建 agent 或复用 agent 并发送 prompt。
- **独立的 `loop.Store/Engine`**：v0.6.3 已经实现 worker/verifier loop，但用的是独立存储和 UI，与 schedule 是并列关系。

路线图中理想的形态是 **Loop 作为 Schedule 的高级类型**（`type: "loop"`）。因此需要把独立 loop 模块的能力迁移到 schedule runner 体系内。

### 1.3 Memory / Workspace 层：项目记忆的骨架已就绪

- **`memory.TurnRecorder` + `Bridge`**：会话级 turn 记录成熟，支持 redaction、seq/parent 链、streaming chunk 合并。
- **`workspace.ProjectRegistry` / `WorkspaceRegistry`**：项目与工作区注册表已存在，能把 `cwd` 映射到 `ProjectID`。
- **AGENTS.md**：Solo 项目自身已经在用 skill 系统，但 daemon 还没有自动读取项目 AGENTS.md 并注入 agent 的机制。

项目记忆可以**在现有 session memory 之上加索引层**，而不是替换存储格式。

### 1.4 App / App-Bridge 层：UI 模式可复用

- **跨主机 dashboard 模式**：`schedules-dashboard-screen.tsx` 和 `tmux-dashboard-screen.tsx` 已经展示了如何聚合多 daemon 状态。
- **React Query + `useQueries` 模式**：schedule、loop 都已采用 server-state-first，不需要新增 Zustand store。
- **Chat RPC**：`app-bridge/src/server/chat/` 已经定义了完整 RPC schema 和 `DaemonClient` 方法，但 daemon 后端与 UI 尚未实现。

---

## 2. 架构映射：路线图支柱 → 现有模块 → 缺口

### 2.1 Provider Hub 映射

| 路线图能力 | 现有模块/能力 | 缺口 |
|-----------|--------------|------|
| 统一管理 Provider 预设 | `ProviderRegistry` + `CustomModels` | 缺少上游 API Provider 抽象（baseURL、authType、protocol、failover） |
| 本地 API 代理 `:17613` | 无 | 需要新建 HTTP proxy，协议转换 anthropic-messages ↔ openai-chat |
| MCP 统一管理 | `McpServerConfig` + `MCPInjectIntoAgents` | 缺少 MCP Server Cards 发现、Capability Attestation、按 agent 同步 |
| Skills / Prompts 管理 | 项目自身 `.agents/skills/` | 缺少 hub 级注册表与 exporter |
| 配置转写到外部 agent | 无 | 需要 `Exporter` 接口 + Claude/Codex/Cursor 实现 |
| 用量/成本监控 | `AgentUsage` 字段、Relay metrics | 缺少统一 UsageTracker 与 Cost Dashboard |
| 智能 Provider 路由 | CLI `ResolveProviderModel`（未真正使用 snapshot） | 需要 `Router.Resolve(agent, intent)` |
| Open Responses 兼容层 | 无 | 需要在 Local API Proxy 上加 `/responses` 兼容 |
| 本地 / BYOK 模型 | 无 | 需要 Ollama/vLLM ProviderClient |

### 2.2 Loop Schedule 映射

| 路线图能力 | 现有模块/能力 | 缺口 |
|-----------|--------------|------|
| `type: "loop"` 支持 | `StoredSchedule` 缺少 Type 字段 | 需要扩展 protocol |
| LLM 决策控制器 | `AgentManager` 可调用任何 provider | 需要 `LoopController` 封装 loop context 与 function calling |
| Step Executor | `daemonRunner` 只能跑 agent prompt | 需要抽象 `bash/test/read/write/git/ask-user` step |
| 状态机与持久化 | `schedule.Store` 已具备 | 需要在 `StoredSchedule` 内嵌 loop state |
| 崩溃恢复 | `schedule.Store` 启动时 `fixupNextRunAt` | 需要 loop 启动时扫描未完成的 loop |
| 人工确认门控 | `PermissionManager` 已用于 agent 权限 | 需要扩展到 loop step 级别 |
| 长时任务支持 | Push 通知、E2EE Relay 已存在 | 需要 step 级 checkpoint 与断线续传 |

### 2.3 Project Memory + Chat 映射

| 路线图能力 | 现有模块/能力 | 缺口 |
|-----------|--------------|------|
| 项目级记忆 | `TurnRecorder` + `ProjectRegistry` | 缺少项目索引与归因 |
| 知识图谱 | 无 | 需要代码结构解析与关系存储 |
| 多 Agent 协作 | `AgentManager` 可管理多个 agent | 缺少 Coordinator 与 A2A Agent Card 注册表 |
| Chat 系统 | `app-bridge` Chat RPC 已定义 | 缺少 daemon backend、UI、hooks |
| AGENTS.md 原生支持 | 项目自身使用 AGENTS.md | 缺少 workspace 启动时扫描与注入 |
| 自动 Onboarding | `workspace.config_schema.go` 读 `solo.json` | 缺少首次打开项目时代码地图生成 |

---

## 3. 架构设计方案

### 3.1 总体设计原则

1. **层内自治，层间薄接口**：Provider Hub、Loop、Project Memory 各自是独立模块，通过稳定的 Go 接口与 protocol 消息与现有 daemon 交互。
2. **协议优先**：任何新功能先定义 `protocol/` 消息类型，再实现后端，最后做 UI。
3. **复用持久化范式**：沿用 Solo 现有的原子 temp+rename JSON / markdown 模式。
4. **本地优先不变**：所有敏感配置、记忆、审计默认只存本地。
5. **A2A-ready，MCP-native**：多 Agent 协作基于 A2A 语义设计，工具层以 MCP 为中心。

### 3.2 Provider Hub 架构设计

#### 3.2.1 模块位置

```
daemon/internal/providerhub/
├── registry.go          # 上游 Provider 预设管理
├── router.go            # 智能路由 Resolve(agent, intent)
├── proxy.go             # Local API Proxy (:17613)
├── exporter.go          # Exporter 接口
├── exporters/           # Claude/Codex/Cursor/OpenCode/Aider/Continue
├── mcp_catalog.go       # MCP Server Cards + attestation
├── usage_tracker.go     # token/成本记录
└── preset.go            # 内置 provider 预设
```

#### 3.2.2 核心类型

```go
// UpstreamProvider 是上游模型/API 供应商的统一抽象。
type UpstreamProvider struct {
    ID           string
    Type         string // official | third-party | local
    TargetAgents []string
    API          APIConfig
    Models       []ModelConfig
    Routing      RoutingConfig
    Cost         CostConfig
    Metadata     ProviderMetadata
}

type APIConfig struct {
    BaseURL         string
    AuthType        string // bearer | api-key-header | oauth | none
    APIKeyRef       string // env:KEY | file:path
    Protocol        string // anthropic-messages | openai-chat | opencode | local
    CustomHeaders   map[string]string
    FullURLEndpoint bool
}

type Router struct {
    registry *Registry
    fallback map[string][]string // agent -> ordered provider IDs
}

func (r *Router) Resolve(agent string, intent RoutingIntent) (*ResolvedProvider, error)
```

#### 3.2.3 与现有模块集成

```
CreateAgentRequest
    │
    ▼
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│ Provider Hub    │───▶│ ProviderRegistry │───▶│ AgentManager    │
│ Router.Resolve  │    │ Get(provider)    │    │ CreateSession   │
└─────────────────┘    └──────────────────┘    └─────────────────┘
    │
    ▼
┌─────────────────┐
│ Local API Proxy │───▶ External agents (Claude/Codex/Cursor)
│ (:17613)        │
└─────────────────┘
    │
    ▼
┌─────────────────┐
│ Config Exporter │───▶ ~/.claude/settings.json, ~/.codex/config.json, ...
└─────────────────┘
```

**关键集成点**：

- **Agent 创建路径**：`Session.handleCreateAgent` 在调用 `AgentManager.CreateAgent` 之前，先调用 `providerhub.Router.Resolve()` 把用户指定的 provider/model 解析为实际上游 provider 与模型。这样用户可以说“用最快的模型”而不是硬编码 provider。
- **Provider Snapshot**：`ProviderRegistry.ToProviderSnapshotEntries()` 合并 `providerhub.Registry` 中启用的上游 provider，让 App 的 provider selector 立刻看到 hub 管理的新 provider。
- **Config 持久化**：`~/.solo/config.json` 的 `daemon.providers` 字段扩展为 `ProvidersConfig`，包含 `Hub *ProviderHubConfig`；`provider-hub/` 目录也保存独立 JSON 文件，便于版本控制。
- **Local API Proxy**：独立 HTTP server 监听 `127.0.0.1:17613`，根据请求 header/path/model 识别目标 agent 与 protocol，做协议转换、failover、用量拦截。
- **用量拦截**：Proxy 层解析响应中的 token 用量；Solo 内部 agent 在 `AgentSession` 层 hook；统一写入 `UsageTracker`。

#### 3.2.4 MCP 工具层设计

```go
type MCPServerRecord struct {
    ID          string
    Source      string // builtin | registry | local
    ServerCard  *mcp.ServerCard
    ManifestSHA string
    AllowedAgents []string
    RiskLevel   string // low | medium | high
}

type MCPCatalog struct {
    registry map[string]*MCPServerRecord
}

func (c *MCPCatalog) Attest(serverCard *mcp.ServerCard) error
```

- **Server Cards 发现**：从 `~/.well-known/mcp-server-card` 或官方 registry 拉取能力清单。
- **Capability Attestation**：校验 manifest 中的工具列表、权限声明；对高权限工具（文件删除、网络、代码执行）自动标记风险。
- **跨 agent 同步**：当用户启用一个 MCP server 时，调用对应 `Exporter.ExportMCP()` 写入各外部 agent 的 MCP 配置文件。

### 3.3 Loop Schedule 架构设计

#### 3.3.1 推荐策略：把 Loop 作为 Schedule 的高级类型

路线图中明确要求扩展 `StoredSchedule.Type` 支持 `"loop"`。相比当前独立的 `loop.Store/Engine`，这样做的好处：

- 单一持久化层、单一 dashboard、单一 CLI/CLI 语义。
- 可以自然支持“每晚触发一次 fix-tests loop”这类 Schedule + Loop 混合场景。
- 复用 `schedule.Executor` 的 tick、恢复、暂停/恢复机制。

#### 3.3.2 模块位置

```
daemon/internal/loop/
├── loop.go              # LoopRecord 类型与状态机
├── controller.go        # LLM 决策控制器
├── engine.go            # 循环执行引擎
├── step.go              # Step 定义与执行器注册表
├── steps/               # agent_step.go, bash_step.go, test_step.go, ...
├── runner.go            # 实现 schedule.Runner 接口
├── human_confirm.go     # 人工确认门控
└── context.go           # LoopContext 构建
```

#### 3.3.3 核心类型

```go
// 扩展 protocol.StoredSchedule
type StoredSchedule struct {
    // 现有字段...
    Type            string                `json:"type,omitempty"` // "schedule" | "loop"
    Goal            string                `json:"goal,omitempty"`
    Controller      *LoopControllerConfig `json:"controller,omitempty"`
    Tools           []string              `json:"tools,omitempty"`
    CurrentIteration int                  `json:"currentIteration,omitempty"`
    Steps           []LoopStep            `json:"steps,omitempty"`
}

type LoopControllerConfig struct {
    Provider         string
    Model            *string
    ModeID           *string
    SystemPrompt     string
    MaxIterations    int
    PauseBetweenIterationsMs int
}

type LoopStep struct {
    ID        string
    Type      string // agent | bash | test | read | write | git | ask-user | wait | terminate
    Input     map[string]interface{}
    Output    *string
    Status    string // pending | running | succeeded | failed | skipped
    StartedAt *time.Time
    EndedAt   *time.Time
    Error     *string
}
```

#### 3.3.4 状态机

```
pending ──▶ running ──▶ evaluating(LLM) ──┬──▶ next_step ──┐
                                          │                │
                                          ├──▶ human_confirm ──▶ paused ──▶ running
                                          │
                                          ├──▶ completed
                                          │
                                          └──▶ failed
```

#### 3.3.5 执行器注册表

```go
type StepExecutor interface {
    Name() string
    Execute(ctx context.Context, step LoopStep, env *LoopEnv) (LoopStepResult, error)
}

type StepRegistry struct {
    executors map[string]StepExecutor
}
```

各 step 复用现有模块：

| Step | 复用模块 |
|------|---------|
| `agent` | `AgentManager.CreateAgent` / `SendAgentMessage` |
| `bash` | `terminal` PTY |
| `test` | `terminal` + test output parser |
| `read` / `write` | `workspace` file ops |
| `git` | `workspace.GitService` |
| `ask-user` | WebSocket push / Push 通知 |

#### 3.3.6 与现有 Loop 模块的兼容

当前 `daemon/internal/loop/` 已经存在。建议：

1. **Phase 1**：保留现有 loop UI/RPC 不变，仅在其内部把持久化目标从 `~/.solo/loops.json` 改为写入 `schedule.Store` 的 `type:"loop"` 记录。
2. **Phase 2**：用新的 `loop.Runner` 替换现有 `loop.Engine` 的执行路径。
3. **Phase 3**：移除 `loop.Store`，统一使用 `schedule.Store`。

### 3.4 Project Memory 架构设计

#### 3.4.1 设计原则

- **不替换 session memory**：继续保留 `~/.solo/memory/sessions/` 的 per-turn markdown 文件。
- **加索引层**：在 session memory 之上建立项目级索引，避免双写。
- **项目归因**：通过 `cwd` 或 `WorkspaceID` 把 session/turn 关联到 `ProjectID`。

#### 3.4.2 模块位置

```
daemon/internal/projectmemory/
├── indexer.go           # 监听 Bridge 事件，建立项目索引
├── query.go             # 检索接口
├── onboarding.go        # 项目首次打开时代码地图生成
├── facts.go             # 事实/ADR 提取
├── knowledge_graph.go   # 代码结构图谱
└── types.go             # ProjectMemory, ProjectFact, QueryResult
```

#### 3.4.3 数据布局

```
~/.solo/memory/
├── sessions.jsonl                    # 现有
├── sessions/{date}/{sessionID}/...   # 现有
└── projects/{projectID}/
    ├── index.jsonl                   # session → project 映射
    ├── facts.jsonl                   # 提取的事实/ADR
    ├── map.md                        # 代码地图
    ├── dependencies.json             # 依赖图谱
    └── embeddings/                   # 可选：向量索引
```

#### 3.4.4 索引流程

```
memory.Bridge.OnAssistantTurnEnd
    │
    ▼
projectmemory.Indexer
    │
    ├── ResolveProject(sessionID, cwd) ──▶ workspace.ProjectRegistry
    │
    ├── Append to ~/.solo/memory/projects/{projectID}/index.jsonl
    │
    ├── Extract facts / ADR / decisions ──▶ facts.jsonl
    │
    └── Update map.md (async, batched)
```

#### 3.4.5 AGENTS.md 注入

在 `AgentManager.CreateAgent` 或 `Session.handleCreateAgent` 中：

1. 解析 `cwd` 到项目根目录。
2. 查找 `{projectRoot}/AGENTS.md` 和 `{projectRoot}/CLAUDE.md`。
3. 若存在，读取内容并追加到 `AgentSessionConfig.SystemPrompt` 或 `Extra["agentsMd"]``。
4. 对 Project Memory 中的 `facts.jsonl` 也可选择性注入最近 N 条。

### 3.5 Chat / 多 Agent 协作架构设计

#### 3.5.1 现状

- `app-bridge/src/server/chat/` 已定义完整 RPC schema。
- Daemon 端需要新增 chat room 生命周期管理与消息路由。
- UI 完全空白。

#### 3.5.2 Daemon 端设计

```
daemon/internal/chat/
├── coordinator.go       # ChatRoom 管理、消息路由
├── room.go              # ChatRoom 状态
├── participant.go       # AgentParticipant / UserParticipant
├── message.go           # ChatMessage
└── a2a_registry.go      # 外部 Agent Card 注册表
```

```go
type ChatRoom struct {
    ID           string
    ProjectID    string
    Coordinator  *CoordinatorAgent
    Specialists  map[string]*SpecialistAgent
    Messages     []ChatMessage
    Status       string
}

type CoordinatorAgent struct {
    AgentID  string
    Provider string
    Model    string
}
```

#### 3.5.3 多 Agent 协作流程

```
用户发送任务到 ChatRoom
    │
    ▼
CoordinatorAgent (Claude/Opus)
    │
    ├── 分解子任务 ──▶ SpecialistAgent (Kimi 审查 / OpenCode 实现 / ...)
    │
    ├── 通过 A2A Task 委托给外部 agent
    │
    └── 汇总结果 ──▶ 用户
```

#### 3.5.4 A2A-ready 设计

- 在 `providerhub` 或 `chat` 模块中维护 `AgentCardRegistry`。
- Solo Agent 可以发布自己的 Agent Card（能力、endpoint、签名）。
- 委托任务时构造 A2A Task，支持同步/异步、状态回调。
- 初期先支持本地/自签名 Agent Card，生产级 Signed Agent Cards 待成熟后启用。

#### 3.5.5 App UI 设计

- 路由：
  - `/chats` — 跨主机 chat 列表（可选）
  - `/h/[serverId]/chats` —  per-host chat 列表
  - `/h/[serverId]/chats/[roomId]` — chat room 详情
- Hooks：`useChatRooms`, `useChatRoom`, `useChatMessages`, `usePostChatMessage`
- 组件：`ChatMessageList`, `ChatInput`, `AgentParticipantCard`
- 模式：复用 schedule/loop dashboard 的 cross-host aggregation 模式。

---

## 4. 数据流与接口契约

### 4.1 Agent 创建数据流（Provider Hub 介入后）

```
App / CLI
    │ create_agent_request {provider:"smart", model:"fast", cwd:"..."}
    ▼
Session.handleCreateAgent
    │
    ▼
providerhub.Router.Resolve("smart", RoutingIntent{TaskType:"code", CostBudget:"low"})
    │
    ▼ 返回 ResolvedProvider {Provider:"kimi", Model:"k2-lite", Upstream:"siliconflow"}
    │
    ▼
AgentManager.CreateAgent(AgentSessionConfig{Provider:"kimi", Model:"k2-lite", ...})
    │
    ▼
ProviderRegistry.Get("kimi") ──▶ AgentSession
```

### 4.2 Loop Schedule 数据流

```
schedule.Executor.tick
    │
    ▼ 发现 due schedule with Type:"loop"
    │
    ▼
loop.Runner.Run(schedule)
    │
    ▼
loop.Engine.Start()
    │
    ├── Load state from StoredSchedule.Steps/CurrentIteration
    │
    ├── LoopController.Decide(LoopContext) ──▶ LLM
    │
    ├── StepRegistry.Execute(nextStep)
    │
    ├── Save state to schedule.Store
    │
    └── Repeat until completed/failed/human_confirm/maxIterations
```

### 4.3 Project Memory 数据流

```
AgentSession 产生 turn
    │
    ▼
memory.Bridge.OnAssistantTurnEnd
    │
    ▼
projectmemory.Indexer
    │
    ├── 解析 cwd → ProjectID
    │
    ├── 更新 project index
    │
    └── 异步提取 facts/ADR
    │
    ▼
AgentManager.CreateAgent 读取 AGENTS.md + project memory 注入 system prompt
```

---

## 5. 实施顺序建议

### Phase 1：Provider Hub 基础（Q3 2026，4 周）

1. 扩展 `protocol/`：`UpstreamProvider`, `ResolvedProvider`, `ProviderHubConfig` 消息类型。
2. 新建 `daemon/internal/providerhub/registry.go` + `router.go`。
3. 扩展 `daemon/internal/config/config.go`：持久化 provider-hub 配置。
4. 在 `Session.handleCreateAgent` 中接入 `Router.Resolve`（可选启用）。
5. CLI：`solo provider ls/add/edit/enable/disable`。
6. App：`ProviderHubSection` 或 `/provider-hub` 路由。

### Phase 2：Loop Schedule 归一（Q3-Q4 2026，3 周）

1. 扩展 `protocol/message_schedule.go`：`StoredSchedule.Type`, `LoopControllerConfig`, `LoopStep`。
2. 新建 `daemon/internal/loop/runner.go` 实现 `schedule.Runner`。
3. 实现 `LoopController` + `StepExecutor`（agent/bash/test/read/write）。
4. 在 `schedule.Executor` 中根据 `Type` 分发 runner。
5. 迁移现有 `loop.Store` 数据到 `schedule.Store`。
6. App：把现有 loop UI 升级为 Loop Dashboard，支持 step timeline。

### Phase 3：MCP 工具层强化（Q4 2026，2 周）

1. 实现 MCP Server Cards 发现。
2. 实现 Capability Attestation 与风险标记。
3. 实现 `ConfigExporter.ExportMCP`。
4. 内置 5+ 常用 MCP server 预设。

### Phase 4：Project Memory Phase 1（Q4 2026，2 周）

1. 扩展 `TurnMetadata` 支持 `ProjectID`/`WorkspaceID`。
2. 新建 `projectmemory.Indexer`，监听 Bridge 事件。
3. 实现 AGENTS.md 自动读取与注入。
4. 实现项目 onboarding 代码地图生成。

### Phase 5：Chat / 多 Agent（Q1 2027，3 周）

1. Daemon 端实现 `chat.Coordinator` 与 chat room 生命周期。
2. 实现 Coordinator + Specialist 模式。
3. App 端实现 chat routes、hooks、screens。
4. A2A Agent Card 注册表基础版。

### Phase 6：Local API Proxy + Open Responses（Q4 2026-Q1 2027，2 周）

1. 新建 `providerhub.LocalAPIProxy` HTTP server on `:17613`。
2. 实现 anthropic-messages ↔ openai-chat 协议转换。
3. 实现 failover 与用量拦截。
4. 添加 Open Responses 兼容层。

---

## 6. 关键风险与缓解

| 风险 | 影响 | 缓解 |
|------|------|------|
| Provider Hub 增加 daemon 启动复杂度 | 中 | Hub 配置懒加载；CLI 提供 `solo providerhub init`。 |
| Loop 合并进 Schedule 导致现有 loop 数据迁移失败 | 高 | 写一次性迁移脚本；保留旧 `loops.json` 备份。 |
| Router 决策错误导致用户被路由到不合适的模型 | 高 | 默认关闭智能路由；提供显式 `provider:smart` 触发；人工可覆盖。 |
| Project Memory 索引拖慢 daemon | 中 | 索引异步化；大项目使用 batched/incremental；提供关闭开关。 |
| A2A / Agent Card 信任模型不成熟 | 中 | 先本地/自签名；Signed Agent Cards 默认关闭。 |
| MCP Capability Attestation 误报导致工具不可用 | 中 | 风险分级而非二进制拦截；用户可手动信任。 |
| 跨语言类型漂移（protocol ↔ app-bridge） | 中 | 新消息类型同步更新 `app-bridge/src/shared/messages.ts`；增加 schema 校验。 |

---

## 7. 与现有架构评审结论的对照

[Architecture First-Principles Review](architecture-first-principles-review-2026-06-18.md) 提出了三个长期风险：

1. **Daemon bloat**：本方案通过新增 `providerhub`、`loop`、`projectmemory`、`chat` 模块，表面上看增加了 daemon 职责，但每个模块都有清晰的接口边界和独立持久化。建议在实现 Phase 2 后评估是否把 `providerhub.LocalAPIProxy` 拆分为独立进程。
2. **Cross-language type drift**：本方案强调“协议优先”，所有新类型先在 `protocol/` 定义，再手动镜像到 `app-bridge/src/shared/messages.ts`。如果消息表面快速增长，应考虑 code generation。
3. **Tmux coupling**：Provider Hub + A2A 的发展方向将逐步减少对 tmux 观测的依赖，鼓励通过标准协议（ACP/Cursor Agent API）与 agent 交互。

---

## 8. 结论

Solo 的现有架构已经为 2026 路线图奠定了坚实基础：

- **Agent / Provider 层**可以直接承载 Provider Hub 的上游 provider 解析与路由。
- **Schedule 层**可以通过扩展 `type:"loop"` 自然吸收 Loop 能力。
- **Memory / Workspace 层**可以通过加索引层实现 Project Memory。
- **App-Bridge Chat RPC** 已经为 Chat / 多 Agent UI 铺好协议通道。

最关键的架构决策是：**不要另起炉灶，要在现有模块边界上长出新层**。具体而言：

1. Provider Hub 放在 `ProviderRegistry` 之前做解析。
2. Loop 放在 `schedule.Store` 之内作为新类型。
3. Project Memory 放在 `memory.Bridge` 之上做索引。
4. Chat 放在现有 RPC 之上补后端与 UI。

按 Phase 1→6 的顺序推进，可以在不破坏现有功能的前提下，逐步实现路线图目标。

---

## 相关文件

- `daemon/internal/agent/manager.go` — `AgentClient` / `AgentSession` 接口
- `daemon/internal/agent/provider_registry.go` — Provider 注册与 snapshot
- `daemon/internal/schedule/store.go` — Schedule 持久化
- `daemon/internal/loop/` — 现有 Loop 实现
- `daemon/internal/memory/bridge/bridge.go` — Session Memory Bridge
- `daemon/internal/workspace/project.go` — Project Registry
- `protocol/message_schedule.go` — Schedule 协议类型
- `protocol/message_common.go` — `AgentSessionConfig`, `McpServerConfig`
- `app-bridge/src/server/chat/chat-rpc-schemas.ts` — Chat RPC schema
- `app-bridge/src/server/schedule/rpc-schemas.ts` — Schedule RPC schema
- `app/src/screens/schedules/schedules-dashboard-screen.tsx` — Dashboard UI 模式
