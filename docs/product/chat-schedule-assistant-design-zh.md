# 基于对话的日程助手 — 产品设计

> **文档类型**：产品 / 架构设计
> **日期**：2026-07-17
> **最后修订**：2026-07-18 — NL 解析从临时 agent 工具链切换为主机配置的 LLM 提供商（设置 → 常规）；日程执行通过 Target Agent 确认保持不变。
> **基准版本**：Solo v0.10.0
> **状态**：已实现（2026-07-18）— protocol、daemon、app-bridge、app UI、E2E 规格已落地；测试面见 §10。
> **受众**：后端、前端、产品
> **相关文档**：
> - [Loop 日程实现规范](loop-schedule-spec.md)
> - [App-Bridge 日程模块](../analysis/app-bridge-schedule-module.md)
> - [创建日程流程](../analysis/create-schedule-flow.md)
> - [ADR-001：共享 Agent 模板](../decisions/adr-001-shared-agent-template-for-loop-and-schedule.md)

---

## 执行摘要

新增一个**日程助手**：位于日程区域内的一个对话面板，用户可以用自然语言创建和编辑日程（如"每个工作日 9:00，汇总夜间 agent 活动"），由用户在**设置 → 常规 → LLM 提供商**中配置的开源自兼容 LLM 提供商驱动。

三个已确认的锁定决策（已与产品负责人确认）：

1. **日程区域内的专用助手面板** — 而非现有 agent 对话内的日程工具。助手作用域明确、行为可预测、安全可靠；agent 对话工具注入可作为后续独立阶段（不在本文范围内）。
2. **通过守护进程的配置 LLM 提供商解析** — 守护进程通过无状态聊天补全调用，使用主机默认 LLM 提供商（从 `config.llmProviders` 解析：第一个启用的提供商；其 `isDefault` 模型，否则为第一个模型）。不创建 agent 会话，不涉及 CLI 工具链，不引入新凭证 — 复用现有 LLM 提供商设置后端，目前该后端无运行时消费者。
3. **执行保持不变 — Target Agent** — 确认的提案通过现有、已验证的 `schedule/create|update|pause|resume|delete` RPC 流转，执行时在触发时刻保持当前运行器行为：目标类型 `agent` 消息发送给运行中的 agent；`new-agent`/`provider` 通过 AgentManager 创建临时 agent。LLM 仅*提议* `ScheduleTarget`，从不触碰执行路径。

核心安全不变量：**LLM 永不修改日程**。它仅生成*提案*；用户通过结构化卡片确认，确认流程通过现有日程 RPC 流转。仅新增**一对 RPC**（`schedule/assist`）。

---

## 1. 问题与目标

### 1.1 问题

- 当前日程创建需要填写表单：提示文本、周期类型、cron 表达式或间隔、目标 agent。Cron 语法是主要摩擦点 — 大多数用户无法编写 `0 9 * * 1-5`，也不应该需要。
- 编辑意味着重新打开表单并重新输入所有字段（`schedule/update` 是全量替换）。
- 日程按主机划分且数量通常较多；小幅调整（"将每日报告移至 7:30"、"暂停此主机上的所有日程"）需要在列表中查找。

### 1.2 目标

| # | 目标 | 成功指标 |
|---|------|----------|
| G1 | 用一句自然语言创建日程 | ≥80% 的常见请求在首次尝试中生成正确提案（通过存根端点 + 真实提供商评估集） |
| G2 | 通过名称引用编辑/暂停/恢复/删除现有日程 | 无歧义名称的正确日程解析；为歧义情况澄清循环 |
| G3 | 零意外修改 | 每次修改都经过显式确认卡片；无自动应用路径 |
| G4 | 提供商无关 | 功能与 LLM 提供商中配置的任何开源自兼容端点兼容；无特定提供商的代码或 UI |
| G5 | 不对现有日程栈造成回归 | 所有现有日程 RPC、UI、时区管道和 Target Agent 执行保持不变并复用 |

### 1.3 非目标（v1）

- 通过聊天创建 Loop 类型日程（`type: "loop"`）— 等待 Loop 日程规范落地。
- 常规 agent 对话会话中的日程工具（未来阶段）。
- 守护进程端对话持久化（对话记录保存在应用中）。
- 面板内 LLM 提供商/模型切换 — v1 始终使用主机默认值（每次请求覆盖为 v1.1 候选，见 §13）。
- 管理 LLM 提供商配置 — 复用现有的设置 → 常规 → LLM 提供商 UI 和守护进程配置后端。
- 多步自主日程管理（助手仅提议；从不链式执行修改）。

---

## 2. 用户故事与流程

### 2.1 创建

```
用户："每个工作日早上 9 点，让后端 agent 汇总夜间测试运行"
助手：[提案卡片]
      CREATE  "夜间测试汇总"
      提示："汇总夜间测试运行"
      周期：每个工作日 09:00  (cron 0 9 * * 1-5，Asia/Shanghai)
      目标：agent "backend-worker"
      下次运行：周二 2026-07-21 09:00
      [确认]  [在表单中编辑]  [取消]
用户：[确认] → schedule/create → "已创建 ✓ — 查看日程"
```

### 2.2 通过引用编辑

```
用户："将夜间汇总移至 7:30"
助手：[提案卡片]
      UPDATE  "夜间测试汇总"
      周期：每个工作日 09:00 → 每个工作日 07:30
      下次运行：周三 2026-07-22 07:30
      [确认]  [在表单中编辑]  [取消]
```

