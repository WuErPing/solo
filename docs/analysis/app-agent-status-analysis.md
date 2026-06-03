# APP Agent Status and Copy Button Display Logic Analysis

## 1. Agent Lifecycle States

Defined at: `app-bridge/src/shared/agent-lifecycle.ts`

| State | Description |
|------|------|
| `initializing` | Initializing |
| `idle` | Idle, waiting for tasks |
| `running` | Running |
| `error` | Error |
| `closed` | Closed |

## 2. Copy Button Display Logic

**Code location**: `app/src/components/agent-stream-view.tsx:566-569`

```typescript
const isEndOfAssistantTurn =
  item.kind === "assistant_message" &&
  (nextItem?.kind === "user_message" ||
    (nextItem === undefined && agent.status !== "running"));
```

### Display Conditions

1. **Current message is an assistant_message** (AI reply)
2. **And one of the following is true**:
   - **Next message is a user_message** (user has sent a new message)
   - **No next message and agent status is not running** (stream has ended)

### State Judgment

| Agent State | Show Copy? | Description |
|-----------|--------------|------|
| `running` | ❌ Not shown | Stream in progress, conversation not ended |
| `idle` | ✅ Shown | Task completed, conversation ended |
| `error` | ✅ Shown | Error occurred, conversation ended |
| `closed` | ✅ Shown | Closed, conversation ended |
| `initializing` | ✅ Shown (theoretically) | Messages after initialization completes |

## 3. Key Conclusions

- **Conversation end determination**: `agent.status !== "running"`
- **Copy button is only shown for assistant_message when the conversation has ended**
- **During streaming output** (status = running), the Copy button is not shown
- **After the user sends a new message**, the previous assistant_message will show the Copy button
