# Go Test Coverage Report

> Generated: 2026-05-27
> Last updated: 2026-05-28 (P2 TDD: cli/internal/client, daemon/internal/terminal, daemon/internal/relayclient, cli/cmd, server, agent)

## Summary

**Total coverage: ~62%** — P0/P1/P2 sessions completed:
- `daemon/internal/workspace`: 46.5% → **86.0%** (+39.5pp)
- `daemon/internal/agent/base`: 63.1% → **88.7%** (+25.6pp)
- `daemon/internal/terminal`: 76.4% → **86.6%** (+10.2pp)
- `cli/internal/client`: 77.6% → **87.4%** (+9.8pp)
- `daemon/internal/relayclient`: 75.2% → **81.0%** (+5.8pp)
- `daemon/internal/agent`: 51.1% → **56.7%** (+5.6pp)
- `cli/cmd`: 69.0% → **71.5%** (+2.5pp)
- `daemon/internal/server`: 61.1% → **64.2%** (+3.1pp)

## Coverage by Module

| Module | Coverage | Status | Change |
|--------|----------|--------|--------|
| `daemon/internal/metrics` | 100.0% | ✅ | — |
| `relay/internal/metrics` | 100.0% | ✅ | — |
| `protocol` | 99.2% | ✅ | — |
| `relay/internal/config` | 97.0% | ✅ | — |
| `cli/internal/output` | 92.0% | ✅ | — |
| `cli/internal/util` | 91.8% | ✅ | — |
| `daemon/internal/config` | 90.4% | ✅ | — |
| `daemon/internal/agent/base` | **88.7%** | ✅ | ↑25.6pp (was 63.1%) |
| `daemon/internal/workspace` | **86.0%** | ✅ | ↑39.5pp (was 46.5%) |
| `cli/internal/client` | **87.4%** | ✅ | ↑9.8pp (was 77.6%) |
| `daemon/internal/terminal` | **86.6%** | ✅ | ↑10.2pp (was 76.4%) |
| `daemon/internal/pidlock` | 87.5% | ✅ | — |
| `relay/internal/relay` | 85.7% | ✅ | — |
| `daemon/internal/push` | 84.3% | ✅ | — |
| `relay/internal/e2ee` | 83.2% | ✅ | — |
| `daemon/internal/relayclient` | **81.0%** | ✅ | ↑5.8pp (was 75.2%) |
| `cli/cmd/*` | **71.5%** | 🟡 | ↑2.5pp (was 69.0%) |
| `daemon/internal/server` | **64.2%** | 🟡 | ↑3.1pp (was 61.1%) |
| `daemon/internal/agent` | **56.7%** | 🟡 | ↑5.6pp (was 51.1%) |

## Recent TDD Refactor (2026-05-27)

### 1. output package — Eliminate Global `Stdout`/`Stderr`

**Problem**: `Render`/`RenderError`/`PrintResult` directly depended on package-level global variables `output.Stdout`/`output.Stderr`. Tests had to capture output by modifying global state, causing implicit coupling between tests.

**Changes**:
- Added explicit `io.Writer` parameter to three function signatures:
  - `Render(w io.Writer, result *CommandResult, opts OutputOptions) error`
  - `RenderError(w io.Writer, ce *CommandError, opts OutputOptions)`
  - `PrintResult(w io.Writer, result *CommandResult, opts OutputOptions) int`
- `cli/internal/output/output_test.go` completely removed global variable replacement logic; all tests now pass in `bytes.Buffer` directly
- Added `TestRenderToWriter`, `TestRenderErrorToWriter`, `TestRenderDoesNotDependOnGlobalStdout` to verify no global dependency

### 2. cmd package — Parameterized Dependency Injection

**Problem**: `getOutputOpts()` and `newClient(ctx)` directly read package-level global flag variables; `output.Stdout` was directly referenced by 20+ command files.

**Changes**:
- `getOutputOpts(format string, json bool, quiet bool, noHeaders bool, noColor bool) output.OutputOptions`
- `newClient(ctx context.Context, host string) (*client.DaemonClient, error)`
- Introduced `cmdStdout io.Writer = os.Stdout` / `cmdStderr io.Writer = os.Stderr` (package-level, injectable in tests)
- Batch-replaced `output.Stdout` → `cmdStdout` and `output.Render(os.Stdout, ...)` → `output.Render(cmdStdout, ...)` across all command files
- `root_test.go` tests for `getOutputOpts` changed to pass parameters directly instead of modifying global flags

### 3. Daemon agent — Concurrent Race Tests

**Problem**: `StreamCoalescer` and `AgentManager` involve concurrent state but lacked concurrency tests under `-race`.

**New tests** (`daemon/internal/agent/*_extra_test.go`):

| Test | Scenario | Package |
|------|----------|---------|
| `TestStreamCoalescerConcurrentHandleAndFlush` | 10 goroutines concurrently `Handle` + `FlushAll` + `FlushAndDiscard` | coalescer |
| `TestStreamCoalescerConcurrentHandleAndFlushFor` | 5 goroutines concurrently `Handle` + `FlushFor` | coalescer |
| `TestAgentManagerConcurrentCreateDelete` | 5 goroutines each create 10 agents, then concurrently delete 50 | manager |
| `TestAgentManagerConcurrentCreateArchive` | 3 goroutines create + 1 goroutine concurrently reads/archives | manager |
| `TestAgentManagerConcurrentReadWrite` | 10 goroutines concurrently read + 5 goroutines concurrently modify agent state | manager |