### 2.3 生命周期操作

```
用户："暂停夜间汇总，直至另行通知"
助手：[提案卡片]  PAUSE "夜间测试汇总"  [确认] [取消]
```

### 2.4 澄清循环

```
用户："改为每 2 小时一次"
助手：[澄清卡片] "您指的是哪个日程 — '夜间测试汇总'
       还是'磁盘清理'？（或说'两者'）"
```

### 2.5 信息性回答（无修改）

```
用户："今天运行什么？"
助手：[回答] "今天有 3 个日程运行：夜间测试汇总 (09:00)，…"
```

### 2.6 失败路径

- **主机上未配置 LLM 提供商**（无启用项，或无可解析模型）→ 错误卡片含深度链接："在设置 → 常规 → LLM 提供商中添加带默认模型的 LLM 提供商。"
- 端点返回无法解析的输出 → 守护进程重试一次，附加验证错误 → 仍失败 → 错误卡片："无法将该请求理解为日程变更。请尝试重新表述，或使用表单。"
- 端点不可达 / 身份验证失败（401/403/429/5xx）→ 命名配置的提供商的错误卡片，含设置深度链接和重试。
- 周期无效（cron 无法解析，间隔 < 60 秒）→ 阐明约束的澄清卡片。
- 引用 agent/日程未找到 → 列出候选项的澄清卡片。

---

## 3. UX 设计

### 3.1 入口点

| 表面 | 入口 | 主机作用域 |
|------|------|------------|
| 按主机日程屏幕（`app/src/screens/schedules-screen.tsx`） | "Ask AI" 标题按钮（ sparkle 图标） | 由屏幕隐含的主机 |
| 日程详情屏幕（`schedule-detail-screen.tsx`） | "用 AI 编辑" 按钮 | 由 `contextScheduleId` 隐含的主机 + 日程 |
| 日程仪表板（`schedules/schedules-dashboard-screen.tsx`） | 助手 FAB | 如果 >1 个主机连接，第一条消息编辑器显示主机选择器（必需，默认为上次使用的主机） |

日程存在于特定守护进程中，`llmProviders` 配置也存在于该守护进程中。因此助手是**按主机作用域**的：它使用管理相同主机的 LLM 提供商进行解析。面板始终显示其正在通信的主机。

### 3.2 面板布局

- 移动端：底部抽屉（现有 `@gorhom/bottom-sheet` 模式）覆盖日程屏幕。
- Web/桌面：右侧固定面板，约 380px。
- 内容：主机芯片、LLM 提供商指示器（见 3.3）、可滚动消息列表、带发送按钮的编辑器。

消息列表中的消息类型：

| 类型 | 渲染 |
|------|------|
| 用户文本 | 标准聊天气泡 |
| 助手文本（澄清/回答） | 标准气泡 |
| **提案卡片** | 结构化卡片，见 3.4 |
| 错误卡片 | 淡化气泡含重试提示（配置错误的含设置深度链接） |
| 已应用回执 | 折叠卡片："已创建 ✓ / 已更新 ✓ / 已暂停 ✓" + 日程详情链接 |

### 3.3 LLM 提供商指示器

- 面板标题中的仅显示芯片，显示已解析的提供商标签 + 模型（在每次 `schedule/assist` 响应中回显）。
- **v1 无面板内切换。** 解析始终使用主机默认值：设置 → 常规 → LLM 提供商中的第一个启用提供商，及其 `isDefault` 模型（否则为第一个模型）。要更改它，用户在设置中编辑提供商列表（重新排序、禁用或更改默认模型）。
- 每次请求的提供商/模型覆盖延期至 v1.1（见 §13）。

### 3.4 提案卡片结构

```
┌────────────────────────────────────────────┐
│ ● CREATE                          (操作徽章)│
│ 夜间测试汇总                   (名称)      │
│ ─────────────────────────────────────────  │
│ 提示     汇总夜间测试运行                   │
│ 周期     每个工作日 09:00                   │
│ 目标     agent · backend-worker             │
│ 工作目录 ~/work/backend                     │
│ 最大运行次数 30 · 过期 2026-08-31 (如设置)   │
│ ─────────────────────────────────────────  │
│ 下次运行：周二 2026-07-21 09:00 (本地)      │
│ ⚠ 警告（如有）                              │
│ [ 确认 ] [ 在表单中编辑 ] [ 取消 ]          │
└────────────────────────────────────────────┘
```

- 操作徽章颜色：创建=绿色，更新=蓝色，暂停=琥珀色，恢复=绿色，删除=红色。
- **更新**卡片显示逐字段差异（旧 → 新）；未更改字段折叠。
- **删除**卡片命名日程及其周期；确认按钮为破坏性样式。v1 无批量删除操作。
- 周期行使用现有 `describeCron()` 渲染，用户读取的内容与存储内容完全一致。
- **目标**从提议的 `ScheduleTarget` 渲染 — 与执行运行器在触发时刻解析的字段相同（现有 agent / 新 agent 模板 / 提供商）。
- **在表单中编辑**打开现有的 `schedule-create-modal.tsx` / `schedule-edit-modal.tsx`，预填充提案值 — 用户可以调整并通过正常路径保存。这保证了一个无需新表单代码的手动退出路径。

### 3.5 空状态、加载状态和离线状态

