# 架构评审报告 — Solo（成熟度评估与改进建议）

## 6. 成熟度评估 (Maturity Assessment)

### 6.1 成熟度评分（基线对比）

基线来源：2026-06-07_main_68c66e5_qodercli_qodercli

| 维度 | 子维度 | 基线评级 (68c66e5, 2026-06-07) | 本次评级 | 变化 | 关键依据 |
|------|--------|-------------------------------|---------|------|----------|
| **结构健康度** | OCP（开闭原则） | 3.7 | 3.8 | ↑ | Provider 注册机制稳定；Panel Registry 是 OCP 典范；Tmux 检测仍硬编码 |
| **结构健康度** | 深模块 | 3.3 | 3.3 | — | Schedule Store 封装完整；daemon-client.ts 4,386 行是深模块但过大 |
| **结构健康度** | 分层清晰度 | 3.6 | 3.6 | — | Go 后端分层优秀（protocol→agent→server→main）；前端无 domain/use-case 层 |
| **结构健康度** | Context-First | 2.0 | 1.8 | ↓ | interface{} 持续增长；TS mega-file 未拆分；跨语言无自动同步 |
| **稳定性** | 资源管控 | 3.5 | 3.5 | — | InMemoryTimelineStore 无上限；WebSocket grace period 90s 合理 |
| **稳定性** | 压力应对 | 4.0 | 4.0 | — | 三级事件优先级、Coalescer、Stall Monitor、Singleflight 不变 |
| **稳定性** | 并发安全 | 4.5 | 4.5 | — | race detector clean；Schedule Store RWMutex 正确；Tmux capture-pane singleflight |
| **可靠性** | 数据正确性 | 3.5 | 3.5 | — | FileBackedRegistry 原子写入（temp+rename）；persistTurn() 仍静默吞错 |
| **可靠性** | 故障隔离 | 4.0 | 4.0 | — | SafeBridge 断路器；Provider crash 不影响其他 Agent；Schedule 故障独立 |
| **可靠性** | 恢复能力 | 3.5 | 3.5 | — | Grace period 重连；Schedule Store 启动自愈；Memory 无 WAL |
| **高可用** | 冗余与容错 | 2.0 | 2.0 | — | ⚠️ 本地单实例工具，无多实例/failover |
| **高可用** | 优雅降级 | 4.0 | 4.0 | — | Grace period + Stall Monitor + Provider crash recovery |
| **高可用** | 部署与运维 | 3.5 | 3.5 | — | Prometheus /metrics + /api/health 不变；仍缺 Agent/Schedule 指标 |
| 安全性 | — | 3.3 | 3.3 | — | CORS 空列表仍返回 true（P0 未修复）；E2EE 实现正确 |
| 性能 | — | 3.7 | 3.7 | — | 本地直连亚毫秒；Tmux singleflight 优化已实现 |

**评级**：1=缺失/严重不足，2=有框架但漏洞多，3=可用有改进空间，4=良好，5=业界优秀

**结构健康度均值**：3.1（基线 3.2 → ↓ -0.1，Context-First 继续拖累）
**稳定性均值**：4.0（基线 4.0 → 持平）
**可靠性均值**：3.7（基线 3.7 → 持平）
**高可用均值**：3.2（基线 3.2 → 持平）

### 6.2 ATAM 质量属性评估

| 质量属性 | 基线评分 | 本次评分 | 变化 | 关键依据 |
|----------|----------|----------|------|----------|
| Performance | 3.7 | 3.7 | — | 无新优化，无退化 |
| Scalability | 2.5 | 2.5 | — | 单进程定位不变 |
| Availability/Reliability | 4.2 | 4.2 | — | 稳定，故障隔离良好 |
| Security | 3.3 | 3.3 | — | CORS 空列表问题未修复 |
| Modifiability/Maintainability | 3.5 | 3.3 | ↓ | Go 后端可修改性好，但前端 God Object + 无抽象层降低整体 |
| Testability | 4.5 | 4.5 | — | Go 接口注入优秀；TS 无 service 抽象层降低可测试性 |
| AI-Inferability | 1.6 | 1.5 | ↓ | interface{} 持续增长；跨语言手动同步增加认知负担 |

---

## 7. 建议与改进 (Suggestions & Improvements)

### 7.1 第一性原理结论

Solo 的根本约束：**本地单进程工具，优先保证开发者体验的低摩擦与移动端可恢复性**。

Go 后端的 Clean Architecture 实践**已达到良好水准**——接口消费者定义、组合根薄、领域逻辑独立于基础设施。这是一个值得保持的架构。

前端的"pragmatic React 架构"在小团队快速迭代时是合理的权衡，但随着代码量增长（108K LOC），God Object 和缺失抽象层的问题正在**线性增加新功能的开发成本**。

**核心矛盾**：Go 后端的类型安全（interface{} 泛滥）在恶化，而前端的类型安全（TS any=0）在改善——两个方向的剪刀差在扩大。

### 7.2 重构建议

#### R1: 前端 — 抽取 Use-Case 层（渐进式）

- **问题**：components 直接调用 DaemonClient，无业务编排层
- **影响**：修改半径大（4+ 文件），业务逻辑无法独立测试
- **建议**：
  1. 在 `app/src/use-cases/` 创建纯函数模块
  2. 从最活跃的 feature 开始：`agent-message.use-case.ts`（发送消息 → 处理流事件 → 更新状态）
  3. 组件只调用 use-case，不再直接操作 DaemonClient