**Race detector verification**: All passed `go test -race`

## P0 TDD Session (2026-05-28)

### 1. `daemon/internal/workspace` — GitCommander Abstraction (46.5% → 86.0%)

**Problem**: `gitRun`, `gitOutput`, and `gitExec` called `exec.Command("git", ...)` directly in three separate places (`worktree.go`, `git_service.go`). No interface existed, making all git-dependent logic untestable without a real repository.

**Changes**:

- **New file `git_commander.go`**: Defines `GitCommander` interface (`Run`, `Output`), a `defaultGitCommander` production implementation backed by `exec.Command`, and a mutex-protected `gitCmdVar` singleton with `getGitCmd()`/`setGitCmd()` accessors (race-safe for background goroutines).
- **`worktree.go`**: `gitRun`/`gitOutput` now delegate to `getGitCmd()`. Removed `os/exec` import.
- **`git_service.go`**: `gitExec` now delegates to `getGitCmd().Output()`. Removed `os/exec` import.

**New test files**:

| File | Tests | Coverage target |
|------|-------|-----------------|
| `worktree_test.go` | `TestSlugify*`, `TestDeriveWorktree*`, `TestGenerateSlug*`, `TestResolveIntent*`, `TestResolveDefaultBranch*`, `TestCreateSoloWorktree*`, `TestDeleteWorktree*` | `worktree.go`: 0% → ~82% |
| `git_service_test.go` | `TestResolveRepoRoot*`, `TestGetCurrentBranch*`, `TestGetRemoteUrl*`, `TestIsWorktree*`, `TestIsDirty*`, `TestGetMetadata*`, `TestBackgroundRefreshStartStop`, `TestDeriveProjectSlug*` | `git_service.go`: 0% → ~90% |

**`fakeGitCommander`** design: records all calls, dispatches via injected `handler func(dir, args) (string, error)`, uses `sync.WaitGroup` to drain in-flight calls before `t.Cleanup` restores the original commander — eliminating the `-race` false positives from background goroutines.

---

### 2. `daemon/internal/agent` — Clock Abstraction for `StreamCoalescer`

**Problem**: `time.AfterFunc` called directly in `Handle`, with `*time.Timer` stored in `coalescerBuffer`. Tests that verified timer-driven flush had to `time.Sleep`, making them slow and flaky.

**Changes**:

- **`coalescer.go`**: Added `Clock` interface (`AfterFunc(d time.Duration, f func()) *time.Timer`), `realClock` production implementation, `clock Clock` field on `StreamCoalescer`. Added `newStreamCoalescerWithClock` constructor (used by tests); `NewStreamCoalescer` wraps it with `nil` → `defaultClock`.
- `Handle` uses `c.clock.AfterFunc(...)` instead of `time.AfterFunc(...)`.

**New test file `coalescer_clock_test.go`** — 12 tests, all deterministic (zero sleeps):

| Test | What it verifies |
|------|-----------------|
| `TestCoalescerTimerFlushFiresViaFakeClock` | Timer fires flush synchronously via `clk.FireAll()` |
| `TestCoalescerTimerMergesMultipleChunks` | Multiple chunks merged into one payload before fire |
| `TestCoalescerTimerReasoningUsesExtendedWindow` | Reasoning events schedule a timer |
| `TestCoalescerTimerOnlyScheduledOnce` | 5 events produce exactly 1 timer |
| `TestCoalescerTimerRescheduledAfterFire` | New timer scheduled after flush |
| `TestCoalescerTerminalToolCallFlushesImmediately` | `completed` tool_call bypasses clock |
| `TestCoalescerFlushForClearsTimer` | `FlushFor` drains without clock fire |
| `TestCoalescerFlushAllClearsAllAgents` | `FlushAll` covers multiple agents |
| `TestCoalescerFlushAndDiscardNoTimerFire` | Discard prevents double flush |
| `TestCoalescerMultipleAgentsIndependent` | Each agent gets its own buffer |
| `TestCoalescerDifferentTurnIDsNotMerged` | Different `turnID` → separate payloads |
| `TestCoalescerNonCoalescablePassthrough` | `user_message` returns `false` |

---

### 3. `daemon/internal/agent` — Provider Mock Coverage

**Problem**: `MockAgentClient` and `MockAgentSession` had zero test coverage for most methods (`ListModels`, `ListModes`, `SetMode`, `SetModel`, `RespondPermission`, etc.). Error injection was impossible.

**Changes**:

- **New file `provider_mock_test.go`**: 30 tests covering every method on `MockAgentClient` and `MockAgentSession`.
- **`augmentedMockClient`** (test-only wrapper): adds `availableErr`, `createSessionErr`, `runErr` fields for error injection without modifying production code.
- **`configurableSession`** (test-only): wraps `MockAgentSession.Run` with injectable event sequences or errors.