- 空对话：3 个示例提示作为建议芯片（"每天早上 9 点运行每日站会汇总"、"暂停磁盘清理"、"本周运行什么？"）。
- 发送中：待定气泡（"思考中…" + 已解析的提供商标签）；超时 120 秒 → 错误卡片含重试。
- 主机上未配置 LLM 提供商：空状态替换为设置卡片，深度链接至设置 → 常规 → LLM 提供商（发送前已知，来自守护进程配置）。
- 主机断开连接：编辑器禁用，含重连提示。

---

## 4. 架构

### 4.1 概览

```
┌──────────────────────────────────────────────────────────────┐
│ Solo 应用                                                      │
│  ScheduleAssistantPanel                                       │
│   ├─ 消息列表（用户 / 助手 / 提案 / 错误）                     │
│   ├─ 编辑器 + 主机芯片 + LLM 提供商指示器                      │
│   └─ ProposalCard → 确认 → 现有 schedule/* RPC                │
└───────────────────────────┬──────────────────────────────────┘
                            │ WebSocket（通过 Relay 的 E2EE，或本地）
┌───────────────────────────▼──────────────────────────────────┐
│ Solo 守护进程（按主机）                                        │
│                                                               │
│  session_schedule_assist.go   handleScheduleAssist()          │
│            │                                                  │
│  ┌─────────▼───────────────────────────────────────────┐     │
│  │ daemon/internal/schedule/（助手文件）                  │     │
│  │  · Assistant        — 编排、速率限制、重试             │     │
│  │  · PromptBuilder   — 系统提示 + 上下文块              │     │
│  │  · Extractor       — JSON 提取 + 验证                 │     │
│  │  · Context         — agent + 日程摘要                 │     │
│  │  · Resolver        — 从 config.LLMProviders 解析默认  │     │
│  │                     提供商/模型                        │     │
│  └─────────┬───────────────────────────────────────────┘     │
│            │ 一次性 HTTPS 聊天补全                             │
│  ┌─────────▼──────────┐     ┌───────────────────────────┐    │
│  │ daemon/internal/llm │────▶ 开源自兼容端点              │    │
│  │ 聊天客户端          │     │ baseURL + apiKey + model   │    │
│  └─────────────────────┘     │ (设置 → 常规 →             │    │
│                              │  LLM 提供商)               │    │
│                              └───────────────────────────┘    │
│            │ 仅验证的提案 — 无修改                              │
│  schedule.Store ── Executor ── daemonRunner ─▶ Target Agent   │
│  （通过现有 RPC 修改；执行路径不变：                             │
│   现有 agent / 通过 AgentManager 的临时提供商 agent）           │
└───────────────────────────────────────────────────────────────┘
```

### 4.2 为何直接调用配置的 LLM 提供商（决策依据）

| 替代方案 | 判决 | 原因 |
|----------|------|------|
| **直接聊天补全至配置的 LLM 提供商（已选）** | ✅ | 复用用户已在维护的 LLM 提供商配置（设置 → 常规）— 目前无运行时消费者；守护进程中一个小型 HTTP 客户端；无守护进程主机上的 CLI 工具链依赖；凭证保留在守护进程配置中；本地和 Relay/E2EE 客户端行为一致 |
| 通过 AgentManager 的临时 agent（本文档的前一版本） | ❌ 已取代 | 需要在守护进程主机上为每个提供商安装 + 认证 CLI 工具链；进程启动 + 会话生命周期开销用于一次性解析；工具链流包装为严格 JSON 契约增加故障模式；完全忽略 LLM 提供商配置 |
| 在应用中解析 | ❌ | 会将 apiKey 发送给每个客户端并遇到 Web 构建中的 CORS/网络限制；按客户端复制解析管道；打破"守护进程拥有提供商凭证"的边界 |

### 4.3 无状态解析 + 客户端持有对话记录

每次 `schedule/assist` 请求携带有界对话记录（最后 ≤10 轮）。守护进程**不保存**助手对话状态 — 一次 HTTP 补全携带完整提示（系统 + 上下文 + 对话记录 + 用户消息）。

依据：

- 单个无状态补全完全不需要会话生命周期、超时或孤儿清理。
- 守护进程重启不丢失任何内容；解析中途崩溃仅失败一次请求。
- LLM 实际需要的上下文（agent、日程、时区、当前时间）在每次请求时重新注入 — 始终准确，永不陈旧。

Rejected alternative: 守护进程端对话存储（增加持久化、迁移和隐私表面，收益甚微）。

### 4.4 上下文增强在守护进程端发生

应用仅发送 `{message, timezone, clientNow, transcript, contextScheduleId?}` — v1 无提供商选择。守护进程注入：

- `agents`：id、名称、提供商、工作目录、状态（使"后端 agent"解析为目标）
- `schedules`：id、名称、周期、状态、下次运行时间（使"夜间汇总"解析为 id）
- 能力约束：允许的操作、周期规则、目标规则

这保持请求负载极小，并使名称解析确定性 — 守护进程在返回提案前验证每个引用的 id。

---

## 5. 协议变更

新文件中新增一对 RPC `protocol/message_schedule_assist.go`；在 `app-bridge/src/server/schedule/rpc-schemas.ts` + `types.ts` + `shared/messages.ts` 中镜像。现有日程消息无变更。

### 5.1 请求

