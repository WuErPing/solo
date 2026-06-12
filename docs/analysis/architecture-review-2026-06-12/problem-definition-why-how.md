# 架构评审报告 — Solo（问题定义、价值与路径）

## 1. 问题定义 (Problem Definition)

### 评审对象

Solo 项目全栈（github.com/WuErPing/solo），以 Clean Architecture 视角审视各模块的依赖方向、层级边界、领域隔离。

### 核心问题

Solo 的架构是否遵循 Clean Architecture 的依赖规则（Dependency Rule）？各层是否正确隔离？从 Go 后端到 React Native 前端，领域逻辑是否独立于框架和基础设施？

### 边界

- **包含**：daemon、protocol、relay-go、cli、app、app-bridge 的架构设计
- **不包含**：功能正确性验证、性能基准测试、安全渗透测试
- **评审深度**：深度（多模块系统，154K+ 行代码）

### 设计决策分析

**Go 后端（daemon/protocol/relay-go/cli）**：
- ✅ 依赖规则严格遵守：`protocol` → `daemon/internal/*` → `daemon/main`，所有跨包接口由消费者定义
- ✅ 18/18 接口均为消费者定义（Consumer-Defined Interface），符合 SOLID 的 DIP
- ✅ 组合根极薄（main.go 65 行），所有具体类型在 `NewDaemon()` 中组装
- ✅ 领域逻辑（agent 状态机、timeline 去重、coalescer、stall monitor）无基础设施导入

**TypeScript 前端（app/app-bridge）**：
- ❌ 违反依赖规则：presentation 层（components/hooks）直接依赖基础设施层（DaemonClient）
- ❌ 无 use-case/interactor 层：stores（状态）和 components（UI）之间缺少业务编排层
- ❌ 无依赖反转：所有模块直接导入具体实现，无 interface/abstract 抽象
- ⚠️ session-context.tsx（1,672 行）是 God Object，混合了基础设施订阅、应用逻辑和展示层

---

## 3. Why：价值与 ROI

### 业务/技术价值

**Go 后端的 Clean Architecture 已产生实际收益**：
1. **可测试性**：所有 provider 通过 `AgentClient`/`AgentSession` 接口注入，mock 测试覆盖率高
2. **可扩展性**：新增 Provider（如 Kimi、Pi）只需实现接口 + 注册，manager 层零修改
3. **可维护性**：Schedule 模块三层分离（Store/Executor/Runner）是新模块的典范
4. **故障隔离**：SafeBridge 断路器、Stall Monitor 等机制确保单点故障不扩散

**前端缺失 Clean Architecture 的代价**：
1. **修改半径大**：新增一个 daemon 事件类型需修改 3-4 个文件（context、store、hooks、components）
2. **测试困难**：业务逻辑嵌入 React 组件，无法独立测试
3. **认知负担高**：session-context.tsx 1,672 行，新人难以理解事件流转全貌

### ROI 考量

- Go 后端：已建立良好架构，维护成本低，ROI 高
- 前端：重构为 Clean Architecture 成本高（涉及 100+ 文件），但长期收益显著。建议**渐进式重构**而非一次性重写

### 不做会怎样

- Go 后端：type erasure（interface{} 987 处）持续恶化，最终降低 AI 可推断性
- 前端：God Object 继续膨胀，新功能开发速度线性下降

---

## 4. How：路径与决策

### 可选路径

| 路径 | 描述 | 适用场景 |
|------|------|----------|
| **A: 渐进式重构** | 逐步抽取 use-case 层，从最活跃的 feature 开始 | 当前推荐 |
| **B: 重写前端架构层** | 新建 domain/ 和 use-cases/ 目录，逐步迁移 | 当 God Object 超过 3000 行 |
| **C: 保持现状** | 接受 pragmatic React 架构，聚焦功能交付 | 团队 < 3 人 |

### 选择依据

推荐路径 A（渐进式重构），原因：
1. 前端代码 108K 行，一次性重写风险过高
2. Zustand stores 已有良好隔离（serverId scoping），可作为 domain 层基础
3. app-bridge 已提供 DaemonTransport 抽象，可在 app 层复用

### 关键假设

1. 新 AI Provider 的增长速度可控（当前 4 个 builtin）
2. 前端功能扩展以 Agent/Workspace 为核心，不引入全新领域
3. 团队规模 < 5 人，不需要严格的分层治理

### 与 OCP/设计哲学

- **Go 后端**：Provider 注册机制符合 OCP（扩展 Provider 不修改 Manager）
- **前端**：Panel Registry 是 OCP 的好例子（新面板类型无需修改注册表）
- **违反 OCP**：Tmux Agent 检测硬编码（基线已指出，未修复）