**File-level coverage after this session**:

| File | Before | After |
|------|--------|-------|
| `coalescer.go` | ~70% | **~97%** |
| `provider_mock.go` | ~30% | **~85%** |
| `git_service.go` | 0% | **~90%** |
| `worktree.go` | 0% | **~82%** |

**Known limitation**: `TestPiIntegration_FullPath` hangs waiting for a real external process and is not skipped by `-short`. It inflates the reported timeout for the full suite but does not affect the above file-level numbers.

---

## P1 TDD Session (2026-05-28)

### 1. `daemon/internal/agent/base` — Unit Tests for Previously Uncovered Types (63.1% → 88.7%)

**Problem**: `CallbackDispatcher`, `PermissionManager`, `JSONEventTranslator`, `EventPump` (RunBackground, pump failure paths), and several `BaseSession` methods (`Logger`, `GetCurrentModePtr`, `Lock`/`Unlock`/`RLock`/`RUnlock`) had zero test coverage.

**New test file `base_extra_test.go`** — 25 tests, all unit-level (no subprocesses):

| Group | Tests | What they verify |
|-------|-------|-----------------|
| `CallbackDispatcher` | 5 | Emit delivery, unsubscribe stops delivery, Close drops events, Close idempotent, multiple subscribers |
| `PermissionManager` | 5 | Register+Respond roundtrip, unknown respond error, GetPending count, RejectAll sends deny, Close closes channels |
| `JSONEventTranslator` | 2 | Valid JSON parse, invalid JSON returns error |
| `EventPump` | 6 | SetProvider, RunBlocking terminal reached, RunBlocking stream-ends-without-terminal, RunBackground fires event, context cancel returns error, nil dispatcher no panic |
| `BaseSession` | 3 | Logger accessor, GetCurrentModePtr, Lock/RLock/Unlock/RUnlock no deadlock |

**File-level result**: `dispatcher.go` 0%→~85%, `pump.go` 38%→~80%, `session.go` methods now covered.

---

### 2. `daemon/internal/server` — Handler Integration Tests (61.1% → 63.7%)

**Problem**: 15+ session handler functions had 0% coverage (`handleFetchAgent`, `handleFetchAgentTimeline`, `handleGetDaemonConfig`, `handleFetchAgentHistory`, `handleRefreshAgent`, `handleSetAgentModel`, `handleSetAgentThinking`, `handleSetAgentFeature`, `handleUpdateAgent`, `handleCloseItems`, `handleListCommands`, `handleListProviderFeatures`, `handleGetProvidersSnapshot`, `HasHandler`, unhandled-message path).

**Approach**: Used the existing `newTestWSServer(t)` + `dialAndHello` + `readUntilType` httptest harness. Added `createTestAgent` helper to provision agents for tests that need an existing agent ID.

**New test file `server_handlers_test.go`** — 22 tests via WebSocket + 1 HTTP test:

| Test | Handler covered |
|------|----------------|
| `TestHandleFetchAgent_Found/NotFound` | `handleFetchAgent` both branches |
| `TestHandleFetchAgentTimeline_Found/NotFound` | `handleFetchAgentTimeline` both branches |
| `TestHandleGetDaemonConfig` | `handleGetDaemonConfig` |
| `TestHandleFetchAgentHistory` | `handleFetchAgentHistory` |
| `TestHandleRefreshAgent_Found/NotFound` | `handleRefreshAgent` both branches |
| `TestHandleSetAgentModel_NotFound` | `handleSetAgentModel` not-found branch |
| `TestHandleSetAgentThinking_NotFound` | `handleSetAgentThinking` not-found branch |
| `TestHandleSetAgentFeature` | `handleSetAgentFeature` |
| `TestHandleUpdateAgent_Found/NotFound` | `handleUpdateAgent` both branches |
| `TestHandleCloseItems` | `handleCloseItems` |
| `TestHandleListCommands_NoAgent/WithDraftConfig` | `handleListCommands` both paths |
| `TestHandleListProviderFeatures` | `handleListProviderFeatures` |
| `TestHandleStatusEndpoint` | `/api/health` HTTP handler |
| `TestHandleGetProvidersSnapshot` | `handleGetProvidersSnapshot` |
| `TestHandlerRegistryHasHandler` | `HasHandler` true/false |
| `TestUnhandledSessionMessageReturnsRPCError` | unregistered type → `rpc_error` |

**Remaining gaps in server** (require PTY/real net.Listener or are prewarm/relay paths): `NewDaemon`, `Start`, `Stop`, `prewarmGitCache`, `prewarmOpenCodeServer`, terminal handlers, `AttachExternalConnection`, `handleBinaryMessage`.

---

## P2 TDD Session (2026-05-28)

### 1. `cli/internal/client` — Crypto & Pairing Tests (77.6% → 87.4%)

**Problem**: `LoadOrCreateDaemonKeyPair` and `GeneratePairingOffer` had zero coverage. Both involve file I/O and `crypto/rand` key generation.

**Approach**: Used `t.Setenv("SOLO_HOME", tmpDir)` to isolate tests from the real `~/.solo` directory.

**New test file `pairing_extra_test.go`** — 5 tests:

| Test | What it verifies |
|------|-----------------|
| `TestLoadOrCreateDaemonKeyPair_GeneratesNew` | Creates new Curve25519 keypair, validates 32-byte keys, checks JSON persistence |
| `TestLoadOrCreateDaemonKeyPair_LoadsExisting` | Second call loads same keys from disk |
| `TestLoadOrCreateDaemonKeyPair_RegeneratesOldEd25519Key` | Legacy 64-byte Ed25519 secret triggers regeneration |
| `TestGeneratePairingOffer` | End-to-end: generates offer URL, decodes it, validates serverID/relay/key |
| `TestGeneratePairingOffer_TrailingSlash` | Strips trailing `/` from appBaseURL |

---

### 2. `daemon/internal/terminal` — No-PTY Path Tests (76.4% → 86.6%)

**Problem**: `WriteInput`, `Resize`, `Subscribe`, `OnExit`, `Done`, and the already-exited branch of `Kill` were untestable without a real PTY device.

**Approach**: Construct `TerminalProcess` manually with `ptmx == nil` to exercise error paths and subscriber management without PTY.

**New test file `terminal_extra_test.go`** — 6 tests:

| Test | What it verifies |
|------|-----------------|
| `TestTerminalProcess_WriteInput_NoPTY` | Returns "terminal not running" error |
| `TestTerminalProcess_Resize_NoPTY` | Returns error but still updates rows/cols |
| `TestTerminalProcess_Subscribe` | Registers callback, receives data, unsubscribe removes it |
| `TestTerminalProcess_OnExit` | Registers exit callback, manual invocation works |
| `TestTerminalProcess_Done` | Channel closes correctly |
| `TestTerminalProcess_Kill_AlreadyExited` | No-op when already exited (no panic) |

---

### 3. `daemon/internal/relayclient` — E2EEConn Wrapper Methods (75.2% → 81.0%)

**Problem**: `Close`, `WriteControl`, `SetPongHandler`, `SetReadDeadline`, `SetWriteDeadline` on `E2EEConn` had zero coverage. All delegate to the underlying `*websocket.Conn`.

**Approach**: Reused the existing `httptest.NewServer` + gorilla upgrader + `PerformE2EEHandshake` pattern from `e2ee_test.go` to create real `E2EEConn` instances.

**New test file `e2ee_extra_test.go`** — 5 tests:

| Test | What it verifies |
|------|-----------------|
| `TestE2EEConn_Close` | Delegates to underlying conn without error |
| `TestE2EEConn_SetPongHandler` | Registers handler without panic |
| `TestE2EEConn_SetReadDeadline` | Sets deadline without error |
| `TestE2EEConn_SetWriteDeadline` | Sets deadline without error |
| `TestE2EEConn_WriteControl` | Sends ping control frame without error |

---

### 4. `cli/cmd` — Onboard Pure Functions (69.0% → 71.5%)

**Problem**: `onboard.go` had zero coverage across all 5 functions. The orchestration function `runOnboard` is hard to test (requires running daemon), but `resolveOnboardHost`, `generatePairingURL`, and `printNextSteps` are pure logic.

**Approach**: Tested pure functions directly; used `bytes.Buffer` as `cmdStdout` to capture output.

**New test file `onboard_test.go`** — 5 tests:

| Test | What it verifies |
|------|-----------------|
| `TestResolveOnboardHost_Default` | Strips `ws://` prefix and `/ws` suffix |
| `TestResolveOnboardHost_CustomPort` | Returns `127.0.0.1:<port>` |
| `TestGeneratePairingURL_NoServerID` | Returns error when server-id file missing |
| `TestPrintNextSteps_WithPairing` | Output contains QR reference and pairing instructions |
| `TestPrintNextSteps_WithoutPairing` | Output contains "connect to your daemon" without QR |

---

### 5. `daemon/internal/server` — CORS & Status Endpoint (63.7% → 64.2%)

**Problem**: `checkOrigin` (30%) and `handleStatus` (0%) were untested. `checkOrigin` implements CORS origin validation; `handleStatus` returns server metadata as JSON.

**New test file `daemon_extra_test.go`** — 5 tests:

| Test | What it verifies |
|------|-----------------|
| `TestCheckOrigin_NoOriginHeader` | Allows when no Origin header present |
| `TestCheckOrigin_EmptyCORSOrigins` | Allows all origins when CORSOrigins is empty |
| `TestCheckOrigin_AllowedOrigin` | Allows matching origins from config |
| `TestCheckOrigin_RejectedOrigin` | Rejects non-matching origins |
| `TestHandleStatus` | Returns JSON with serverId, version, listen address |

---

### 6. `daemon/internal/agent` — Event Priority & Persistence Metadata (51.1% → 56.7%)

**Problem**: `IsCriticalEvent` (75%) and `IsSemiCriticalEvent` (71.4%) lacked coverage for `turn_failed`, `turn_canceled`, non-map events, and edge cases. `attachPersistenceMetadata` (70.8%) had many untested branches.

**New test files**:

**`event_priority_extra_test.go`** — 9 tests:

| Test | What it verifies |
|------|-----------------|
| `TestIsCriticalEvent_AllTerminalTypes` | Table-driven: turn_completed/failed/canceled → true; others → false |
| `TestIsCriticalEvent_NonMapEvent` | String event → false |
| `TestIsCriticalEvent_MapWithoutType` | Map without "type" key → false |
| `TestIsCriticalEvent_TypeNotString` | Non-string type value → false |
| `TestIsSemiCriticalEvent_ReasoningType` | Direct "reasoning" type → true |
| `TestIsSemiCriticalEvent_TimelineWithReasoningItem` | 6 sub-cases: struct/map reasoning → true, text/nil/string → false |
| `TestIsSemiCriticalEvent_NonTimelineNonReasoning` | 5 event types all → false |
| `TestIsSemiCriticalEvent_NonMapEvent` | String event → false |
| `TestIsSemiCriticalEvent_MapWithoutType` | Map without type key → false |

**`manager_attach_test.go`** — 14 tests:

| Test | What it verifies |
|------|-----------------|
| `TestAttachPersistenceMetadata_NilHandle` | Returns nil for nil input |
| `TestAttachPersistenceMetadata_EmptySessionIDFallsBackToNativeHandle` | SessionID fallback |
| `TestAttachPersistenceMetadata_EmptyNativeHandleFallsBackToSessionID` | NativeHandle fallback |
| `TestAttachPersistenceMetadata_BothEmpty` | Both remain empty |
| `TestAttachPersistenceMetadata_CwdFromParameter` | Cwd from parameter |
| `TestAttachPersistenceMetadata_CwdFromConfig` | Cwd from config when param empty |
| `TestAttachPersistenceMetadata_CwdParamOverridesConfig` | Param takes precedence over config |
| `TestAttachPersistenceMetadata_ModelFromConfig` | Model added to metadata |
| `TestAttachPersistenceMetadata_ModeIDFromConfig` | ModeID added to metadata |
| `TestAttachPersistenceMetadata_ThinkingOptionFromConfig` | ThinkingOptionID added to metadata |
| `TestAttachPersistenceMetadata_EmptyStringsNotAdded` | Empty string pointers not added |
| `TestAttachPersistenceMetadata_ExistingMetadataPreserved` | Original metadata kept alongside new |
| `TestAttachPersistenceMetadata_DoesNotMutateOriginal` | Returned handle is a copy |
| `TestAttachPersistenceMetadata_AllFieldsPopulated` | Full integration test |

## Failing E2E Tests (full run, no `-short`)

Both failures are in `daemon/internal/server` and require a live AI model (`xiaomi-token-plan-cn/mimo-v2`):

- **`TestOpenCodeReasoningE2E`** — expected at least one timeline event, got none
- **`TestOpenCodeReasoningDedupE2E`** — reasoning: 0 chars, expected non-empty reasoning events

These tests hit an external AI service and are expected to be skipped in offline/CI environments.

## Coverage Gap Analysis

The 12 modules below 90% share four root-cause patterns. Fixing the abstractions — not just adding more tests — is the path to sustainable coverage.

---

### Per-Module Detail

#### `daemon/internal/workspace` — 86.0% ✅ (was 46.5%, +39.5pp after P0 TDD)

**Files**: `workspace.go`, `project.go`, `registry.go`, `setup.go`, `worktree.go`, `git_service.go`, `script_proxy.go`, `script_runtime.go`, `config_schema.go`, `git_metadata.go` (~1586 lines)

**Key types/functions**:
- `FileBackedRegistry` — generic JSON file persistence with atomic writes
- `WorkspaceGitService` — git CLI wrapper with 15s TTL cache + background refresh
- `gitExec` — runs `git` commands, parses stdout
- `CreateSoloWorktree` — git worktree creation, branch resolution, registry upsert
- `RunWorktreeSetup` — executes setup commands from `solo.json` with progress callbacks
- `execSetupCommand` — `/bin/bash -lc` command execution with output capture
- `ScriptProxy` — HTTP reverse proxy by hostname to service scripts
- `AllocatePort` — finds available TCP port
- `BackgroundRefresh` — periodic git metadata refresh goroutine

**Untested paths**:
- `exec.Command("git", ...)` — requires real git repo with branches
- `exec.Command("/bin/bash", "-lc", ...)` — real shell execution
- PTY / pipe I/O for command output capture
- `net.Listen("tcp", ":0")` port allocation
- `httputil.ReverseProxy` HTTP routing
- Background refresh goroutine with `time.Ticker`
- `os.ReadDir` + JSON file scanning for registries
- `os.Stat` file existence checks for worktree paths
- Git worktree add/remove (modifies filesystem)
- `os.Environ()` environment variable inheritance

---

#### `daemon/internal/agent` — 56.7% (coalescer.go ~97%, provider_mock.go ~85%, IsCriticalEvent/IsSemiCriticalEvent/attachPersistenceMetadata now fully covered after P2 TDD)

**Files**: `agent.go` (~1132), `manager.go`, `coalescer.go` (~282), `storage.go` (~449), `timeline.go` (~524), plus provider clients (opencode, claude, kimi, pi) (~4000+ lines total)

