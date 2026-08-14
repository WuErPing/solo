# Solo 2026 产品/技术路线图（Task 交付版）

> **文档类型**：统一产品路线图
> **版本**：v3.0
> **日期**：2026-08-11
> **基线版本**：Solo v0.11.0（App）
> **目标读者**：产品、技术负责人、核心开发者、投资者
> **关联文档**：
> - [PRD: Product + Task Unification](prd/product-task-unification.md)（v3.0 的产品模型基座）
> - [Multica 能力调研](../analysis/multica-capability-research.md)（v3.0 的外部输入）
> - [Provider Hub Design](prd/provider-hub-design.md)
> - [Loop Schedule Implementation Spec](prd/loop-schedule-spec.md)
> - [Solo Roadmap Architecture Mapping](../analysis/solo-roadmap-architecture-mapping.md)

---

## 0. 2026 年 Agentic 生态趋势研判

本路线图基于截至 2026 年 8 月的行业动态制定。以下趋势直接影响 Solo 的产品优先级与技术选型。

### 0.1 MCP 成为 Agent-Tool 事实标准

- **标准中立化**：Anthropic 于 2025 年 12 月将 MCP 捐赠给 Linux Foundation 下的 **Agentic AI Foundation（AAIF）**，与 OpenAI 的 AGENTS.md、Block 的 goose 并列为三大创始项目。AAIF 目前已有近 150 家成员，包括 Google、Microsoft、AWS、Cloudflare、Bloomberg 等。
- **生态规模**：MCP SDK 月下载量达数千万量级，公开 MCP server 超过 10,000 个，被 Claude Code、Codex、Cursor、ChatGPT、Goose 等主流 agent 原生支持。
- **协议演进**：Streamable HTTP 成为生产级传输标准；SSE 逐步退出；2026 年重点方向包括 **长时任务（MCP Tasks）**、**Server Cards 发现机制**、**Capability Attestation** 与 **MCP Apps（工具可返回沙箱化 UI）**。

**对 Solo 的启示**：Provider Hub 不应只是 API 配置管理器，而应以 MCP 为统一工具层，成为 Agent 的"工具与上下文中枢"。每个项目工作目录都应拥有一套可版本化、可共享、可审计的 MCP + AGENTS.md 配置。

### 0.2 A2A 补齐 Agent-Agent 协作层

- Google 提出的 **Agent2Agent（A2A）** 协议专注于 agent 之间的发现、委托与任务协调，与 MCP（agent ↔ tool）互补而非竞争。
- A2A v1.0 已支持 **Agent Card** 能力声明、基于任务的通信、**Signed Agent Cards** 信任模型。

**对 Solo 的启示**：Solo 的多 Agent 协作（Coordinator + Specialist）应优先基于 A2A 语义设计，确保未来可与外部 agent（Claude Code sub-agents、Codex、Goose 等）互操作。项目级 Agent Card 应随仓库同步，让外部协作者（包括其他 agent）理解项目规范。

### 0.3 OpenAI 的开放与锁定并存

- **Codex CLI 开源**：OpenAI Codex CLI 以 Apache-2.0 开源（Rust），默认沙箱化执行，支持本地 OSS 模型（`--oss`）。
- **Open Responses**：OpenAI 2026 年 2 月发布开放标准，试图统一 agentic API 接口，Hugging Face、Vercel、OpenRouter、Ollama 等已支持。
- **Assistants API 退役**：OpenAI 计划在 2026-08-26 关闭 Assistants API，生态被迫迁移到 Responses API / Conversations API。

**对 Solo 的启示**：Codex Provider 后端已落地（v0.11，含 `exec resume` 会话恢复）；Local API Proxy 可增加 Open Responses 兼容层，降低外部 agent 的接入摩擦。项目中的 `.codex/AGENTS.md` 应可由 Solo 一键生成。

### 0.4 开源 Agent 与 BYOK 模型爆发

- 开源 CLI agent 竞争激烈：OpenCode（~170k stars）、OpenAI Codex CLI（~90k）、OpenHands、Cline、Pi、Goose、Aider 等。
- 模型选择多元化：Kimi K2.7-Code（开源权重，256K 上下文）、DeepSeek-V3.2、Qwen3、Llama 4 等成为成本敏感场景的可选项。
- 地缘政治风险：2026 年 6 月 Claude Fable 5 因出口管制暂停全球访问，凸显"不绑定单一模型/厂商"的必要性。

**对 Solo 的启示**：Provider Hub 的智能路由与多 Provider 中立定位从"nice-to-have"升级为"风险对冲能力"。项目级路由规则（如"本项目审查用 Kimi，实现用 Claude"）比全局规则更重要。

### 0.5 安全与成本成为核心关切

- **MCP 安全**：Capability attestation、SAFE-MCP、OWASP Agentic Top 10（2026）推动工具调用前授权与 manifest 校验。
- **成本治理**：GitHub Copilot 于 2026-06-01 转向按量 AI credits，用户更关注 token 预算、模型路由与成本可视化。
- **长时任务可靠性**：agent 任务从分钟级扩展到小时级，需要状态持久化、断点恢复、Push 通知与人工门控。

**对 Solo 的启示**：本地优先 + E2EE + 成本治理 + 崩溃恢复，构成 Solo 区别于云 agent 的核心壁垒。

### 0.6 "项目为中心"正在重塑个人工作流

- **Cursor Composer**、**Claude Code Project Knowledge**、**Devin workspace**、**Aider repomap** 都把"项目工作目录"当作核心上下文容器，而非孤立的聊天会话。
- **AGENTS.md / CLAUDE.md** 成为项目规则的通用载体：像 `README.md` 服务人类一样，服务 agent。
- **多工具并行成为常态**：开发者同时使用 Cursor（IDE）、Claude Code（终端）、Devin（云端）、Aider（本地），但每换一次工具就要重新说明项目规则。

**对 Solo 的启示**：Solo 的机会不是再做一个 agent，而是成为"项目级 AI 能力的聚合层与配置中枢"——让项目规则、记忆、工具、模型、工作流围绕工作目录一次配置，处处生效。

### 0.7 Loop Engineering 成为行业共识

2026 年中，"Loop Engineering"从 Claude Code 负责人 Boris Cherny 和 OpenClaw 创始人 Peter Steinberger 的公开讨论迅速演变为 AI 原生开发的核心范式。Andrew Ng 将其定义为"人不再写代码，而是设计让 AI 自主写代码的循环系统"，提出三层循环模型（内环分钟级编码迭代 → 中环小时级产品决策 → 外环天/周级用户验证）与六大构建块（Automation / Worktree / Skill / MCP / Sub-Agent / Memory）。Shubham Saboo 从 PM 视角提炼出循环五要素（Trigger / Action / Proof / Memory / Stop），强调 Stop condition 是最被低估的部分，并提出 artifact（可复用、可版本化、可评估的项目知识载体）是 PM 在 AI 时代的核心投资资产。Addy Osmani 则系统性地指出了三大暗面风险：验证债（"完成"是声明不是事实）、理解债（产出速度远超理解速度）、认知投降（用 Loop 替代思考而非放大思考）。

**对 Solo 的启示**：

- **三层循环模型直接映射 Solo 架构**：Solo 已有的 Loop（Task iterate 模式）对应内环，Coordinator / 人工评审对应中环，External Feedback Loop 对应外环。三层通过 Product Spec（项目规则 + AGENTS.md）统一接口。
- **五要素框架验证 Solo 的 Stop 条件设计**：Solo Task 的 `done` / `failed` / `blocked` / `maxIterations` / 预算门控完整覆盖了 Saboo 所强调的"能说'不'的循环才是生产级循环"。
- **Artifact 概念强化 Project Memory 定位**：Solo 的 AGENTS.md / CLAUDE.md / 代码地图 / ADR 不仅是配置，而是 Saboo 所说的"可投资资产"——被复用时产生复利，但需要版本化、评估、防 drift。
- **三大暗面风险必须纳入设计**：验证债、理解债、认知投降是 Solo 的 Evidence Gate / Human Confirm Gate / Intervention Primitive 必须对抗的系统性风险。

### 0.8 交付工程化崛起：Task 成为人机协作的基本单位（Multica 研判）

