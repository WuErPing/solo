# Kimi & Cursor-Agent Provider Integration Analysis & Solution

> Analysis date: 2026-05-21
> Repo: /Users/wuerping/code/wuerping/solo
> Scope: daemon (Go) + app-bridge (TS) + app (React Native)
>
> **Status update (2026-06-01)**: Kimi Wire mode provider is **fully implemented** (`provider_kimi.go`, 758 LOC, 23 tests). Pi provider is also **implemented** (`provider_pi.go`). Cursor-Agent remains planned. Codex has registry definition only, no backend implementation.

---

## 1. Executive Summary

| Provider | Current Status | Recommended Integration | Complexity |
|---------|---------|------------|--------|
| **Kimi** | **✅ Implemented** (JSON-RPC 2.0 stdio, EventPump) | **Wire mode** (`kimi --wire`) | Medium-High |
| **Cursor-Agent** | Not implemented | **Print mode** (`cursor agent --print --output-format stream-json`) | Medium |

---

## 2. Current State Deep Analysis

### 2.1 Kimi — Implemented

#### Implementation Overview

- ✅ `daemon/internal/agent/provider_kimi.go` — **758 LOC**, Wire mode, JSON-RPC 2.0 stdio, EventPump
- ✅ `daemon/internal/agent/provider_kimi_test.go` — **23 unit tests**
- ✅ `daemon/internal/agent/provider_registry.go` — `kimi` ID/Label/Modes in `BuiltinProviderDefinitions()`
- ✅ `app-bridge/src/server/agent/provider-manifest.ts` — `KIMI_MODES` and `AGENT_PROVIDER_DEFINITIONS` defined
- ✅ `app/src/utils/provider-command-templates.ts` — `kimi --resume {sessionId}` command template

#### Historical Context (Pre-implementation)

> The following records pre-implementation research, kept for reference:
> - `kimi --print --output-format stream-json` outputs complete JSON (non-streaming), poor experience
> - Ultimately chose `kimi --wire` (JSON-RPC 2.0 over stdin/stdout) for true bidirectional streaming

#### CLI Research Results

```bash
$ kimi --version
kimi-cli version: 1.43.0
agent spec versions: 1
wire protocol: 1.10
```

`kimi` CLI installed at `/Users/wuerping/.local/bin/kimi`.

**Print mode test:**

```bash
$ kimi --print --output-format stream-json --prompt "hi"
{"role":"assistant","content":[{"type":"think","think":"...","encrypted":null},{"type":"text","text":"Hi there! ..."}]}
To resume this session: kimi -r <session-id>
```

⚠️ **Key finding**: `--print --output-format stream-json` outputs **complete JSON (non-streaming)** — one line per turn only after the entire turn completes. UI must wait for the full turn before displaying content, resulting in poor experience.

✅ **Best integration point**: `kimi --wire` provides **JSON-RPC 2.0 over stdin/stdout** Wire protocol, supporting true bidirectional streaming.

#### Wire Protocol Core Features

Wire is Kimi Code CLI's low-level communication protocol, designed specifically for external program integration.

**Protocol basics:**
- Transport: Line-by-line JSON-RPC 2.0 via stdin/stdout
- Version: 1.10
- Direction: Bidirectional (Client request ↔ Server event/request)

**Client request methods:**

| Method | Direction | Type | Description |
|------|------|------|------|
| `initialize` | Client → Agent | Request | Handshake, negotiate protocol version, register external tools |
| `prompt` | Client → Agent | Request | Send user input, start agent turn |
| `cancel` | Client → Agent | Request | Cancel current turn |
| `set_plan_mode` | Client → Agent | Request | Set plan mode toggle |
| `steer` | Client → Agent | Request | Inject additional input into running turn |
| `replay` | Client → Agent | Request | Trigger history replay |

**Server push notifications:**

| Method | Direction | Type | Description |
|------|------|------|------|
| `event` | Agent → Client | Notification | Streaming events (no response needed) |
| `request` | Agent → Client | Request | Permission/tool call requests (must respond) |

**Event types (key):**