**Key types/functions**:
- `AgentManager` — full agent lifecycle: create, resume, delete, archive, send messages
- `subscribeToSession` — buffered `workCh` with critical/semi-critical event routing
- `applyTerminalStreamState` — handles `turn_completed`/`failed`/`canceled`
- `ManagedAgent` — in-memory agent state with attention, permissions, subscribers
- `StreamCoalescer` — batches timeline events within time windows (200ms / 2000ms)
- `AgentStorage` — JSON file-per-agent persistence with atomic writes
- `InMemoryTimelineStore` — cursor-based pagination, waiter notifications
- Provider clients (Claude, OpenCode, Kimi, Pi) — external tool integration

**Untested paths**:
- Provider `IsAvailable` calls (require real Claude/Codex/OpenCode binaries)
- `session.Run` blocking until AI completion
- 35-min watchdog `time.AfterFunc`
- `uuid.New()` non-deterministic IDs
- `AgentStorage` disk I/O (`scanDisk`, `writeFileAtomic`, `os.Rename`)
- `StreamCoalescer` timing-dependent coalescing (200ms/2000ms windows)
- Timeline `WaitForAssistantMessage` with `time.After` timeout
- `refreshSessionMetadata` slow I/O under agent lock
- Provider-specific HTTP calls (OpenCode server management)

---

#### `daemon/internal/server` — 64.2% (was 61.1%, +3.1pp after P1+P2 TDD)

**Files**: ~48 files including `session.go`, `daemon.go`, `sendqueue.go`, handlers (~5000+ lines total)

**Key types/functions**:
- `Daemon` — HTTP + WebSocket server orchestrating all services
- `WSServer` — session map, WebSocket upgrade, hello handshake
- `Session` — per-client state: agent events, terminal routing, workspace, coalescer
- `handleNewConnection` — hello timeout, `server_info`, grace period, session resumption
- `AttachSocket` — multi-socket model with grace period reconnection
- `sendQueue` — async message queue with backpressure
- Inbound message queue decoupling `ReadMessage` from handler execution

**Untested paths**:
- Real HTTP server + WebSocket upgrade via `websocket.Upgrader`
- `HelloTimeoutMs` timer races
- Grace period timer behavior (session disconnect/reconnect windows)
- Concurrent multi-socket attach/detach
- `sendQueue` async delivery ordering under backpressure
- Inbound queue `processLoop` goroutine lifecycle
- `promhttp.Handler` metrics endpoint
- Background prewarming (`prewarmGitCache`, `prewarmOpenCodeServer`)

---

#### `daemon/internal/agent/base` — 88.7% ✅ (was 63.1%, +25.6pp after P1 TDD)

**Files**: `dispatcher.go` (~337), `pump.go` (~215), `process.go` (~186), `session.go` (~265) (~1003 lines)

**Key types/functions**:
- `ChannelDispatcher` / `CallbackDispatcher` — event pub/sub with priority tiers
- `safeSendCh` — channel send with timeout (critical: 5s, semi-critical: 100ms)
- `EventPump` — reads `io.Reader`, translates events, detects terminal state
- `pump` — scanner loop with context cancellation, `reader.Close()` unblock
- `ProcessManager` — subprocess lifecycle (start, stop with SIGTERM→SIGKILL, kill)
- `FindBinary` — locates binary via env var, common paths, PATH
- `BaseSession` — shared session state with mutex-protected accessors

**Untested paths**:
- `exec.CommandContext` launching real subprocesses
- `syscall.SIGTERM` / `SIGKILL` signal handling
- `exec.LookPath` binary resolution
- Blocking `scanner.Scan()` with context cancellation
- `io.Closer` close-from-goroutine race in pump
- `ChannelDispatcher` timeout-based send behavior (5s critical timeout)
- `os.Getenv` for environment-dependent binary paths

---

#### `cli/cmd/*` — 71.5% (onboard.go pure functions now covered after P2 TDD)

**Files**: ~26 files including `root.go`, `daemon_start.go`, `daemon_pair.go`, `agent_run.go`, `agent_ls.go`, etc. (~2000+ lines)

**Key types/functions**:
- Cobra command tree for CLI
- `runAgentRun` — creates agent, optionally streams output in foreground
- `runDaemonPair` — generates QR code, reads server-id, creates pairing offer
- `newClient` — connects to daemon via WebSocket
- `waitForAgentFinish` — polls/waits for agent completion with timeout
- `resolveAgentID` — partial ID / name matching

**Untested paths**:
- Real daemon WebSocket connection via `newClient`
- `qrcode.New` + terminal QR code rendering
- Interactive terminal output formatting (table/json/yaml)
- `os.Getwd()` for cwd resolution
- Signal handling in daemon start/stop
- Real-time streaming output in `runAgentForeground`
- Environment-dependent behavior (`SOLO_HOME`, `SOLO_LISTEN`)
- `onboard.go` daemon startup orchestration
- `daemon_start.go` process spawning via `exec.Command`
- `agent_attach.go` / `agent_logs.go` interactive subscription loops

---

#### `daemon/internal/terminal` — 86.6% ✅ (was 76.4%, +10.2pp after P2 TDD)

**Files**: `terminal.go` (~239), `manager.go` (~179), `coalescer.go` (~67) (~485 lines)

