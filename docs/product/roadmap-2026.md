# Solo 2026 产品/技术路线图

> **文档类型**：统一产品路线图
> **日期**：2026-06-13
> **基线版本**：Solo v0.6.0
> **目标读者**：产品、技术负责人、核心开发者、投资者
> **关联文档**：
> - [Feature Directions 2026](feature-directions-2026.md)
> - [Provider Hub / CC-Switch Migration Design](agent-profile-switch-export-design.md)
> - [Loop Schedule Design](loop-schedule-design.md)
> - [Loop Schedule Deep Dive](loop-schedule-deep-dive.md)

---

## 1. 产品愿景

> **Solo 是开发者的本地 AI 工作中枢。**
>
> 它常驻于你的机器，安全地连接你偏好的大模型和 coding agent，让你通过桌面、移动端或任何终端随时发起、监控和接管自动化开发任务。

### 1.1 核心差异化

| 差异化 | 说明 |
|--------|------|
| **本地优先 + E2EE** | 代码和配置默认不出本机；远程访问通过端到端加密中继。 |
| **移动端指挥中心** | iOS/Android/Web 统一客户端，随时随地查看/干预 agent。 |
| **多 Provider / 多 Agent 中立** | 不绑定单一模型，支持 Claude、Kimi、OpenCode、Codex、Cursor-Agent 等。 |
| **自治循环** | 从单次 prompt 到多轮 Loop，让 agent 自主完成复杂任务。 |
| **配置中枢** | 一套 Provider/MCP/Skill/Prompt 配置，同步到多个 coding agent。 |

---

## 2. 2026 战略目标

1. **把 Solo 从“客户端”升级为“中枢”**：本地 daemon 7×24 值守，移动端随时接管。
2. **实现自治开发循环**：Schedule → Loop，让 agent 能自主计划、执行、验证、修复。
3. **成为 AI coding agent 的配置中枢**：Provider Hub 统一管理和转写配置到 Claude/Codex/Cursor/OpenCode 等。
4. **建立项目级长期记忆**：跨会话、跨 agent 的项目知识沉淀。
5. **覆盖完整研发交付流**：从编码到测试、审查、PR。

---

## 3. 三大产品支柱

```
┌─────────────────────────────────────────────────────────────────────┐
│                     Solo 2026 产品支柱                                │
├─────────────────┬─────────────────────┬─────────────────────────────┤
│  支柱 1          │  支柱 2              │  支柱 3                      │
│  Provider Hub   │  Loop Schedule      │  Project Memory + Chat      │
│  配置中枢        │  自治循环            │  上下文与协作                 │
├─────────────────┼─────────────────────┼─────────────────────────────┤
│ · Provider 管理  │ · 大模型驱动决策     │ · 项目级记忆                 │
│ · 本地 API 代理  │ · agent/bash/test   │ · 多 Agent 协作              │
│ · MCP/Skill/Prompt│ · 自动验证/修复     │ · 长期上下文                 │
│ · 配置转写      │ · 崩溃恢复          │ · 团队共享                   │
│ · 用量监控      │ · 人工确认门控       │ · 自动索引                   │
└─────────────────┴─────────────────────┴─────────────────────────────┘
```

### 3.1 支柱 1：Provider Hub（配置中枢）

**目标**：让 Solo 成为开发者管理所有 coding agent 配置的地方。

**核心能力**：
- 统一管理 Provider 预设和 API 配置。
- 本地 API 代理（`:17613`），支持协议转换和 failover。
- MCP / Skills / Prompts 跨 agent 同步。
- 用量和成本监控。
- 配置一键转写到 Claude Code、Codex、Cursor、OpenCode、Windsurf、Aider、Continue。

**商业价值**：
- 降低多 agent 用户的配置管理成本。
- 为团队提供统一的模型接入和成本治理。

### 3.2 支柱 2：Loop Schedule（自治循环）

**目标**：让 Schedule 从“定时执行单次任务”进化为“大模型驱动的自治循环”。