| Event | Description | Solo Mapping |
|-------|------|----------|
| `TurnBegin` | Turn starts | `thread_started` |
| `ContentPart(text)` | Text chunk | `timeline(assistant_message)` |
| `ContentPart(think)` | Thinking chunk | `timeline(reasoning)` |
| `ToolCall` | Tool call | `timeline(tool_call)` |
| `ToolResult` | Tool execution result | `timeline(tool_call completed)` |
| `ApprovalResponse` | Approval completed | — |
| `TurnEnd` | Turn ends | `turn_completed` |
| `StepBegin` | Step starts | — |
| `StepRetry` | Step retry | — |
| `CompactionBegin/End` | Context compaction | `timeline(compaction)` |
| `StatusUpdate` | Status update | — |

**Request types (require client response):**

| Request | Description | Solo Mapping |
|---------|------|----------|
| `ApprovalRequest` | Operation approval request | `permission_requested` |
| `ToolCallRequest` | External tool call | — (e.g., register external tools) |
| `QuestionRequest` | Structured question (AskUserQuestion) | — |
| `HookRequest` | Hook processing request | — |

**Error codes:**

| Code | Description |
|------|------|
| `-32000` | Turn in progress / unsupported operation |
| `-32001` | LLM not configured |
| `-32002` | Specified LLM not supported |
| `-32003` | LLM service error |
| `-32700` | Invalid JSON format |
| `-32601` | Method not found |

**Example interaction:**

```json
// 1. Client sends initialize
{"jsonrpc":"2.0","method":"initialize","id":"1","params":{"protocol_version":"1.10","client":{"name":"solo","version":"0.1.0"},"capabilities":{"supports_question":true}}}

// 2. Server responds
{"jsonrpc":"2.0","id":"1","result":{"protocol_version":"1.10","server":{"name":"Kimi Code CLI","version":"1.43.0"},"slash_commands":[...],"capabilities":{"supports_question":true}}}

// 3. Client sends prompt
{"jsonrpc":"2.0","method":"prompt","id":"2","params":{"user_input":"Hello"}}

// 4. Server pushes events (streaming)
{"jsonrpc":"2.0","method":"event","params":{"type":"TurnBegin","payload":{"user_input":"Hello"}}}
{"jsonrpc":"2.0","method":"event","params":{"type":"ContentPart","payload":{"type":"text","text":"Hi"}}}
{"jsonrpc":"2.0","method":"event","params":{"type":"ContentPart","payload":{"type":"text","text":" there!"}}}
{"jsonrpc":"2.0","method":"event","params":{"type":"TurnEnd","payload":{}}}

// 5. Prompt request final response
{"jsonrpc":"2.0","id":"2","result":{"status":"finished"}}
```

**Approval request interaction:**

```json
// Server sends ApprovalRequest
{"jsonrpc":"2.0","method":"request","id":"req-1","params":{"type":"ApprovalRequest","payload":{"id":"approval-1","tool_call_id":"tc-1","sender":"Shell","action":"run shell command","description":"Run command `ls`","display":[]}}}

// Client responds
{"jsonrpc":"2.0","id":"req-1","result":{"request_id":"approval-1","response":"approve"}}
```

---

### 2.2 Cursor-Agent — Not Implemented

#### Current Status

- No `cursor-agent` definitions, implementations, or icons in the project
- `app/assets/images/editor-apps/cursor.png` exists (editor app icon, not provider icon)

#### CLI Research Results

```bash
$ cursor agent --help
Usage: cursor agent [options] [command] [prompt...]

Start the Cursor Agent
```

`cursor` CLI installed at `/Users/wuerping/.local/bin/cursor`.

**Core options:**

| Option | Description |
|------|------|
| `-p, --print` | Non-interactive print mode |
| `--output-format <format>` | `text` / `json` / `stream-json` |
| `--stream-partial-output` | Stream text increment output |
| `--mode <mode>` | `plan` / `ask` |
| `--plan` | Plan mode shorthand |
| `--resume [chatId]` | Resume session |
| `--continue` | Continue previous session |
| `--model <model>` | `gpt-5`, `sonnet-4`, `sonnet-4-thinking`, etc. |
| `-f, --force` / `--yolo` | Auto-approve all operations |
| `--trust` | Trust current workspace (required for headless) |
| `--workspace <path>` | Specify working directory |
| `--sandbox <mode>` | Sandbox mode toggle |

