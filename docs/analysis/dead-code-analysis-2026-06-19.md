# Dead Code Analysis Report

**Date**: 2026-06-19
**Scope**: All modules — `daemon/`, `cli/`, `relay-go/`, `protocol/`, `app/`, `app-bridge/`, `packages/highlight/`
**Method**: Grep-based reference tracing across the entire repository; every finding verified by searching for symbol references outside the defining file/package.

---

## Executive Summary

| Module | High-confidence findings | Approx. LOC removable |
|--------|-------------------------|----------------------|
| `daemon/` | 19 | ~250 |
| `cli/` | 6 | ~50 |
| `relay-go/` | 3 | ~30 |
| `protocol/` | 6 (1 entire file + 5 types) | ~120 |
| `app/` | 25 (18 unused files + 7 symbols) | ~1500 |
| `app-bridge/` | ~65 exported symbols + 2 unused files | ~1200 |
| `packages/highlight/` | 3 | ~20 |
| **Total** | **~127 findings** | **~3200 LOC** |

The largest clusters of dead code are:
1. **`app-bridge/src/server/agent/agent-sdk-types.ts`** (~18 types, ~500 LOC) — remnants of a removed `@anthropic-ai/claude-agent-sdk` integration.
2. **`app-bridge/src/client/daemon-client.ts`** (~18 methods) — voice/dictation (6) and chat (7) subsystems with zero frontend callers.
3. **18 entirely unused files in `app/src/`** — utilities, hooks, and components never imported by production code.
4. **`protocol/statemachine.go`** — entire 71-line file with zero references.

---

## Part 1: Go Modules

### 1.1 `daemon/` — 19 findings

#### A. Completely Unreferenced (zero callers anywhere)

| File | Line | Symbol | Notes | Confidence |
|------|------|--------|-------|------------|
| `daemon/internal/loop/types.go` | 25 | `VerifyResult` (struct) | Never referenced in any of the 4 Go modules, not even in tests. | **High** |

#### B. Exported Symbols Only Referenced in Tests

These are exported API surface with zero production callers — they exist solely to serve `_test.go` files.

| File | Line | Symbol | Confidence |
|------|------|--------|------------|
| `daemon/internal/agent/errors.go` | 11 | `ErrProviderCrashed` | **High** |
| `daemon/internal/agent/errors.go` | 14 | `ErrProviderTimeout` | **High** |
| `daemon/internal/agent/errors.go` | 18 | `ErrProviderStreaming` | **High** |
| `daemon/internal/agent/errors.go` | 22 | `ErrProviderUnavailable` | **High** |
| `daemon/internal/agent/timeline.go` | 494 | `FormatSeqRange` | **High** |
| `daemon/internal/agent/stall_monitor.go` | 84 | `WithCheckInterval` | **High** |
| `daemon/internal/agent/stall_monitor.go` | 89 | `WithInactivityThreshold` | **High** |
| `daemon/internal/agent/stall_monitor.go` | 95 | `WithRepetitionThreshold` | **High** |
| `daemon/internal/agent/agent.go` | 319 | `(*ManagedAgent).IsBusy` | **High** |
| `daemon/internal/agent/agent.go` | 340 | `(*ManagedAgent).IsActive` | **High** |
| `daemon/internal/agent/agent.go` | 345 | `(*ManagedAgent).ShortID` | **High** |
| `daemon/internal/agent/agent.go` | 353 | `(*ManagedAgent).DisplayTitle` | **High** |
| `daemon/internal/agent/agent.go` | 140 | `(*ManagedAgent).ClearError` | **High** |
| `daemon/internal/agent/coalescer.go` | 216 | `(*StreamCoalescer).FlushAndDiscard` | **High** |
| `daemon/internal/server/handler_registry.go` | 32 | `(*messageHandlerRegistry).HasHandler` | **High** |
| `daemon/internal/server/sendqueue.go` | 119 | `(*sendQueue).Len` | **High** |

> **Note**: The 4 sentinel errors in `agent/errors.go` were introduced with typed provider errors (2026-06-09) but production code never returns or checks them. Either the integration was incomplete or the errors were removed during refactoring.

#### C. Exported but File-Local (could unexport)