**核心能力**：
- 扩展 `StoredSchedule` 支持 `type: "loop"`。
- Loop Controller 调用 LLM 决策下一步。
- Step Executor 支持 agent、bash、test、read、write、git、ask-user。
- 状态机、持久化、崩溃恢复、人工确认门控。
- 安全沙箱、成本控制、可观测性。

**商业价值**：
- 把“辅助编码”推进到“自动值守和修复”。
- 典型场景：每晚自动修复测试失败、自动更新依赖、自动重构。

### 3.3 支柱 3：Project Memory + Chat（上下文与协作）

**目标**：让 Solo 越用越懂项目和用户。

**核心能力**：
- 项目级记忆：代码地图、ADR、用户偏好、历史尝试。
- 多 Agent 协作：Coordinator + Specialist Agent 分工。
- Chat 系统：项目级指挥中心。

**商业价值**：
- 减少重复沟通，提升 agent 输出质量。
- 支持团队共享项目知识。

---

## 4. 季度路线图

### Q3 2026：Provider Hub MVP + Loop Schedule 基础

**主题**：把 cc-switch 的核心能力和 Loop 的基础能力落地。

| 优先级 | 功能 | 里程碑 | 成功标准 |
|--------|------|--------|----------|
| P0 | Provider Hub 独立进程 (`solo-provider-hub`) | 可管理 5+ provider 预设 | 切换 provider 耗时 < 1s |
| P0 | 本地 API Proxy | 支持 Claude/Codex 指向 Solo 端口 | 协议转换成功率 > 95% |
| P0 | Loop Schedule 基础 | 支持 `type: "loop"` 和 agent/bash/test step | 跑通 "create hello.go" 场景 |
| P1 | Config Exporter MVP | 支持 Claude Code + Cursor 配置转写 | 导出文件可直接被目标 agent 读取 |
| P1 | MCP 统一管理 MVP | 一个 MCP 同步到 2+ agent | 配置一致率 100% |
| P2 | Usage Tracker 基础 | 记录 provider 请求数和 token | 数据误差 < 5% |

### Q4 2026：Loop 自治 + Provider Hub 完善

**主题**：让 Loop 真正自治，让 Provider Hub 覆盖更多 agent。

| 优先级 | 功能 | 里程碑 | 成功标准 |
|--------|------|--------|----------|
| P0 | Loop Controller 决策优化 | 支持 function calling / tool use | fix-tests 场景 70% 自主完成 |
| P0 | 人工确认门控 + App UI | 危险操作 100% 经审批 | 无未经审批的破坏性操作 |
| P1 | Config Exporter 扩展 | 支持 Codex/OpenCode/Windsurf/Aider/Continue | 覆盖 80% 主流 agent |
| P1 | Skills Market + Prompts 库 | 可安装/更新 skills | 10+ 内置 skill 模板 |
| P2 | Project Memory Phase 1 | 代码地图 + ADR 索引 | 新项目 onboarding 时间减半 |

### Q1 2027：多 Agent 协作 + 团队协作

**主题**：从单 agent 到多 agent，从个人到团队。

| 优先级 | 功能 | 里程碑 | 成功标准 |
|--------|------|--------|----------|
| P0 | Chat / 多 Agent 协作 | Coordinator + Specialist 模式 | 复杂任务分解成功率 > 80% |
| P0 | Project Memory Phase 2 | 跨会话检索 + 用户偏好学习 | 用户重复指令减少 50% |
| P1 | 团队共享配置 | Provider Hub 团队空间 | 团队配置一致率 100% |
| P1 | PR 自动审查 | Agent 自动 review 并输出评论 | 审查覆盖率 > 60% |
| P2 | 语音/截图输入 | 移动端专属交互 | 支持语音创建 loop |

### Q2 2027：生态与规模化

**主题**：开放生态、性能优化、企业级能力。

| 优先级 | 功能 | 里程碑 | 成功标准 |
|--------|------|--------|----------|
| P0 | Provider Hub Marketplace | 第三方 provider / skill 市场 | 50+ 公开 provider 预设 |
| P0 | Loop Template Market | 常见 loop 模板 | 20+ 内置模板 |
| P1 | 企业 SSO / 审计日志 | 团队权限和合规 | 支持 OIDC + 审计 |
| P1 | 性能优化 | Loop 延迟降低 50% | 单 step 决策 < 2s |
| P2 | 云端同步选项 | 可选加密云同步 | 用户可自主选择 |