**stream-json output format (per docs and third-party SDK):**

```jsonl
{"type": "start", "chatId": "abc123"}
{"type": "content", "delta": "Analyzing..."}
{"type": "tool_use", "tool": "read_file", "args": {"path": "main.py"}}
{"type": "content", "delta": "Found 5 functions..."}
{"type": "end", "result": "Analysis complete"}
```

⚠️ **Note**: Currently unable to test `cursor agent --print` streaming output due to network/auth issues. Implementation is inferred from official docs and third-party SDKs (`cursor-cli`, `@nothumanwork/cursor-agents-sdk`).

---

## 3. Solo Provider Technical Architecture Review

Each Provider must implement two Go interfaces:

### 3.1 AgentClient (Provider level)

```go
type AgentClient interface {
    Provider() string
    IsAvailable(ctx context.Context) error
    CreateSession(ctx context.Context, config *protocol.AgentSessionConfig) (AgentSession, error)
    ResumeSession(ctx context.Context, handle *protocol.AgentPersistenceHandle) (AgentSession, error)
    ListModels(ctx context.Context, cwd string) ([]protocol.AgentModelDefinition, error)
    ListModes(ctx context.Context, cwd string) ([]protocol.AgentMode, error)
    ListClientCommands(ctx context.Context, cwd string) ([]protocol.AgentSlashCommand, error)
}
```

### 3.2 AgentSession (Session level)

```go
type AgentSession interface {
    Run(ctx context.Context, text string, images []protocol.ImageAttachment, attachments []protocol.AgentAttachment) (*AgentRunResult, error)
    StartTurn(ctx context.Context, text string, images []protocol.ImageAttachment, attachments []protocol.AgentAttachment) (<-chan AgentStreamEvent, error)
    Subscribe() <-chan AgentStreamEvent
    Interrupt(ctx context.Context) error
    Close() error
    RespondPermission(requestID string, response protocol.AgentPermissionResponse) error
    GetRuntimeInfo(ctx context.Context) (*protocol.AgentRuntimeInfo, error)
    GetAvailableModes(ctx context.Context) ([]protocol.AgentMode, error)
    GetCurrentMode(ctx context.Context) (*string, error)
    SetMode(modeID string) error
    SetModel(modelID string) error
    SetThinkingOption(optionID string) error
    DescribePersistence() *protocol.AgentPersistenceHandle
    GetPendingPermissions() []interface{}
    ListCommands(ctx context.Context) ([]protocol.AgentSlashCommand, error)
    StreamHistory(ctx context.Context) ([]AgentStreamEvent, error)
}
```

### 3.3 Existing Reference Implementations

| Provider | File | Mode | Characteristics |
|---------|------|------|------|
| Claude | `provider_claude.go` | stdio | Launches `claude --print --output-format stream-json`, line-by-line SDK message parsing, custom translator + terminal detector |
| OpenCode | `provider_opencode.go` | HTTP server | Launches local server, `/session` API + SSE `/global/event`, supports reasoning/thinking |
| Mock | `provider_mock.go` | In-memory | For testing, simulates event stream |

**Key infrastructure:**
- `base.BaseSession` — Common session state management (sessionID, mode, model, cancelFn)
- `base.ProcessManager` — Subprocess lifecycle management (Start/Stop/Interrupt/Kill/DrainStderr/WaitForExit)
- `base.ChannelDispatcher` — Event dispatch (subscribe/broadcast)
- `base.EventPump` — Blocking/background event pump (line-by-line stdout reading, calls translator + terminal detector)

---

## 4. Recommended Solution

### 4.1 Kimi — Wire Mode (Strongly Recommended)

**Rationale:**
1. Wire protocol is Kimi's official protocol designed for "embedding in other applications", well-documented
2. True bidirectional streaming, supports incremental content, tool calls, permission requests
3. Most similar architecture to Claude's stdio mode (line-by-line stdout reading, translated to internal events)
4. Supports session persistence (`--session` / `--continue`)
5. Avoids Print mode's "full turn buffering" issue

**Architecture diagram:**

