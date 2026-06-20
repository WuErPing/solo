# Solo 2026 产品/技术路线图

> **文档类型**：统一产品路线图
> **日期**：2026-06-20
> **基线版本**：Solo v0.6.3
> **目标读者**：产品、技术负责人、核心开发者、投资者
> **关联文档**：
> - [Feature Directions 2026](feature-directions-2026.md)
> - [Provider Hub / CC-Switch Migration Design](agent-profile-switch-export-design.md)
> - [Loop Schedule Design](loop-schedule-design.md)
> - [Loop Schedule Deep Dive](loop-schedule-deep-dive.md)
> - [Loop Schedule Implementation Spec](loop-schedule-spec.md)

---

## 0. 2026 年 Agentic 生态趋势研判

本路线图基于截至 2026 年 6 月的行业动态制定。以下趋势直接影响 Solo 的产品优先级与技术选型。

### 0.1 MCP 成为 Agent-Tool 事实标准

- **标准中立化**：Anthropic 于 2025 年 12 月将 MCP 捐赠给 Linux Foundation 下的 **Agentic AI Foundation（AAIF）**，与 OpenAI 的 AGENTS.md、Block 的 goose 并列为三大创始项目。AAIF 目前已有近 150 家成员，包括 Google、Microsoft、AWS、Cloudflare、Bloomberg 等。
- **生态规模**：MCP SDK 月下载量达数千万量级，公开 MCP server 超过 10,000 个，被 Claude Code、Codex、Cursor、ChatGPT、Goose 等主流 agent 原生支持。
- **协议演进**：Streamable HTTP 成为生产级传输标准；SSE 逐步退出；2026 年重点方向包括 **长时任务（MCP Tasks）**、**Server Cards 发现机制**、**Capability Attestation** 与 **MCP Apps（工具可返回沙箱化 UI）**。

**对 Solo 的启示**：Provider Hub 不应只是 API 配置管理器，而应以 MCP 为统一工具层，成为 Agent 的“工具与上下文中枢”。

### 0.2 A2A 补齐 Agent-Agent 协作层

- Google 提出的 **Agent2Agent（A2A）** 协议专注于 agent 之间的发现、委托与任务协调，与 MCP（agent ↔ tool）互补而非竞争。
- A2A v1.0 已支持 **Agent Card** 能力声明、基于任务的通信、**Signed Agent Cards** 信任模型。

**对 Solo 的启示**：Solo 的多 Agent 协作（Coordinator + Specialist）应优先基于 A2A 语义设计，确保未来可与外部 agent（Claude Code sub-agents、Codex、Goose 等）互操作。

### 0.3 OpenAI 的开放与锁定并存

- **Codex CLI 开源**：OpenAI Codex CLI 以 Apache-2.0 开源（Rust），默认沙箱化执行，支持本地 OSS 模型（`--oss`），是 Solo 必须补齐的 Provider。
- **Open Responses**：OpenAI 2026 年 2 月发布开放标准，试图统一 agentic API 接口，Hugging Face、Vercel、OpenRouter、Ollama 等已支持。
- **Assistants API 退役**：OpenAI 计划在 2026-08-26 关闭 Assistants API，生态被迫迁移到 Responses API / Conversations API。

**对 Solo 的启示**：Codex Provider 后端必须尽快落地；Local API Proxy 可增加 Open Responses 兼容层，降低外部 agent 的接入摩擦。

### 0.4 开源 Agent 与 BYOK 模型爆发

- 开源 CLI agent 竞争激烈：OpenCode（~170k stars）、OpenAI Codex CLI（~90k）、OpenHands、Cline、Pi、Goose、Aider 等。
- 模型选择多元化：Kimi K2.7-Code（开源权重，256K 上下文）、DeepSeek-V3.2、Qwen3、Llama 4 等成为成本敏感场景的可选项。
- 地缘政治风险：2026 年 6 月 Claude Fable 5 因出口管制暂停全球访问，凸显“不绑定单一模型/厂商”的必要性。