```go
type ScheduleAssistRequest struct {
    Type      string `json:"type"`
    RequestID string `json:"requestId"`

    Message  string `json:"message"`            // 用户自然语言输入，≤ 2000 字符

    Timezone  string `json:"timezone"`           // IANA，必需 — 如 "Asia/Shanghai"
    ClientNow string `json:"clientNow"`          // RFC3339，客户端墙钟（相对时间："明天"）

    ContextScheduleID string                `json:"contextScheduleId,omitempty"` // 从详情屏幕打开
    Transcript        []ScheduleAssistTurn  `json:"transcript,omitempty"`        // ≤ 10 轮，最旧的在前
}

type ScheduleAssistTurn struct {
    Role    string `json:"role"`    // "user" | "assistant"
    Content string `json:"content"` // 该轮的纯文本渲染（提案摘要）
}

func (m ScheduleAssistRequest) MsgType() string { return "schedule/assist" }
```

**v1 无提供商字段**：守护进程始终使用从 `config.llmProviders` 解析的主机默认 LLM 提供商进行解析（§6.4）。每次请求的 `llmProviderId`/`model` 覆盖为 v1.1 候选（§13）。

### 5.2 响应

```go
type ScheduleAssistResponse struct {
    Type    string                          `json:"type"` // "schedule/assist/response"
    Payload ScheduleAssistResponsePayload   `json:"payload"`
}

type ScheduleAssistResponsePayload struct {
    RequestID string                  `json:"requestId"`
    Kind      string                  `json:"kind"` // "proposal" | "clarify" | "answer" | "error"
    Message   string                  `json:"message,omitempty"`  // 澄清问题 / 回答文本 / 错误详情
    Proposal  *ScheduleAssistProposal `json:"proposal,omitempty"`
    Error     *string                 `json:"error,omitempty"`    // 传输/配置错误代码，如 "no_llm_provider"、"llm_unreachable"、"rate_limited"

    LLMProvider string `json:"llmProvider,omitempty"` // 已解析的提供商配置 id — 用于面板指示器
    Model       string `json:"model,omitempty"`       // 已解析的模型 id — 用于面板指示器
}

type ScheduleAssistProposal struct {
    Op         string             `json:"op"` // "create" | "update" | "pause" | "resume" | "delete"
    ScheduleID string             `json:"scheduleId,omitempty"` // 更新/暂停/恢复/删除的已解析 id
    Name       string             `json:"name,omitempty"`
    Prompt     string             `json:"prompt,omitempty"`
    Cadence    *ScheduleCadence   `json:"cadence,omitempty"`    // 请求时区中的本地 cron/间隔
    Target     *ScheduleTarget    `json:"target,omitempty"`
    Cwd        string             `json:"cwd,omitempty"`
    MaxRuns    *int               `json:"maxRuns,omitempty"`
    ExpiresAt  string             `json:"expiresAt,omitempty"`

    Summary   string   `json:"summary"`           // LLM 生成的一行人类描述
    Warnings  []string `json:"warnings,omitempty"` // 如 "将'morning'解释为 09:00"
    NextRunAt *string  `json:"nextRunAt,omitempty"` // 守护进程计算的预览（RFC3339）
}
```

语义：

- `Kind` 由守护进程验证的 LLM 输出驱动，非仅由 LLM 断言。
- 当 `config.llmProviders` 中无带可解析模型的启用提供商时，`Error == "no_llm_provider"`；应用渲染设置深度链接（§3.5）。
- 提案中的 `Cadence` 以**客户端时区**表示（本地 cron）。守护进程验证其可解析（通过现有 `cron.go`）并计算 `NextRunAt`。确认时，**应用**在调用 `schedule/create` / `schedule/update` 前通过现有 `cronToUTC()` 转换为 UTC；存储约定（"前端转换本地 → UTC；后端评估 UTC"）保持与今天完全一致。
- 提案中的 `Target` 是普通 `ScheduleTarget`（`agent` / `new-agent` / `provider`）— 与 `schedule/create` 通过 `validateScheduleTarget` 验证的相同形状。
- `pause` / `resume` / `delete` 提案仅携带 `Op + ScheduleID + Name + Summary`。
- 客户端超时：120 秒（解析在较慢端点上可能需要数十秒）。所有其他日程 RPC 保持 10 秒。

### 5.3 确认路径 — 无新修改 RPC

| 提案操作 | 确认时应用调用 |
|----------|---------------|
| `create` | `schedule/create`（payload 1:1 映射，周期 → UTC） |
| `update` | `schedule/update`（完整记录：提案字段合并过检查的当前记录） |
| `pause` | `schedule/pause` |
| `resume` | `schedule/resume` |
| `delete` | `schedule/delete` |

对于 `update`，应用先获取 `schedule/inspect` 并合并，因为 `schedule/update` 是全量替换；卡片上显示的差异来自相同的 inspect 结果。

触发时刻的执行完全由现有运行器行为处理 — Target Agent 解析（`agent` → 消息发送给运行中的 agent；`new-agent`/`provider` → 通过 AgentManager 的临时 agent）。助手在该路径中不引入任何新内容。

---

## 6. 守护进程设计

### 6.1 新文件

```
daemon/internal/llm/
├── client.go              # 开源自兼容聊天补全客户端
└── client_test.go

daemon/internal/schedule/
├── assistant.go           # 助手：编排、速率限制、重试、防护
├── assistant_prompt.go    # PromptBuilder：系统提示 + 上下文渲染
├── assistant_extract.go   # Extractor：JSON 提取 + schema 验证
├── assistant_resolve.go   # Resolver：从配置解析默认 LLM 提供商/模型
└── assistant_test.go      # 单元测试

daemon/internal/server/
└── session_schedule_assist.go   # handleScheduleAssist + 响应发送器
```