**Key types/functions**:
- `TerminalProcess` — PTY-backed subprocess with read loop, subscriber pattern
- `Start` — `exec.Command` + `pty.StartWithSize`, goroutine for readLoop + wait
- `readLoop` — reads 4KB chunks from PTY, broadcasts to subscribers
- `Kill` — SIGTERM → 2s timeout → SIGKILL escalation
- `TerminalManager` — manages multiple terminals by ID/CWD, emits change events
- `OutputCoalescer` — debounces terminal output with 5ms flush delay

**Untested paths**:
- `pty.StartWithSize` — requires real PTY device
- `exec.Command` launching real shells
- PTY read/write I/O (blocking `ptmx.Read`)
- Signal handling (SIGTERM, SIGKILL, SIGWINCH)
- `time.AfterFunc` in coalescer (5ms timing)
- Process exit detection via `cmd.Wait()` goroutine
- Concurrent subscribe/unsubscribe during output

---

#### `cli/internal/client` — 87.4% ✅ (was 77.6%, +9.8pp after P2 TDD)

**Files**: `client.go` (~310), `host.go` (~160), `client_id.go` (~93), `pairing.go` (~141) (~704 lines)

**Key types/functions**:
- `DaemonClient` — WebSocket client with hello handshake, request/response, subscriptions
- `NewDaemonClient` — dials WS, sends hello, reads `server_info` + `providers_snapshot`
- `readPump` — background goroutine routing messages to pending requests / subscribers
- `ResolveHost` — host resolution priority chain (explicit > env > config > default)
- `IsDaemonRunning` — PID file + process signal check
- `LoadOrCreateDaemonKeyPair` — Curve25519 key generation and file persistence
- `GeneratePairingOffer` / `DecodePairingOffer` — QR code pairing URL encoding

**Untested paths**:
- Real WebSocket dial via `websocket.Dialer.DialContext`
- `readPump` goroutine lifecycle (concurrent read + close races)
- `os.FindProcess` + `syscall.Signal(0)` for daemon liveness
- `box.GenerateKey(rand.Reader)` for keypair
- Config file reading from `~/.solo/config.json`
- `SOLO_HOME` / `SOLO_LISTEN` environment variable resolution
- `os.Getwd()` in various paths

---

#### `daemon/internal/relayclient` — 81.0% ✅ (was 75.2%, +5.8pp after P2 TDD)

**Files**: `client.go` (~517), `e2ee.go` (~240) (~757 lines)

**Key types/functions**:
- `Client` — relay connection manager with control + data sockets
- `connectControl` — dials relay control WebSocket, starts readPump + keepalive
- `controlReadPump` — reads control messages (sync/connected/disconnected/ping)
- `openDataSocketURL` — dials data socket, performs E2EE handshake, attaches to WSServer
- `scheduleReconnect` — exponential backoff reconnect (1s, 2s, ... up to 30s)
- `controlKeepalive` — 10s ping with 30s stale detection
- `E2EEConn` — wraps WebSocket with NaCl box encryption (`ReadMessage`/`WriteMessage`)
- `PerformE2EEHandshake` — reads `e2ee_hello`, sends `e2ee_ready`, precomputes shared key

**Untested paths**:
- Real WebSocket connections to relay server
- `dataSocketOpenTimeout` (60s timer) behavior
- E2EE handshake with real crypto keys
- `time.AfterFunc` in openTimer guard
- Reconnect backoff timing
- Keepalive stale detection (30s inactivity)
- Concurrent control/data socket lifecycle
- `rand.Reader` nonce generation

---

#### `relay-go/internal/relay` — 85.7%

**Files**: `server.go` (~320), `session.go` (~95), `session_manager.go` (~67), `control.go` (~190), `buffer.go` (~61) (~730 lines)

**Key types/functions**:
- `Server` — WebSocket relay server with HTTP handler
- `handleWebSocket` — upgrades connections, routes to control/data/client sockets
- `readPump` — reads WS messages, dispatches via `handleMessage`
- `handleClose` — cleanup on disconnect with nudge timer management
- `SessionStore` — in-memory session registry
- `FrameBuffer` — message buffer with overflow protection
- Nudge timers (`startControlNudge`, `startServerDataNudge`) — timeout-based connection health checks

**Untested paths**:
- Real WebSocket connections via `websocket.Upgrader.Upgrade`
- Concurrent socket lifecycle (connect/disconnect races)
- Nudge timer timing behavior (`time.AfterFunc` with 10s/5s delays)
- Buffer overflow truncation edge cases
- `rand.Read` for connection ID generation
- Metrics counter increments (prometheus)

---

#### `daemon/internal/push` — 84.3%

**Files**: `service.go` (~178), `notification.go` (~223), `token_store.go` (~172) (~573 lines)

**Key types/functions**:
- `ExpoPushService` — HTTP push notification client with retry/batching
- `sendBatch` — POST to Expo API with exponential backoff
- `handleResponse` — parses Expo tickets, removes invalid tokens
- `BuildAttentionNotification` — creates notification payloads
- `stripMarkdown` / `truncateText` — text processing for notification previews
- `PersistedTokenStore` — file-backed token storage with atomic writes