**对 Solo 的启示**：Provider Hub 的智能路由与多 Provider 中立定位从“ nice-to-have ”升级为“风险对冲能力”。

### 0.5 安全与成本成为核心关切

- **MCP 安全**：Capability attestation、SAFE-MCP、OWASP Agentic Top 10（2026）推动工具调用前授权与 manifest 校验。
- **成本治理**：GitHub Copilot 于 2026-06-01 转向按量 AI credits，用户更关注 token 预算、模型路由与成本可视化。
- **长时任务可靠性**：agent 任务从分钟级扩展到小时级，需要状态持久化、断点恢复、Push 通知与人工门控。

**对 Solo 的启示**：本地优先 + E2EE + 成本治理 + 崩溃恢复，构成 Solo 区别于云 agent 的核心壁垒。

---

## 1. 产品愿景

> **Solo 是开发者的本地 AI 工作中枢。**
>
> 它常驻于你的机器，安全地连接你偏好的大模型和 coding agent，让你通过桌面、移动端或任何终端随时发起、监控和接管自动化开发任务。

### 1.1 核心差异化

| 差异化 | 说明 |
|--------|------|
| **本地优先 + E2EE** | 代码和配置默认不出本机；远程访问通过端到端加密中继。 |
| **背景 Agent 值守** | 本地 daemon 7×24 运行 Loop / Schedule，人离开时 agent 继续工作。 |
| **移动端指挥中心** | iOS/Android/Web 统一客户端，随时随地查看/干预 agent。 |
| **多 Provider / 多 Agent 中立** | 不绑定单一模型，支持 Claude、Kimi、OpenCode、Codex、Cursor-Agent、本地 OSS 模型等。 |
| **MCP 原生工具中枢** | 统一管理 MCP server，跨 Solo 内部 agent 与外部 coding agent 同步。 |
| **配置 + 规则中枢** | 一套 Provider/MCP/Skill/Prompt/AGENTS.md 配置，同步到多个 coding agent。 |
| **成本可观测** | 统一用量、token、延迟监控，支持基于成本的智能路由。 |

---

## 2. 2026 战略目标

1. **把 Solo 从“客户端”升级为“中枢”**：本地 daemon 7×24 值守，移动端随时接管。
2. **实现自治开发循环**：Schedule → Loop，让 agent 能自主计划、执行、验证、修复。
3. **成为 AI coding agent 的配置中枢**：Provider Hub 统一管理和转写配置到 Claude/Codex/Cursor/OpenCode 等。
4. **成为 MCP 原生工具层**：MCP server 一次配置，多处生效；支持 Server Cards 发现与安全 attestation。
5. **成为 AGENTS.md / CLAUDE.md 原生工具**：项目规则自动注入所有接入 agent，降低重复沟通。
6. **建立项目级长期记忆**：跨会话、跨 agent 的项目知识沉淀，支持知识图谱检索。
7. **覆盖完整研发交付流**：从编码到测试、审查、PR。
8. **建立成本治理能力**：用量监控、预算门控、模型路由，让 agent 规模化使用可控。
9. **面向 A2A / Open Responses 开放生态**：多 Agent 协作与外部 API 兼容层提前布局。

---

## 3. 三大产品支柱

```
┌─────────────────────────────────────────────────────────────────────┐
│                     Solo 2026 产品支柱                                │
├─────────────────┬─────────────────────┬─────────────────────────────┤
│  支柱 1          │  支柱 2              │  支柱 3                      │
│  Provider Hub   │  Loop Schedule      │  Project Memory + Chat      │
│  配置与工具中枢   │  自治循环            │  上下文与协作                 │
├─────────────────┼─────────────────────┼─────────────────────────────┤
│ · Provider 管理  │ · 大模型驱动决策     │ · 项目级记忆                 │
│ · 本地 API 代理  │ · agent/bash/test   │ · 多 Agent 协作              │
│ · MCP/Skill/Prompt│ · 自动验证/修复     │ · 长期上下文                 │
│ · MCP Server Cards│ · 崩溃恢复         │ · 团队共享                   │
│ · 配置转写      │ · 人工确认门控       │ · 自动索引                   │
│ · 智能路由      │ · 背景值守           │ · AGENTS.md 原生             │
│ · 用量/成本监控 │ · A2A-ready         │ · A2A Agent Card             │
└─────────────────┴─────────────────────┴─────────────────────────────┘
```