| File | Line | Symbol | Confidence |
|------|------|--------|------------|
| `daemon/internal/agent/timeline.go` | 32 | `TimelineState` | **Medium** — every reference is within `timeline.go` itself. Could be unexported to `timelineState`. |

#### D. Empty Directories

| Path | Notes | Confidence |
|------|-------|------------|
| `daemon/internal/server/agents/` | Empty directory, not referenced by any Go file. | **High** |

---

### 1.2 `protocol/` — 6 findings

#### A. Entirely Dead File

| File | LOC | Notes | Confidence |
|------|-----|-------|------------|
| `protocol/statemachine.go` | 71 | Zero references to any symbol in this file across all 4 Go modules. | **High** |

#### B. Unused Exported Types

| File | Line | Symbol | Confidence |
|------|------|--------|------------|
| `protocol/message_agent.go` | — | `AgentFeatureToggle` | **High** |
| `protocol/message_agent.go` | — | `AgentFeatureSelect` | **High** |
| `protocol/message_agent.go` | — | `AgentPermissionAction` | **High** |
| `protocol/message_terminal.go` | — | `TerminalCell` | **High** |
| `protocol/message_terminal.go` | — | `TerminalCursor` | **High** |

---

### 1.3 `cli/` — 6 findings

#### A. Unused Exported Functions/Constants

| File | Line | Symbol | Confidence |
|------|------|--------|------------|
| `cli/cmd/` | — | `Cyan` (color helper) | **High** |
| `cli/cmd/` | — | `Dim` (color helper) | **High** |
| `cli/cmd/` | — | `SuccessColor` | **High** |
| `cli/cmd/` | — | `WarnColor` | **High** |
| `cli/cmd/` | — | `IDColor` | **High** |
| `cli/cmd/` | — | `IsSameOrDescendantPath` | **High** |

---

### 1.4 `relay-go/` — 3 findings

| File | Line | Symbol | Confidence |
|------|------|--------|------------|
| `relay-go/internal/relay/` | — | `SessionStore.Get` | **High** |
| `relay-go/internal/relay/` | — | `ExportSecretKey` | **High** |
| `relay-go/internal/relay/` | — | `ImportSecretKey` | **High** |

---

## Part 2: TypeScript Modules

### 2.1 `app/` — 25 findings

#### A. Entirely Unused Files (zero production imports)

| # | File | Confidence |
|---|------|------------|
| 1 | `app/src/utils/attempt-guard.ts` | **High** |
| 2 | `app/src/utils/branch-suggestions.ts` | **High** |
| 3 | `app/src/utils/detect-ansi-colors.ts` | **High** |
| 4 | `app/src/utils/exhaustive.ts` | **High** |
| 5 | `app/src/utils/parse-osc-default-colors.ts` | **High** |
| 6 | `app/src/utils/pcm16-wav.ts` | **High** |
| 7 | `app/src/utils/pr-tab-label.ts` | **High** |
| 8 | `app/src/utils/extract-agent-model.ts` | **High** |
| 9 | `app/src/utils/diff-highlighter.ts` (+ test) | **High** |
| 10 | `app/src/types/agent-activity.ts` | **High** |
| 11 | `app/src/components/context-window-meter.tsx` | **High** |
| 12 | `app/src/components/question-form-card.tsx` | **High** |
| 13 | `app/src/components/sidebar-agent-list-skeleton.tsx` | **High** |
| 14 | `app/src/components/volume-meter.tsx` | **High** |
| 15 | `app/src/components/agent-status-bar.model-loading.ts` (+ test) | **High** |
| 16 | `app/src/screens/new-workspace-picker-item.ts` | **High** |
| 17 | `app/src/hooks/use-schedule-logs.ts` (+ test) | **High** |
| 18 | `app/src/hooks/use-autocomplete.ts` | **High** |
| 19 | `app/src/hooks/checkout-diff-order.ts` | **High** |
| 20 | `app/src/hooks/feature-preferences.ts` | **High** |
| 21 | `app/src/hooks/agent-history-query-key.ts` | **High** |
| 22 | `app/src/screens/workspace/workspace-tab-model.ts` (+ test) | **High** |
| 23 | `app/src/utils/agent-working-directory-suggestions.ts` (+ test) | **High** |