处理器注册在 `session_register_handlers.go` 中：

```go
r.Register("schedule/assist", typeHandler(s.handleScheduleAssist))
```

### 6.2 助手编排

```go
type Assistant struct {
    store   *Store
    agents  assistantAgentLister // 只读：列出上下文的 agent
    llm     *llm.Client
    cfg     llmConfigSource      // 解析默认提供商/模型（config.LLMProviders）
    limiter *rateLimiter         // 按连接
    logger  *slog.Logger
}

func (a *Assistant) Assist(ctx context.Context, req protocol.ScheduleAssistRequest) (*protocol.ScheduleAssistResponsePayload, error)
```

与运行器的 `scheduleAgentManager` 接缝（用于创建/删除 agent 以*执行*日程）不同，助手的 agent 接缝是**只读**的 — 它列出上下文的 agent。解析路径从不创建 agent 会话。

流程：

1. **防护**：验证 `Message` 非空/≤2000 字符，`Timezone` 有效 IANA；强制执行速率限制和每个连接单次在途解析。
2. **解析** 从 `config.llmProviders` 解析默认 LLM 提供商 + 模型（§6.4）；无法解析 → `Kind: "error"`，`Error: "no_llm_provider"`。
3. **构建提示**（§6.3）：系统提示 + 上下文块（agent、日程、时区、clientNow）+ 对话记录 + 用户消息。
4. **一次性补全**（§6.4）：单次 HTTPS 聊天补全，收集响应文本。
5. **提取 + 验证**（§6.5）：JSON → 类型化意图 → 针对守护进程/agent 状态的语义验证。失败时，**重试一次**，将验证错误附加至提示。
6. **解析引用**：日程/agent 名称 → id；未知 → `clarify` 含候选列表。
7. **增强**：通过现有 `NextRunAt(cadence, now)` 计算 `NextRunAt` 预览；附加警告；在响应中回显已解析的 `llmProvider`/`model`。
8. 返回类型化 payload。助手**从不**调用 `Store.Create/Update/...`。

### 6.3 提示构建

系统提示（简化版；完整模板见附录 A）：

- 角色："你将用户关于 recurring tasks 的请求转换为单个 JSON 对象。"
- 输出契约：*仅* JSON，其中一种 `{kind:"proposal", ...}`、`{kind:"clarify", ...}`、`{kind:"answer", ...}`；精确字段 schema；JSON 外无数 prose。
- 周期规则：对时钟时间优先 `cron`，对纯间隔优先 `every`；最小间隔 60000ms；在给定时区中表达 cron。
- 目标规则：仅当列出匹配的现有 agent 时使用 `agent`；否则当可推断时使用 `new-agent` + 提供商 + 工作目录，否则 `clarify`。
- 锚定规则：仅使用上下文块中的 agent/日程；精确引用 id；当引用有歧义时，询问 — 绝不猜测。
- 回复语言：匹配用户的语言。

上下文块：

```
当前时间（客户端）：2026-07-17T22:50:00+08:00
客户端时区：Asia/Shanghai

现有 agent：
- id=a1b2… name="backend-worker" provider=claude cwd=~/work/backend status=running

现有日程：
- id=f3e9… name="夜间测试汇总" cadence=cron "0 9 * * 1-5" status=active nextRunAt=…
```

对话记录：最后 ≤10 轮渲染为 `User:` / `Assistant:` 行（提案摘要为一行，如 `Assistant: [proposal] 更新 "夜间测试汇总" 周期 → 07:30`）。

提示大小防护：上下文块 + 对话记录限制（约 8k 字符）；日程/agent 列表截断为各 50 项，附"…及其他 N 项"备注。

### 6.4 默认提供商解析 + 一次性补全

**解析**（每次请求，从守护进程配置新鲜读取 — 设置更改立即生效）：

1. 候选 = `cfg.LLMProviders` 中 `enabled != false` 的条目，按数组顺序（数组顺序 = 用户优先级，匹配设置列表顺序）。
2. 提供商 = 第一个具有非空 `baseURL` 和 `apiKey` 的候选。
3. 模型 = 该提供商的 `models` 中 `isDefault == true` 的条目；否则为 `models` 的第一个条目。
4. 无候选或无模型 → `Kind: "error"`，`Error: "no_llm_provider"`，附带引导用户至设置 → 常规 → LLM 提供商的消息。

**补全：**

```go
func (a *Assistant) runCompletion(ctx context.Context, p resolvedProvider, systemPrompt, userPrompt string) (string, error)
```

- `llm.Client.ChatCompletion`：`POST {baseURL}/chat/completions`（开源自兼容），`Authorization: Bearer <apiKey>`，body `{model, messages: [system, user], temperature: 0, max_tokens: 1024}`。
- v1 中**不发送** `response_format: {"type":"json_object"}` — 各"开源自兼容"端点支持程度不同；提示契约 + 宽容 Extractor + 一次验证重试承载 JSON 保证。v1.1 重新审视。
- 超时：60 秒（上下文截止时间）。传输错误（401/403/429/5xx、网络）立即作为错误卡片 surfaced（`llm_unreachable` / `llm_auth` / `rate_limited`）— 无静默重试；用户从 UI 重试。助手流程中的单次重试专用于*验证*失败（§6.5）。
- 输出视为**不受信文本**；仅 Extractor 赋予其含义。

端点兼容性立场（v1）：