### 3.1 支柱 1：Provider Hub（配置与 MCP 工具中枢）

**目标**：让 Solo 成为开发者管理所有 coding agent 配置和工具的地方。

**核心能力**：
- 统一管理 Provider 预设和 API 配置。
- 本地 API 代理（`:17613`），支持协议转换、Open Responses 兼容层和 failover。
- MCP / Skills / Prompts / AGENTS.md 跨 agent 同步。
- **MCP Server Cards 与注册表发现**：从公开 registry 或 `.well-known` 自动拉取 server 能力清单。
- **MCP Capability Attestation**：校验 server manifest，标记危险工具权限。
- 用量和成本监控，支持预算告警。
- 智能模型路由：按任务类型、成本、延迟、provider 健康状态自动选择模型。
- 配置一键转写到 Claude Code、Codex、Cursor、OpenCode、Windsurf、Aider、Continue。

**商业价值**：
- 降低多 agent 用户的配置管理成本。
- 为团队提供统一的模型接入、工具治理和成本治理。

### 3.2 支柱 2：Loop Schedule（自治循环）

**目标**：让 Schedule 从“定时执行单次任务”进化为“大模型驱动的自治循环”。

**核心能力**：
- 扩展 `StoredSchedule` 支持 `type: "loop"`。
- Loop Controller 调用 LLM 决策下一步。
- Step Executor 支持 agent、bash、test、read、write、git、ask-user。
- 状态机、持久化、崩溃恢复、人工确认门控。
- 与 Schedule 结合，支持“每晚自动修复测试失败”等背景值守场景。
- 安全沙箱、成本控制、可观测性。
- **长时任务友好**：step 可跨网络断连持久化，与 MCP Tasks / A2A Task 语义对齐。

**商业价值**：
- 把“辅助编码”推进到“自动值守和修复”。
- 典型场景：每晚自动修复测试失败、自动更新依赖、自动重构。

### 3.3 支柱 3：Project Memory + Chat（上下文与协作）

**目标**：让 Solo 越用越懂项目和用户，并支持与外部 agent 协作。

**核心能力**：
- 项目级记忆：代码地图、ADR、用户偏好、历史尝试。
- 知识图谱检索：函数/模块/依赖关系结构化存储。
- 多 Agent 协作：Coordinator + Specialist Agent 分工，**基于 A2A 语义设计委托与 Agent Card**。
- Chat 系统：项目级指挥中心。
- AGENTS.md / CLAUDE.md 原生支持：项目规则自动注入所有 agent。

**商业价值**：
- 减少重复沟通，提升 agent 输出质量。
- 支持团队共享项目知识。

---

## 4. 季度路线图

### Q3 2026：Provider Hub MVP + Loop Schedule 基础

**主题**：把 cc-switch 的核心能力和 Loop 的基础能力落地，补齐 Chat/多 Agent 核心缺口；紧跟 MCP 标准化浪潮。