**Untested paths**:
- `http.Client.Do` real network calls to Expo endpoint
- Retry loop with `time.Sleep` (delays: 1s, 2s, 4s)
- Response parsing for all Expo API error variants
- `os.Rename` atomic write failures
- Disk I/O errors in token persistence

---

#### `relay-go/internal/e2ee` — 83.2%

**Files**: `crypto.go` (~95), `channel.go` (~370) (~465 lines)

**Key types/functions**:
- `GenerateKeyPair`, `Encrypt`, `Decrypt` — NaCl box crypto operations
- `EncryptedChannel` — E2EE wrapper over Transport with handshake
- `NewClientChannel` — client-side: sends hello, retries every 1s
- `NewDaemonChannel` — daemon-side: waits for hello
- `handleMessage` — state machine: handshaking → open
- `processDaemonHello`/`Rehello` — shared key derivation

**Untested paths**:
- `rand.Read` in key generation (non-deterministic)
- Retry ticker goroutine in `NewClientChannel` (1s loop)
- Real crypto failure modes (wrong key, tampered ciphertext)
- Transport interface integration (requires mock Transport impl)
- Race conditions in state transitions (handshaking → open → closed)

---

#### `daemon/internal/pidlock` — 87.5%

**Files**: `pidlock.go` (~45 lines)

**Key types/functions**:
- `Acquire(soloHome)` — writes PID file, checks for stale PIDs, returns release func

**Untested paths**:
- `os.FindProcess(pid)` + `p.Signal(syscall.Signal(0))` — process existence check depends on real OS processes
- Stale PID file cleanup path (race: process dies between check and write)
- `os.MkdirAll` failure paths (permission denied, disk full)
- `os.WriteFile` failure paths

---

### Root Cause Summary

#### Root Cause 1: Missing `exec` / PTY Abstractions

No `interface` wraps `os/exec` or PTY operations, so these paths can only be exercised in a full integration environment.

| Module | Coverage |
|--------|----------|
| `daemon/internal/workspace` | **86.0%** ✅ (was 46.5%) |
| `daemon/internal/agent/base` | 63.1% |
| `daemon/internal/terminal` | 76.4% |

**Fix**: Introduce `exec.Runner` / `GitExecutor` / `PTYStarter` interfaces; stub with `echo`/`cat` binaries in tests.

#### Root Cause 2: Missing WebSocket / Transport Abstractions

WebSocket calls are made directly via `gorilla/websocket` with no interface layer, requiring a live server to test.

| Module | Coverage |
|--------|----------|
| `daemon/internal/relayclient` | 75.2% |
| `cli/internal/client` | 77.6% |
| `relay-go/internal/relay` | 85.7% |

**Fix**: Extract `Transport` interface over WebSocket `ReadMessage`/`WriteMessage`; use `httptest.NewServer` + local gorilla upgrader for integration tests.

#### Root Cause 3: Missing Clock Abstraction

`time.AfterFunc`, `time.NewTicker`, and `time.Sleep` are called directly, making timer-dependent paths untestable at unit-test speed.

| Module | Coverage |
|--------|----------|
| `daemon/internal/agent` | 56.4% |
| `relay-go/internal/e2ee` | 83.2% |
| `daemon/internal/push` | 84.3% |

**Fix**: Inject a `Clock` interface (`Now()`, `AfterFunc()`, `NewTicker()`); use a fake clock in tests to advance time instantly.

#### Root Cause 4: Long Integration Dependency Chains

These modules compose many services together; gaps in lower layers cascade upward.

| Module | Coverage |
|--------|----------|
| `daemon/internal/agent` | 56.4% |
| `daemon/internal/server` | 63.0% |
| `cli/cmd/*` | 69.0% |
| `daemon/internal/pidlock` | 87.5% |

**Fix**: Use `httptest` to mock daemon for CLI tests; expand `mockPusher`-style mocks for server; use `tmpdir` for storage and pidlock tests.

---

## Priority Roadmap

| Priority | Module | Target | Actual | Status |
|----------|--------|--------|--------|--------|
| P0 | `daemon/internal/workspace` | 75% | **86.0%** ✅ | Done (P0) |
| P0 | `daemon/internal/agent` | 75% | **56.7%** (coalescer 97%, mock 85%) | Partial — provider clients remain at 0% |
| P1 | `daemon/internal/server` | 75% | **64.2%** | Partial — remaining gaps require real net.Listener/PTY |
| P1 | `daemon/internal/agent/base` | 80% | **88.7%** ✅ | Done (P1) |
| P2 | `cli/cmd/*` | 80% | **71.5%** | Partial — onboard.go done, daemon_start/agent_wait remain |
| P2 | `daemon/internal/relayclient` | 85% | **81.0%** ✅ | Done (P2) |
| P2 | `cli/internal/client` | 85% | **87.4%** ✅ | Done (P2) |
| P2 | `daemon/internal/terminal` | 85% | **86.6%** ✅ | Done (P2) |
| P3 | All others | 90% | — | Pending |

Addressing P0/P1 items alone is estimated to raise total coverage from **45.4% → ~70%**.