| 方面 | v1 立场 |
|------|---------|
| API 形状 | 开源自兼容聊天补全（`POST {baseURL}/chat/completions`）— 配置提供商的公共基线 |
| 认证 | 仅 `Authorization: Bearer <apiKey>` |
| 流式传输 | 无 — 单次非流式响应 |
| JSON 模式 | 不请求（兼容性）；extractor 处理围栏/ prose 包裹的 JSON |
| 模型 | 配置的默认模型 whatever；无能力探测 |

### 6.5 提取与验证

Extractor 阶段：

1. 定位 JSON：优先 ```json 围栏块；否则最外层平衡的 `{…}` 跨度。
2. 解码为类型化 `assistIntent` 结构（kind + 可选字段）。
3. **Schema 验证**：每操作所需字段（创建 → prompt + cadence + target；更新 → scheduleRef + ≥1 更改字段；暂停/恢复/删除 → scheduleRef）。
4. **语义验证**针对实时状态：
   - cron 可解析（`cron.go`），`everyMs ≥ 60000`，prompt ≤ 4000 字符
   - 引用日程 id 存在（否则以 ≤5 个名称候选模糊匹配列出澄清）
   - 引用 agent id 存在且 `validateScheduleTarget` 的目标规则成立
   - `expiresAt` 在未来；`maxRuns > 0`
5. 任何失败 → 一次重试往返包括错误；第二次失败 → `Kind: "error"` 含用户可操作消息。

### 6.6 速率限制与资源防护

| 防护 | 值 | 作用域 |
|------|-----|--------|
| 速率限制 | 10 次 assist 请求 / 分钟 | 按连接 |
| 并发 | 1 次在途解析 | 按连接（否则 `rate_limited` 错误） |
| LLM 调用超时 | 每次补全 60 秒（客户端 RPC 预算 120 秒覆盖一次重试） | 按请求 |
| 守护进程出口 | 每次解析 1 次 HTTPS 调用，验证重试 +1 | 按请求 |
| 消息大小 | ≤ 2000 字符用户消息；对话记录 ≤ 10 轮 | 按请求 |

### 6.7 指标与日志

```
solo_schedule_assist_requests_total{llmProvider, kind}
solo_schedule_assist_parse_failures_total{llmProvider, stage}
solo_schedule_assist_duration_seconds{llmProvider}
solo_schedule_assist_confirms_total{op}        // 通过现有遥测路径由应用报告
```

（`llmProvider` 标签 = 已解析提供商的配置 id，如 `"openai"`。）

slog：请求 id、提供商 id、模型、kind、重试次数、验证错误、类 token 大小。**任何级别永不记录原始用户提示或 API 密钥**；提示日志受调试门控控制，默认关闭（隐私）。

---

## 7. 应用设计

### 7.1 新组件与 hooks

```
app/src/components/schedule-assistant/
├── schedule-assistant-panel.tsx    # 抽屉/固定容器，主机芯片 + LLM 指示器
├── assistant-message-list.tsx      # 气泡 + 卡片
├── proposal-card.tsx               # 操作徽章，字段/差异，操作
└── assistant-composer.tsx          # 输入 + 发送 + 建议芯片