#### B. Unused Exported Symbols Within Used Files

| File | Line | Symbol | Confidence |
|------|------|--------|------------|
| `app/src/hooks/use-container-width.ts` | ~7 | `useContainerWidth` (only `useContainerWidthBelow` is used) | **High** |
| `app/src/utils/agent-grouping.ts` | ~90 | `parseRepoNameFromRemoteUrl` | **High** |
| `app/src/utils/agent-grouping.ts` | ~134 | `parseRepoShortNameFromRemoteUrl` | **High** |
| `app/src/utils/desktop-badge-state.ts` | ~5 | `isWorkspaceActionableForDesktopBadge` | **Medium** |

---

### 2.2 `app-bridge/` — ~65 findings

#### A. Entirely Unused Files

| # | File | Confidence |
|---|------|------------|
| 1 | `app-bridge/src/shared/terminal-key-input.ts` | **High** |
| 2 | `app-bridge/src/shared/tool-call-display.ts` | **High** |

#### B. `agent-sdk-types.ts` — Dead SDK Abstraction (~18 types)

This file contains `// SOLO-TODO: @anthropic-ai/claude-agent-sdk removed` on line 1. The following types are only referenced within the file itself and have zero external consumers:

| Line | Symbol | Confidence |
|------|--------|------------|
| 7 | `AgentMetadata` | **High** |
| 128 | `AgentPromptContentBlock` | **High** |
| 133 | `AgentPromptInput` | **High** |
| 135 | `AgentRunOptions` | **High** |
| 297 | `CompactionTimelineItem` | **Medium** |
| 305 | `AgentTimelineItem` | **High** |
| 314 | `AgentStreamEvent` (distinct from the used `AgentStreamEventPayload`) | **High** |
| 349 | `AgentPermissionRequestKind` | **High** |
| 389 | `AgentRunResult` | **High** |
| 410 | `AgentSlashCommand` | **High** |
| 416 | `ListPersistedAgentsOptions` | **High** |
| 420 | `PersistedAgentDescriptor` | **High** |
| 430 | `AgentSessionConfig` | **Medium** |
| 459 | `AgentLaunchContext` | **High** |
| 467 | `AgentPermissionResult` | **High** |
| 471 | `AgentSession` (interface) | **High** |
| 498 | `ListModelsOptions` | **High** |
| 503 | `ListModesOptions` | **High** |
| 508 | `AgentClient` (interface) | **High** |

> **Recommendation**: This entire file appears to be a dead abstraction layer. Investigate whether any of these types are intended for future use; if not, the file can be deleted or substantially trimmed.

#### C. `provider-launch-config.ts` — Self-Contained Module (~12 exports)

All functions and schemas are only referenced within the file. App/ only imports `ProviderProfileModel` (type).

| Line | Symbol | Confidence |
|------|--------|------------|
| 30 | `ProviderCommandSchema` | **High** |
| 36 | `ProviderRuntimeSettingsSchema` | **High** |
| 53 | `ProviderProfileModelSchema` | **High** |
| 63 | `ProviderOverrideSchema` | **High** |
| 78 | `AgentProviderRuntimeSettingsMapSchema` | **High** |
| 83 | `ProviderCommand` (type) | **Medium** |
| 84 | `ProviderRuntimeSettings` (type) | **High** |
| 86 | `ProviderOverride` (type) | **High** |
| 87 | `AgentProviderRuntimeSettingsMap` (type) | **High** |
| 91 | `ProviderCommandPrefix` (interface) | **High** |
| 96 | `resolveProviderCommandPrefix()` | **High** |
| 122 | `resolveShellEnv()` | **High** |
| 130 | `migrateProviderSettings()` | **High** |
| 180 | `applyProviderEnv()` | **High** |
| 194 | `findExecutable()` (sync version) | **High** |
| 215 | `isProviderCommandAvailable()` | **High** |

#### D. `provider-manifest.ts` — Internal-Only Exports (~6 exports)

