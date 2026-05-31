# APP Agent 状态与 Copy 按钮显示逻辑分析

## 1. Agent 生命周期状态

定义位置：`app-bridge/src/shared/agent-lifecycle.ts`

| 状态 | 说明 |
|------|------|
| `initializing` | 初始化中 |
| `idle` | 空闲，等待任务 |
| `running` | 执行中 |
| `error` | 出错 |
| `closed` | 已关闭 |

## 2. Copy 按钮显示逻辑

**代码位置**：`app/src/components/agent-stream-view.tsx:566-569`

```typescript
const isEndOfAssistantTurn =
  item.kind === "assistant_message" &&
  (nextItem?.kind === "user_message" ||
    (nextItem === undefined && agent.status !== "running"));
```

### 显示条件

1. **当前消息是 assistant_message**（AI 回复）
2. **且满足以下任一**：
   - **下一条消息是 user_message**（用户已发送新消息）
   - **没有下一条消息且 agent 状态不是 running**（流已结束）

### 状态判断

| Agent 状态 | 是否显示 Copy | 说明 |
|-----------|--------------|------|
| `running` | ❌ 不显示 | 流进行中，对话未结束 |
| `idle` | ✅ 显示 | 任务完成，对话结束 |
| `error` | ✅ 显示 | 出错，对话结束 |
| `closed` | ✅ 显示 | 已关闭，对话结束 |
| `initializing` | ✅ 显示（理论上） | 初始化完成后的消息 |

## 3. 关键结论

- **对话结束判定**：`agent.status !== "running"`
- **Copy 按钮仅在 assistant_message 且对话结束时显示**
- **流式输出期间**（status = running）不显示 Copy 按钮
- **用户发送新消息后**，上一条 assistant_message 会显示 Copy 按钮
