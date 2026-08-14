# Multica 能力调研（multica-ai/multica）

> **调研日期**：2026-08-11
> **用途**：Solo 2026 路线图 v3.0 的输入；Multica 能力对照与可借鉴机制归档
> **来源**：[github.com/multica-ai/multica](https://github.com/multica-ai/multica)、[multica.ai/docs](https://multica.ai/docs)、[VISION.md](https://github.com/multica-ai/multica/blob/main/VISION.md)、[SELF_HOSTING.md](https://github.com/multica-ai/multica/blob/main/SELF_HOSTING.md)、[multica-cli](https://github.com/multica-ai/multica-cli)、[Discussion #5545 oh-my-multica](https://github.com/multica-ai/multica/discussions/5545)

---

## 0. 项目概览

- 主语言 Go，45.2k stars / 5.7k forks（截至 2026-08-11），默认分支 `main`，工作日高频发版。
- License：**Apache 2.0 全文 + 附加条件**（针对托管服务、商业嵌入、品牌），属 source-available 风格。
- 官网 [multica.ai](https://multica.ai)，有云版与桌面版下载。

## 1. 产品定位与核心理念

定位原话（README）：

> "**Agents that show up on the board.** Multica is an open-source workspace where you assign work to AI coding agents the way you'd assign it to a teammate — they pick up the issue, report progress, raise blockers, and hand it back for review. Self-hostable, works with 20 agent CLIs, no lock-in."
>
> "_Your next 10 hires won't be human._"

核心理念（VISION.md）：

> "**Make humans and AI agents work as one team.** … Multica is not an autonomous company running beyond human control. It is the shared operating system for people and agents doing consequential work together. People set direction and remain accountable. Agents keep the work moving."

- 命名致敬 Multics（分时操作系统）："把分时复用带回来，只是这次复用系统的'用户'同时是人类和 agent"。
- 关键设计主张：
  - Agent 是**一等队友**（assignee 选择器、activity timeline、任务生命周期、runtime 基础设施从第一天就围绕此构建）。
  - 工作产物（plan、diff、测试结果、未决问题）而非 "status theatre" 是评审对象。
  - **历史不随 session 消失**——intent、决策、行动、产物、结果保持关联，"the next agent does not start from zero"。
- 自述痛点：每个 agent 各居一个 terminal tab、session 结束即失忆、人反复重述上下文，"the more agents you add, the more of your day goes to babysitting them"。

## 2. 核心概念 / 领域模型

| 概念 | 职责 | 关键点 |
|---|---|---|
| **Workspace** | 团队工作的自包含边界；配置、issue、权限按 workspace 隔离 | 同一 GitHub App installation 可接多个 workspace；issue 前缀（`MUL-`、`ENG-`）按 workspace 区分 |
| **Issue** | 一件工作 + 描述、讨论、状态、历史；日常工作基本单元 | 状态机 `backlog / todo / in_progress / in_review / done / blocked / cancelled`；可路由 KEY（`MUL-123`）；看板 `position` 排序、父子 issue、每 issue KV **metadata**（≤50 key、8KB，记录 PR 号、pipeline 状态等"会被后续 run 重读"的高信号状态）、subscribers |
| **Project** | 按目标组织相关 issue、跟踪总进度，可绑定 repo/目录作为执行上下文 | 有 lead（人或 agent），状态 `planned/in_progress/paused/completed/cancelled` |
| **Agent** | workspace 里的 AI 协作者 = **可复用配置**（name、instructions、model、skills、Access、runtime）；**不是常驻进程** | 描述不入 prompt；instructions 每次 run 都用；Access 三级（Only me / Entire workspace / Specific people）；归档取消其未完成任务 |
| **Skill** | 可复用能力包（`SKILL.md` + scripts/templates/references）；"instructions 定义 agent 是谁，skill 定义一类工作怎么做" | 多对多绑定 agent；可手动创建、从 URL 导入、**从 runtime 机器扫描复制**（快照式）；导入内容不审查不沙箱，明示信任风险 |
| **Runtime** | 执行实际发生处：**一台已连接电脑 + 其上某一个 AI coding 工具**（或 custom runtime profile） | "agent 是身份，runtime 是执行它的电脑"；daemon 为每个 workspace × 每个检测到的 CLI 注册一个 runtime；默认 private；离线 >7 天且无绑定 agent 自动清理 |
| **Task** | agent 的一次具体执行记录；每次触发产生一个 task，历史永不覆盖 | 状态机 `deferred → queued → dispatched → waiting_local_directory → running → completed/failed/cancelled`；queued 超 2h 判 failed；dispatched 卡 5 分钟判 failed；running 靠 15s 心跳活性判断；**"task completed ≠ issue done"** |
| **Squad** | 一个 leader agent + 若干成员（agent 或人）；issue 指派给 squad = 唤醒 leader 由其路由 | leader 收到系统注入的 **Squad Operating Protocol**（首回合置父 issue in_progress、用精确 mention markdown `[@Name](mention://agent/<uuid>)` 派单、每回合记录评估、派单后即停）+ Squad Roster + Squad Instructions；精细的 leader 再触发规则表 + 去重防循环 |
| **Chat** | 不挂 issue 的对话，每条消息触发一次 run | 适合问答/快速试验 |
| **Inbox** | 人类成员的通知中心（订阅、@提及、指派） | **agent 没有 inbox**——对 agent 的 @ 是执行触发而非通知 |
| **Autopilot** | 定时/事件触发的自动化 | `create_issue`（新建 issue 并指派）与 `run_only`（直接入队 task）两种模式；触发器含 cron/webhook/api，CLI 只暴露 cron |
| **Custom runtime profile** | 自有 wrapper / 固定版本 / 固定参数的 runtime | 不新增协议，须归入已支持的 protocol family（如 ACP）；workspace 级共享 |

## 3. 技术架构

```
Web · Desktop (macOS/Windows/Linux) · iOS
              │
   Next.js frontend ──> Go backend (Chi + WS) ──> PostgreSQL (pgvector)
                             │ tasks over WebSocket
                        Agent daemon（跑在用户机器上、贴着代码）
                             │ spawns
                        Claude Code · Codex · Cursor · …（20 种 CLI）
```

| 层 | 技术 |
|---|---|
| Web | Next.js 16 (App Router) |
| Desktop | Electron，复用 web UI 包 |
| Mobile | Expo / React Native（iOS，源码构建），`apps/mobile/` |
| Backend | Go 单二进制，Chi router + sqlc + gorilla/websocket；REST + WebSocket |
| DB | PostgreSQL 17 + pgvector；sqlc 迁移（103+ 个 migration） |
| Agent runtime | 本机 daemon（Go，与 CLI 同二进制） |
| 仓库 | pnpm + turbo monorepo：`apps/{web,desktop,mobile,docs}`、`server/`、`packages/`、`deploy/helm/`、`e2e/`（Playwright） |

**部署**：Docker Compose（`make selfhost`）或 Helm chart。多副本安全（DB-backed 调度器 + advisory lock）。

**Daemon 工作方式**（与 Solo daemon 最可比）：

- 启动探测 PATH 上的 agent CLI，为每个监听 workspace 注册 runtime；持久连接 + 周期轮询兜底（3s 轮询、15s 心跳；心跳断约 3 分钟标记 offline）。
- 收到 task 建**隔离工作目录**（issue 级目录 + **目录锁**——同目录被占时 task 进 `waiting_local_directory`），spawn agent CLI，流式回传；默认整机并发 20、单 agent 并发 6。
- **Repo 缓存**：`.repos/` 下 bare clone 共享 object store，每个 task 工作目录是 `git worktree`。
- **GC 体系**：完成 issue 的 task 目录 24h TTL、孤儿目录 72h、可再生产物（node_modules 等 basename 模式）12h、repo 缓存 720h。
- **自更新跟随**：daemon 检测到与新 binary 版本不一致时，等运行中任务结束后自愈重启；agent CLI 升级只重新探测并重新注册 runtime。
- **会话恢复**：retry 时若 session 安全且同一 runtime 认领，续上之前的 session；context overflow 等毒化 session 的错误在原工作目录开新 session。
- 接入协议分族：ACP（MCP 通过 `session/new` 的 `McpServer` 数组下发，而非写配置文件）与各家私有流式协议。

**API 形态**：REST + WebSocket；CLI 是 API 的一等客户端（"Every surface is scriptable. Agents drive Multica through the same CLI you do"）。

## 4. 关键工作流：从 issue 到交付

1. **建 issue**：标题/描述/优先级/指派人（人、agent 或 squad），可挂 project（绑定 repo 上下文）。
2. **触发**：四种方式——指派 issue、评论 @mention、Chat、Autopilot；触发即创建 task 入队。
3. **认领与执行**：runtime 经 WS/轮询认领 → 建隔离工作目录（git worktree off 共享 bare clone）→ 按 agent 配置组装 prompt（instructions + skills + 任务简报）→ spawn CLI。
4. **进度回写**：agent 通过 `multica` CLI（multica-cli skill 教 agent 安全操作：读 issue、`--content-file` 发评论、写 metadata、处理 mention/状态副作用）把进度、阻塞、结果写成 issue 评论；人类经 inbox 收到"需要决策"的通知。
5. **完成与评审**：agent 把 issue 推到 `in_review`（`done` 留给人或集成）；run 完整消息流（tool call、thinking、错误）可在 Execution log 回放；token 用量按 run/agent/issue 聚合。
6. **GitHub 闭环**：GitHub App（只读权限）按 issue KEY 自动链接 PR（分支名/标题含 `MUL-123`，或 body 用 `Closes/Fixes/Resolves MUL-123`）；**merge-to-Done** 需同时满足：有 close-intent 的已合并 PR、无其他 Open/Draft 工作 PR、issue 非 done/cancelled。自托管版支持 GitLab/Gitea/Forgejo。
7. **失败与重试**：平台侧瞬时故障（runtime 掉线、daemon 重启回收、超时、skill 包下载失败）自动重试至 2 次；agent 自身错误（鉴权、配额、配置、context overflow 等，`agent_error.*` 分类）不自动重试；无活动 task 且无待重试时，`in_progress` issue 自动回 `todo`。
8. **IM 渠道**：Slack、Lark 官方集成，DingTalk/WeCom 社区维护。

## 5. oh-my-multica（Discussion #5545）核心机制

- 定位：Multica 已提供执行地基（shared workspaces、work items、task queues、runtimes、Skills、run history），缺的是**跨 run 的交付控制层**。
- 双层控制：
  - **Planner / Orchestrator agents**：检查仓库现状，产出设计、验收标准（acceptance definition）、项目规则、**manifest DAG**。
  - **确定性 Loop（deterministic loop）**：结果收集、DAG 就绪节点派发、**evidence gates**、**bounded rework**（有界返工）、中途恢复、**merge conditions**、最终验收。
- 价值主张：有界任务可下发给低成本模型并行跑，而 Loop 与独立 reviewer 保证"项目完成不变成某一个 agent 的一家之言"。
- 演示：Webhook Inbox 需求 → 5 节点 DAG → 5 个经评审的 PR → 86 测试、97.18% 覆盖率、CI 多 Python 版本、11 条最终验收流。

## 6. 差异化能力清单（相比"本地 daemon + 移动端"类产品）

1. **Issue/看板是系统中心，不是会话列表**：agent 作为 assignee 出现在看板上，状态机、排序、父子 issue、订阅、inbox 都是项目管理级（Linear 式），而非 chat 包装。
2. **20 种 agent CLI 的统一 runtime 抽象**：协议族 + custom runtime profile，换 provider 是下拉框而非迁移；CLI 凭证、代码不出本机。
3. **人机混合协作语义**：@mention = 执行触发；agent 无 inbox；squad leader 路由协议是少见的多 agent 协调落地方案。
4. **Skill 作为团队资产**：workspace 级存储、多 agent 复用、从 runtime 扫描导入。
5. **可审计的 run 历史与成本**：逐 tool call 回放、token 用量聚合到 issue/agent、失败原因分类体系（平台侧 vs `agent_error.*`）。
6. **Git 平台闭环**：只读 GitHub App 自动链 PR、CI/mergeability 镜像、close-intent 驱动的 merge-to-Done。
7. **运维细节深度**：目录锁、git worktree 共享缓存、四类 GC、daemon 自更新跟随、session 续跑策略、多副本安全的 DB 调度器——"真跑过生产"的痕迹。
8. **全端覆盖**：Web + Electron 桌面 + Expo iOS + IM bots。

## 7. 可借鉴点清单（对 Solo）

按可吸收难度从低到高：

1. **任务/issue 状态机与"run ≠ done"语义**：task completed 只代表一次运行结束，issue 完成由人或 merge 条件判定；失败时 `in_progress → todo` 自动回退。
2. **失败原因二分法与有界自动重试**：平台侧瞬时故障自动重试 ≤2 次；`agent_error.*` 永不自动重试，先修因再手动 retry。
3. **daemon 自愈细节**：心跳 15s / 3 分钟判离线、重启后 reclaim 未正常结束的 task、等运行任务结束后跟进新 binary、agent CLI 升级只重探测不重启。
4. **执行目录治理**：issue 级隔离目录 + 目录锁（`waiting_local_directory` 排队态）、bare clone + git worktree 共享缓存、按 basename 模式的产物 GC。
5. **issue KV metadata 约定**：小 KV map（≤50 key/8KB）专存"会被后续 run 重读"的跨 run 状态（PR 号、pipeline 状态、阻塞原因）；反模式：不存 attempts、日志、密钥。
6. **mention 即触发的路由协议**：squad leader 通过精确 `mention://agent/<uuid>` markdown 派单 + 再触发规则表 + 去重防循环。
7. **Git 集成的只读姿态**：只读权限 + issue KEY 约定自动关联 PR 并驱动状态流转——低权限、低侵入。
8. **文档与错误信息工程**：CLI 错误统一翻译层（友好单行 + `--debug` 展开 + 分级 exit code）、面向 agent 消费的分页设计（thread/tail/cursor/since）。
9. **产品叙事**："review the work itself, not status theatre"。
10. **谨慎参考**：oh-my-multica 的 manifest DAG + deterministic loop + evidence gates 属社区实验，说明"交付控制层"是生态公认缺口；Solo 应把它当**需求证据而非成熟方案**。

**调研局限**：`multica.ai/docs` 的 issues、autopilots、members-roles、security model 等页面未逐一抓取；GitHub integration 自托管配置细节、环境变量全表未展开。如需更细的 API 端点清单或数据库 schema 可再下钻。