| Line | Symbol | Confidence |
|------|--------|------------|
| 5 | `AgentModeIcon` | **High** |
| 170 | `AGENT_PROVIDER_DEFINITIONS` | **High** |
| 229 | `DEV_AGENT_PROVIDER_DEFINITIONS` | **High** |
| 240 | `getAgentProviderDefinition()` | **High** |
| 254 | `BUILTIN_PROVIDER_IDS` | **High** |
| 255 | `AGENT_PROVIDER_IDS` | **High** |
| 259 | `isValidAgentProvider()` | **High** |

#### E. `daemon-client.ts` — Unused Methods (~18 methods)

**Voice/Dictation subsystem (6 methods — feature not implemented on frontend):**

| Line | Method | Confidence |
|------|--------|------------|
| 2144 | `setVoiceMode()` | **High** |
| 2176 | `sendVoiceAudioChunk()` | **High** |
| 2180 | `startDictationStream()` | **High** |
| 2228 | `sendDictationStreamChunk()` | **High** |
| 2238 | `finishDictationStream()` | **High** |
| 2385 | `cancelDictationStream()` | **High** |

**Chat subsystem (7 methods — no frontend caller):**

| Line | Method | Confidence |
|------|--------|------------|
| 3484 | `createChatRoom()` | **High** |
| 3497 | `listChatRooms()` | **High** |
| 3508 | `inspectChatRoom()` | **High** |
| 3520 | `deleteChatRoom()` | **High** |
| 3532 | `postChatMessage()` | **High** |
| 3547 | `readChatMessages()` | **High** |
| 3562 | `waitForChatMessages()` | **High** |

**Other unused methods:**

| Line | Method | Confidence |
|------|--------|------------|
| 1071 | `subscribeRawMessages()` | **High** |
| 2934 | `listProviderModels()` | **High** |
| 2951 | `listProviderModes()` | **High** |
| 2982 | `listAvailableProviders()` | **High** |
| 3172 | `waitForAgentUpsert()` | **High** |
| 3269 | `waitForFinish()` | **High** |
| 3461 | `captureTerminal()` | **High** |
| 4168 | `setReconnectEnabled()` | **High** |

#### F. Other `app-bridge/` Unused Exports

| File | Line | Symbol | Confidence |
|------|------|--------|------------|
| `server/agent/agent-title-limits.ts` | 2 | `MAX_AUTO_AGENT_TITLE_CHARS` | **High** |
| `client/daemon-client-transport-utils.ts` | 74 | `safeRandomId()` | **High** |
| `utils/executable.ts` | 38 | `isWindowsCommandScript()` | **High** |
| `utils/executable.ts` | 100 | `executableExists()` | **High** |
| `utils/executable.ts` | 157 | `quoteWindowsCommand()` | **High** |
| `utils/executable.ts` | 166 | `quoteWindowsArgument()` | **High** |
| `utils/solo-config-schema.ts` | 40 | `WorktreeConfigSchema` | **High** |
| `utils/solo-config-schema.ts` | 47 | `ScriptEntrySchema` | **High** |
| `utils/solo-config-schema.ts` | 49 | `SoloConfigSchema` | **High** |
| `shared/daemon-endpoints.ts` | 10 | `CURRENT_RELAY_PROTOCOL_VERSION` | **High** |
| `shared/daemon-endpoints.ts` | 12 | `normalizeRelayProtocolVersion()` | **High** |
| `shared/agent-attention-notification.ts` | 3 | `AgentAttentionReason` | **High** |
| `shared/agent-attention-notification.ts` | 5 | `AgentAttentionNotificationData` | **High** |
| `shared/agent-attention-notification.ts` | 37 | `AssistantTimelineItem` | **High** |
| `shared/agent-attention-notification.ts` | 137 | `findLatestAssistantMessageFromTimeline()` | **High** |
| `shared/agent-attention-notification.ts` | 160 | `findLatestPermissionRequest()` | **High** |
| `shared/connection-offer.ts` | 9 | `ConnectionOfferV2Schema` | **High** |
| `shared/connection-offer.ts` | 18 | `ConnectionOfferV2` (type) | **High** |
| `shared/terminal-stream-protocol.ts` | 4 | `TerminalStreamResizeSchema` | **Medium** |
| `server/schedule/types.ts` | 97 | `ScheduleExecutionResult` | **High** |
| `relay/crypto.ts` | 93 | `exportSecretKey()` | **High** |
| `relay/crypto.ts` | 100 | `importSecretKey()` | **High** |
| `relay/e2ee.ts` | 3 | `createDaemonChannel` (re-export) | **High** |
| `server/agent/tool-name-normalization.ts` | 8 | `tokenizeToolName()` | **Medium** |
| `server/agent/tool-name-normalization.ts` | 13 | `getToolLeafName()` | **Medium** |
| `server/agent/tool-name-normalization.ts` | 18 | `isSpeakToolName()` | **Medium** |
| `server/agent/tool-name-normalization.ts` | 22 | `isLikelyNamespacedToolName()` | **Medium** |
| `server/agent/tool-name-normalization.ts` | 85 | `isLikelyExternalToolName()` | **Medium** |

