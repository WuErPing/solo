# Agent Provider 状态统一方案

## 现状分析

当前代码存在**三套重复的状态定义**：

### 1. 协议层 (`protocol/protocol.go`)
- `AgentLifecycleStatus`: initializing, idle, running, error, closed
- `ProviderStatus`: ready, loading, error, unavailable

### 2. 内部实现层 (`daemon/internal/agent/agent.go`)
- `AgentLifecycle`: initializing, idle, running, error, closed （与 AgentLifecycleStatus 值完全相同）

### 3. 前端层 (`app-bridge/src/shared/agent-lifecycle.ts`)
- `AGENT_LIFECYCLE_STATUSES` 数组定义

### 核心问题
- **重复定义**：`AgentLifecycle` 和 `AgentLifecycleStatus` 值完全相同，只是类型名不同
- **语义重叠**：`ProviderStatus.error` 与 `AgentLifecycleStatus.error` 含义不同但命名相同
- **职责不清**：Provider 可用性状态和 Agent 生命周期状态混用

---

## 统一方案（严格遵循开闭原则）

### 设计原则

1. **对扩展开放**：允许新增状态值而不修改现有代码
2. **对修改关闭**：不修改现有接口和行为
3. **单一职责**：Provider 状态和 Agent 状态职责分离
4. **向后兼容**：保留旧类型别名，避免破坏性变更

### 方案：引入统一基类 + 职责分离

```go
// protocol/protocol.go

// Status 是所有状态类型的统一基类
// 对扩展开放：新增状态只需定义新的类型别名
type Status string

// IsTerminal 判断状态是否为终止状态
func (s Status) IsTerminal() bool {
    return s == StatusClosed || s == StatusError
}

// IsActive 判断状态是否为活跃状态
func (s Status) IsActive() bool {
    return s == StatusRunning || s == StatusIdle
}

// ---- Agent 生命周期状态 ----
// AgentStatus 是 Agent 的运行时状态
type AgentStatus Status

const (
    AgentInitializing AgentStatus = "initializing"
    AgentIdle         AgentStatus = "idle"
    AgentRunning      AgentStatus = "running"
    AgentError        AgentStatus = "error"
    AgentClosed       AgentStatus = "closed"
)

// ToProtocol 转换为协议层类型（向后兼容）
func (s AgentStatus) ToProtocol() AgentLifecycleStatus {
    return AgentLifecycleStatus(s)
}

// ---- Provider 可用性状态 ----
// ProviderAvailabilityStatus 是 Provider 的可用性状态
type ProviderAvailabilityStatus Status

const (
    ProviderReady       ProviderAvailabilityStatus = "ready"
    ProviderLoading     ProviderAvailabilityStatus = "loading"
    ProviderError       ProviderAvailabilityStatus = "error"
    ProviderUnavailable ProviderAvailabilityStatus = "unavailable"
)

// ToProtocol 转换为协议层类型（向后兼容）
func (s ProviderAvailabilityStatus) ToProtocol() ProviderStatus {
    return ProviderStatus(s)
}

// ---- 向后兼容别名 ----
// 保留旧类型名，避免破坏性变更
type AgentLifecycleStatus = AgentStatus
type ProviderStatus = ProviderAvailabilityStatus
```

---

## 修改清单

| 文件 | 修改内容 |
|------|----------|
| `protocol/protocol.go` | 引入 `Status` 基类，重命名并保留别名 |
| `daemon/internal/agent/agent.go` | 将 `AgentLifecycle` 改为 `protocol.AgentStatus` |
| `daemon/internal/agent/manager.go` | 更新状态类型引用 |
| `app-bridge/src/shared/agent-lifecycle.ts` | 同步更新 TypeScript 定义 |

---

## 状态映射关系

| 场景 | 使用类型 | 说明 |
|------|----------|------|
| Provider 可用性检查 | `ProviderAvailabilityStatus` | 静态：二进制是否存在、服务是否启动 |
| Agent 运行时状态 | `AgentStatus` | 动态：初始化、空闲、运行、错误、关闭 |
| 前端显示 | `AgentStatus` | 统一显示逻辑 |

---

## 好处

1. **消除重复**：`AgentLifecycle` 和 `AgentLifecycleStatus` 合并
2. **职责清晰**：Provider 状态和 Agent 状态不再混淆
3. **类型安全**：编译期检查，避免字符串硬编码
4. **向后兼容**：保留旧类型别名，不破坏现有接口
5. **易于扩展**：新增状态只需定义新的常量

---

## 前端同步

```typescript
// app-bridge/src/shared/agent-lifecycle.ts
export const AGENT_STATUSES = [
  "initializing",
  "idle",
  "running",
  "error",
  "closed",
] as const;

export type AgentStatus = (typeof AGENT_STATUSES)[number];
```

---

## 实施建议

1. **第一阶段**：修改 `protocol/protocol.go`，引入统一基类
2. **第二阶段**：修改 `daemon/internal/agent/agent.go`，替换内部类型
3. **第三阶段**：同步更新前端类型定义
4. **第四阶段**：运行测试验证兼容性

---

*文档生成时间：2026-05-30*