2026 年下半年最值得注意的产品信号是 **[Multica](https://github.com/multica-ai/multica)**（45.2k stars，2026-08；Apache 2.0 + 附加条款；Go 后端 + Next.js + PostgreSQL，自托管，支持 20 种 agent CLI）。它的定位一句话说清了行业方向：

> "**Agents that show up on the board.**" —— 像给队友派活一样把 issue 派给 AI coding agent：agent 领取任务、汇报进度、提出阻塞、交还评审。

Multica 的 VISION.md 提出三个与 Solo 高度共鸣但更进一步的理念：

1. **人机是一个团队**："People set direction and remain accountable. Agents keep the work moving."——agent 不是聊天对象，而是有 assignee、有任务生命周期、有交付责任的一等队友。
2. **评审工作产物，而非"状态表演"（status theatre）**：plan、diff、测试结果、未决问题才是评审对象，而不是 agent 自述的"我完成了"。
3. **历史不随 session 消失**：intent、决策、行动、产物、结果保持关联，"the next agent does not start from zero"。

其领域模型与工程实践中有多项已被生产验证的机制（详见 [Multica 能力调研](../analysis/multica-capability-research.md)）：

- **Issue 是一等实体**：完整状态机（`backlog / todo / in_progress / in_review / done / blocked / cancelled`）、可路由 KEY（`MUL-123`）、每 issue KV metadata（≤50 key / 8KB，专存"会被后续 run 重读"的高信号状态）。
- **run ≠ done**：一次执行（task/run）完成只代表运行结束；issue 的 `done` 由人或合并条件判定；无活动 run 且无待重试时 `in_progress` 自动回退 `todo`。
- **失败二分法**：平台侧瞬时故障（runtime 掉线、daemon 重启、超时）自动重试 ≤2 次；`agent_error.*`（鉴权 / 配额 / context overflow）永不自动重试，先修因再手动 retry。
- **执行目录治理**：issue 级隔离目录 + 目录锁（`waiting_local_directory` 排队态）、bare clone + git worktree 共享缓存、四类 GC TTL。
- **低权限 Git 闭环**：GitHub App 只申请只读权限，靠 issue KEY 约定自动关联 PR；merge-to-Done 需同时满足 close-intent 已合并 PR + 无其他 open/draft PR + issue 未关闭。
- **平台可被 agent 操作**："Every surface is scriptable. Agents drive Multica through the same CLI you do."——multica-cli skill 教任何本地 agent 安全地读写 issue、回写进度、处理 mention 副作用。
- **Squad 协作协议**：leader agent 通过精确的 `mention://agent/<uuid>` markdown 派单 + 再触发规则表 + 去重防循环，把多 agent 协作落到 plain-text 协议。

更重要的是生态信号：社区项目 [oh-my-multica](https://github.com/multica-ai/multica/discussions/5545) 在 Multica 的执行地基（work items、task queues、runtimes、skills、run history）之上补建了**跨 run 的交付控制层**——Planner/Orchestrator agents 产出设计、验收定义与 **manifest DAG**，再由**确定性 Loop** 负责就绪节点派发、结果收集、**evidence gates**、**bounded rework**、中途恢复与 **merge conditions**。这说明"交付控制层"是 agent 平台的公认缺口：**执行可以交给概率性的 agent，交付必须交给确定性的系统。**

**对 Solo 的启示**：

1. **Task（工作项）必须成为一等实体**。Solo 的 Product+Task Unification PRD（6 概念 → 2 概念）方向被 Multica 完整验证，且应吸收其状态机语义（`in_review` / `blocked` / 自动回退）。
2. **"完成"由证据与人/合并条件判定，而非 agent 自述**——与 §0.7 的验证债对策合流，构成 Evidence Gate 的工程设计依据。
3. **确定性编排 × 概率性执行**：编排层（DAG 派发、返工、合并判定）必须是确定性代码；LLM 只做规划与执行。
4. **运维深度是护城河**：目录锁、worktree 缓存、GC、daemon 自更新、session 续跑是"真跑过生产"的痕迹，Solo daemon 应对照补齐。
5. **平台要被 agent 可操作**：Solo CLI 应升级为 agent 一等接口，配套操作 skill，让外部 agent 能安全地驱动 Solo。
6. **Solo 的差异化不变**：Multica 是云端团队产品（Postgres 中心化、多租户 workspace）；Solo 坚持 **local-first + E2EE + 移动端对等工位**，做"个人与小微团队的本地交付工作台"。

### 0.9 开发方法论基础：PADD × 控制论 × 全栈矩阵

Solo 的开发方法论建立在三个相互支撑的理论框架之上，它们共同回答"做什么、怎么做、谁来做、如何验证"：

#### 0.9.1 PADD：产品与架构双驱动

Solo 采用 **Product & Architecture Driven Development（PADD）**——产品价值为北极星，架构约束为长期底座，双线并行、相互制衡：

- **产品侧**回答"做什么、先做什么"：项目为锚点的上下文聚合、AI-Native 工位、Task 交付闭环。
- **架构侧**回答"怎么做、边界在哪"：四层聚合模型、External Agent Bus 协议、分层验证体系。
- **冲突协商规则**：短期迭代允许适度妥协，但所有妥协必须记录在 ADR（Architecture Decision Record），设定偿还时间窗口；架构方案必须绑定产品路线图，不设计产品永远用不上的能力。

> **对 Solo 的启示**：每个季度路线图评审必须同时通过产品影响评审与架构影响评审。需求变更不是"产品说了算"或"架构说了算"，而是双向评审后协商。

#### 0.9.2 工程控制论：AI Coding 的本质

AI Coding 的工程本质是**控制论**——通过反馈机制，在不确定环境中维持期望的稳态。Solo 的 Task / Delivery Loop / Verification 体系本质上是控制论四要素的实现：

| 控制论要素 | Solo 实现 |
|:----:|:-----|
| **约束** | AGENTS.md / 架构规则 / 权限边界 / 沙箱 / 执行前后 hook |
| **反馈** | 编译 → 测试 → E2E → LLM-as-judge → 业务指标（六层验证，§4.8） |
| **评估** | Evidence Gate / 验收条件 / 结构化评分卡 |
| **演化** | 失败案例 → 新约束 / 新测试 / 规则迭代（Self-improvement） |

> **核心洞察**：LLM 首次让"传感器"（语义理解/评估）和"执行器"（代码生成/重构）在语义层统一，使得软件创造层的反馈回路得以闭合。Solo 的工作是为这个回路构建可控的"调速器"。

#### 0.9.3 全栈矩阵：人 × 任务 × 组织的匹配

Solo 团队的任务分配与能力建设遵循**全栈矩阵**（三维度 × 三层级）：

- **横轴（问题尺码 S–XXL）**：由影响（错了会怎样）× 规模（跑起来多大）× 上下文（懂它要多少背景）三维取最高档。Solo 当前核心功能多在 L–XL 码（核心业务模块 / 平台级服务）。
- **纵轴（能力层级 L1–L3）**：L1 技能全栈（实现面齐）→ L2 职能全栈（交付链齐）→ L3 领域全栈（领域判断齐）。
- **AI 时代的刻度变化**：L1 被压缩（生成商品化），单人可承载尺码上移（OPC 成立），L3 验证价值上升（生成变便宜，验证成瓶颈）。

> **对 Solo 的启示**：
> - 季度路线图中的每个功能应标注问题尺码（S–XXL），用于匹配执行者层级。
> - Solo 自身的开发团队应优先在左下区（S~M）试行组织先行（端到端交付），在右上区（XL~XXL）坚持工具先行（先建验证器和约束体系再调组织）。
> - Solo 作为产品，其价值正是帮助外部开发者在各自项目中实现"AI 放大单人承载尺码"——从 L1/M 上移到 L2/L。

#### 0.9.4 三框架协同

```
PADD（做什么 + 底线）
  ├── 产品侧：路线图优先级、用户价值、PR/FAQ
  └── 架构侧：约束体系、ADR、分层验证
        │
控制论（怎么跑）
  ├── 约束 → 反馈 → 评估 → 演化
  └── Loop Engineering / Delivery Loop 是控制论在 AI Coding 的具体实现
        │
全栈矩阵（谁来做 + 做多大）
  ├── 问题尺码定任务难度
  ├── 能力层级定执行者
  └── 容错率定变革顺序
```

三者统一于一个闭环：**PADD 定义目标与约束 → 控制论驱动执行与验证 → 全栈矩阵匹配人与任务 → 验证结果反馈回 PADD 的下一轮对齐**。

---

## 1. 核心理念：以 Product 为锚点、以 Task 为单位，释放个人生产效率

### 1.1 问题：工具碎片化与"保姆式 Agent 管理"正在吞噬开发者的心流

现代开发者每天要在多个上下文之间切换：IDE、终端、浏览器、聊天窗口、手机通知、不同 AI agent。研究表明，知识工作者每次上下文切换后需要 **10–23 分钟** 才能恢复深度思考状态；开发者同时维护多个项目时，重复解释"这个项目的规范、技术栈、偏好"的成本极高。

AI coding agent 放大了这个问题。Multica 的 README 精准描述了 2026 年的典型困境：

> 每个 agent 各居一个 terminal tab、session 结束即失忆、人反复重述上下文——"the more agents you add, the more of your day goes to babysitting them"。

具体表现为：

- 每个 agent 都有自己的配置文件（`.cursorrules`、`.claude/CLAUDE.md`、`.codex/config.json`、`.aider.conf.yml`、`.opencode/opencode.json`）。
- 每个会话都是"冷启动"，agent 需要重新探索项目结构；session 结束即失忆。
- 每次模型切换都要重新说明上下文。
- 不同工具对同一项目的规则理解不一致，导致输出风格、构建命令、安全策略出现漂移。
- 背景任务与实时会话之间缺乏共享上下文，人离开后 agent 容易"失忆"。
- **agent 的产出散落在各个会话里**：没有"一件事"的统一载体，人无法在碎片时间快速回答"这件事做到哪了、证据是什么、还差什么"。

Solo 自身也存在同类问题：当前有 6 个用户可感知概念（Project / Workspace / Agent / Loop / Schedule / Tmux Pane），用户必须理解全部交叉关系才能有效使用产品——Loop 和 Schedule 都是"让 agent 做事"却是两套心智模型；Agent 是执行器却被暴露为一等管理对象；Tmux Pane 是实现细节却迫使用户直接管理。

### 1.2 解：收敛到两个用户概念——Product 与 Task

> **Solo 的核心设计原则：一切能力围绕项目工作目录聚合，一切工作以 Task 为载体流转。**
>
> **Solo 的北极星指标：让每个项目的 AI 配置与上下文从"重复劳动"变成"一次投入、持续复利"；让每件事的交付从"保姆式盯梢"变成"看板式托付"。**

Solo 把用户心智模型收敛到两个概念（详见 [Product + Task Unification PRD](prd/product-task-unification.md)）：

- **Product（我在做什么）**：一个产品/项目的完整上下文容器——关联代码仓库、规则、记忆、MCP 工具、Provider 路由、验证基线。Project / Workspace 归约到 Product。
- **Task（要完成什么事）**：一件工作的统一载体——标题、意图、执行策略（once / iterate / interactive / delivery）、触发方式（manual / cron / webhook / on_complete）、执行历史（Runs）、证据与评审状态。Loop / Schedule / 后台值守全部归约到 Task 的 mode 与 trigger。

用户只需回答两个问题：**我在做什么产品？这个产品下有什么事要做？** 其余一切（Agent 配置、tmux pane、循环、定时）降为内部实现。

以 `~/work/solo` 为例，Solo 应该做到：

1. **进入项目即加载上下文**：AGENTS.md、CLAUDE.md、项目记忆、MCP 工具、Provider 路由规则自动注入。开发者无需再向每个 agent 重复介绍项目。
2. **一次配置，处处生效**：项目规则同步到 Claude/Codex/Cursor/OpenCode/Aider/Continue/Goose。工具选择回归开发者偏好，项目规范由 Solo 统一保证。
3. **Task 看板托付工作**：人把意图变成 Task 指派给 agent，agent 像队友一样领取、汇报进度、提出阻塞、交付证据——人评审的是 plan、diff、测试结果，而非 agent 的"状态表演"。
4. **背景 agent 值守**：人离开后，cron/webhook 触发的 Task 继续基于项目目录执行，结果与证据沉淀回 Task 历史与项目记忆。
5. **移动端随时接管**：无论身在何处，都能在 Board / Inbox 中查看 Task 状态、审阅证据、干预纠偏，把碎片时间转化为项目推进时间。
6. **跨会话记忆累积**：项目知识与 Task 历史随使用增长，"下一个 run 不从零开始"，人工干预边际递减。
7. **成本与风险可控**：按项目设定预算、路由规则、沙箱策略；成本按 Run / Task 聚合，避免全局配置导致的安全与成本失控。

### 1.3 借鉴的思想框架

| 思想/框架 | 来源 | 在 Solo 中的映射 |
|---|---|---|
| **Second Brain（第二大脑）** | Tiago Forte | 项目记忆把项目知识外部化，让开发者的大脑专注于创造而非存储。 |
| **PARA 方法** | Tiago Forte | 按 Projects / Areas / Resources / Archives 组织项目记忆，主动项目进入高频上下文，归档项目进入低能耗存储。 |
| **Zettelkasten（卡片盒笔记法）** | Niklas Luhmann | 代码地图、ADR、失败案例作为原子笔记相互链接，支持涌现式洞察。 |
| **认知负荷理论** | John Sweller | 通过项目级预加载规则与记忆、以及 6→2 概念收敛，减少工作记忆负担。 |
| **心流理论** | Mihaly Csikszentmihalyi | 本地优先、低延迟、自动上下文注入、看板式托付，减少打断，延长深度工作时间。 |
| **OODA 循环** | John Boyd | Task iterate 模式的 Observe-Orient-Decide-Act 循环：观察 workspace → 判断状态 → LLM 决策 → 执行 step。 |
| **ReAct / Reflexion** | Yao et al. / Shinn et al. | agent 通过"推理→行动→观察"循环迭代，失败时反思并调整策略。 |
| **Loop Engineering** | Andrew Ng / Boris Cherny / Shubham Saboo | 三层循环（分钟/小时/天周）× 五要素（Trigger/Action/Proof/Memory/Stop）× 六大构建块；Task iterate 是内环实现，Project Memory 是 artifact 载体，Evidence Gate 是 Proof 的工程化，Human Confirm Gate 是 Stop 条件的工程化。 |
| **工作项团队协作模型** | Multica | Agent 是一等队友：Task 看板 + 状态机 + run≠done + KV metadata + Squad 派单；评审工作产物而非 status theatre。 |
| **确定性交付控制** | oh-my-multica | 概率性 Planner Agent 产出 manifest DAG，确定性 Delivery Loop 负责派发、证据门、有界返工、合并条件。 |
| **PADD（产品架构双驱动）** | 自研范式 | 产品价值为北极星、架构约束为底座；需求变更双向评审（产品影响 + 架构影响）；妥协记录在 ADR 并设偿还窗口；拒绝过度设计。 |
| **全栈矩阵（三维度 × 三层级）** | 自研框架 | 问题尺码（S–XXL）定任务难度，能力层级（L1–L3）定执行者；容错率定变革顺序（左下组织先行、右上工具先行）；AI 压缩 L1、放大单人承载、提升 L3 验证价值。 |
| **工程控制论** | Norbert Wiener / 工程实践 | 约束 → 反馈 → 评估 → 演化四要素；LLM 首次闭合语义层反馈回路；分层验证（静态→测试→运行时→非功能→语义→价值）驾驭概率性生成。 |

### 1.4 业界实践映射

| 业界产品/标准 | 现状与核心做法（2026-08） | Solo 的对齐策略 |
|---|---|---|
| **Multica** | ~45k stars；issue 为一等实体的 agent 团队协作平台；20 种 agent CLI runtime；run≠done 语义；Squad 派单协议；只读 Git 闭环 | 吸收 Task 状态机、失败二分法、KV metadata、执行目录治理、交付控制层设计；坚持 local-first + 移动端对等工位的差异化定位。 |
| **Claude Code** | ~131k stars；四级记忆层次（Managed/User/Project/Local）；五层 compaction pipeline | 自动读取项目 CLAUDE.md/AGENTS.md；把项目记忆作为对话前置上下文。 |
| **Cursor** | `.cursorrules`、Composer multi-file agent、codebase awareness | 导出 `.cursorrules`；提供项目级 codebase 索引给内部 agent。 |
| **Devin** | VM-based autonomous workspace；Plan → Execute → Verify | Task iterate 模式提供本地版 autonomous workspace，项目目录即 workspace。 |
| **Aider** | ~45k stars；repomap、git integration、multi-model | Project Memory 自动生成 repomap；Task 自动跑测试并修复。 |
| **OpenCode** | ~172k stars，最活跃的 open-source coding agent；75+ provider；Plan/Build 双 agent | 原生 OpenCode provider；项目级配置可双向同步；Task 可委托 Plan/Build specialist。 |
| **OpenHands** | ~75k stars；开源 autonomous coding、sandboxed CI runs | Task 可调用 OpenHands-style sandbox specialist 执行高风险任务。 |
| **Codex CLI** | ~90k stars；Terminal-Bench #1；沙箱化执行、AGENTS.md 支持、本地 OSS 模型 | Codex provider 已落地（v0.11）；导出 `.codex/AGENTS.md`；复用其沙箱思路。 |
| **Pi** | ~54k stars；Armin Ronacher 出品；sub-1k token system prompt；"lazy skills" | 借鉴其轻量 prompt 与 lazy skill 思想，优化 Project Memory 的上下文压缩。 |
| **Goose** | ~48k stars；已加入 Linux Foundation AAIF；MCP-first；70+ MCP extensions | Provider Hub 以 MCP 为统一工具层；支持 Goose 作为 external specialist。 |
| **Continue** | ~33k stars；跨 IDE 开源 assistant；支持 PR checks | Config Exporter 覆盖 Continue；项目规则注入其 IDE 扩展。 |
| **AGENTS.md 标准** | 项目级 agent 规则 markdown；与 MCP 并列为 AAIF 创始项目 | 项目根目录 AGENTS.md 自动注入所有接入 agent，并支持版本化、审计。 |
| **MCP Servers** | 官方仓库 86k+ stars；10,000+ 公开 server；97M 月下载 | Provider Hub 作为项目级 MCP 统一入口，支持 Server Cards 发现与安全 attestation。 |

### 1.5 Solo 的效率飞轮：从"重复劳动"到"持续复利"

Solo 的效率提升不是单次工具替换，而是围绕项目目录构建的复利系统：

```
            ┌─────────────────────────────────────────┐
            │         Product（项目工作目录）           │
            │  AGENTS.md · Memory · MCP · Provider    │
            └──────────────────┬──────────────────────┘
                               │
        ┌──────────────────────┼──────────────────────┐
        ▼                      ▼                      ▼
   ┌─────────┐           ┌─────────┐           ┌─────────┐
   │  人工作业 │           │ 背景值守 │           │ 多 Agent │
   │ 心流模式 │           │ cron/   │           │ Squad   │
   │         │           │ webhook │           │ 协作     │
   └────┬────┘           └────┬────┘           └────┬────┘
        │                      │                      │
        └──────────────────────┼──────────────────────┘
                               ▼
            ┌─────────────────────────────────────────┐
            │        Task 看板（工作的统一载体）         │
            │  意图 → Run → 证据 → 评审 → 合并          │
            └──────────────────┬──────────────────────┘
                               ▼
            ┌─────────────────────────────────────────┐
            │      项目记忆沉淀（越用越懂项目）          │
            │  ADR · 代码地图 · 偏好 · Run 历史与证据   │
            └─────────────────────────────────────────┘
```

**飞轮效应**：

1. **项目上下文一次投入**：Onboarding 时自动生成代码地图、ADR、AGENTS.md 草案，降低后续所有 agent 的探索成本。
2. **多工具一致性降低摩擦**：配置同步到 OpenCode/Codex/Cursor/Claude/Aider 后，开发者切换工具时无需重新解释项目规则，上下文切换成本从"10–23 分钟恢复心流"压缩到"秒级"。
3. **Task 历史消除重述**：每个 Task 的意图、Run、证据、决策永久关联，"下一个 run 不从零开始"，人无需向 agent 重复交代前情。
4. **人工干预边际递减**：项目记忆与 Task 历史越丰富，agent 输出越准，审查和纠偏时间越少。
5. **背景值守放大可用时间**：cron/webhook Task 在人离开时继续执行测试修复、依赖更新、文档生成等任务，把"非工作时间"转化为"项目推进时间"。
6. **证据沉淀形成信任复利**：每次交付的证据（测试、diff、评审结论）留痕，人对自治交付的信任基于可核查的历史，而非盲目的信心。

**北极星公式**：

> **个人生产效率提升 = Σ（每个项目的上下文切换时间节省 + 背景值守产出 + agent 输出准确率提升） / 配置与治理成本**

Solo 的目标是让分子持续放大、分母趋近于一次性的项目 Onboarding 投入。

### 1.6 GitHub Top N 开源趋势对产品设计的启示

2026 年上半年 GitHub 开源榜单所揭示的趋势，进一步验证了 Solo"以项目为锚点"方向的必要性，并指明了需要优先补齐的能力。

#### 1.6.1 开源 agent 大爆发，但"项目上下文"仍是最大摩擦点

GitHub 上 OpenCode、Cline、Goose、Aider、Pi、OpenHands、Codex CLI 等开源 agent 累计 stars 已超 600k，但几乎所有工具都假设"用户会手动把项目规则告诉 agent"。这带来两个机会：

- **配置中枢机会**：开发者需要一处管理所有工具的项目级配置，而不是在每个 agent 里重复维护 `.cursorrules`、`.codex/AGENTS.md`、`.claude/CLAUDE.md`、`.aider.conf.yml`。
- **上下文同步机会**：项目规则、MCP 工具、provider 路由、记忆应当以项目目录为单位版本化，并自动转写到所有接入 agent。

#### 1.6.2 MCP 成为"AI 的 USB-C"，但发现与安全仍是痛点

MCP 官方 servers 仓库 86k+ stars，公开 MCP server 超过 10,000 个，月 SDK 下载量近一亿。然而生产环境中面临：

- **发现难**：Anthropic registry、Smithery、Cline marketplace、OpenAI registry 索引互不统一。
- **安全风险**：MCP server 来源鱼龙混杂，Perplexity 为此专门开源了供应链扫描工具 Bumblebee。
- **能力不透明**：server 能做什么、需要什么权限，往往只有运行后才知道。

**Solo 的应对**：Provider Hub 不仅要聚合 MCP server，更要提供项目级的 Server Cards 发现、Capability Attestation、危险工具标记、来源审计，把 MCP 从"能连"推进到"敢连、易管"。

#### 1.6.3 背景值守与自治循环成为新战场

OpenHands、Devin、Codex CLI 都在强调"人离开后 agent 继续工作"。GitHub Copilot 也在 2026-06 转向按量 AI credits，推动用户把重复任务交给后台 agent。

**Solo 的应对**：Task 的 cron/webhook trigger 与 iterate mode 必须支持跨网络断连持久化、崩溃恢复、人工门控，让"每晚自动修复测试失败""自动更新依赖"等场景真正可托付。

#### 1.6.4 多 Provider / BYOK 从"可选项"变为"必选项"

OpenCode 支持 75+ provider，Cline/Aider/Goose 均支持本地模型，Codex CLI 也支持 `--oss` 本地推理。2026 年 6 月 Claude Fable 5 因出口管制暂停全球访问，再次证明不绑定单一厂商的必要性。

**Solo 的应对**：Provider Hub 的智能路由与项目级 provider 规则从 nice-to-have 升级为风险对冲能力。例如：

- 本项目审查用 Kimi，实现用 Claude，本地快速验证用 Ollama。
- 不同项目可指定不同默认模型，避免全局切换带来的上下文丢失。

#### 1.6.5 项目规则文件成为"新的基础设施"

andrej-karpathy-skills（156k stars）把四条编码原则写进一个 `CLAUDE.md` 就获得极高关注，说明开发者强烈需要把个人/团队的编码偏好沉淀为可复用、可共享的项目规则。

**Solo 的应对**：AGENTS.md / CLAUDE.md 不应是手写后束之高阁的文档，而应是 Solo 自动生成、自动注入、自动同步的"活配置"。Onboarding 时自动生成草案，Task 执行时自动读取，配置 exporter 时自动转写。

### 1.7 AI-Native 开发范式：Web/App 即开发工位

前述章节解决了"项目上下文如何聚合"与"工作以什么为载体"的问题，但尚未回答一个更根本的问题：**开发者在哪里、以什么方式完成开发工作？** 当前路线图隐含地假设"真正的代码编辑发生在 IDE 或终端里的外部 AI Coding Agent"，Solo 主要充当配置中枢与移动端指挥中心。这一假设在 2026 年 AI Coding Agent 能力跃迁之后已不再成立。

#### 1.7.1 范式转移：从"人写代码"到"人驱动 Agent 写代码"

传统开发流程以"键盘 → 代码编辑器"为核心，即便引入 AI Coding Agent，人的主工作仍是"在 IDE/终端里与 Agent 对话"。Solo 的下一步范式转移是：

> **开发行为本身应当完全在 Solo Web/App 内完成，外部 AI Coding Agent（Claude Code / Codex CLI / OpenCode / Cursor-Agent 等）作为 Solo 编排下的执行单元，而非人直接交互的主战场。**

这一范式的核心逻辑：

- **输入形态无关紧要**：键盘打字、系统 dictation、微信/豆包语音输入、截图粘贴，都只是"意图载体"。Solo 不需要自研语音/多模态能力，直接复用操作系统与 IME 提供的输入通道。
- **意图归一化是关键**：所有形态的输入由 Solo 的 **Intent Normalizer**（基于 LLM）归一化为结构化的 Task 定义（目标、约束、验收条件），交给外部 Agent 执行。
- **执行可观察、可干预**：外部 Agent 的执行流（diff、命令、推理链、工具调用）实时流式回传到 Solo Web/App，人随时可以插入指令打断或纠偏。
- **验收在 App 内闭环**：diff review、测试运行、staging preview、PR 发起与合并全部在 Web/App 内完成，不需要切到 IDE 或终端。

#### 1.7.2 Solo 作为 Meta-Agent 编排层的精确定位

Solo 不替代 Claude Code / Codex / OpenCode 等外部 AI Coding Agent——这些工具在代码理解与编辑能力上持续演进，Solo 应当充分利用而非重复造轮子。Solo 的精确定位是**外部 Agent 的编排、观测、干预、验收、记忆层**：

```
   人 (Web/App)
       │ 意图（文本/语音/截图）
       ▼
┌────────────── Solo (Meta-Agent Layer) ─────────────┐
│ 意图归一化 → Task → 编排 → 观测 → 干预 → 验收 → 沉淀 │
└────────────────────────────────────────────────────┘
       │ 结构化指令                   ▲ 执行流 + 结果
       ▼                              │
  外部 Agent                    外部 Agent
  (Claude Code 编辑代码)         (Codex CLI 跑测试)
```

#### 1.7.3 四个本质特征

Solo Web/App 要成为 AI-Native 开发工位，必须实现以下四个特征：

| 特征 | 含义 | 关键能力 |
|---|---|---|
| **意图是唯一输入**（Intent as Input） | 人只表达意图，形态不限 | Intent Normalizer 把口语/碎片文本归一化为结构化 Task 定义 |
| **执行全程可观察**（Observable Execution） | 外部 Agent 的工作实时流式可见 | Agent Execution Stream：diff / 命令 / 推理链 / 工具调用全部渲染到 Web/App |
| **干预是开发原语**（Intervention as Primitive） | 人随时可以插入指令打断或纠偏 | Inline Intervention Protocol：执行中向 Agent stdin 注入指令 |
| **验收在 App 内闭环**（In-App Verification） | 不再切到 IDE/终端完成验收 | In-App Diff Review / Test Runner / Staging Preview / PR 发起合并 |

#### 1.7.4 语音输入的战略简化

Solo 不自研语音识别（STT）。语音作为一种输入形态，由操作系统级与 IME 级能力承担：

- iOS / Android / macOS 原生 dictation
- 微信、豆包、讯飞等 IME 的语音输入
- Siri / Google Assistant 级快捷入口（远期）

Solo 只需保证：**任何文本输入通道产生的内容，经过 Intent Normalizer 后都能驱动 Agent。** 这把语音工作几乎归零，让 Solo 的资源聚焦于 Agent 编排与执行观测。

#### 1.7.5 架构影响：Tmux 作为 Agent 执行总线

AI-Native 范式要求 Solo 能够统一接入多种外部 Agent 而不为每个 Agent 写深度原生集成。这一需求天然契合 Solo 已有的 **Tmux Dashboard** 基础设施，并将其从"只读观测窗"升级为"**外部 Agent 的统一执行总线**"：

- 每个外部 Agent 在一个 tmux pane 中运行
- Solo daemon 通过 tmux API 读取 pane 的 stdout 流（观测）
- Solo daemon 可向 pane 发送 stdin 按键/文本（干预）
- Solo daemon 可新建、拆分、销毁 pane（编排）
- Solo daemon 捕获 pane 输出中的结构化事件（diff / tool_call / error）

只要 Agent 能在终端运行，就能被 Solo 编排——这是 Solo 在 AI Coding Agent 生态里独有的、可复用的架构资产。

#### 1.7.6 对路线图的影响

AI-Native 范式对后续章节的主要调整：

- Solo 定位从"客户端"升级为"Meta-Agent 编排层 + 交付工作台"。
- 架构在四层聚合模型之上叠加意图层、执行观测层、干预层、验收层。
- 三大支柱重新定位为"Task + Intent / Runtime Bus / Delivery + Verification"。
- 季度路线图新增 Task Board、Delivery Loop、Agent Execution Stream、Inline Intervention、Intent Normalizer、In-App Diff Review 等里程碑；调低 Config Exporter 与自研 Solo Agent 的优先级。
- 删除"移动端不是完整编辑"的旧假设：当 Agent 承担所有执行，Web/App 只需表达意图与验收结果，移动端不再是"缩小的桌面端"，而是对等的 AI-Native 工位。

### 1.8 Multica 能力借鉴地图

基于 [Multica 能力调研](../analysis/multica-capability-research.md)，Solo 对 Multica 的吸收分四档：

#### 1.8.1 领域模型对照

| Multica | Solo（v3.0 目标） | 说明 |
|---|---|---|
| Workspace | **Product** | Solo 的 Product 绑定本地 repo，天然 local-first；无多租户需求 |
| Issue（KEY / 状态机 / KV metadata） | **Task** | 吸收状态机语义、可路由 KEY、KV metadata 约定 |
| Task（一次执行记录） | **Run** | 历史永不覆盖；run ≠ done |
| Agent（可复用配置，非进程） | **AgentProfile** | 与 PRD 一致：profile = instructions + model + skills + runtime |
| Runtime（机器 × CLI） | **External Agent Bus 上的 Runtime** | tmux pane + provider adapter 组合即 runtime；补注册/心跳/并发上限 |
| Skill | **Skill**（`~/.solo/skills/`） | 文件系统即库；补"从 runtime 扫描导入" |
| Squad（leader + mention 派单） | **Squad**（Q1 2027） | 吸收 Squad Operating Protocol 与防循环规则表 |
| Autopilot（cron/webhook） | **Task trigger**（cron/webhook/on_complete） | Solo Schedule 已具备 cron 基础 |
| Inbox（仅人类） | **Inbox / Push Notifications** | agent 无 inbox，@agent 是执行触发 |
| Chat | **Task(mode=interactive) / 轻量问答** | 不挂 Task 的闲聊降级为辅助入口 |

#### 1.8.2 直接吸收（低成本，Q3–Q4 2026）

- **run ≠ done 语义与 Task 状态机**：`in_review` / `blocked` 状态、无活动 run 自动回退 `queued`。
- **失败二分法与有界自动重试**：平台侧故障自动重试 ≤2 次；`agent_error.*` 永不自动重试。
- **Task KV metadata**：≤50 key / 8KB，专存跨 run 高信号状态（PR 号、pipeline 状态、阻塞原因）；反模式写进文档。
- **执行目录治理**：Task 级隔离目录 + 目录锁（`waiting_local_directory` 排队态）、git worktree 共享缓存、产物 GC TTL 表。
- **daemon 自愈对照**：心跳判离线、重启 reclaim 未完成 run、自更新等运行任务结束后跟随。

#### 1.8.3 改造吸收（中成本，Q4 2026–Q1 2027）

- **只读 Git 闭环**：复用 v0.11 已有的 GitHub issue/PR 搜索 RPC，按 Task KEY 约定（分支名/标题/close intent）自动关联 PR，merge-to-Done 条件判定。
- **成本按 Run / Task 聚合**：token 用量聚合到 Task 维度，支撑预算告警与路由决策。
- **mention 即触发路由**：Squad leader 派单协议 + 再触发规则表 + 去重防循环（Q1 2027）。
- **CLI 升级为 agent 一等接口**：`solo` CLI 全面脚本化 + 配套操作 skill，让外部 agent 安全驱动 Solo。

#### 1.8.4 作为需求证据（高成本，谨慎推进）

- **manifest DAG + deterministic Delivery Loop + evidence gates + bounded rework + merge conditions**：oh-my-multica 证明了需求真实存在，但方案本身是社区实验。Solo 分两步走：先做**单 Task 的证据门**（Q4 2026），再做多节点 DAG 交付（Q4 2026–Q1 2027），每阶段保留可回退的简化版。

#### 1.8.5 明确不做

- **云端多人协作 workspace**：Multica 的多租户团队模型依赖 Postgres 中心化，与 Solo local-first 定位冲突；团队协作以"共享配置 + Git 为单一事实源"的轻量方式覆盖。
- **IM bots（Slack/Lark/钉钉/企微）**：Solo 的移动端 App + Push 已覆盖"随时接管"场景，IM 渠道列为 P3 以后评估。
- **Agent 的 Inbox**：遵循 Multica 语义——对 agent 的 @ 是执行触发，不是通知。

---

## 2. 产品愿景

> **Solo 是开发者的本地 AI-Native 交付工作台。**
>
> 它以 Product（项目工作目录）为锚点、以 Task（工作项）为基本单位：人表达意图，Solo 把意图变成 Task 并派给外部 AI Coding Agent（Claude Code / Codex CLI / OpenCode 等）执行——Agent 像队友一样在看板上领取任务、汇报进度、提出阻塞、交付证据。人评审的是工作产物（plan、diff、测试结果），而非 agent 的"状态表演"。交付由确定性 Delivery Loop 把关：证据门、有界返工、合并条件，确保"完成"是事实而非声明。无论桌面还是移动端，都是对等的 AI-Native 工位，而非"监控面板"。

### 2.1 核心差异化

| 差异化 | 说明 |
|--------|------|
| **Task 为一等实体** | Product + Task 两个用户概念承载全部工作：Task 拥有状态机（含 `in_review` / `blocked`）、可路由 KEY、Run 历史、KV metadata。Chat / Loop / Schedule 不再是并列概念，而是 Task 的 mode 与 trigger。 |
| **run ≠ done** | 一次 Run 完成只是运行结束；Task 的 `done` 由人评审或合并条件（merge-to-Done）判定。无活动 Run 时 Task 自动回退待办。 |
| **确定性交付循环** | 编排层是确定性代码（DAG 派发、证据门、有界返工、合并条件、恢复），LLM 只做规划与执行——"项目完成不变成某一个 agent 的一家之言"。 |
| **证据驱动验收** | 每次交付附带结构化证据（测试、diff stat、CI、截图、独立 Reviewer 结论），沉淀为可审计、可回放的 Run 历史。 |
| **Meta-Agent 编排层** | 不替代 Claude Code / Codex / OpenCode 等外部 AI Coding Agent，而是作为其编排、观测、干预、验收、记忆层。外部 Agent 是执行单元，Solo 是工位。 |
| **AI-Native 对等工位** | Web 与 App 在能力上对等：表达意图、观察执行、干预纠偏、验收结果。移动端不再是"缩小的桌面端"或"监控面板"。 |
| **意图归一化** | 任何来源的输入（键盘、系统 dictation、微信/豆包语音、截图粘贴）经过 Intent Normalizer 后都能驱动外部 Agent，输入形态是 OS/IME 的事，Solo 只消费归一化后的意图。 |
| **本地优先 + E2EE** | 代码和配置默认不出本机；远程访问通过端到端加密中继。与 Multica 等云端团队产品的根本差异。 |
| **项目为锚点** | 每个项目工作目录拥有独立的规则、记忆、工具、模型路由。 |
| **背景 Agent 值守** | 本地 daemon 7×24 运行 cron/webhook Task，人离开时 agent 继续工作。 |
| **多 Provider / 多 Agent 中立** | 不绑定单一模型，支持 Claude、Kimi、OpenCode、Codex、Cursor-Agent、本地 OSS 模型等。 |
| **MCP 原生工具中枢** | 统一管理 MCP server，跨 Solo 内部 Agent 与外部 Coding Agent 同步。 |
| **配置 + 规则中枢** | 一套 Provider/MCP/Skill/Prompt/AGENTS.md 配置，按项目同步到多个 Coding Agent。 |
| **成本可观测** | 统一用量、token、延迟监控，按 Run / Task / Product 聚合，支持基于成本的智能路由。 |
| **跨会话项目记忆** | 代码地图、ADR、偏好、历史尝试随项目沉淀，减少重复沟通。 |
| **Human-Agent Handoff** | 人在 Web/App 开始任务，可平滑移交给后台 Task；Task 需要决策或遇到阻塞时，通过 Inbox/App 推送将控制权交还给人。 |
| **Agent-to-Agent Handoff** | 基于 A2A 语义与 Squad 派单协议，Coordinator 可把子任务连上下文一并委托给 Specialist Agent；任务完成后 Specialist 把结果与状态交回 Coordinator。 |
| **平台可被 Agent 操作** | Solo CLI 是 API 的一等客户端，配套操作 skill，让外部 agent 能安全地创建 Task、回写进度、读取上下文。 |

---

## 3. 2026 战略目标

1. **完成 Product + Task 概念统一**：把 6 个用户可感知概念（Project / Workspace / Agent / Loop / Schedule / Tmux Pane）收敛为 2 个（Product / Task），Task Board + Inbox 成为 Web/App 的主界面。
2. **把 Solo 打造为外部 AI Coding Agent 的 Meta-Agent 编排层**：不替代 Claude Code / Codex / OpenCode 等外部 Agent，而是作为其编排、观测、干预、验收、记忆层，让 Web/App 成为对等的 AI-Native 开发工位。
3. **建立确定性交付循环（Delivery Loop）**：manifest DAG 派发、Evidence Gate、有界返工、合并条件、中途恢复——让多步骤交付可托付、可审计、可恢复。
4. **把 Solo 从"客户端"升级为"项目级 AI 中枢"**：本地 daemon 7×24 值守，移动端随时接管，项目目录即上下文入口。
5. **实现自治开发循环**：Task（iterate / cron / webhook / on_complete）让 Agent 能基于项目目录自主计划、执行、验证、修复。
6. **成为 AI Coding Agent 的配置中枢**：Provider Hub 统一管理和转写配置到 Claude/Codex/Cursor/OpenCode 等，按项目生效。
7. **成为 MCP 原生工具层**：MCP server 一次配置，多处生效；支持 Server Cards 发现与安全 attestation。
8. **成为 AGENTS.md / CLAUDE.md 原生工具**：项目规则自动注入所有接入 Agent，降低重复沟通。
9. **建立项目级长期记忆**：跨会话、跨 Agent、跨 Run 的项目知识沉淀，支持知识图谱检索。
10. **覆盖完整研发交付流**：从编码到测试、审查、PR、合并，全部在项目目录内完成闭环，且验收全程在 Web/App 内。
11. **建立成本治理能力**：用量监控、预算门控、模型路由，让 Agent 规模化使用可控。
12. **面向 A2A / Open Responses 开放生态**：多 Agent 协作与外部 API 兼容层提前布局；Solo CLI 升级为 agent 可操作的一等接口。

---

## 4. 架构：领域模型 + 聚合模型 + AI-Native 工位模型

Solo 的架构由三个维度构成：

- **领域模型**：Product / Task / Run / Evidence 四个核心实体及其状态机（§4.1）。
- **纵向聚合模型（四层）**：围绕项目工作目录聚合上下文、工具、智能体、交互能力（§4.2–4.5）。
- **横向 AI-Native 工位模型（四层）**：贯穿 Web/App 与外部 Agent 的意图 → 编排 → 观测/干预 → 验收闭环（§4.6）。

### 4.1 领域模型：Product / Task / Run / Evidence

详细数据模型见 [Product + Task Unification PRD](prd/product-task-unification.md)；本节定义 v3.0 在 PRD 基础上吸收的 Multica 语义增强。

#### 4.1.1 实体关系

```
Product（我在做什么）
  └── Task（要完成什么事）── KEY: {productPrefix}-{seq}，如 SOLO-123
        ├── Execution（怎么跑：once / iterate / interactive / delivery）
        ├── Trigger（什么时候跑：manual / cron / webhook / on_complete）
        ├── Metadata（KV ≤50 key / 8KB：跨 Run 高信号状态）
        └── Run[]（每次执行的历史记录，永不覆盖）
              ├── Evidence（结构化交付证据）
              └── Cost（token / 时长 / 费用）
```

#### 4.1.2 Task 状态机（吸收 Multica 语义）

```
queued → running → in_review → done
   ↑        │  ↘ blocked（agent 提出阻塞，等待人决策）
   │        ↓
   │      failed
   └────────┘  无活动 Run 且无待重试时自动回退 queued
```

核心规则：

- **run ≠ done**：Run 完成只把 Task 推到 `in_review`；`done` 由人评审或 merge-to-Done 条件（§4.7.5）判定。
- **blocked 是一等状态**：agent 遇到需要人决策的情况（权限、歧义、依赖缺失）主动置 `blocked` 并写明阻塞原因（存入 metadata），经 Inbox/Push 通知人。
- **自动回退**：无活动 Run 且无待重试时，`running` 自动回退 `queued`，避免"僵尸进行中"。

#### 4.1.3 Run 与失败二分法

Run 状态机：`queued → dispatched → waiting_local_directory → running → completed / failed / cancelled`。

- `waiting_local_directory`：Task 级隔离执行目录被占用时排队等待（目录锁，见 §4.4）。
- **失败分类**（`failureKind`）：
  - **平台侧瞬时故障**（runtime 掉线、daemon 重启回收、超时、网络中断）：自动重试 ≤2 次。
  - **agent 自身错误**（`agent_error.*`：鉴权失效、配额耗尽、配置错误、context overflow）：**永不自动重试**，先修因再手动 retry。
- **会话恢复**：retry 时若 session 安全且同一 runtime 认领，续上之前的 session；context overflow 等毒化 session 的错误在原工作目录开新 session。

#### 4.1.4 Evidence（交付证据）

每次 Run 结束产出结构化证据，作为 `in_review` 的评审对象：

```json
{
  "diffStat": {"files": 5, "additions": 120, "deletions": 34},
  "verifyResults": [{"check": "make ci", "pass": true, "durationMs": 43200}],
  "testSummary": {"total": 86, "passed": 86, "coverage": 0.9718},
  "ciStatus": "success",
  "prRef": {"number": 42, "url": "..."},
  "reviewerVerdict": {"agent": "reviewer-profile", "verdict": "approve", "notes": "..."},
  "screenshots": ["..."]
}
```

证据是 Evidence Gate（§4.7.3）的输入，也是项目记忆的沉淀素材。

#### 4.1.5 概念归约映射

| 现有概念 | 归约到 | 理由 |
|----------|--------|------|
| Project | **Product** | 语义更准确——不只是 repo，是"我在做的东西" |
| Workspace | Product 的运行态 | tmux session 是 Product 的执行环境，不是独立概念 |
| Loop | Task（execution.mode=iterate） | 迭代验证是执行策略 |
| Schedule | Task（trigger.type=cron） | 定时触发是触发方式 |
| Agent | Task 的内部执行器（AgentProfile） | 用户不需要直接"管理 agent" |
| Tmux Pane | Task 的实时视图 | 执行时的窗口，不是管理对象 |

### 4.2 上下文层（Context Layer）

项目是所有 AI 上下文的容器。

- **AGENTS.md / CLAUDE.md**：项目规则文件，声明技术栈、构建命令、安全策略、代码风格。Solo 自动注入所有接入 Agent。
- **Project Memory**：代码地图（repomap）、架构决策记录（ADR）、用户偏好、历史尝试、失败案例。
- **Session Memory**：跨会话的 turn 记录，支持长期对话连续性。
- **Task 历史与证据**：每个 Task 的意图、Run、证据、评审结论永久关联，是"下一个 run 不从零开始"的基础。
- **Knowledge Graph**：函数/模块/依赖关系结构化存储，支持语义检索。

### 4.3 工具层（Tool Layer）

工具配置随项目生效，而不是全局生效。

- **MCP Servers**：数据库、浏览器、GitHub、Notion 等工具按项目启用。
- **Skills**：可复用的 Agent 能力包（如 `go-backend`、`testing`、`frontend-ui`），支持从 runtime 机器扫描导入。
- **Prompts**：项目级 system prompt 预设。
- **Local API Proxy**：外部 Agent 指向 Solo 端口，自动走项目配置的 provider/model。
- **Config Exporter**：把项目配置转写到 `.cursorrules`、`.codex/AGENTS.md`、`.claude/mcp.json` 等。

### 4.4 智能体层（Agent Layer）

智能体层的核心定位是"**外部 AI Coding Agent 的编排与委托**"，并吸收 Multica 的 Runtime 抽象与 daemon 运维深度：

- **Provider Hub / Router**：按项目选择最优 provider/model，支持成本、延迟、能力路由。
- **External Agent Bus → Runtime 抽象**：外部 Agent（Claude Code / Codex CLI / OpenCode / Aider / Goose 等）在 tmux pane 中运行；每个"daemon 主机 × agent CLI"组合注册为一个 **Runtime**，具备：
  - **注册与心跳**：daemon 启动时探测 PATH 上的 agent CLI 并注册 runtime；15s 心跳，心跳断约 3 分钟标记 offline。
  - **并发上限**：整机级与单 profile 级并发控制（参考 Multica：整机 20 / 单 agent 6）。
  - **执行目录治理**：Task 级隔离目录 + 目录锁（`waiting_local_directory`）、bare clone + git worktree 共享缓存、产物 GC TTL（完成 24h / 孤儿 72h / 可再生产物 12h / repo 缓存 720h）。
  - **观测**：读取 pane 的 stdout 流；捕获结构化事件（diff / tool_call / error）。
  - **干预**：向 pane 发送 stdin 文本/按键。
  - **编排**：新建、拆分、销毁 pane。
- **External Agent Adapters**：为每种外部 Agent 编写 stdout 解析器（Claude Code 的 JSON stream、Codex CLI 的 TTY 输出、OpenCode 的 stream 格式）。
- **Solo Agent（轻量 fallback）**：仅用于简单问答与元操作，重任务一律委托外部 Agent。
- **Delivery Loop**：确定性交付控制器，见 §4.7。
- **A2A Specialist Agents**：把子任务委托给外部 specialist（如安全审查、文档生成）。
- **Agent-to-Agent Handoff**：基于 A2A Task 语义在 Coordinator 与 Specialist 之间移交任务上下文、中间状态与执行结果，支持跨进程、跨厂商 agent 协作。

### 4.5 交互层（Interface Layer）

- **Task Board**：Web/App 主界面。按 Product 组织的 Task 看板（Running / Queued / Blocked / In Review / Done），卡片显示实时进度（"iter 3/5"、"running 2m"、"next: 14:00"）。
- **Inbox / Push Notifications**：人的通知中心——Task 完成待评审、阻塞待决策、@提及、指派。关键决策点主动推送，不打断心流。**Agent 没有 Inbox**：对 agent 的 @ 是执行触发而非通知。
- **Chat / Coordinator**：项目级指挥中心；不挂 Task 的轻量问答作为辅助入口保留。
- **Tmux Agent Bus（升级自 Tmux Dashboard）**：外部 Agent 的统一执行总线，不再是只读观测窗。
- **CLI**：API 的一等客户端——"every surface is scriptable"，人与 agent 通过同一个 CLI 驱动 Solo。
- **Human-Agent Handoff**：人在 Web/App 开始任务，可平滑移交给后台 Task；Task 阻塞或需审批时自动将上下文与可选动作推送给用户。

### 4.6 AI-Native 工位模型（横向四层）

AI-Native 工位模型与 §4.2–4.5 的聚合模型正交，贯穿 Web/App 与外部 Agent 的端到端开发流：

#### 4.6.1 意图层（Intent Layer）

把任何形态的人类输入归一化为结构化的 Task 定义：

- **输入形态**：键盘文本、系统 dictation（iOS/Android/macOS）、微信/豆包/讯飞等 IME 语音输入、截图粘贴、PR 评论。
- **Intent Normalizer**：基于 LLM 把口语化/碎片化输入重写为结构化 Task 定义（含目标、约束、验收条件）。
- **Solo 不自研 STT**：语音识别由 OS/IME 承担，Solo 只消费归一化后的意图文本，把研发资源聚焦在 Agent 编排与执行观测。

#### 4.6.2 编排层（Orchestration Layer）

Coordinator 接收结构化意图后拆解并分发：

- **任务拆解**：把大需求分解为可串行/并行的 Task DAG（manifest，见 §4.7）。
- **Agent 选择**：按任务类型、成本、延迟、能力路由到合适的外部 Agent（如 Claude Code 做架构、Codex CLI 跑测试、Kimi 做审查）。
- **Runtime 调度**：在 External Agent Bus 中申请 runtime、新建/复用 pane 启动对应 Agent。
- **A2A Task 委托**：对支持 A2A 的外部 Agent，按 A2A Task 语义移交上下文与中间状态。

#### 4.6.3 观测/干预层（Observable Execution Layer）

外部 Agent 的执行流实时流式渲染到 Web/App：

- **观测**：diff、命令输出、推理链、工具调用、错误全部以结构化方式呈现。
- **干预（Inline Intervention Protocol）**：人随时可以插入指令打断 Agent，Agent 实时响应。
- **执行状态机**：与 Task / Run 状态机统一（§4.1）。
- **Human Confirm Gate 泛化**：从"危险操作的保险"升级为"开发循环内的原语"，Agent 提出 diff 后由人在 App 内 approve/comment。

#### 4.6.4 验收层（In-App Verification Layer）

验收闭环全程在 Web/App 内完成，不再切出到 IDE 或终端：

- **Evidence Review**：Task 的 `in_review` 界面以证据为中心——diff、测试结果、CI、截图、Reviewer 结论一屏呈现。
- **Diff Review**：PR 级别的 diff 渲染，支持行级评论，Agent 据此迭代。
- **Test Runner**：点击即可跑相关测试/构建/lint，结果在 App 内渲染。
- **Staging Preview**：一键启动 preview 环境，支持截图/录屏。
- **PR / Merge**：发起 PR、review、合并，全程在 App 内；合并满足条件后 Task 自动 `done`。
- **沉淀**：本次开发的 ADR、代码地图更新、偏好、失败案例、证据回流到 Project Memory 与 Task 历史。

### 4.7 确定性交付循环（Deterministic Delivery Loop）

Multica 生态的 oh-my-multica 实验证明：agent 平台提供执行地基后，**跨 run 的交付控制层**是决定"自治交付能不能被信任"的关键。Solo 的 Delivery Loop 遵循一个总原则：

> **确定性编排 × 概率性执行。** Agent 是概率性的（规划、编码、修复），交付控制必须是确定性的（派发、验收、返工、合并）——"项目完成不变成某一个 agent 的一家之言"。

#### 4.7.1 双层结构

```
┌── 概率性层（LLM Agent）────────────────────────────┐
│ Planner Agent：读仓库现状 → 设计 + 验收定义 +        │
│                manifest DAG（节点 = Task 定义）      │
│ Worker Agents：执行各节点 Task，产出 diff + 证据     │
│ Reviewer Agent：独立于 Worker 评审证据（不自评）     │
└────────────────────────────────────────────────────┘
┌── 确定性层（Go 代码，无 LLM）──────────────────────┐
│ Delivery Loop：                                    │
│  · 就绪节点派发（DAG 拓扑序 + 目录锁 + 并发上限）    │
│  · 结果收集与证据校验（Evidence Gate）              │
│  · 有界返工（bounded rework）                       │
│  · 中途恢复（daemon 重启后从 manifest 状态续跑）     │
│  · 合并条件判定（merge-to-Done）                    │
│  · 最终验收与升级人工                               │
└────────────────────────────────────────────────────┘
```

#### 4.7.2 Manifest DAG

Planner Agent 检查仓库现状后产出 manifest（可被人编辑的版本化文件）：

- 每个节点：Task 定义（目标、验收条件、依赖、profile、模型档位）。
- 边：依赖关系；无依赖节点可并行（受目录锁与并发上限约束）。
- **成本路由**：bounded（边界清晰、验收明确）的节点下发低成本模型并行跑；高风险节点走高能力模型。

#### 4.7.3 Evidence Gate

每个节点完成后，Delivery Loop 用确定性规则校验证据（测试全过？lint 干净？diff 触及范围符合声明？），再交独立 Reviewer Agent 做语义评审。两道都过，节点才标记完成，下游节点才解锁。**生成与评审分离**——不让 worker 自评。

#### 4.7.4 Bounded Rework（有界返工）

- 节点级返工预算（如 ≤2 轮）：证据门或评审不通过时，携评审意见返工。
- 全局返工预算：整个 manifest 的总返工轮数上限。
- 超出预算：节点/Task 置 `blocked`，升级人工决策——这是 Stop 条件的工程化。

#### 4.7.5 Merge Conditions（合并条件）

Task 的 `done` 由确定性条件判定（merge-to-Done），而非 agent 自述：

1. 存在带 close-intent（`Closes/Fixes/Resolves {KEY}`）的已合并 PR；
2. 该 Task 无其他 open/draft 的工作 PR；
3. Task 非 `done`/`cancelled` 状态。

Git 集成保持**只读姿态**：按 Task KEY 约定（分支名/标题含 KEY）自动关联 PR，镜像 CI 与 mergeability 状态，不需要写权限。

#### 4.7.6 恢复（Recovery）

manifest 执行状态持久化在 daemon；daemon 重启、机器休眠、网络断连后，Delivery Loop 从最近一致状态续跑：运行中节点 reclaim 或按失败分类重试，未启动节点按拓扑序继续。

#### 4.7.7 与现有 Loop 的关系

- Task `iterate` 模式 = 单节点交付循环（prompt + verify 直到通过或达上限）。
- Delivery Loop = 多节点 DAG 版，节点即 Task。
- 两者共享 Evidence、VerifyChecks、失败分类、状态机与预算门控。

### 4.8 分层验证体系与架构治理

Solo 的质量保障不是单点测试，而是**六层递进验证体系**——成本递增、确定性递减，但覆盖面逐层逼近真实业务价值。每一层都对应控制论中的"传感器"，为 LLM 的概率性输出提供确定性信号。

#### 4.8.1 六层验证模型

| 层次 | 验证方式 | 验证对象 | 成本 | 确定性 | Solo 落地 |
|:----:|:-------|:-------|:----:|:----:|:-----|
| **静态验证** | 编译器 + 类型检查 + linter | 语法正确性 | 极低 | 极高 | Go build / tsc --noEmit / eslint / golangci-lint |
| **测试验证** | 单元测试 + 集成测试 | 行为正确性（用例范围内） | 低 | 较高 | go test -race / vitest / 覆盖率门控 |
| **运行时验证** | E2E、Playwright、MCP 工具 | 真实用户场景 | 中 | 中高 | Playwright E2E / In-App Test Runner |
| **非功能验证** | 混沌工程、故障注入、性能压测 | 稳定性、容错、降级、自愈 | 中高 | 中高 | daemon 崩溃恢复测试 / 网络分区模拟 / 负载测试 |
| **语义验证** | LLM 评估器 + 结构化评分 | 质量、意图符合度、设计合理性 | 中 | 中高 | 独立 Reviewer Agent / LLM-as-judge / Evidence Gate |
| **价值验证** | 业务指标、A/B 测试、人工终审 | 是否解决真实问题 | 高 | 中 | 用户留存 / 上下文切换时间节省 / Task 自治交付率 |

> **生成-验证不对称性**：判断一个解的质量所需的搜索空间，远小于从零生成该解。验证不需要 100% 确定，只需要比生成更便宜、更稳定。Solo 的 Evidence Gate（§4.7.3）与 In-App Verification 层（§4.6.4）正是这个不对称性的工程化。

#### 4.8.2 验证体系与 AI-Native 工位的映射

| 验证层 | 在 Solo 工位中的触发点 |
|:----:|:-----|
| 静态 + 测试 | Agent 每次提交前自动运行（CI 快速失败）；In-App Test Runner 按需触发 |
| 运行时 | Task iterate 的 verify step；Staging Preview 截图对比 |
| 非功能 | daemon 健康检查；Task 超时 / 崩溃恢复测试；定期压测 |
| 语义 | 独立 Reviewer Agent（不让 worker 自评）；LLM-as-judge 对 diff 评分 |
| 价值 | 新闻稿验收（PR/FAQ 兑现度）；KPI 仪表盘；人工终审 |

#### 4.8.3 架构治理：ADR 与漂移检测

PADD 要求架构约束是"活的"——不是一次性设计文档，而是持续校验的基线：

- **ADR（Architecture Decision Record）**：每个架构决策（含妥协）记录为 ADR，包含上下文、决策、后果、偿还时间窗口。Solo 项目记忆原生支持 ADR 索引与检索。
- **漂移检测**：定期（每季度 / 每次大版本后）校验实现是否符合架构约束。信号包括：模块边界被绕过、依赖方向反转、接口契约被破坏。
- **主动演化**：根据产品路线图主动迭代架构，而非被动救火重构。架构方案必须绑定产品中长期路线图，不设计产品永远用不上的能力。

> **Solo 自身的实践**：Solo 的 `docs/analysis/` 目录存放架构映射文档；每次重大架构变更（如 Tmux Dashboard → External Agent Bus 升级、6→2 概念归约）必须产出 ADR 并更新架构映射。

---

## 5. 三大产品支柱（Task 交付视角）

三大支柱以 Task 的完整生命周期组织——从意图进入，到执行，再到证据交付：

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    Solo 2026 产品支柱（Task 交付视角）                         │
├─────────────────────┬─────────────────────┬─────────────────────────────────┤
│      支柱 1          │      支柱 2          │          支柱 3                  │
│  Task Board +       │   Runtime Bus +     │   Delivery Loop +               │
│  Intent             │   Provider Hub      │   Evidence                      │
│  任务入口与意图归一化 │  执行总线与模型中枢   │  确定性交付与证据验收            │
├─────────────────────┼─────────────────────┼─────────────────────────────────┤
│ · Product/Task 领域  │ · Tmux Agent Bus    │ · Delivery Loop（DAG 派发/      │
│   模型（状态机/KEY/  │   (Runtime 注册/    │   证据门/有界返工/合并条件/      │
│   KV metadata）      │   心跳/并发/目录锁/  │   恢复）                        │
│ · Task Board + Inbox │   worktree/GC）     │ · Evidence Schema + 独立        │
│ · Intent Normalizer  │ · External Agent    │   Reviewer                      │
│ · mention 即触发      │   Adapters          │ · Agent Execution Stream        │
│ · Trigger（manual/   │ · Provider Hub /    │ · Inline Intervention           │
│   cron/webhook/      │   Router            │ · In-App Diff Review /          │
│   on_complete）      │ · Local API Proxy   │   Test Runner / Staging         │
│ · Human-Agent        │ · MCP/Skill/Prompt  │   Preview / PR / Merge          │
│   Handoff            │ · Config Exporter   │ · Run History 回放              │
│                     │ · 用量/成本按 Run 聚合│ · Project Memory 沉淀           │
└─────────────────────┴─────────────────────┴─────────────────────────────────┘
```

### 5.1 支柱 1：Task Board + Intent（任务入口与意图归一化）

**目标**：让 Solo 成为开发者"托付工作"的唯一入口——任何形态的意图都归一化为 Task，Task 在看板上可跟踪、可评审、可托付。

**核心能力**：

- **Product + Task 领域模型**：6 概念归约 2 概念；Task 状态机（`queued / running / blocked / in_review / done / failed`）、可路由 KEY、KV metadata（≤50 key / 8KB）。
- **Task Board + Inbox**：Web/App 主界面。Board 按 Product 组织，卡片实时显示进度；Inbox 聚合"待评审 / 待决策 / @提及"。Agent 无 Inbox——@agent 是执行触发。
- **Intent Normalizer**：基于 LLM 把口语化/碎片化输入（键盘、系统 dictation、微信/豆包语音、截图粘贴、PR 评论）重写为结构化 Task 定义（目标、约束、验收条件）。
- **Trigger 体系**：manual / cron / webhook / on_complete 四种触发统一收敛到 Task；现有 Loop / Schedule 平滑迁移。
- **mention 即触发**：Task 讨论中 @agent 即创建 Run；Squad 场景下 leader 通过精确 mention 派单（Q1 2027）。
- **Human-Agent Handoff**：人在 Web/App 开始的任务可平滑移交后台；Task 阻塞/需审批时经 Inbox/Push 交还控制权与上下文。
- **Solo 不自研 STT**：语音识别由 OS/IME 承担，Solo 只消费归一化后的意图文本。

**商业价值**：

- 把 Solo 从"配置器 + 聊天客户端"升级为"工作托付入口"。
- 概念收敛降低学习成本：用户只需理解 Product 与 Task。
- 任何输入形态都能驱动 Agent，消除键盘 vs 语音 vs 截图的差异。

### 5.2 支柱 2：Runtime Bus + Provider Hub（执行总线与模型/工具中枢）

**目标**：让 Solo 成为调度与观测所有外部 AI Coding Agent 的统一总线，同时作为项目的模型、工具、配置中枢。

**核心能力**：

- **Tmux Agent Bus → Runtime 抽象**：每个"daemon 主机 × agent CLI"注册为 Runtime；注册/心跳（15s，3 分钟判离线）、并发上限（整机/单 profile）、Task 级隔离目录 + 目录锁（`waiting_local_directory`）、git worktree 共享缓存、产物 GC TTL。
- **External Agent Adapters**：为每种外部 Agent 编写 stdout 解析器（Claude Code 的 JSON stream、Codex CLI 的 TTY 输出、OpenCode 的 stream 格式）。
- 项目级 Provider 预设和 API 配置，支持 `.solo/provider-hub.json`。
- 本地 API 代理（`:17613`），支持协议转换、Open Responses 兼容层和 failover。
- MCP / Skills / Prompts / AGENTS.md 按项目跨 agent 同步；Skills 支持从 runtime 机器扫描导入。
- **MCP Server Cards 与注册表发现**：从公开 registry 或 `.well-known` 自动拉取 server 能力清单。
- **MCP Capability Attestation**：校验 server manifest，标记危险工具权限。
- **用量与成本按 Run / Task 聚合**，支持项目级预算告警。
- 智能模型路由：按任务类型、成本、延迟、provider 健康状态自动选择模型；bounded 任务下发低成本模型。
- 配置一键转写到 Claude Code、Codex、Cursor、OpenCode、Windsurf、Aider、Continue（P2，Solo 本身成为工位后此能力重要性下降）。

**商业价值**：

- 让 Solo 成为外部 Agent 的统一控制面，无需为每个 Agent 写原生集成。
- 降低多 Agent 用户的配置管理成本。
- 为团队提供统一的模型接入、工具治理和成本治理。

### 5.3 支柱 3：Delivery Loop + Evidence（确定性交付与证据验收）

**目标**：让交付可托付、可审计、可恢复——"完成"是证据支持下的事实，不是 agent 的声明。

**核心能力**：

- **Delivery Loop（确定性编排）**：manifest DAG 拓扑派发、结果收集、Evidence Gate、有界返工（节点 + 全局双层预算）、中途恢复、merge-to-Done 条件判定、超限升级人工。
- **Evidence Schema**：结构化交付证据（diff stat、测试结果、CI、PR 引用、截图、Reviewer 结论）；Task 的 `in_review` 以证据为中心呈现。
- **独立 Reviewer Agent**：生成与评审分离，worker 不自评。
- **Agent Execution Stream**：外部 Agent 的 stdout、diff、命令输出、推理链、工具调用实时流式渲染到 Web/App。
- **Inline Intervention Protocol**：人随时可以插入指令打断 Agent，Agent 实时响应；执行状态机与 Task/Run 统一。
- **Human Confirm Gate 泛化**：从"危险操作的保险"升级为"开发循环内的原语"。
- **In-App Diff Review / Test Runner / Staging Preview / PR / Merge**：验收闭环全程在 Web/App 内完成；合并满足条件后 Task 自动 `done`。
- **Run History 回放**：每次 Run 的完整消息流（tool call、thinking、错误）可回放，成本按 Run 聚合。
- **Project Memory 沉淀**：ADR、代码地图更新、偏好、失败案例、证据回流到项目记忆与 Task 历史。

**商业价值**：

- Web/App 不再是"监控面板"，而是对等的 AI-Native 交付工位。
- 把"写代码 → review → 测试 → 合并"的完整研发交付流闭环在 Solo 内。
- 可审计的证据历史建立人对自治交付的信任——信任基于可核查的记录，而非盲目的信心。

---

## 6. 季度路线图

### Q3 2026：Product+Task 统一 + External Agent Bus MVP + 项目入口

**主题**：落地 Product+Task 领域模型（6→2 概念收敛），把 Tmux Dashboard 升级为带 Runtime 抽象的 External Agent Bus，让 Web/App 开始具备 AI-Native 工位的"观测 + 干预"能力；紧跟 MCP 标准化浪潮。

> **问题尺码分布**（§0.9.3）：Product+Task 统一迁移、External Agent Bus 升级、Agent Execution Stream、Inline Intervention 属 **XL 码**（平台级服务，影响 L / 规模 L / 上下文 L），需 L2~L3 层级执行，工具先行；Provider Hub、Run History、Onboarding 属 **L 码**（核心业务模块），需 L2 层级；MCP 管理、Config Exporter 属 **M 码**，L1~L2 即可。本季度变革策略：XL 码功能先建协议与验证器（工具先行），L/M 码功能端到端交付（组织先行）。

| 优先级 | 功能 | 里程碑 | 成功标准 |
|--------|------|--------|----------|
| P0 | **Product+Task 数据模型落地** | `daemon/internal/task/` + protocol `task/*` RPC（约 25 个 RPC 收敛旧 loop/schedule/agent 面）；Task 状态机含 `in_review` / `blocked` / 自动回退 | 6 概念归约 2 概念；旧 RPC 兼容层不破坏现有客户端 |
| P0 | **Task Board + Inbox UI** | Web/App 主界面：按 Product 的看板（Running / Queued / Blocked / In Review / Done）+ Inbox 通知中心 | 用户只接触 Product 与 Task 两个概念即可完成全流程 |
| P0 | **Run History 与证据记录** | 每次 Run 留痕：消息流、diff stat、verify 结果、成本；支持回放 | 任意历史 Run 可完整回放 |
| P0 | **Tmux Dashboard → External Agent Bus 升级** | pane stdout 流式读取 + stdin 文本注入 + 结构化事件捕获 + Runtime 注册/心跳 | Claude Code / Codex CLI / OpenCode 三种外部 Agent 可被 Solo 编排 |
| P0 | **Agent Execution Stream (MVP)** | 外部 Agent 的 stdout/diff/工具调用流式渲染到 Web/App | 人可实时观察 Agent 执行过程，延迟 < 500ms |
| P0 | **Inline Intervention (MVP)** | 执行中人可向 Agent stdin 注入文本指令打断/纠偏 | 干预成功率 > 90%，Agent 能正确响应 |
| P0 | **项目 Onboarding 自动索引** | 首次打开项目生成代码地图、技术栈摘要、AGENTS.md 草案 | 新项目上手时间减半 |
| P0 | Provider Hub 独立进程 (`solo-provider-hub`) | 可管理 5+ provider 预设，支持项目级配置 | 切换 provider 耗时 < 1s |
| P0 | 本地 API Proxy | 支持 Claude/Codex 指向 Solo 端口 | 协议转换成功率 > 95% |
| P1 | **Loop / Schedule → Task 迁移** | 现有 loop 与 schedule 数据无缝迁移为 Task（iterate / cron）；旧入口保留只读视图 | 迁移零数据丢失，存量 Task 全部可运行 |
| P1 | **失败二分法与有界自动重试** | `failureKind` 分类落地；平台侧故障自动重试 ≤2 次；`agent_error.*` 不自动重试 | Run 失败 100% 带分类标签 |
| P1 | **执行目录治理** | Task 级隔离目录 + 目录锁（`waiting_local_directory`）+ 产物 GC TTL | 并发 Task 不互相污染；磁盘占用有界 |
| P1 | MCP 统一管理 MVP | 一个 MCP 同步到 2+ Agent | 配置一致率 100% |
| P1 | AGENTS.md 原生支持 | Solo Agent 启动时自动读取项目 AGENTS.md | 项目规则注入率 100% |
| P1 | 本地 / BYOK 模型支持 | 支持 Ollama / vLLM / LM Studio 作为 Provider | 至少一种本地推理后端跑通 |
| P2 | Config Exporter MVP | 支持 Claude Code + Cursor 配置转写 | 导出文件可直接被目标 Agent 读取 |
| P2 | Usage Tracker 基础 | 记录 provider 请求数和 token，按 Run / Task 聚合 | 数据误差 < 5% |
| P2 | MCP Server Cards 发现 | 支持从 `.well-known/mcp-server-card` 拉取能力清单 | 5+ 公开 server 可被发现 |

### Q4 2026：Delivery Loop + Intent Normalizer + In-App Verification + Git 闭环

**主题**：落地确定性交付循环（manifest DAG + 证据门 + 有界返工 + 合并条件），让多步骤交付可托付；Intent Normalizer 把多模态输入归一化为 Task 定义；In-App Diff Review 与 Test Runner 构成验收闭环；Git 只读集成打通 PR → merge-to-Done。

> **问题尺码分布**：Delivery Loop（DAG 派发/恢复/合并判定）、Intervention Primitive 属 **XL 码**（影响 L：错误操作不可逆；上下文 L：跨多状态机），需 L2~L3 + 工具先行；Intent Normalizer、In-App Diff Review、Git 闭环属 **L 码**，需 L2；智能路由、MCP Attestation 属 **M~L 码**。本季度重点：XL 码功能的验证器必须先于功能本身就绪（六层验证中语义验证层落地）；Delivery Loop 分两步走——先单 Task 证据门，再多节点 DAG。

| 优先级 | 功能 | 里程碑 | 成功标准 |
|--------|------|--------|----------|
| P0 | **Evidence Schema + 单 Task 证据门** | Run 结束产出结构化证据；Task verify 通过后进 `in_review`；独立 Reviewer Agent 语义评审 | 证据 schema 强校验通过率 100%；worker 不自评 |
| P0 | **Delivery Loop MVP** | manifest DAG 拓扑派发 + 结果收集 + 有界返工（节点 ≤2 轮 + 全局预算）+ daemon 重启恢复 | 5 节点 DAG 交付场景端到端跑通；中途 kill daemon 可续跑 |
| P0 | **Intervention Primitive（泛化 Human Confirm Gate）** | 危险操作、diff 审批、Agent 请求澄清统一走 App 内 approve/comment | 无未经审批的破坏性操作，Agent 能基于评论迭代 |
| P0 | **Intent Normalizer (MVP)** | LLM 把口语化/碎片化文本重写为结构化 Task 定义（含目标/约束/验收条件） | 5 种口语样本归一化准确率 > 85% |
| P0 | 智能 Provider 路由 | 基于任务类型/成本/延迟自动选择 provider；bounded 节点下发低成本模型 | 简单任务成本降低 30% |
| P1 | **Git PR 闭环（只读）** | 按 Task KEY 约定自动关联 PR（复用 GitHub issue/PR 搜索 RPC）；镜像 CI/mergeability；merge-to-Done 条件判定 | PR 自动关联准确率 > 90%；合并后 Task 自动 `done` |
| P1 | **In-App Diff Review** | PR 级别 diff 渲染 + 行级评论，Agent 据此迭代 | diff 在 App 内完整可读，评论能驱动 Agent 下一轮修改 |
| P1 | **In-App Test Runner** | 点击即可跑相关测试/构建/lint，结果在 App 内渲染 | 测试输出可折叠、失败项可点击跳转 |
| P1 | **Human-Agent Handoff** | 支持桌面端任务一键移交后台；阻塞/需审批时通过 Inbox/App 推送将控制权与上下文交还用户 | 任务交接成功率 > 90%，用户平均响应时间 < 5min |
| P1 | Skills 库 + Prompts 预设 | 可安装/更新 skills；支持从 runtime 机器扫描导入 | 10+ 内置 skill 模板 |
| P1 | MCP 工具目录 + Capability Attestation | 内置常用 MCP 一键启用；manifest 校验 + 危险工具标记 | 高风险工具调用前 100% 告警 |
| P2 | **Solo CLI 升级为 agent 一等接口** | 全部 RPC 可脚本化；配套 solo-ops skill 教外部 agent 安全操作 Solo（读 Task、回写进度、写 metadata） | 外部 agent 经 CLI 完成"领取 → 执行 → 回写"全链路 |
| P2 | Config Exporter 扩展 | 支持 Codex/OpenCode/Windsurf/Aider/Continue | 覆盖 80% 主流 Agent |
| P2 | Project Memory Phase 1 | 代码地图 + ADR 索引 | 新项目 onboarding 时间减半 |
| P2 | 成本预算告警 | 按 Product 设置月度预算和阈值告警 | 预算超支前主动通知 |
| P2 | Open Responses 兼容层 | Local API Proxy 支持 Open Responses 接口 | 主流 Open Responses 客户端可接入 |

### Q1 2027：Squad 多 Agent 协作 + 团队协作 + AI-Native 工位闭环

**主题**：从单 Agent 到 Squad（leader + 成员，mention 派单），从个人到团队；A2A 与 MCP Apps 进入实用阶段；补全 AI-Native 工位最后两块拼图——Staging Preview 与截图入 Agent，让 Web/App 端到端闭环开发流程。

> **问题尺码分布**：Squad 协作（leader 路由协议）、A2A 适配属 **XL~XXL 码**（跨组织上下文、多系统状态纠缠），需 L3 主导；Project Memory Phase 2、PR 自动审查属 **L 码**；Staging Preview、截图入 Agent 属 **M~L 码**。本季度进入右上区治理阶段：XXL 码功能必须由 L3 把关人主导，先建派单防循环规则与验证器再扩展。

| 优先级 | 功能 | 里程碑 | 成功标准 |
|--------|------|--------|----------|
| P0 | **Squad 多 Agent 协作** | leader + 成员模型；Squad Operating Protocol（首回合置父 Task running、精确 mention 派单、派单后即停）+ 再触发规则表 + 去重防循环 | 复杂任务分解成功率 > 80%；无 mention 死循环 |
| P1 | **Agent-to-Agent Handoff** | 基于 A2A Task 语义在 Coordinator 与 Specialist 之间移交任务上下文与执行结果 | 跨 Agent 委托成功率 > 80%，上下文丢失率 < 5% |
| P0 | Project Memory Phase 2 | 跨会话检索 + 用户偏好学习 | 用户重复指令减少 50% |
| P1 | 团队共享配置 | Provider Hub 团队空间（Git 为单一事实源） | 团队配置一致率 100% |
| P1 | PR 自动审查 | Agent 自动 review 并输出评论 | 审查覆盖率 > 60% |
| P1 | Auto Test / Fix Loop | 代码修改后自动运行相关测试并进入 fix loop | 测试失败自主修复率 > 50% |
| P1 | A2A v1.0 适配 | Solo Agent 可发布 Agent Card 并委托外部 Agent | 跨 Agent 委托成功率 > 80% |
| P1 | **Staging Preview** | Web/App 一键启动 preview 环境 + 截图/录屏，作为验收闭环的一部分 | 主流框架（Vite/Next/Expo）可一键 preview |
| P1 | **截图入 Agent** | 用户粘贴截图/拍照，Intent Normalizer 与 Coordinator 可识别并驱动 Agent 据此修改 UI | UI 复刻准确率 > 70% |
| P2 | MCP Apps UI 支持 | 工具返回的沙箱化 UI 可在 App/桌面渲染 | 1+ 官方 MCP App 可用 |

### Q2 2027：生态与规模化

**主题**：开放生态、性能优化、企业级能力。

> **问题尺码分布**：Marketplace、企业 SSO / 审计属 **XL~XXL 码**（行业级基础设施、合规要求），需 L3 + 工具先行；性能优化、MCP Registry 集成属 **L 码**；云端同步选项属 **M 码**。本季度全面进入治理区：XXL 码功能（企业合规、跨公司生态）必须验证器完备后才可上线。

| 优先级 | 功能 | 里程碑 | 成功标准 |
|--------|------|--------|----------|
| P0 | Provider Hub Marketplace | 第三方 provider / skill 市场 | 50+ 公开 provider 预设 |
| P0 | Task Template Market | 常见 task / delivery manifest 模板 | 20+ 内置模板 |
| P1 | 企业 SSO / 审计日志 | 团队权限和合规 | 支持 OIDC + 审计 |
| P1 | 性能优化 | Task 调度延迟降低 50% | 单 step 决策 < 2s |
| P1 | A2A / 跨进程 Agent 协作 | 支持 A2A 协议委托任务 | 跨 Agent 委托成功率 > 90% |
| P1 | MCP Registry 集成 | 一键安装公开 MCP server | 100+ server 可被发现 |
| P2 | 云端同步选项 | 可选加密云同步 | 用户可自主选择 |
| P2 | IM 渠道评估 | 评估 Slack/Lark/钉钉/企微 触发与通知（对照 Multica 的 IM 集成） | 完成可行性评估与决策 |

---

## 7. 技术依赖关系

```
Product（Project CWD）
    │
    ├── Domain Layer（领域模型）
    │   ├── daemon/internal/task/ ──▶ Product / Task / Run / Evidence Store
    │   │   ├── Task 状态机（in_review / blocked / 自动回退）
    │   │   ├── Run 失败分类（platform ≤2 重试 / agent_error.* 不重试）
    │   │   ├── Task KV metadata（≤50 key / 8KB）
    │   │   └── protocol task/* RPC（~25 个，收敛旧 loop/schedule/agent 面）
    │   └── daemon/internal/delivery/ ──▶ Delivery Loop
    │       ├── Manifest DAG 解析与拓扑派发
    │       ├── Evidence Gate（确定性校验 + 独立 Reviewer）
    │       ├── Bounded Rework（节点 + 全局预算）
    │       ├── Merge-to-Done 条件判定 ──▶ Git 只读集成（Task KEY 约定）
    │       └── Recovery（manifest 状态持久化，daemon 重启续跑）
    │
    ├── Context Layer
    │   ├── AGENTS.md / CLAUDE.md ──▶ External Agents (Claude/Codex/Cursor/...)
    │   ├── Project Memory ──▶ Solo Agents + Planner
    │   ├── Task 历史与证据 ──▶ 下一个 Run 的前置上下文
    │   └── Knowledge Graph ──▶ Chat / Coordinator
    │
    ├── Tool Layer
    │   ├── Local API Proxy ──▶ External Agents
    │   ├── Open Responses Layer ──▶ Open Responses compatible clients
    │   ├── Config Exporter ──▶ External Agent Config Files (P2)
    │   ├── MCP/Skills/Prompts Hub ──▶ Solo Agents + External Agents
    │   └── MCP Server Cards / Registry ──▶ Tool Discovery & Attestation
    │
    ├── Agent Layer
    │   ├── Provider Hub
    │   │   ├── Registry ──▶ Provider Presets
    │   │   ├── Router ──▶ Cost / Latency / Capability Routing（bounded 节点走低档模型）
    │   │   └── Usage Tracker ──▶ 按 Run / Task / Product 聚合成本
    │   │
    │   ├── External Agent Bus (Tmux) ──▶ Runtime 抽象
    │   │   ├── Runtime 注册 / 心跳 / 并发上限
    │   │   ├── 执行目录治理（目录锁 / worktree 缓存 / GC TTL）
    │   │   ├── Claude Code Adapter (JSON stream)
    │   │   ├── Codex CLI Adapter (TTY)
    │   │   ├── OpenCode Adapter (stream)
    │   │   └── ... 其他外部 Agent Adapter
    │   │
    │   ├── Solo Agent (轻量 fallback) ──▶ Provider Client ──▶ Provider Hub
    │   ├── Planner / Worker / Reviewer Agents ──▶ Delivery Loop 的概率性层
    │   └── A2A Agent Card Registry ──▶ External Agent Discovery
    │
    ├── Interface Layer
    │   ├── Task Board + Inbox ──▶ Web/App 主界面（agent 无 Inbox，@ 即触发）
    │   ├── Chat Coordinator ──▶ Multiple Agents
    │   ├── Tmux Agent Bus ──▶ External Agent Panes
    │   ├── CLI ──▶ API 一等客户端（every surface is scriptable）+ solo-ops skill
    │   └── Push Notifications ──▶ Intervention Primitive / Task 状态变更
    │
    └── AI-Native Workstation Layer (横向贯穿)
        ├── Intent Normalizer ──▶ LLM function calling / 结构化 Task 定义
        │   └── 输入来源：键盘文本 / OS dictation / 微信豆包等 IME 语音 / 截图粘贴
        ├── Coordinator / Task DAG ──▶ Delivery Loop / External Agent Bus
        ├── Agent Execution Stream ──▶ External Agent Bus stdout → Web/App 实时渲染
        ├── Inline Intervention Protocol ──▶ Web/App 输入 → External Agent Bus stdin
        └── In-App Verification ──▶ Evidence Review / Diff Review / Test Runner / Staging Preview / PR
```

---

## 8. 关键成功指标（KPIs）

### 8.1 项目上下文与记忆

| 指标 | 2026 年底目标 | 2027 年底目标 |
|------|--------------|---------------|
| 项目 Onboarding 自动生成 AGENTS.md 采纳率 | 60% | 85% |
| 新项目上手时间（vs 当前） | -50% | -70% |
| AGENTS.md 注入覆盖率 | 100% | 100% |
| 项目记忆检索准确率 | — | 85% |
| 用户每周上下文切换时间节省 | 2h | 4h |

### 8.2 Provider 与工具治理

| 指标 | 2026 年底目标 | 2027 年底目标 |
|------|--------------|---------------|
| Provider Hub 管理 provider 数 | 20+ | 50+ |
| 支持的本地 / BYOK 推理后端 | 1+ | 3+ |
| MCP server 可发现/启用数 | 10+ | 50+ |
| MCP 跨 Agent 配置一致率 | 100% | 100% |
| 配置导出覆盖 Agent 类型 | 5 | 10 |
| 简单任务成本节省（vs 固定模型） | 20% | 40% |

### 8.3 Agent 执行与自治

| 指标 | 2026 年底目标 | 2027 年底目标 |
|------|--------------|---------------|
| External Agent Bus 可编排 Agent 数 | 3+（Claude Code / Codex CLI / OpenCode） | 8+ |
| Agent Execution Stream 延迟（pane stdout → Web/App 渲染） | < 500ms | < 200ms |
| Inline Intervention 成功率 | > 90% | > 98% |
| Intent Normalizer 口语归一化准确率 | > 85% | > 95% |
| Task iterate 自主完成率（fix-tests） | 70% | 90% |
| In-App 验收闭环覆盖率（开发任务在 Web/App 内闭环完成的比例） | > 60% | > 90% |
| A2A 跨 Agent 委托成功率 | — | 90% |

### 8.4 交付质量（新增）

| 指标 | 2026 年底目标 | 2027 年底目标 |
|------|--------------|---------------|
| Task 自治交付率（无人工干预到达 `in_review`） | 40% | 70% |
| 证据一次通过率（Evidence Gate first-pass rate） | 60% | 85% |
| Delivery Loop 平均返工轮数（预算内收敛） | ≤ 2.5 | ≤ 1.5 |
| Run 失败自动分类覆盖率（platform vs agent_error） | 100% | 100% |
| merge-to-Done 自动化率（合并后 Task 自动 `done`） | 50% | 90% |
| 概念统一迁移完成率（存量 Loop/Schedule → Task） | 100% | — |

### 8.5 平台健康与增长

| 指标 | 2026 年底目标 | 2027 年底目标 |
|------|--------------|---------------|
| App 月活跃用户（MAU） | — | 10K+ |
| Solo daemon 崩溃恢复成功率 | 95% | 99% |
| Delivery Loop 中断恢复成功率（daemon 重启后续跑） | 95% | 99% |

---

## 9. 风险与应对

| 风险 | 影响 | 应对 |
|------|------|------|
| **概念统一迁移破坏现有用户习惯**（6→2 概念归约改变交互心智） | 高 | ADR-002 锁定决策；旧 RPC 兼容层 + 数据双写期；迁移向导；存量 Loop/Schedule 自动迁移为 Task；旧入口保留只读视图一个季度。 |
| **证据门形式化**（证据造假、过宽或 schema 漂移，"完成"再次沦为声明） | 高 | 生成与评审分离（独立 Reviewer Agent）；证据 schema 强校验；Evidence Gate 用确定性规则做第一道闸；定期用 3 强 + 3 弱样本回归测试证据门。 |
| **DAG 规划漂移**（Planner 产出的 manifest 与仓库现状脱节） | 中 | Planner 规划前强制读仓库快照；节点幂等设计；仓库重大变更触发重规划；manifest 版本化、人可编辑。 |
| **返工爆炸**（bounded rework 预算设置不当导致循环返工） | 中 | 节点级 + 全局双层返工预算；超限自动 `blocked` 升级人工；返工轮数纳入 KPI 监控（§8.4）。 |
| **交付控制层过度工程化**（oh-my-multica 是社区实验而非成熟方案） | 中 | 分两步走：先单 Task 证据门（Q4），再多节点 DAG（Q4–Q1）；每阶段保留可回退的简化版；把 Multica 当需求证据而非蓝图。 |
| **Squad mention 死循环**（leader 与成员互相触发） | 中 | 再触发规则表（谁评论、是否带 @ 决定是否唤醒 leader）+ 去重 + 派单深度上限；参考 Multica Squad Operating Protocol。 |
| Provider Hub 独立进程增加安装复杂度 | 中 | 提供 `solo install hub` 一键安装和自动启动。 |
| Task 自治执行产生破坏 | 高 | 沙箱、审批门控、maxIterations、early stop、`blocked` 升级人工。 |
| 大模型决策不稳定 | 高 | Prompt 工程 + function calling + 决策校验 + 人工确认。 |
| 外部 Agent 配置格式变化 | 中 | Exporter 接口隔离 + 单元测试 + 社区反馈。 |
| **External Agent Adapter 碎片化**（每种外部 Agent 的 stdout 格式不同且频繁变化） | 高 | 定义通用的 pane 输出 schema；为每种 Agent 编写独立 adapter 并单元测试；监控上游 Agent 版本变化并告警。 |
| **Intent Normalizer 归一化不准**（口语化/碎片输入改写为 Task 定义时误解意图） | 高 | 使用 LLM function calling + 结构化输出；保留"原样转发"降级通道；收集用户纠正样本持续迭代 prompt；支持"意图确认"交互避免误操作。 |
| **Tmux Agent Bus 可靠性**（tmux 进程崩溃、pane 输出丢失、stdin 注入时序问题） | 高 | Tmux server 健康检查与自动重启；pane 输出双写到持久化日志；stdin 注入走队列保证顺序；关键操作前先 snapshot；对照 Multica daemon 补齐心跳/reclaim/自更新细节。 |
| 竞品（Cursor/Copilot/Cloud Agents/Multica）快速迭代 | 中 | 坚持本地优先 + E2EE + 移动端对等工位 + 开放生态的差异化；持续跟踪 Multica 的 runtime 协议族演进。 |
| Agent token 成本爆炸 | 高 | 用量按 Run/Task 聚合监控、预算告警、智能路由、缓存复用。 |
| AGENTS.md / MCP 标准碎片化 | 中 | 紧跟 Linux Foundation Agentic AI Foundation 标准，保持 exporter 可插拔。 |
| 云 Agent 在复杂任务上表现更优 | 中 | 本地优先做"值守+监控+治理"，复杂任务可选路由到云端。 |
| 模型厂商出口管制或访问受限 | 高 | 多 Provider 中立 + 本地/BYOK 模型支持，避免单一厂商锁定。 |
| MCP server 工具滥用或 manifest 伪造 | 高 | Capability attestation、危险工具标记、权限最小化、审计日志。 |
| A2A Agent Card 信任模型不成熟 | 中 | 先支持自签名/本地 Agent Card，生产级依赖 Signed Agent Cards 待验证后启用。 |
| 项目记忆过度膨胀导致上下文爆炸 | 中 | 分层记忆策略：活跃项目全量加载，归档项目按需检索；定期压缩与归档。 |
| 多项目配置漂移 | 中 | 支持全局默认模板 + 项目级覆盖；提供 `solo project check` 一致性校验。 |
| **验证债**（Task 声称"完成"但实际未达标——"完成"是声明不是事实） | 高 | run ≠ done 语义；独立 Reviewer Agent；Evidence Gate + Human Confirm Gate 泛化为开发原语；In-App Diff Review / Test Runner 作为独立验证层；merge-to-Done 确定性判定。 |
| **理解债**（Agent 产出代码的速度远超开发者理解代码的速度） | 中 | Agent Execution Stream 实时渲染执行过程；In-App Diff Review 强制可读；项目记忆沉淀 ADR 与变更摘要；定期 "code comprehension review" 机制。 |
| **认知投降**（开发者对 Agent 输出照单全收，停止主动判断） | 中 | Intervention Primitive 要求人在关键节点介入；审批策略默认 `dangerous-only` 而非 `auto`；Inbox/Push 在 Task 完成/失败时主动提醒审查；文档化 "Build the loop, but stay the PM" 原则。 |
| **Artifact Drift**（AGENTS.md / 项目规则 / eval rubric 随时间漂移而无人监控） | 中 | Project Memory 版本化每个 artifact 变更；定期 eval 对照测试（3 强 + 3 弱样本）；artifact 变更自动关联 decision log；CI 中校验 artifact 一致性。 |
| **架构漂移**（实现逐渐偏离架构约束，模块边界被绕过、依赖方向反转） | 高 | ADR 记录每个架构决策与妥协；每季度架构漂移检测（模块边界 / 依赖方向 / 接口契约）；重大变更必须产出 ADR 并更新架构映射文档；CI 中增加依赖方向校验。 |
| **PADD 失衡**（产品侧与架构侧单向妥协） | 中 | 需求变更强制双向评审（产品影响 + 架构影响）；妥协必须记录在 ADR 并设偿还时间窗口；架构方案必须绑定产品路线图，拒绝过度设计；季度回顾检查 ADR 偿还进度。 |

---

## 10. 近期行动项（未来 2 周）

按主题分组，便于并行推进：

### 架构决策

1. **ADR-002 评审收口**：Product+Task Unification 架构决策定稿，锁定 6→2 概念归约与迁移策略。
2. **确定 Delivery Loop 模块边界**：`daemon/internal/delivery/` 与 task / agent / schedule 模块的接口划分；manifest 文件格式与存放位置。
3. **确定 Task KEY 方案**：per-Product 前缀 + 序号的生成规则，与 Git 分支/PR 标题约定的规范。

### Task 模型与 Protocol

4. **创建 `daemon/internal/task/` 模块骨架**：Product / Task / Run / Evidence 存储与状态机（含 `in_review` / `blocked` / 自动回退）。
5. **扩展 protocol**：`task/*` RPC 草案（约 25 个），设计旧 loop/schedule RPC 兼容层。
6. **失败分类表落地**：定义 `failureKind` 枚举（平台侧 vs `agent_error.*`），作为运维文档模板。
7. **CLI 命令设计**：`solo task create/run/status/logs/review`；`solo product init/context/check`。

### Delivery Loop 与证据

8. **Manifest DAG schema v0**：节点（Task 定义 + 验收条件 + 依赖 + profile）与边的序列化格式。
9. **Evidence schema v0**：diff stat / verify results / CI / PR 引用 / Reviewer 结论的强校验 schema。
10. **Bounded rework 预算策略**：节点级与全局默认值的取值依据与配置面。

### 借鉴 Multica 的运维深度

11. **daemon 自愈对照清单**：对照 Multica 补齐心跳判离线、重启 reclaim 未完成 Run、自更新等运行任务结束后跟随、agent CLI 升级重探测。
12. **执行目录治理方案**：Task 级隔离目录 + 目录锁 + worktree 共享缓存 + GC TTL 参数表（参考值：完成 24h / 孤儿 72h / 可再生产物 12h / repo 缓存 720h）。
13. **Task KV metadata 规范**：≤50 key / 8KB 约定与反模式（不存 attempts、日志、密钥）写入开发文档。

### AI-Native 工位

14. **Tmux Agent Bus 技术调研**：验证 tmux API 在 stdout 流式读取、stdin 文本注入、结构化事件捕获上的能力边界与性能上限；确认 Claude Code / Codex CLI / OpenCode 三种 Agent 的 stdout 格式差异。
15. **External Agent Bus 协议设计**：定义通用的 pane 输出 schema、Agent Adapter 接口、结构化事件（diff / tool_call / error / thinking）格式。
16. **Agent Execution Stream UX**：设计 Web/App 如何流式渲染外部 Agent 的执行过程，确保移动端与桌面端对等。
17. **Inline Intervention Protocol 设计**：定义人插入指令打断 Agent 的消息协议、状态机转换规则、与 Task/Run 状态机的统一方案。
18. **Intent Normalizer 原型**：基于 LLM function calling + 结构化输出，把 5–10 种典型口语化输入样本改写为结构化 Task 定义，验证准确率与延迟。

### 方法论与治理

19. **建立 ADR 实践**：在 `docs/decisions/` 完善 ADR 索引；为近期重大决策（Product+Task 统一、Tmux → Agent Bus 升级、Provider Hub 进程模型、Delivery Loop 方案）补写 ADR。
20. **季度路线图问题尺码标注**：为 Q3–Q4 每个 P0/P1 功能标注问题尺码（S–XXL），用于匹配执行者层级与验证深度。
21. **六层验证管线 MVP**：确认当前 CI 覆盖了哪几层（静态 + 测试 + 运行时），识别非功能验证与语义验证的缺口，制定补齐计划。
22. **PADD 双向评审机制**：在下一次需求评审中试行"产品影响 + 架构影响"双清单，验证流程可行性并迭代。

---

## 11. 文档索引

| 文档 | 用途 |
|------|------|
| [PRD: Product + Task Unification](prd/product-task-unification.md) | Product/Task/Run 数据模型、RPC 面、UI 结构、迁移计划（v3.0 的产品模型基座） |
| [Multica 能力调研](../analysis/multica-capability-research.md) | Multica 领域模型、工作流、oh-my-multica 交付控制层与可借鉴清单 |
| [Provider Hub Design](prd/provider-hub-design.md) | Provider Hub 详细设计 |
| [Loop Schedule Implementation Spec](prd/loop-schedule-spec.md) | Loop Schedule 实现规范（protocol、模块、迁移计划），含设计原理附录 |
| [Solo Roadmap Architecture Mapping](../analysis/solo-roadmap-architecture-mapping.md) | 路线图到架构的映射 |
| [Product Features](features.md) | 当前 Solo 完整功能清单 |
| **（待撰写）Delivery Loop Spec** | Manifest DAG schema、Evidence Gate、Bounded Rework、Merge Conditions、Recovery 的实现规范 |
| **（待撰写）AI-Native Workstation Design** | AI-Native 工位四层模型（Intent / Orchestration / Observable Execution / In-App Verification）的协议、UX、模块拆分实现规范 |
| **（待撰写）External Agent Bus Spec** | Runtime 抽象、Tmux Agent Bus 协议、External Agent Adapter 接口、结构化事件 schema |
| **（待撰写）Intent Normalizer Spec** | LLM 改写 prompt、结构化输出 schema、口语样本基准集 |

---

## 12. 修订记录

| 日期 | 版本 | 变更 |
|------|------|------|
| 2026-06-13 | v1.0 | 初始版本，整合 Feature Directions、Provider Hub、Loop Schedule 三大方向 |
| 2026-06-19 | v1.1 | 结合 2026 年 6 月竞品动态（Cursor 3 / Claude Code Sonnet 4.5 / OpenAI Codex / Windsurf / MCP 生态 / AGENTS.md 标准）迭代：强化背景 Agent 值守、成本治理、智能路由、AGENTS.md 原生支持、多 Agent 协作、A2A 规划 |
| 2026-06-20 | v1.2 | 新增"2026 年 Agentic 生态趋势研判"章节；强化 MCP 原生工具层、MCP Server Cards / Capability Attestation、A2A-ready、Open Responses 兼容层、本地/BYOK 模型支持；更新 KPIs、风险与近期行动项 |
| 2026-06-22 | v2.0 | **项目中心版重构**：以"面向项目工作目录、聚合连接所有能力、提升个人生产效率"为主线，新增核心理念与效率飞轮、四层聚合架构、思想框架与业界实践映射，调整战略目标、季度优先级与 KPIs，强调项目 Onboarding、AGENTS.md 原生、跨会话项目记忆与多 Agent 协作。 |
| 2026-06-23 | v2.1 | **结合 GitHub Top N 开源趋势强化核心理念**：新增 1.6 节"GitHub Top N 开源趋势对产品设计的启示"；更新 1.4 业界实践映射，补充 OpenCode / Pi / OpenHands / Continue 等 stars 与对齐策略；扩展 1.1 问题定义与 1.2 解决方案，突出"一次投入、持续复利"的效率飞轮和北极星公式；将"以项目为锚点，释放个人生产效率"贯穿第 1 章。 |
| 2026-06-24 | v2.2 | **新增 Handoff 双特性**：补充 Human-Agent Handoff（2.1 / 4.4 / Q4 2026）与 Agent-to-Agent Handoff（2.1 / 4.3 / Q1 2027），覆盖人-机交接与基于 A2A 语义的 agent 间任务委托。 |
| 2026-06-25 | v2.3 | **AI-Native 工位重构**。Solo 定位升级为外部 AI Coding Agent 的 **Meta-Agent 编排层**（§1.7）；新增 AI-Native 工位四层模型 Intent / Orchestration / Observable Execution / In-App Verification（§4.5）；Tmux Dashboard 升级为 External Agent Bus（§4.3 / §5.1）；重写三大支柱（§5）；季度路线图新增 Agent Bus 升级、Execution Stream、Intervention、Intent Normalizer、In-App Diff Review、Staging Preview、截图入 Agent；Config Exporter 降 P2；Solo 不自研 STT；新增 5 项 AI-Native KPI、3 项风险、5 项行动项 |
| 2026-06-25 | v2.4 | **清晰度优化**：统一"Agent/agent"大小写规范（中文行文一律使用大写 Agent）；修复 §4 ASCII 图"交互层 (原)"等不规整标签；删除 §4 / §5 引言中的章节自指；§1.7.6 去除逐节前瞻；§8 KPIs 按主题分组为四类；§10 近期行动项按主题分组为四类 |
| 2026-07-07 | v2.5 | **Loop Engineering 行业趋势整合**：新增 §0.7 "Loop Engineering 成为行业共识"，引入 Andrew Ng 三层循环模型、Saboo 五要素框架与 artifact 概念、Osmani 三大暗面风险；§1.3 思想框架表新增 Loop Engineering 行；§4.3 扩展 Loop Schedule 描述以映射三层循环模型；§9 新增验证债、理解债、认知投降、Artifact Drift 四项风险；合并 loop-schedule-design 与 loop-schedule-deep-dive 到 loop-schedule-spec 附录 |
| 2026-07-22 | v2.6 | **开发方法论基础整合**：新增 §0.8 "开发方法论基础：PADD × 控制论 × 全栈矩阵"，建立产品架构双驱动（PADD）、工程控制论四要素、全栈矩阵（S–XXL × L1–L3）三框架协同；§1.3 思想框架表新增 PADD / 全栈矩阵 / 工程控制论三行；新增 §4.6 "分层验证体系与架构治理"（六层验证模型 + 验证与工位映射 + ADR 与漂移检测）；§9 新增架构漂移、PADD 失衡两项风险；§10 新增"方法论与治理"行动项分组（ADR 实践 / 问题尺码标注 / 六层验证管线 / PADD 双向评审） |
| 2026-08-11 | v3.0 | **Task 交付版重构（结合 Multica 能力深度重写）**。新增 §0.8 "交付工程化崛起"（Multica 研判：issue 一等实体、run≠done、失败二分法、执行目录治理、只读 Git 闭环、oh-my-multica 交付控制层）；核心理念升级为"以 Product 为锚点、以 Task 为单位"，吸收 Product+Task Unification PRD 的 6→2 概念收敛（§1.2）；新增 §1.8 Multica 能力借鉴地图（直接吸收 / 改造吸收 / 需求证据 / 明确不做四档）；愿景升级为"本地 AI-Native 交付工作台"（§2）；架构新增 §4.1 领域模型（Product/Task/Run/Evidence、Task 状态机、失败分类、KV metadata）与 §4.7 确定性交付循环（manifest DAG、Evidence Gate、Bounded Rework、Merge Conditions、Recovery），External Agent Bus 升级为 Runtime 抽象（注册/心跳/并发/目录锁/worktree/GC）；三大支柱重组为 Task Board + Intent / Runtime Bus + Provider Hub / Delivery Loop + Evidence（§5）；季度路线图重排（Q3：Product+Task 统一 + Agent Bus MVP；Q4：Delivery Loop + 证据门 + Git 闭环；Q1 2027：Squad mention 派单）；新增 §8.4 交付质量 KPI 组（自治交付率 / 证据一次通过率 / 返工轮数 / merge-to-Done 自动化率）；新增 6 项交付相关风险；Codex Provider 标记已落地（v0.11）；新增《Multica 能力调研》分析文档 |