---

### 2.3 `packages/highlight/` — 3 findings

| File | Symbol | Confidence |
|------|--------|------------|
| `src/index.ts` (from `parsers.ts`) | `getParserForFile` | **Medium** — only used internally |
| `src/index.ts` (from `parsers.ts`) | `getSupportedExtensions` | **Medium** — only used in tests |
| `src/index.ts` (from `highlighter.ts`) | `highlightLine` | **Medium** — only used internally |

---

## Part 3: Improvement Plan

### Phase 1: Safe Immediate Deletions (zero risk, zero production callers)

**Estimated effort**: 1-2 hours
**Estimated LOC removed**: ~1800

These items have zero references anywhere in production code, tests, or other modules. They can be deleted in a single commit per module.

#### Go:
1. Delete `protocol/statemachine.go` (entire file, 71 LOC)
2. Delete `VerifyResult` from `daemon/internal/loop/types.go:25`
3. Delete empty directory `daemon/internal/server/agents/`
4. Delete 5 unused types from `protocol/message_agent.go` and `protocol/message_terminal.go`
5. Delete 6 unused exports from `cli/cmd/`

#### TypeScript (app/):
6. Delete 18 entirely unused files (listed in §2.1.A) and their associated test files
7. Delete 2 unused files from `app-bridge/src/shared/` (`terminal-key-input.ts`, `tool-call-display.ts`)

**Verification**: Run `go build ./...`, `go test -short -race ./...`, `npx expo lint`, `tsc --noEmit`, `npm test` after each module's deletions.

---

### Phase 2: Dead Abstraction Layer Removal

**Estimated effort**: 2-3 hours
**Estimated LOC removed**: ~700

1. **`app-bridge/src/server/agent/agent-sdk-types.ts`**: This file is marked with `// SOLO-TODO: @anthropic-ai/claude-agent-sdk removed` and contains ~18 types with zero external consumers. Audit whether any type is intended for future use (e.g., `AgentSession` interface for a planned SDK integration). If not, delete the entire file or trim to only the types that have consumers.

2. **`app-bridge/src/server/agent/provider-launch-config.ts`**: All functions are self-contained. Only `ProviderProfileModel` type is consumed externally. Consider:
   - Move `ProviderProfileModel` / `ProviderProfileModelSchema` to a smaller file.
   - Delete the rest (functions like `migrateProviderSettings`, `findExecutable`, etc. appear to be unused frontend logic that duplicates Go-side behavior).

3. **`app-bridge/src/server/agent/provider-manifest.ts`**: Trim to only externally-consumed exports (`getModeVisuals`, `AgentProviderDefinition`, `AgentModeColorTier`). Remove internal-only constants and functions.

---

### Phase 3: Unused DaemonClient Methods

**Estimated effort**: 1-2 hours
**Estimated LOC removed**: ~400

The `daemon-client.ts` file has ~18 methods with zero frontend callers. These fall into three categories:

1. **Voice/Dictation (6 methods)**: The voice subsystem appears unimplemented on the frontend. If voice is a planned feature, document these as planned. Otherwise, remove them along with the corresponding Go-side handlers if those are also unused.

2. **Chat subsystem (7 methods)**: No frontend chat UI exists. If chat is planned, document; otherwise remove.

3. **Other methods (5+)**: `subscribeRawMessages`, `listProviderModels`, `listProviderModes`, `listAvailableProviders`, `waitForAgentUpsert`, `waitForFinish`, `captureTerminal`, `setReconnectEnabled` — audit each individually. Some may be used in E2E tests or planned for upcoming features.