| 优先级 | 功能 | 里程碑 | 成功标准 |
|--------|------|--------|----------|
| P0 | Provider Hub 独立进程 (`solo-provider-hub`) | 可管理 5+ provider 预设 | 切换 provider 耗时 < 1s |
| P0 | 本地 API Proxy | 支持 Claude/Codex 指向 Solo 端口 | 协议转换成功率 > 95% |
| P0 | Loop Schedule 基础 | 支持 `type: "loop"` 和 agent/bash/test step | 跑通 "create hello.go" 场景 |
| P0 | Codex Provider 后端实现 | 补齐当前只有前端定义的缺口 | Codex 可正常对话和流式输出 |
| P1 | Config Exporter MVP | 支持 Claude Code + Cursor 配置转写 | 导出文件可直接被目标 agent 读取 |
| P1 | MCP 统一管理 MVP | 一个 MCP 同步到 2+ agent | 配置一致率 100% |
| P1 | AGENTS.md 原生支持 | Solo Agent 启动时自动读取项目 AGENTS.md | 项目规则注入率 100% |
| P1 | 本地 / BYOK 模型支持 | 支持 Ollama / vLLM / LM Studio 作为 Provider | 至少一种本地推理后端跑通 |
| P2 | Usage Tracker 基础 | 记录 provider 请求数和 token | 数据误差 < 5% |
| P2 | 项目 Onboarding 自动索引 | 首次打开项目生成代码地图和技术栈摘要 | 新项目上手时间减半 |
| P2 | MCP Server Cards 发现 | 支持从 `.well-known/mcp-server-card` 拉取能力清单 | 5+ 公开 server 可被发现 |

### Q4 2026：Loop 自治 + Provider Hub 完善

**主题**：让 Loop 真正自治，让 Provider Hub 覆盖更多 agent，引入成本治理与 MCP 安全。

| 优先级 | 功能 | 里程碑 | 成功标准 |
|--------|------|--------|----------|
| P0 | Loop Controller 决策优化 | 支持 function calling / tool use | fix-tests 场景 70% 自主完成 |
| P0 | 人工确认门控 + App UI | 危险操作 100% 经审批 | 无未经审批的破坏性操作 |
| P0 | 智能 Provider 路由 | 基于任务类型/成本/延迟自动选择 provider | 简单任务成本降低 30% |
| P1 | Config Exporter 扩展 | 支持 Codex/OpenCode/Windsurf/Aider/Continue | 覆盖 80% 主流 agent |
| P1 | Skills Market + Prompts 库 | 可安装/更新 skills | 10+ 内置 skill 模板 |
| P1 | MCP 工具目录 | 内置常用 MCP 服务器一键启用 | 5+ 常用 MCP 开箱即用 |
| P1 | MCP Capability Attestation | Server manifest 校验 + 危险工具标记 | 高风险工具调用前 100% 告警 |
| P2 | Project Memory Phase 1 | 代码地图 + ADR 索引 | 新项目 onboarding 时间减半 |
| P2 | 成本预算告警 | 设置月度预算和阈值告警 | 预算超支前主动通知 |
| P2 | Open Responses 兼容层 | Local API Proxy 支持 Open Responses 接口 | 主流 Open Responses 客户端可接入 |

### Q1 2027：多 Agent 协作 + 团队协作

**主题**：从单 agent 到多 agent，从个人到团队；A2A 与 MCP Apps 进入实用阶段。

| 优先级 | 功能 | 里程碑 | 成功标准 |
|--------|------|--------|----------|
| P0 | Chat / 多 Agent 协作 | Coordinator + Specialist 模式 | 复杂任务分解成功率 > 80% |
| P0 | Project Memory Phase 2 | 跨会话检索 + 用户偏好学习 | 用户重复指令减少 50% |
| P1 | 团队共享配置 | Provider Hub 团队空间 | 团队配置一致率 100% |
| P1 | PR 自动审查 | Agent 自动 review 并输出评论 | 审查覆盖率 > 60% |
| P1 | Auto Test / Fix Loop | 代码修改后自动运行相关测试并进入 fix loop | 测试失败自主修复率 > 50% |
| P1 | A2A v1.0 适配 | Solo Agent 可发布 Agent Card 并委托外部 agent | 跨 agent 委托成功率 > 80% |
| P2 | 语音/截图输入 | 移动端专属交互 | 支持语音创建 loop |
| P2 | MCP Apps UI 支持 | 工具返回的沙箱化 UI 可在 App/桌面渲染 | 1+ 官方 MCP App 可用 |

