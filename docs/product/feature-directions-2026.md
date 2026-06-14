# Solo 产品新功能方向建议（2026）

> **文档类型**：产品方向建议
> **分析日期**：2026-06-13
> **版本**：v0.6.0 基线
> **目标读者**：产品、技术负责人、核心开发者

---

## 执行摘要

Solo 已从一个实验性 AI 客户端成长为功能完整的本地优先 AI 编码助手平台：

- **App**：React Native / Expo 跨平台客户端
- **Daemon**：Go 核心服务，管理 Agent、会话、工作区、终端
- **Relay**：E2EE WebSocket 中继，支持移动端远程接入
- **Provider 层**：Claude、Kimi（Wire）、OpenCode（SSE）、Pi、Mock 统一接入
- **近期落地**：Session Memory Phase 1、Schedule 自动化、Tmux Dashboard、Agent Stall Detection

当前完成度约 **85–90%**，主要缺口集中在 **Chat/多 Agent 协作**、**Cursor-Agent / ACP Provider** 支持。

结合 Cursor、Claude Code、GitHub Copilot/Windsurf、Aider、Devin、Replit Agent、v0/Lovable 等业界产品的发展趋势，本文提出 Solo 下一阶段的产品新功能方向与路线图建议。

---

## 1. 差异化定位： Solo 应该成为什么

Solo 不应盲目复制 Cursor 的 IDE 内深度编辑体验，而应放大自身独特资产：

| 独特资产 | 可支撑的产品方向 |
|---|---|
| **本地优先 + E2EE** | 隐私敏感场景、企业本地代码、离线/弱网可用 |
| **移动端 App + Relay** | 随时随地查看/干预、Push 通知驱动、移动办公 |
| **Tmux Dashboard** | 不替代现有终端，而是“接管和观测”已有 AI agent |
| **Schedule 调度** | 从“随叫随到”升级为“自动化值守” |
| **多 Provider 抽象** | 模型路由、成本优化、任务适配 |

**产品主线建议**：

> 把 Solo 从“一个能在手机上用的 AI 编码客户端”升级为“**开发者工作的持续运行中枢 + 移动端指挥中心**”——本地 daemon 7×24 值守，手机随时接管。

---

## 2. 高优先级新功能方向

### 2.1 Chat / 多 Agent 协作系统

**状态**：文档已标记为高优先级缺失。

**业界参考**：Cursor Composer、Claude Code multi-agent、Devin swarm、OpenAI Codex CLI multi-turn 任务委托。

**Solo 切入点**：
- 不要做成普通聊天，而是“**项目级指挥中心**”：一个 Coordinator Agent 分析任务，分发子任务给不同 Provider 的 Specialist Agent。
  - 例如：Claude 做架构、Kimi 做审查、OpenCode 做实现。
- 利用已有 Schedule + Tmux 能力，让多 Agent 在后台并行执行，App 端实时看进度。
- 移动端卡片式展示每个子 Agent 状态，一键暂停/重试/接管。

**预期价值**：填补最大功能缺口，形成与单 Agent 工具的显著差异。

---

### 2.2 长期记忆与项目知识库（Session Memory → Project Memory）

**状态**：Session Memory Phase 1 已落地，是良好升级基础。

**业界参考**：Cursor `.cursorrules` + 索引、Claude Project Knowledge、Devin Memory、Aider repomap。

**Solo 切入点**：
- 从“会话级 turn 记录”升级到“**项目级记忆**”，自动维护 `~/.solo/memory/projects/{project}/`：
  - 代码结构地图（repomap）
  - 关键决策 / ADR
  - 用户偏好（编码风格、拒绝过的方案）
  - 失败/成功的历史尝试
- Agent 启动时自动注入相关记忆。
- 支持 `@project` / `@memory` 快捷引用。

**预期价值**：越用越懂用户，减少重复沟通；本地优先让敏感代码记忆不上云，是隐私卖点。

---

### 2.3 任务编排与 Loop 工作流

**状态**：Loop、Tasks 系统缺失。

**业界参考**：Claude Code `/loop`、Cursor Agent 模式自动迭代、Devin Plan → Execute → Verify。

**Solo 切入点**：
- **Tasks 系统**：把一次大需求拆成可串行/并行的 Task DAG，支持依赖、重试、超时。
- **Loop 模式**：Agent 自动执行“改代码 → 跑测试 → 看结果 → 再改”循环，直到测试通过或用户叫停。
- 与 Schedule 结合，形成“**定时任务 + 自主循环**”：例如每晚自动跑 lint/fix/test loop，早晨 push 结果到分支。

**预期价值**：从“辅助编码”进入“自动值守”。

---

### 2.4 智能模型路由（Provider Router）

**状态**：多 Provider 已接入，但当前需手动选择。

**业界参考**：OpenRouter、LiteLLM 路由、GitHub Copilot 模型选择。

**Solo 切入点**：
- 基于任务类型、成本、延迟、当前 Provider 健康状态自动路由：
  - 简单问题 → Kimi / OpenCode（快、便宜）
  - 复杂架构 → Claude
  - 长上下文 → 选择支持长窗口的模型
- 允许用户配置规则（如“审查用 Kimi，实现用 Claude”）。
- 利用已有 Provider 注册表扩展。

**预期价值**：降低用户选择成本，优化 token 成本，成为“模型中立的 AI 入口”。

---

## 3. 中优先级新功能方向

### 3.1 代码审查与 PR 自动化

**状态**：GitHub 集成已有基础（PR 状态、diff、分支），可深化。

