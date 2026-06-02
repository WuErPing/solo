# Analysis: Create Schedule Flow

## Overview

End-to-end flow for creating a schedule in Solo's schedule subsystem. A schedule periodically executes a prompt against an agent using either a cron expression or a fixed interval.

---

## Flow Diagram

```
[UI: ScheduleCreateModal]
        ↓
[Hook: useCreateSchedule] ──→ [React Query: invalidate schedules list]
        ↓
[Client: daemon-client.scheduleCreate()]
        ↓
[WebSocket RPC: "schedule/create"]
        ↓
[Daemon: Session.handleScheduleCreate()]
        ↓
[Store: schedule.Store.Create()] ──→ [In-memory storage]
        ↓
[Return ScheduleSummary]
```

---

## 1. User Interface Layer

**File:** `app/src/components/schedule-create-modal.tsx`

The modal collects the following fields from the user:

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `name` | No | `""` | Human-readable schedule name |
| `prompt` | **Yes** | `""` | The instruction sent to the agent on each run |
| `cadenceType` | **Yes** | `"cron"` | `"cron"` or `"every"` |
| `cronExpression` | Conditional | `"0 9 * * *"` | Standard cron expression (when `cadenceType=cron`) |
| `everyMs` | Conditional | `"3600000"` | Interval in milliseconds (when `cadenceType=every`) |
| `selectedAgentId` | **Yes** | `null` | Existing agent to execute the prompt |

### Form Validation

- Prompt cannot be empty
- Must select an agent
- For `every` mode: interval must be a positive integer
- For `cron` mode: expression must be non-empty (no further validation in UI)

### State Reset on Close

When the modal closes, all form fields reset to defaults:
```typescript
setName("");
setPrompt("");
setCadenceType("cron");
setCronExpression("0 9 * * *");
setEveryMs("3600000");
setSelectedAgentId(null);
setError(null);
```

---

## 2. React Hook Layer

**File:** `app/src/hooks/use-create-schedule.ts`

### Interface

```typescript
interface CreateScheduleInput {
  name?: string | null;
  prompt: string;
  cadence: ScheduleCadence;
  target: ScheduleTarget;
  maxRuns?: number;
  expiresAt?: string;
}

interface CreateScheduleResult {
  createSchedule: (input: CreateScheduleInput) => Promise<ScheduleSummary>;
  isCreating: boolean;
}
```

### Behavior

- Wraps `useMutation` from `@tanstack/react-query`
- Calls `client.scheduleCreate(...)` via `useHostRuntimeClient`
- On success: invalidates the schedules query cache to refresh the list
- Returns `isCreating` (maps to `mutation.isPending`) for loading states

### Target Construction

The UI only supports targeting an existing agent:
```typescript
const target: ScheduleTarget = { type: "agent", agentId: selectedAgentId };
```

Note: The protocol also supports `type: "new-agent"` with a full agent configuration, but the UI does not expose this option.

---

## 3. App-Bridge Client Layer

**File:** `app-bridge/src/client/daemon-client.ts:3497`

### Method Signature

```typescript
async scheduleCreate(options: CreateScheduleOptions): Promise<ScheduleCreatePayload>
```

### RPC Message Construction

```typescript
{
  type: "schedule/create",
  requestId: options.requestId,
  prompt: options.prompt,
  cadence: options.cadence,
  target: options.target,
  ...(options.name ? { name: options.name } : {}),
  ...(typeof options.maxRuns === "number" ? { maxRuns: options.maxRuns } : {}),
  ...(options.expiresAt ? { expiresAt: options.expiresAt } : {}),
}
```

### Request/Response Pattern

- Sends correlated session request via WebSocket
- Waits for response type: `"schedule/create/response"`
- Timeout: 10 seconds

---

## 4. Protocol Layer

**File:** `protocol/message_schedule.go`

### Key Types

```go
type ScheduleCreateRequest struct {
    RequestID string          `json:"requestId"`
    Prompt    string          `json:"prompt"`
    Name      string          `json:"name,omitempty"`
    Cadence   ScheduleCadence `json:"cadence"`
    Target    ScheduleTarget  `json:"target"`
    MaxRuns   *int            `json:"maxRuns,omitempty"`
    ExpiresAt string          `json:"expiresAt,omitempty"`
}

type ScheduleCreateResponse struct {
    Type    string                    `json:"type"`
    Payload ScheduleCreateResponsePayload `json:"payload"`
}

type ScheduleCreateResponsePayload struct {
    RequestID string           `json:"requestId"`
    Schedule  *ScheduleSummary `json:"schedule"`
    Error     *string          `json:"error,omitempty"`
}
```