### Q2 2027：生态与规模化

**主题**：开放生态、性能优化、企业级能力。

| 优先级 | 功能 | 里程碑 | 成功标准 |
|--------|------|--------|----------|
| P0 | Provider Hub Marketplace | 第三方 provider / skill 市场 | 50+ 公开 provider 预设 |
| P0 | Loop Template Market | 常见 loop 模板 | 20+ 内置模板 |
| P1 | 企业 SSO / 审计日志 | 团队权限和合规 | 支持 OIDC + 审计 |
| P1 | 性能优化 | Loop 延迟降低 50% | 单 step 决策 < 2s |
| P1 | A2A / 跨进程 Agent 协作 | 支持 A2A 协议委托任务 | 跨 agent 委托成功率 > 90% |
| P1 | MCP Registry 集成 | 一键安装公开 MCP server | 100+ server 可被发现 |
| P2 | 云端同步选项 | 可选加密云同步 | 用户可自主选择 |

---

## 5. 技术依赖关系

```
Provider Hub
    │
    ├── Local API Proxy ──▶ External Agents (Claude/Codex/Cursor/...)
    │
    ├── Open Responses Layer ──▶ Open Responses compatible clients
    │
    ├── Config Exporter ──▶ External Agent Config Files
    │
    ├── MCP/Skills/Prompts Hub ──▶ Solo Agents + External Agents
    │
    ├── MCP Server Cards / Registry ──▶ Tool Discovery & Attestation
    │
    ├── AGENTS.md / CLAUDE.md Sync ──▶ Project Context Injection
    │
    └── Usage Tracker + Router ──▶ Cost Dashboard

Loop Schedule
    │
    ├── Loop Controller ──▶ Provider Client ──▶ Provider Hub
    │
    ├── Step Executor ──▶ Agent Manager / Terminal / Workspace
    │
    ├── State Store ──▶ Schedule Store extension
    │
    ├── Human Confirm Gate ──▶ App / CLI
    │
    └── A2A Task Delegation ──▶ External Specialist Agents

Project Memory + Chat
    │
    ├── Memory Backend ──▶ File/SQLite/Middleware
    │
    ├── Indexer ──▶ Workspace / Git / AGENTS.md
    │
    ├── Knowledge Graph ──▶ Code Structure + Dependencies
    │
    ├── Chat Coordinator ──▶ Multiple Agents
    │
    └── A2A Agent Card Registry ──▶ External Agent Discovery
```

---

## 6. 关键成功指标（KPIs）

| 指标 | 2026 年底目标 | 2027 年底目标 |
|------|--------------|---------------|
| Provider Hub 管理 provider 数 | 20+ | 50+ |
| 支持的本地 / BYOK 推理后端 | 1+ | 3+ |
| MCP server 可发现/启用数 | 10+ | 50+ |
| MCP 跨 agent 配置一致率 | 100% | 100% |
| Loop 自主完成率（fix-tests） | 70% | 90% |
| 配置导出覆盖 agent 类型 | 5 | 10 |
| AGENTS.md 注入覆盖率 | 100% | 100% |
| 项目记忆检索准确率 | — | 85% |
| 简单任务成本节省（vs 固定模型） | 20% | 40% |
| App 月活跃用户（MAU） | — | 10K+ |
| Solo daemon 崩溃恢复成功率 | 95% | 99% |
| A2A 跨 agent 委托成功率 | — | 90% |

---

## 7. 风险与应对