app/src/hooks/
├── use-schedule-assist.ts          # mutation：client.scheduleAssist()
└── use-assistant-thread.ts         # 对话状态，对话记录窗口化
```

- 状态：Zustand 存储中的按主机对话（`useAssistantStore`，按 `serverId` 键控）；v1 仅会话持久化（无磁盘）。
- `useScheduleAssist` 包装 bridge 调用带 React Query mutation；收到 `proposal` 时推送卡片消息；收到 `clarify`/`answer` 时推送气泡；收到 `error` 且 `no_llm_provider` 时推送含至 `/settings/general` 深度链接的错误卡片。
- LLM 提供商指示器芯片从最新 assist 响应读取 `llmProvider`/`model`（可通过 `useDaemonConfig` 预检查空状态配置）。
- 确认处理器：映射操作 → 现有 hooks（`useCreateSchedule`、更新/暂停/恢复/删除等效），成功时使日程查询失效；卡片折叠为已应用回执。

### 7.2 App-bridge 新增

- `scheduleAssist(options)` 在 `client/daemon-client.ts` 中 — 关联请求，响应类型 `schedule/assist/response`，超时 120 秒。
- Zod schemas 镜像 §5 在 `server/schedule/rpc-schemas.ts`；类型在 `types.ts`；联合注册在 `shared/messages.ts`。请求无提供商字段（主机默认在守护进程端解析）；响应携带 `llmProvider`/`model` 回显。

### 7.3 复用、未变更

- `cron-timezone.ts`（`cronToUTC`、`describeCron`、`detectTimezone`）— 确认路径 + 卡片渲染。
- `schedule-create-modal.tsx` / `schedule-edit-modal.tsx` — "在表单中编辑"预填充（添加可选 `initialValues` prop；默认行为不变）。
- 现有日程 hooks/存储 — 确认后列表失效。
- `llm-providers-section.tsx`（设置 → 常规）— 助手消费其中配置的任何内容，通过现有 `get/set_daemon_config` RPC 和 `config.llmProviders` 存储。**已知差距**：设置 UI 今天无法编辑 `models` / 选择默认模型（它保留 `models` 但从不编辑它们）；v1 需要每个提供商至少一个模型条目 — 见 §13 的解析选项。
- 日程执行（`daemonRunner`、`Executor`、Target Agent 解析）— 未触碰。

---

## 8. 时区处理

| 阶段 | 约定（与今天一致） |
|------|-------------------|
| 解析 | LLM 在**客户端时区**中生成 cron，使用请求中的 `timezone` + `clientNow` |
| 验证 | 守护进程解析表达式（`cron.go`）并计算 `nextRunAt` 预览 |
| 显示 | 卡片显示 `describeCron(expression, timezone)` + 本地下次运行预览 |
| 存储 | 应用通过 `cronToUTC` 在确认时转换；守护进程存储/评估 UTC |
| 相对时间 | "明天早上 7 点" 针对 `clientNow` 解析，永不针对守护进程时钟 |

这端到端复用已测试的时区管道；助手仅新增 LLM 的本地 cron 生成，由守护进程验证和人类可读预览双重检查。

---

## 9. 安全、安全与成本

1. **确认前不应用（不变量）**：守护进程解析路径无代码路径到 `Store` 修改；提案仅通过用户确认通过现有已验证 RPC 应用。
2. **不受信的 LLM 输出**：从不执行，从不渲染为 HTML/带链接的 markdown；提案字段渲染为纯文本；schema + 语义验证门控一切。
3. **提示注入 containment**：上下文数据（日程名称、现有日程的提示）在提示中引用/转义；即使恶意日程名称操纵输出，爆炸半径是确认卡片上的错误提案 — 无静默应用。
4. **无 agent 会话，无工具**：解析路径是纯 HTTPS 聊天补全 — 无进程启动、无工具执行表面、解析后无清理内容。日程执行（Target Agent）保持不变，仍受现有运行器规则约束。
5. **E2EE 与出口**：应用↔守护进程通道不变（通过 Relay 的 E2EE，或本地）。解析请求（用户句子 + 日程/agent 摘要）然后从**守护进程主机**出口至用户配置的 LLM 端点，通过 HTTPS — 与今天 CLI 工具链提供商相同的信任姿态，它们也调用云 API。端点明确在设置中用户配置。
6. **成本控制**：按连接速率限制 + 单次在途 + 60 秒超时；每次解析是一次有界补全（上下文 ≤ 约 8k 字符，`max_tokens` 1024）；指标暴露按配置提供商的使用。
7. **隐私与凭证**：对话记录保持在应用内存中；`apiKey` 仅存在于守护进程配置（现有 `llmProviders` 存储）中，永不记录，assist 响应中永不回显；守护进程日志仅元数据，不提示。
8. **删除纪律**：删除提案在卡片中始终携带名称 + 周期；破坏性样式确认；v1 无批量删除。

---

## 10. 测试策略

### 10.1 Go（守护进程，`-short -race`）

- `assistant_resolve_test.go`：表格驱动解析器 — 启用顺序、跳过禁用/缺失 baseURL/apiKey、`isDefault` 模型 vs 第一个模型、空配置 → `no_llm_provider`。
- `client_test.go`（`internal/llm`）：针对 `httptest.Server` — 成功 body、auth 头发送、401/429/500 映射、畸形 JSON body、超时。
- `assistant_prompt_test.go`：黄金测试 — 上下文渲染、对话记录窗口化、大小限制。
- `assistant_extract_test.go`：表格驱动 — 围栏 JSON、原始 JSON、周围有 prose 的 JSON、截断的 JSON、无效 schema、每操作所需字段、语义失败（未知日程、错误 cron、间隔太小）。
- `assistant_test.go`：编排 — 速率限制、单次飞行、重试一次后错误、引用解析、下次运行预览、提供商/模型回显。
- 集成：存根聊天补全服务器返回 fixture JSON → 完整 `schedule/assist` 往返 → 断言提案 payload；存根返回垃圾 → 重试 → 错误类型；无提供商的配置 → `no_llm_provider` 错误。
- 注册测试：在 `session_register_handlers_test.go` 中添加 `"schedule/assist"`。

### 10.2 应用（Vitest）

- `use-schedule-assist` 映射测试：每操作 → 正确的现有 RPC 带正确 payload；cadence 通过 `cronToUTC` 传递；通过 inspect 更新合并。
- `proposal-card` 渲染测试：创建/更新(差异)/暂停/删除变体；警告；应用时禁用确认。
- 澄清/回答/错误渲染；超时 → 错误卡片；`no_llm_provider` → 含 `/settings/general` 深度链接的错误卡片。
- LLM 指示器芯片显示已解析的提供商标签 + 模型。

### 10.3 Bridge（Vitest）

- assist 请求/响应的 schema 往返；联合注册。

### 10.4 E2E（Playwright，夜间）

- 测试中的守护进程配置为**存根 LLM 端点**（`llmProviders` 配置中的本地测试服务器）。发送"每个工作日早上 9 点汇总测试" → 提案卡片 → 确认 → 日程以预期周期出现在列表中。
- 编辑流程："将其移至 7:30" → 含差异的更新卡片 → 确认 → 周期更新。
- 歧义：两个名称相似的日程 → 澄清卡片。
- 无提供商状态：空 `llmProviders` → 含设置深度链接的设置卡片。

### 10.5 评估集（手动，发布前）

约 30 个规范 utterance（创建/编辑/生命周期/歧义/相对时间/zh+en）针对团队真实配置的端点运行；记录首次尝试正确性以朝向 G1 ≥80% 目标。

---

## 11. 发布阶段

| 阶段 | 范围 | 成功标准 |
|------|------|----------|
| **P1 — 守护进程解析路径**（1 周） | 协议类型、`internal/llm` 客户端 + 解析器、助手 + 提示/提取、速率限制、Go 测试 | `schedule/assist` 对存根端点 + 一个真实配置提供商返回验证的创建/更新/暂停/恢复/删除提案 |
| **P2 — 应用面板 + 创建**（1 周） | 面板 UI、LLM 指示器芯片、提案卡片、确认 → 创建、"在表单中编辑"预填充、`no_llm_provider` 深度链接 | Web + 移动端端到端 NL 创建；单元测试通过 |
| **P3 — 编辑与生命周期操作**（1 周） | 更新差异卡片、澄清循环、暂停/恢复/删除、回答类型 | 所有 §2 流程工作；E2E 存根端点规范通过 |
| **P4 — 端点兼容性与加固**（1 周） | 验证 2–3 个真实开源自兼容端点、指标、评估集、文档 | 评估 ≥80% 首次尝试；指标上线；E2E 夜间通过 |

总计：约 4 周。P1–P2 交付垂直切片（NL 创建），降低其余风险。

---

## 12. 风险与缓解

| 风险 | 影响 | 缓解 |
|------|------|------|
| LLM 错误解析周期（如错误日期） | 高 | 确认卡片含 `describeCron` + 下次运行预览；守护进程 cron 验证；评估集跟踪 |
| 端点不兼容（认证怪癖、非 JSON、不支持的参数） | 中 | 最小请求形状（无 `response_format`、无流式）；宽容 extractor + 一次重试；命名提供商的错误卡片；每端点评估 |
| 未配置默认 LLM 提供商（首次运行 UX） | 中 | `no_llm_provider` 错误 + 设置卡片深度链接至设置 → 常规 → LLM 提供商；文档；§13 中指出的设置 UI 模型编辑差距 |
| 歧义日程引用 | 中 | 绝不猜测：含候选的澄清卡片；`contextScheduleId` 从详情屏幕偏向 |
| 通过日程/agent 名称的提示注入 | 低 | 引用上下文块；输出不受信；确认门控；无工具执行 |
| 解析延迟（慢端点） | 中 | 60 秒调用超时 / 120 秒客户端预算；含提供商标签的待定 UI |
| Token 成本滥用 | 低 | 速率限制、单次飞行、有界上下文 + `max_tokens`、指标 |
| 时区混淆 | 中 | 提示中明确 tz+now；卡片上本地预览；现有 UTC 存储管道未触碰 |

---

## 13. 待决问题

1. **对话持久化**：跨应用重启保留对话（AsyncStorage）还是仅会话？v1：仅会话。
2. **低风险自动应用**：允许"快速模式"切换无需确认的暂停/恢复？v1：始终确认。
3. **来源元数据**：标记助手创建的日程（如可选 `origin: "assistant"` 字段）用于审计/过滤？延期 — 需要协议字段；v1 非必需。
4. **CLI 表面**：`solo schedule assist "..."` 复用相同守护进程路径？不错的后续，非 v1。
5. **提供商/模型覆盖与默认语义**：每次请求 `llmProviderId`/`model` 覆盖（v1.1），和/或提供商级 `isDefault` 标记 vs v1 的数组顺序优先级 — 与设置 UX 一起决定。相关差距：设置 UI 今天无法编辑 `models` 或标记默认模型（它仅保留数组）；要么为 `llm-providers-section.tsx` 添加最小默认模型 affordance，要么为 v1 记录手动 `config.json` 编辑。

---

## 附录 A — 解析提示模板（简化版）

```
你是 Solo 日程助手。将用户的请求转换为一个 JSON 对象，仅输出该 JSON（可选在 ```json 围栏中）。无数 prose。

输出形状：
{"kind":"proposal","op":"create","name":string,"prompt":string,
 "cadence":{"type":"cron","expression":string} | {"type":"every","everyMs":number},
 "target":{"type":"agent","agentId":string} | {"type":"new-agent","config":{"provider":string,"cwd":string}},
 "maxRuns":number?,"expiresAt":string?,"summary":string,"warnings":string[]?}
{"kind":"proposal","op":"update","scheduleId":string, ...更改字段..., "summary":string}
{"kind":"proposal","op":"pause"|"resume"|"delete","scheduleId":string,"name":string,"summary":string}
{"kind":"clarify","message":string}
{"kind":"answer","message":string}

规则：
- Cron 表达式在下方给出的客户端时区中。最小间隔：60000ms。
- 对时钟时间优先 "cron"（"at 9"、"工作日早晨"）；对纯间隔优先 "every"。
- 仅引用上下文块中的 agent/日程；精确复制 id。
- 引用有歧义或缺少所需信息（目标 agent、时间）→ kind="clarify"。
- 关于日程的纯问题 → kind="answer"。
- 用用户的语言回复。
```

## 附录 B — 示例交换

**B.1 相对时间 + 间隔**
```
用户："每 15 分钟 ping 暂存健康检查"
→ 创建提案，周期 {type:"every","everyMs":900000}，
  警告：["未指定 agent — 使用默认 new-agent 与提供商 claude"]
```

**B.2 歧义引用**
```
用户："暂停备份"
上下文：日程 "DB 备份"、"文件备份"
→ 澄清："哪一个 — 'DB 备份' 还是 '文件备份'？"
```

**B.3 带差异的更新**
```
用户："改为在 7:30 运行夜间汇总"
→ 更新提案 id=f3e9…，周期 {cron "30 7 * * 1-5"}，
  摘要："将 '夜间测试汇总' 从工作日的 09:00 移至 07:30"
```

（文件结束 - 共 686 行）