---

## 5. Daemon Handler Layer

**File:** `daemon/internal/server/session_schedule.go`

### Handler

```go
func (s *Session) handleScheduleCreate(m *protocol.ScheduleCreateRequest) {
    sched, err := s.scheduleStore.Create(*m)
    if err != nil {
        s.sendScheduleCreateResponse(m.RequestID, nil, err.Error())
        return
    }
    summary := toScheduleSummary(sched)
    s.sendScheduleCreateResponse(m.RequestID, &summary, "")
}
```

### Error Handling

- Store-level validation errors → returned as string in `error` field
- Success → returns `ScheduleSummary` (runs array omitted)

---

## 6. Schedule Store Layer

**File:** `daemon/internal/schedule/store.go`

### Validation Rules

| Check | Error Message |
|-------|--------------|
| `Prompt == ""` | `"prompt is required"` |
| `Cadence.Type` not in `{"cron", "every"}` | `"invalid cadence type: X"` |
| `Cadence.Type == "every" && EveryMs <= 0` | `"everyMs must be positive"` |
| `Cadence.Type == "cron" && Expression == ""` | `"cron expression is required"` |

### Schedule Construction

```go
schedule := &protocol.StoredSchedule{
    ID:        generateID(),      // 16-char random hex
    Prompt:    input.Prompt,
    Cadence:   input.Cadence,
    Target:    input.Target,
    Status:    "active",
    CreatedAt: nowISO(),
    UpdatedAt: nowISO(),
    NextRunAt: computeNextRun(input.Cadence),
    Runs:      []protocol.ScheduleRun{},
}
```

### Next Run Computation

```go
func computeNextRun(cadence protocol.ScheduleCadence) *string {
    now := time.Now().UTC()
    var next time.Time
    switch cadence.Type {
    case "every":
        next = now.Add(time.Duration(cadence.EveryMs) * time.Millisecond)
    case "cron":
        // Simplified: just add 1 hour. Full cron parsing can be added later.
        next = now.Add(time.Hour)
    default:
        return nil
    }
    s := next.Format(time.RFC3339)
    return &s
}
```

**⚠️ Known Limitation:** Cron expressions are not fully parsed. The next run is always computed as `now + 1 hour`, regardless of the actual cron expression.

### Storage

- In-memory `map[string]*protocol.StoredSchedule`
- Protected by `sync.RWMutex`
- **Not persisted to disk** — all schedules lost on daemon restart

---

## 7. Response Flow

### Success Response

```json
{
  "type": "schedule/create/response",
  "payload": {
    "requestId": "...",
    "schedule": {
      "id": "...",
      "name": "...",
      "prompt": "...",
      "cadence": { "type": "cron", "expression": "0 9 * * *" },
      "target": { "type": "agent", "agentId": "..." },
      "status": "active",
      "createdAt": "2026-06-02T...",
      "updatedAt": "2026-06-02T...",
      "nextRunAt": "2026-06-02T..."
    },
    "error": null
  }
}
```

### Error Response

```json
{
  "type": "schedule/create/response",
  "payload": {
    "requestId": "...",
    "schedule": null,
    "error": "prompt is required"
  }
}
```

---

## Key Design Decisions & Limitations

| Aspect | Current Implementation | Limitation |
|--------|----------------------|------------|
| **Storage** | In-memory map | Lost on daemon restart |
| **Cron Parsing** | Simplified (always +1 hour) | Does not respect actual cron expression |
| **Target Types** | UI only supports existing agent | `new-agent` target available in protocol but not UI |
| **Concurrency** | Mutex-protected | Single-node only |
| **Validation** | Basic field checks | No cron syntax validation |
| **ID Generation** | 8-byte random hex | Not UUID format (though protocol uses `string.uuid()` in Zod) |

---

## Related Files

| Layer | File | Purpose |
|-------|------|---------|
| UI | `app/src/components/schedule-create-modal.tsx` | Form modal |
| UI | `app/src/screens/schedules-screen.tsx` | List view |
| Hook | `app/src/hooks/use-create-schedule.ts` | Creation mutation |
| Hook | `app/src/hooks/use-schedules.ts` | List query |
| Client | `app-bridge/src/client/daemon-client.ts:3497` | RPC client method |
| Schema | `app-bridge/src/server/schedule/rpc-schemas.ts` | Zod request/response schemas |
| Protocol | `protocol/message_schedule.go` | Go message types |
| Handler | `daemon/internal/server/session_schedule.go` | Session RPC handlers |
| Store | `daemon/internal/schedule/store.go` | In-memory storage |
| Types | `app-bridge/src/server/schedule/types.ts` | Domain model types |