```
┌─────────────────┐      stdin      ┌─────────────────┐
│                 │ ──────────────> │                 │
│  Solo Daemon    │   JSON-RPC req  │  kimi --wire    │
│  (provider_kimi)│                 │  (Wire server)  │
│                 │ <────────────── │                 │
│                 │   stdout        │                 │
│                 │   JSON-RPC      │                 │
│                 │   event/request │                 │
└─────────────────┘                 └─────────────────┘
         │                                    │
         │ Translate Wire events              │ LLM / Tools
         │ to AgentStreamEvent                │
         v                                    v
┌─────────────────┐                 ┌─────────────────┐
│  Solo Event     │                 │  Moonshot AI    │
│  Pipeline       │                 │  Kimi API       │
│  (timeline,     │                 │                 │
│   coalescer)    │                 │                 │
└─────────────────┘                 └─────────────────┘
```

**Implementation notes:**

1. **Process launch**: `kimi --wire --work-dir <cwd> --session <id>` (or `--continue`)
2. **Handshake**: Send `initialize` request via stdin, negotiate `protocol_version: "1.10"`
3. **Start Turn**: Send `prompt` request via stdin
4. **Read events**: Line-by-line stdout reading, parse JSON-RPC
   - `method: "event"` → translate to `AgentStreamEvent`, push to dispatcher
   - `method: "request"` → handle `ApprovalRequest`, write response via stdin
5. **Event translation mapping:**

| Wire Event | Solo Event |
|-----------|-----------|
| `TurnBegin` | `thread_started` |
| `ContentPart(text)` | `timeline(assistant_message)` |
| `ContentPart(think)` | `timeline(reasoning)` |
| `ToolCall` | `timeline(tool_call)` |
| `ToolResult` | `timeline(tool_call completed)` |
| `ApprovalRequest` | `permission_requested` |
| `CompactionBegin` | `timeline(compaction loading)` |
| `CompactionEnd` | `timeline(compaction completed)` |
| `TurnEnd` | `turn_completed` |
| `StepRetry` | `timeline(error)` |

6. **Permission response**: For `ApprovalRequest`, write JSON-RPC response via stdin in `RespondPermission()`
7. **Interrupt**: Send `cancel` request (JSON-RPC), or send SIGINT

### 4.2 Cursor-Agent — Print Stream-JSON Mode

**Rationale:**
1. Cursor has no public Wire/JSON-RPC/ACP protocol
2. `--print --output-format stream-json --stream-partial-output` provides line-by-line NDJSON stream
3. Most similar architecture to Claude's stdio mode, can reuse `base.EventPump` + translator pattern

**Architecture diagram:**

```
┌─────────────────┐      stdout     ┌─────────────────────────┐
│                 │ <────────────── │                         │
│  Solo Daemon    │   NDJSON lines  │  cursor agent --trust   │
│  (provider_     │                 │  --print                │
│   cursor_agent) │                 │  --output-format        │
│                 │                 │   stream-json           │
│                 │                 │  --stream-partial-output│
└─────────────────┘                 └─────────────────────────┘
         │                                        │
         │ Translate Cursor events                │ Cursor Cloud / LLM
         │ to AgentStreamEvent                    │
         v                                        v
┌─────────────────┐                 ┌─────────────────────────┐
│  Solo Event     │                 │  Cursor Services        │
│  Pipeline       │                 │                         │
└─────────────────┘                 └─────────────────────────┘
```

**Implementation notes:**

1. **Process launch**: `cursor agent --trust --print --output-format stream-json --stream-partial-output --workspace <cwd> --resume <id> <prompt>`
2. **Read events**: Line-by-line NDJSON from stdout
3. **Event translation mapping (speculative, needs real testing):**

| Cursor Event | Solo Event |
|-------------|-----------|
| `start` | `thread_started` |
| `content(delta)` | `timeline(assistant_message)` (incremental accumulation) |
| `thinking` / `reasoning` | `timeline(reasoning)` |
| `tool_use` | `timeline(tool_call)` |
| `tool_result` | `timeline(tool_call completed)` |
| `permission_request` | `permission_requested` |
| `end` | `turn_completed` |
| `error` | `turn_failed` |

4. **Interrupt**: Send SIGINT to process
5. **Trust handling**: Must always include `--trust` to avoid interactive confirmation hang in headless mode