**业界参考**：GitHub Copilot Code Review、CodeRabbit、PR-Agent、Claude Code `/pr`。

**Solo 切入点**：
- **自动审查 Agent**：提交/推送前自动 review，输出评论到 PR 或本地报告。
- **PR 摘要生成**：自动生成 PR description、changelog。
- **与 Schedule 结合**：定时扫描仓库，发现潜在问题并创建 draft PR。
- 移动端推送审查结果，支持一键 approve/comment。

**预期价值**：把“写代码”延展到“代码交付全流程”。

---

### 3.2 测试驱动自动化（Auto Test / Fix Loop）

**状态**：终端、Schedule 已具备，缺少测试领域抽象。

**业界参考**：Cursor run tests、Claude Code 测试意识、CodiumAI、Symflower。

**Solo 切入点**：
- Agent 修改代码后，自动识别相关测试并运行。
- 测试失败时自动进入 fix loop，尝试修复。
- 生成测试覆盖率报告，标记回归。
- 与 Tmux Dashboard 结合，在后台 tmux pane 中运行测试并捕获输出。

**预期价值**：显著提升 Agent 输出可信度，减少“看起来对但跑不通”。

---

### 3.3 MCP 工具市场与编排

**状态**：Daemon 端 MCP 服务器已实现，App 端有注入开关。

**业界参考**：Claude Desktop MCP 生态、Cursor MCP 支持。

**Solo 切入点**：
- **MCP 工具目录**：内置常用 MCP 服务器（浏览器、数据库、搜索、GitHub、Notion），一键启用。
- **工具组合**：支持把多个 MCP 工具 + Agent prompt 打包成“工作流模板”。
- **移动端管理**：在 App 中查看已启用工具、调用日志、权限。

**预期价值**：从“接入 MCP”升级为“MCP 生态入口”，增强 Agent 能力边界。

---

### 3.4 项目 Onboarding 与自动索引

**状态**：Workspace/Project 注册表已有。

**业界参考**：Cursor “Add to Chat”索引、Sourcegraph Cody repo understanding、Devin initial exploration。

**Solo 切入点**：
- 首次打开项目时，Agent 自动遍历代码，生成：
  - 项目结构图
  - 关键文件说明
  - 技术栈/依赖总结
  - 运行/构建/测试命令
- 存储到项目记忆中，后续对话可快速引用。

**预期价值**：降低新项目上手成本，特别适合接手遗留代码。

---

## 4. 低优先级 / 差异化探索方向

### 4.1 移动端专属交互创新

**业界参考**：无直接对标，是 Solo 独有机会。

**方向**：
- **语音速记需求**：在手机上语音描述一个 bug 或想法，Solo 自动创建 Schedule/Task，回家后电脑上已有 draft PR。
- **截图转代码**：拍照 UI 草图或错误截图，Agent 生成/修改代码。
- **Watch 通知/快捷回复**：Apple Watch 查看 Agent 状态，语音批复。
- **Widget / Live Activity**：iOS 锁屏显示当前 Agent 进度。

---

### 4.2 团队协作（轻量级）

**业界参考**：Lovable / v0 share、Cursor 团队功能。

**方向**：
- 共享 Agent 配置、Prompt 模板、Schedule 模板。
- 团队级项目记忆（可配置是否共享）。
- 审查工作流：Agent 改动需要另一人 approve 后才 push。

**注意**：与本地优先定位有张力，建议先做“导出/导入配置包”，再做可选云端。

---

### 4.3 安全与合规增强

**业界参考**：Snyk、GitGuardian、AWS CodeGuru。

**方向**：
- 自动扫描 Agent 生成代码中的 secrets、SQL 注入、不安全依赖。
- 敏感操作二次确认（push 到 main、删除文件、执行危险命令）。
- 审计日志：谁/哪个 Agent 在何时做了什么。

---

## 5. 建议的产品路线图（6–12 个月）

| 阶段 | 主题 | 功能 |
|---|---|---|
| **Q1** 打基础 | 补齐核心缺口 | Chat 系统（多 Agent 会话）、Project Memory、Codex/Cursor-Agent Provider |
| **Q2** 自动化 | 从辅助到值守 | Tasks 系统、Loop 模式、智能 Provider 路由、Schedule + Loop 结合 |
| **Q3** 交付闭环 | 覆盖完整研发流 | PR 自动审查、Auto Test/Fix Loop、MCP 工具市场 |
| **Q4** 差异化 | 移动/本地优先体验 | 语音速记、截图转代码、Watch/Live Activity、团队模板共享 |

---

## 6. 关键取舍建议

1. **不做另一个 Cursor**：避免在 IDE 内深度编辑上和 Cursor 正面竞争，Solo 的战场是“本地 daemon + 移动指挥 + 自动化值守”。
2. **优先强化后端能力**：前端架构当前较重（参见架构评审中的 God Object 问题），新增大型功能前先落地 use-case/domain 层拆分，否则迭代成本会指数上升。
3. **移动端不是缩小的桌面端**：把移动端做成“状态监控 + 轻量干预 + 通知驱动”，而不是完整代码编辑。
4. **拥抱开放生态**：MCP、多 Provider、GitHub 集成都是壁垒，不要只绑定单一模型。

---

## 参考文档

- [Product Features](features.md) — 当前完整功能清单
- [Architecture Review 2026-06-12](../analysis/architecture-review-2026-06-12/) — 技术成熟度与重构建议
- [Session Memory Spec](session-memory-spec.md) — Phase 1 实现规范
- [Kimi & Cursor-Agent Integration](../providers/kimi-cursor-integration.md) — Provider 扩展计划