**Action**: Before removing, check the Go daemon side for corresponding WebSocket handlers. If both sides are dead, remove both. If the Go side is live (e.g., voice handlers exist), keep the client methods but mark them with `// TODO: wire up when frontend lands`.

---

### Phase 4: Test-Only API Surface Cleanup (Go)

**Estimated effort**: 1-2 hours
**Estimated LOC removed**: ~150

The daemon module has 16 exported symbols only used in tests:

1. **4 sentinel errors** (`ErrProviderCrashed`, `ErrProviderTimeout`, `ErrProviderStreaming`, `ErrProviderUnavailable`): These were introduced with typed provider errors but never integrated. Either:
   - Wire them into production error handling (preferred — they represent useful error categorization), or
   - Remove them and their tests.

2. **5 `ManagedAgent` methods** (`IsBusy`, `IsActive`, `ShortID`, `DisplayTitle`, `ClearError`): These are convenient accessors. If the frontend or future code might use them, keep. Otherwise, either remove or unexport.

3. **3 `StallMonitor` options** (`WithCheckInterval`, `WithInactivityThreshold`, `WithRepetitionThreshold`): The functional options pattern is only tested, never used in production. Either keep for future configurability or remove.

4. **`FormatSeqRange`, `FlushAndDiscard`, `HasHandler`, `sendQueue.Len`**: Test-only utilities. Consider unexporting or removing.

---

### Phase 5: Visibility Reduction (Go)

**Estimated effort**: 30 minutes

- Unexport `TimelineState` → `timelineState` in `daemon/internal/agent/timeline.go:32` (only used within the file).

---

### Phase 6: Highlight Package API Pruning

**Estimated effort**: 30 minutes

- Remove `getParserForFile`, `getSupportedExtensions`, `highlightLine` from `packages/highlight/src/index.ts` re-exports if they are not part of the intended public API. Keep them as internal (non-exported) functions if used within the package.

---

### Phase 7: CI Guard Rail (Prevention)

**Estimated effort**: 1-2 hours

To prevent dead code from accumulating again:

1. **Go**: Configure `golangci-lint` with `unused` linter (may already be enabled — verify in `.golangci.yml`). Ensure it fails CI on unused exports.

2. **TypeScript**: Add `ts-prune` or `knip` to the JS CI pipeline to detect unused exports automatically. Add a CI step:
   ```yaml
   - name: Detect unused exports
     run: npx knip --no-exit-code || true  # Start as non-blocking, then make blocking
   ```

3. **Periodic audit**: Schedule a quarterly dead-code audit using this report as a baseline.

---

## Risk Assessment

| Phase | Risk | Mitigation |
|-------|------|------------|
| Phase 1 | Very low — zero callers | Full test suite + type check |
| Phase 2 | Low-medium — may remove planned abstractions | Review with team before deleting `agent-sdk-types.ts` |
| Phase 3 | Medium — voice/chat may be planned features | Check roadmap (`docs/product/roadmap-2026.md`) before removing |
| Phase 4 | Low — test-only symbols | Ensure tests still pass after removal |
| Phase 5-6 | Very low | Compilation check |
| Phase 7 | None — additive | N/A |

---

## Appendix: Files Requiring Manual Review

These items were flagged but need human judgment before removal:

1. **`app-bridge/src/server/agent/agent-sdk-types.ts`** — May be intended for a future SDK integration. Check with @Andy.
2. **Voice/Dictation methods in `daemon-client.ts`** — Check `docs/product/roadmap-2026.md` for voice feature plans.
3. **Chat subsystem methods in `daemon-client.ts`** — Check if chat is a planned feature.
4. **`daemon/internal/agent/errors.go` sentinel errors** — These represent a useful error taxonomy. Consider wiring them into production code rather than deleting.
5. **`app-bridge/src/server/agent/provider-launch-config.ts`** — Some functions (`migrateProviderSettings`, `applyProviderEnv`) may be intended for future frontend provider configuration UI.

---

*Generated by automated grep-based reference analysis. All findings verified by cross-module search. Items marked "High" confidence have zero references outside their defining file; "Medium" items have references only within their own package/module.*