- **预期收益**：修改半径从 4 文件降到 2 文件；use-case 可独立单元测试

#### R2: 前端 — 拆分 session-context.tsx

- **问题**：1,672 行 God Object，混合基础设施、应用逻辑、展示层
- **影响**：新人难以理解；修改一个事件类型需在巨大文件中定位
- **建议**：
  1. 将事件订阅逻辑抽取到 `session-event-handlers.ts`（已部分完成：session-stream-reducers.ts）
  2. 将 OS 通知逻辑抽取到 `session-notifications.ts`
  3. 将 Agent 操作（send/cancel/delete）抽取到 `agent-operations.ts`
  4. session-context.tsx 只保留 React provider 壳
- **预期收益**：每个模块 < 300 行，职责清晰

#### R3: Go — 拆分 agent/ 包

- **问题**：agent/ 9,798 LOC，含 4 个 provider 实现 + manager + storage + timeline + coalescer
- **影响**：包内文件过多，AI 推断成本高
- **建议**：
  1. 将 provider 实现移到 `agent/providers/claude/`、`agent/providers/kimi/` 等子包
  2. 保留 `agent/` 为接口定义 + manager
  3. provider 子包只导入 `agent` 接口，不导入 `server`
- **预期收益**：每个 provider < 500 LOC，独立可测试

#### R4: Go — 拆分 Session struct

- **问题**：server/session.go 的 Session struct 有 30+ 字段，处理 agent、terminal、workspace、push、schedule、tmux 所有消息
- **影响**：新增消息类型需修改巨大文件
- **建议**：
  1. 将 terminal 处理逻辑抽取到 `session_terminal.go`
  2. 将 tmux 处理逻辑抽取到 `session_tmux.go`
  3. 将 schedule 处理逻辑抽取到 `session_schedule.go`
  4. Session struct 保留核心生命周期管理
- **预期收益**：每个 handler 文件 < 500 行

### 7.3 重写建议

#### W1: Go↔TS 类型同步机制（中期）

- **触发条件**：当手动同步导致的 bug 频率 > 1/月
- **方案**：从 Go struct 生成 TS Zod schema（或反向），使用 codegen 工具
- **预期收益**：消除跨语言类型不一致风险

#### W2: 前端 Domain 层（长期）

- **触发条件**：当 session-context.tsx 超过 2,500 行，或 use-case 层已建立但仍感痛苦
- **方案**：创建 `app/src/domain/` 目录，将 Agent、Workspace、Schedule 的领域模型从 Zustand store 中分离
- **预期收益**：实现真正的依赖反转，components → domain ← infrastructure

### 7.4 优先行动

| 优先级 | 问题 | 措施 | 收益 | 工作量 |
|--------|------|------|------|--------|
| **P0** | CORS 空列表不 fail-fast | checkOrigin() 空列表拒绝非 localhost | 防止安全回归 | < 1h |
| **P0** | persistTurn() 静默吞错 | 返回 error，上层记录 WARN 日志 | 可观测性 | < 2h |
| **P1** | Go interface{} 持续增长 | 新增 Provider/事件类型时强制使用泛型或具名类型 | 类型安全 + AI 推断 | 每次 PR review |
| **P1** | session-context.tsx God Object | 抽取事件处理/通知/操作到独立模块 | 可维护性 | 2-3 天 |
| **P2** | TS mega-file 未拆分 | daemon-client.ts 按功能域拆分 | AI 推断 + 修改半径 | 1-2 天 |

---

## 附录 A：Clean Architecture 合规性总结

| 原则 | Go 后端 | TypeScript 前端 |
|------|---------|-----------------|
| **依赖规则** | ✅ 严格遵守 | ❌ presentation 直接依赖 infrastructure |
| **接口消费者定义** | ✅ 18/18 | ❌ 无接口抽象 |
| **组合根薄** | ✅ main.go 65 行 | ⚠️ _layout.tsx 937 行 |
| **领域独立于框架** | ✅ agent/ 无 infra 导入 | ❌ 业务逻辑嵌入 React 组件 |
| **可替换性** | ✅ Provider 可热插拔 | ❌ DaemonClient 不可替换 |
| **测试隔离** | ✅ mock via interface | ❌ 无法隔离测试业务逻辑 |

**结论**：Go 后端是 Clean Architecture 的良好实践；前端是 pragmatic React 架构，需渐进式重构。

---

## 附录 B：子模块评审摘要

> 本次为 Clean Architecture 专项评审，不触发子模块递归评审。各模块的架构合规性已在主体报告中覆盖。

| 子模块 | 代码行数 | Clean Architecture 评级 | 关键发现 |
|--------|----------|------------------------|----------|
| daemon | ~23,549 | 4.0/5.0 | 接口消费者定义优秀；Session struct 过大；interface{} 恶化 |
| protocol | ~3,130 | 4.5/5.0 | 零依赖 leaf 包；消息注册模式良好 |
| relay-go | ~3,200 | 4.0/5.0 | 职责单一；E2EE 实现正确 |
| cli | ~4,500 | 3.5/5.0 | 标准 CLI 结构；依赖 daemon client |
| app | ~107,987 | 2.0/5.0 | 无 domain/use-case 层；God Object；无依赖反转 |
| app-bridge | ~12,506 | 3.5/5.0 | Transport 抽象好；mega-file 需拆分 |