---

## 5. Implementation Plan

### Phase 1: Kimi Provider (Priority, estimated 2-3 days)

| # | File | Operation | Description |
|---|------|------|------|
| 1 | `daemon/internal/agent/provider_kimi.go` | **Create** | `KimiAgentClient` + `kimiSession` + `kimiWireTranslator` + `kimiWireTerminalDetector` |
| 2 | `daemon/internal/server/daemon.go` | **Modify** | Add `registry.Register(agent.NewKimiAgentClient("", logger))` |
| 3 | `daemon/internal/agent/provider_kimi_test.go` | **Create** | Unit tests (mock stdin/stdout Wire interactions) |

**`provider_kimi.go` structure draft:**

```go
package agent

// KimiAgentClient implements AgentClient
type KimiAgentClient struct {
    binaryPath string
    logger     *slog.Logger
}

// kimiSession implements AgentSession
type kimiSession struct {
    mu sync.Mutex
    base       *base.BaseSession
    dispatcher *base.ChannelDispatcher
    process    processManager
    binaryPath string
    cmd        *exec.Cmd
    stdinPipe  io.WriteCloser
    stdoutPipe io.ReadCloser
    activeTurnID string
    // JSON-RPC state
    nextRequestID int
    pendingApprovals map[string]chan string // requestID -> response channel
}

// kimiWireTranslator translates Wire events to AgentStreamEvent
type kimiWireTranslator struct {
    session *kimiSession
}

// kimiWireTerminalDetector detects turn end
type kimiWireTerminalDetector struct {
    session *kimiSession
}
```

### Phase 2: Cursor-Agent Provider (estimated 1-2 days)

| # | File | Operation | Description |
|---|------|------|------|
| 4 | `daemon/internal/agent/provider_cursor_agent.go` | **Create** | `CursorAgentClient` + `cursorSession` + `cursorTranslator` + `cursorTerminalDetector` |
| 5 | `daemon/internal/server/daemon.go` | **Modify** | Add `registry.Register(agent.NewCursorAgentClient("", logger))` |
| 6 | `daemon/internal/agent/provider_cursor_agent_test.go` | **Create** | Unit tests |

### Phase 3: Frontend Definitions & Icons (estimated 0.5 days)

| # | File | Operation | Description |
|---|------|------|------|
| 7 | `app-bridge/src/server/agent/provider-manifest.ts` | **Modify** | Add `CURSOR_AGENT_MODES` and `cursor-agent` to `AGENT_PROVIDER_DEFINITIONS` |
| 8 | `daemon/internal/agent/provider_registry.go` | **Modify** | Add `cursor-agent` to `BuiltinProviderDefinitions()` |
| 9 | `app/src/utils/provider-command-templates.ts` | **Modify** | Add `cursor-agent` resume template |
| 10 | `app/src/components/icons/kimi-icon.tsx` | **Create** | Kimi Logo SVG React component |
| 11 | `app/src/components/icons/cursor-agent-icon.tsx` | **Create** | Cursor Logo SVG React component |
| 12 | `app/src/components/provider-icons.ts` | **Modify** | Add `kimi` and `cursor-agent` mappings |

### Phase 4: Integration Testing & Optimization (estimated 1-2 days)

| # | Task | Description |
|---|------|------|
| 13 | E2E test | Create Kimi/Cursor-Agent agent via App or CLI, verify full lifecycle |
| 14 | Streaming event verification | Confirm timeline events reach frontend correctly, no loss or reordering |
| 15 | Permission request test | Verify ApprovalRequest/permission_requested end-to-end flow |
| 16 | Session resume test | Verify `--resume` / `--continue` restores existing sessions |
| 17 | Error handling test | Verify graceful degradation for LLM not configured, network errors, etc. |

---

## 6. Risks & Considerations

### 6.1 Kimi Wire Mode