---

## 5. 技术依赖关系

```
Provider Hub
    │
    ├── Local API Proxy ──▶ External Agents (Claude/Codex/Cursor/...)
    │
    ├── Config Exporter ──▶ External Agent Config Files
    │
    ├── MCP/Skills/Prompts Hub ──▶ Solo Agents + External Agents
    │
    └── Usage Tracker ──▶ Cost Dashboard

Loop Schedule
    │
    ├── Loop Controller ──▶ Provider Client ──▶ Provider Hub
    │
    ├── Step Executor ──▶ Agent Manager / Terminal / Workspace
    │
    ├── State Store ──▶ Schedule Store extension
    │
    └── Human Confirm Gate ──▶ App / CLI

Project Memory + Chat
    │
    ├── Memory Backend ──▶ File/SQLite/Middleware
    │
    ├── Indexer ──▶ Workspace / Git
    │
    └── Chat Coordinator ──▶ Multiple Agents
```

---

## 6. 关键成功指标（KPIs）

| 指标 | 2026 年底目标 | 2027 年底目标 |
|------|--------------|---------------|
| Provider Hub 管理 provider 数 | 20+ | 50+ |
| Loop 自主完成率（fix-tests） | 70% | 90% |
| 配置导出覆盖 agent 类型 | 5 | 10 |
| 项目记忆检索准确率 | — | 85% |
| App 月活跃用户（MAU） | — | 10K+ |
| Solo daemon 崩溃恢复成功率 | 95% | 99% |

---

## 7. 风险与应对

| 风险 | 影响 | 应对 |
|------|------|------|
| Provider Hub 独立进程增加安装复杂度 | 中 | 提供 `solo install hub` 一键安装和自动启动。 |
| Loop 可能进入死循环或产生破坏 | 高 | 沙箱、审批门控、maxIterations、early stop。 |
| 大模型决策不稳定 | 高 | Prompt 工程 + function calling + 决策校验 + 人工确认。 |
| 外部 agent 配置格式变化 | 中 | Exporter 接口隔离 + 单元测试 + 社区反馈。 |
| 移动端体验不及桌面端 | 中 | 移动优先设计监控/轻量干预，不做完整代码编辑。 |
| 竞品（Cursor/Copilot）快速迭代 | 中 | 坚持本地优先 + 开放生态 + 移动差异化。 |

---

## 8. 近期行动项（未来 2 周）

1. **确定 Provider Hub 进程模型**：独立进程 vs daemon 内置，选定后锁定架构。
2. **Loop Schedule MVP 范围**：确认先做 `agent/bash/test` 三种 step 和 CLI。
3. **创建 `daemon/internal/loop/` 模块骨架**：定义 `LoopEngine`、`LoopController`、`StepExecutor` 接口。
4. **扩展 protocol**：`StoredSchedule.Type` 支持 `"loop"`，新增 `LoopControllerConfig`。
5. **CLI 命令设计**：`solo loop create/start/status/logs/abort`。

---

## 9. 文档索引

| 文档 | 用途 |
|------|------|
| [Feature Directions 2026](feature-directions-2026.md) | 原始方向分析，含业界对标 |
| [Provider Hub / CC-Switch Migration Design](agent-profile-switch-export-design.md) | Provider Hub 详细设计 |
| [Loop Schedule Design](loop-schedule-design.md) | Loop Schedule 高层设计 |
| [Loop Schedule Deep Dive](loop-schedule-deep-dive.md) | Loop 技术深度分析 |
| [Product Features](features.md) | 当前 Solo 完整功能清单 |

---

## 10. 修订记录

| 日期 | 版本 | 变更 |
|------|------|------|
| 2026-06-13 | v1.0 | 初始版本，整合 Feature Directions、Provider Hub、Loop Schedule 三大方向 |