| 风险 | 影响 | 应对 |
|------|------|------|
| Provider Hub 独立进程增加安装复杂度 | 中 | 提供 `solo install hub` 一键安装和自动启动。 |
| Loop 可能进入死循环或产生破坏 | 高 | 沙箱、审批门控、maxIterations、early stop。 |
| 大模型决策不稳定 | 高 | Prompt 工程 + function calling + 决策校验 + 人工确认。 |
| 外部 agent 配置格式变化 | 中 | Exporter 接口隔离 + 单元测试 + 社区反馈。 |
| 移动端体验不及桌面端 | 中 | 移动优先设计监控/轻量干预，不做完整代码编辑。 |
| 竞品（Cursor/Copilot/Cloud Agents）快速迭代 | 中 | 坚持本地优先 + 开放生态 + 移动差异化。 |
| Agent token 成本爆炸 | 高 | 用量监控、预算告警、智能路由、缓存复用。 |
| AGENTS.md / MCP 标准碎片化 | 中 | 紧跟 Linux Foundation Agentic AI Foundation 标准，保持 exporter 可插拔。 |
| 云 Agent 在复杂任务上表现更优 | 中 | 本地优先做“值守+监控+治理”，复杂任务可选路由到云端。 |
| 模型厂商出口管制或访问受限 | 高 | 多 Provider 中立 + 本地/BYOK 模型支持，避免单一厂商锁定。 |
| MCP server 工具滥用或 manifest 伪造 | 高 | Capability attestation、危险工具标记、权限最小化、审计日志。 |
| A2A Agent Card 信任模型不成熟 | 中 | 先支持自签名/本地 Agent Card，生产级依赖 Signed Agent Cards 待验证后启用。 |

---

## 8. 近期行动项（未来 2 周）

1. **确定 Provider Hub 进程模型**：独立进程 vs daemon 内置，选定后锁定架构。
2. **Loop Schedule MVP 范围**：确认先做 `agent/bash/test` 三种 step 和 CLI。
3. **创建 `daemon/internal/loop/` 模块骨架**：定义 `LoopEngine`、`LoopController`、`StepExecutor` 接口。
4. **扩展 protocol**：`StoredSchedule.Type` 支持 `"loop"`，新增 `LoopControllerConfig`。
5. **CLI 命令设计**：`solo loop create/start/status/logs/abort`。
6. **AGENTS.md 读取机制**：在 workspace 启动时扫描并注入项目级规则。
7. **Codex Provider 后端实现**：补齐当前只有前端定义的缺口；参考开源 Codex CLI 的协议实现。
8. **本地模型 Provider 调研**：确认 Ollama / vLLM 哪一种优先接入 Provider Hub。

---

## 9. 文档索引

| 文档 | 用途 |
|------|------|
| [Feature Directions 2026](feature-directions-2026.md) | 原始方向分析，含业界对标 |
| [Provider Hub / CC-Switch Migration Design](agent-profile-switch-export-design.md) | Provider Hub 详细设计 |
| [Loop Schedule Design](loop-schedule-design.md) | Loop Schedule 高层设计 |
| [Loop Schedule Deep Dive](loop-schedule-deep-dive.md) | Loop 技术深度分析 |
| [Loop Schedule Implementation Spec](loop-schedule-spec.md) | Loop Schedule 实现规范（protocol、模块、迁移计划） |
| [Product Features](features.md) | 当前 Solo 完整功能清单 |

---

## 10. 修订记录

| 日期 | 版本 | 变更 |
|------|------|------|
| 2026-06-13 | v1.0 | 初始版本，整合 Feature Directions、Provider Hub、Loop Schedule 三大方向 |
| 2026-06-19 | v1.1 | 结合 2026 年 6 月竞品动态（Cursor 3 / Claude Code Sonnet 4.5 / OpenAI Codex / Windsurf / MCP 生态 / AGENTS.md 标准）迭代：强化背景 Agent 值守、成本治理、智能路由、AGENTS.md 原生支持、多 Agent 协作、A2A 规划 |
| 2026-06-20 | v1.2 | 新增“2026 年 Agentic 生态趋势研判”章节；强化 MCP 原生工具层、MCP Server Cards / Capability Attestation、A2A-ready、Open Responses 兼容层、本地/BYOK 模型支持；更新 KPIs、风险与近期行动项 |