| Risk | Impact | Mitigation |
|------|------|---------|
| **Protocol version evolution** | Wire 1.10 may change in the future | Explicitly declare `protocol_version` during `initialize`, reject incompatible versions with upgrade prompt |
| **ApprovalRequest response delay** | Agent hangs if user doesn't respond in time | Set reasonable timeout (default 30s), auto-reject on timeout; support `approve_for_session` |
| **JSON-RPC ID collision** | Concurrent request/response may get out of order | Use UUID or atomic incrementing ID, maintain `pendingRequests` map |
| **stdin write concurrency** | `RespondPermission` and main loop write stdin simultaneously | Use locked writer, or serialize all writes through a single goroutine |
| **Process crash detection** | `kimi --wire` fails to start or crashes at runtime | Reuse Claude provider's 100ms startup health check mechanism |
| **Session directory conflict** | Solo's cwd differs from Kimi's `--work-dir` | Always map `config.Cwd` to `--work-dir`, session ID to `--session` |

### 6.2 Cursor-Agent

| Risk | Impact | Mitigation |
|------|------|---------|
| **Output format not tested** | stream-json field names/structure may differ from docs | Add lenient parsing logic (ignore unknown fields), reserve debug log toggle |
| **Trust requirement** | Missing `--trust` causes headless mode to hang waiting for user input | Force-add `--trust` parameter; if confirmation still needed, pre-detect and return clear error |
| **Auth issues** | `cursor agent` requires login or `CURSOR_API_KEY` | Run `cursor agent status` or lightweight command in `IsAvailable()` to detect auth status |
| **Network dependency** | Cursor calls cloud services, may timeout or fail | Set reasonable process timeout (e.g., 35min watchdog), output `turn_failed` on error |
| **Model name changes** | `--model` supported model list may change | `ListModels()` should prefer `cursor agent models` for dynamic retrieval, fall back to static list on failure |

### 6.3 General

| Risk | Impact | Mitigation |
|------|------|---------|
| **Streaming vs complete output** | Both providers need to translate external event formats | Establish unified translator test framework, verify output for each event type |
| **Model list** | Dynamic query may fail or be slow | Support static fallback, cache dynamic results |
| **History** | `StreamHistory()` implementation is complex | Phase 1 can return `nil, nil` (non-blocking), iterate later via CLI's replay/history commands |
| **Icon copyright** | Using brand logos may involve copyright issues | Use generic `Bot` icon as placeholder, or confirm brand usage license before replacing

---

## 7. Effort Estimation

| Phase | Content | Estimated LOC | Estimated Time |
|-------|------|-----------|---------|
| Phase 1 | Kimi Provider (Go) | ~800 lines | 2-3 days |
| Phase 2 | Cursor-Agent Provider (Go) | ~500 lines | 1-2 days |
| Phase 3 | Frontend definitions + icons (TS/TSX) | ~200 lines | 0.5 days |
| Phase 4 | Integration testing + tuning | — | 1-2 days |
| **Total** | | **~1500 lines** | **5-7.5 days** |

---

## 8. Appendix

### A. Kimi CLI Key Command Reference

```bash
# Start Wire server
kimi --wire --work-dir /path/to/project

# Specify session
kimi --wire --session <session-id>

# Continue previous session
kimi --wire --continue

# Specify model
kimi --wire --model k2

# Plan mode
kimi --wire --plan

# YOLO mode
kimi --wire --yolo

# View info
kimi info
```

### B. Cursor Agent CLI Key Command Reference

```bash
# Start agent (headless)
cursor agent --trust --print --output-format stream-json --stream-partial-output "prompt"

# Specify workspace
cursor agent --trust --workspace /path/to/project --print ...

# Resume session
cursor agent --trust --resume <chatId> --print ...

# Plan mode
cursor agent --trust --plan --print ...

# YOLO mode
cursor agent --trust --yolo --print ...

# List models
cursor agent models
```

### C. Related Documentation Links

- [Kimi Wire Mode docs](https://moonshotai.github.io/kimi-cli/en/customization/wire-mode.md)
- [Kimi Print Mode docs](https://moonshotai.github.io/kimi-cli/en/customization/print-mode.md)
- [Cursor CLI docs (PraisonAI)](https://docs.praison.ai/docs/cli/cursor-cli)
- [Cursor Agents SDK (npm)](https://www.npmjs.com/package/@nothumanwork/cursor-agents-sdk)
- [Kimi CLI Issue #2179 — Incremental token deltas feature request](https://github.com/MoonshotAI/kimi-cli/issues/2179)
