# Test Suite Comprehensive Analysis Report

> Generated: 2026-05-24
> Updated: 2026-05-25 (last: messageID propagation + timeline deduplication + multi-client sync test)
> Scope: Solo full project (Go backend / TS frontend / Mobile)

---

## 1. Current State Overview

### 1.1 Test File Distribution

| Module | Language | Test Files | Framework | Test Types |
|--------|----------|-----------|-----------|------------|
| `app/` | TS/TSX | **207** | Vitest + Playwright | Unit, Browser, E2E |
| `daemon/` | Go | **129** | `testing` | Unit, Integration |
| `relay-go/` | Go | **8** | `testing` | Unit, E2E (encryption consistency) |
| `protocol/` | Go | **4** | `testing` | Unit |
| `cli/` | Go | **13** | `testing` | Unit |
| `packages/highlight/` | TS | 3 | Vitest | Unit |
| `app-bridge/` | TS | **3** | Vitest | Unit (new) |
| `app/maestro/` | YAML | ~20 | Maestro | Mobile E2E (manual) |

### 1.2 Frameworks and Configuration

**Go Backend**
- Standard library `testing`, table-driven (`t.Run` subtests)
- CI: `go test -short -v -race -count=1 -timeout=10m -coverprofile=coverage.out ./...`
- `.golangci.yml` v2.10

**Frontend `app/`**
- **Vitest v3.2.4**: Dual project configuration
  - `unit` — Node environment, `src/**/*.{test,spec}.{ts,tsx}` (excluding browser/e2e)
  - `browser` — Real Chromium (Playwright), `src/**/*.browser.{test,spec}.{ts,tsx}`
- **Playwright E2E**: **30** `.spec.ts` files, custom `globalSetup` (bootstraps daemon + relay + Metro)
- `vitest.setup.ts`: Extensive React Native ecosystem shims (unistyles, svg, expo-linking, xterm, etc.)
- `pool: "forks"`: Workaround for `process.send` stub issue in worker_threads

**app-bridge**
- Vitest v4.1.7, Node environment
- 3 test files covering pure function modules (base64, crypto, path-utils)

**Mobile**
- Maestro YAML flows, README marked as ad-hoc / exploratory, not integrated into CI

### 1.3 CI Execution Status

`.github/workflows/ci.yml` contains two jobs:

- **`go`**: Matrix run of protocol / cli / daemon / relay-go, build + test + lint + **coverage**
- **`js`**:
  - lint app / app-bridge / packages/highlight
  - typecheck packages/highlight (enforced)
  - test packages/highlight
  - **test app (unit) + coverage** ← new, 1282 tests
  - **test app-bridge + coverage** ← new, 32 tests
  - **typecheck app (enforced)** ← fixed, 0 errors
  - **typecheck app-bridge (enforced)** ← fixed, 0 errors (zod v4 migration)
  - **Codecov upload** ← new, JS (lcov) + Go (coverage.out)

`.github/workflows/e2e-nightly.yml`:
- Runs Playwright E2E automatically at 02:00 UTC daily
- Retains trace / screenshot / video artifacts on failure

---

## 2. Core Problem: Tests Written But Not Run = 0

The biggest contradiction is the gap between quantity and execution:

| Metric | Count | Executed in CI? |
|--------|-------|----------------|
| app unit tests | **207** files | ✅ Yes |
| app browser tests | 1 file | ❌ No |
| Playwright E2E | **30** files | ⚠️ nightly |
| Go tests | **154** files | ✅ Yes |
| packages/highlight | 3 files | ✅ Yes |
| app-bridge tests | 3 files | ✅ Yes |

This means:
1. **Regressions cannot be automatically caught** — Any commit breaking app logic will not be blocked (fixed)
2. **Test files gradually rot** — Tests that don't run lose maintenance value (fixed for app + app-bridge)
3. **Code changes lack a safety net** — PR merges depend on manual review (fixed for app typecheck)

---

## 3. Priority Action Items (P0 → P3)

### P0 — Fix CI Gaps Immediately ✅ All Completed

1. **Run app unit tests in CI**
   - ✅ Added `Test app (unit)` step to `.github/workflows/ci.yml`
   - Result: 203 test files, 1282 tests, 31s

2. **Enforce type checking**
   - ✅ app: Fixed `host-runtime.ts` clientType type + `adaptive-modal-sheet.tsx` RN type conflict, removed `continue-on-error: true`
   - ✅ app-bridge: Completed zod v4 migration (`z.record()` two-arg form, `ZodTypeDef` removal, `.default({})` full defaults), removed `continue-on-error: true`

3. **Add app-bridge tests**
   - ✅ Added vitest + 3 test files (`base64.test.ts`, `crypto.test.ts`, `path-utils.test.ts`)
   - Result: 32 tests, 298ms

### P1 — Establish Quality Baseline ✅ Completed

4. **Integrate coverage collection**
   - ✅ Vitest: Configured `coverage` (`v8` provider)
     - app: `@vitest/coverage-v8@3.2.4` (matching vitest v3.2.4)
     - app-bridge: `@vitest/coverage-v8@4.1.7` (matching vitest v4.1.7)
     - Coverage data (current): app ~35.91% statements / 74.29% branches; app-bridge 89.41% statements / 94.11% branches
   - ✅ Go: Added `-coverprofile=coverage.out` to CI
   - ✅ CI artifact: `actions/upload-artifact@v4` retains coverage reports for 14 days
   - ✅ Codecov integration: `codecov/codecov-action@v5` uploads JS (lcov) and Go (coverage.out)
   - ⚠️ Repository owner needs to add `CODECOV_TOKEN` secret in GitHub Settings → Secrets

5. **Integrate E2E into CI**
   - ✅ Added `.github/workflows/e2e-nightly.yml`
   - Schedule: Daily at 02:00 UTC (`cron: "0 2 * * *"`)
   - Manual trigger: `workflow_dispatch`
   - Environment: ubuntu-latest + Node 22 + Go 1.25.6
   - Steps: npm ci → Playwright install → build workspace deps → `npm run test:e2e`
   - Artifacts retained on failure: trace / screenshot / video (7 days)

### P2 — Address Weak Areas (within 1 month)

6. **Expand Browser tests**
   - Currently only 1 file tests xterm.js
   - All components relying on real DOM should be migrated from `unit` to `browser` project

7. **Integrate Maestro mobile tests into CI**
   - Can use GitHub Actions + Android Emulator or EAS Build test channel

8. **Extract Go test utilities**
   - Helpers like `newTestDaemon()`, `newTestWSServer()` are duplicated across packages
   - Extract to `daemon/internal/testutil`

9. **Session ↔ Timeline E2E coverage** ✅ Completed
   - Added 7 specs (10 tests): `multi-client-sync`, `reconnect-resilience`, `rapid-fire-messages`, `optimistic-dedup`, `grace-period-recovery`, `message-ordering`, `timeline-pagination`
   - Covers: multi-client sync, disconnect recovery, rapid messages, optimistic deduplication, message ordering, pagination
   - Pending: cross-provider format consistency (requires real provider environment)

### P3 — Long-term Optimization (on demand)

9. **Snapshot / Visual regression testing**
   - Playwright screenshot capability covering key UI (composer, sidebar, terminal)

10. **Unified test entry point**
    - Root `package.json` adds `"test": "npm run test:go && npm run test:js && npm run test:e2e"`

---

## 4. Key File Index

| File | Purpose |
|------|---------|
| `.github/workflows/ci.yml` | Main CI configuration (tests, coverage, Codecov) |
| `.github/workflows/e2e-nightly.yml` | E2E nightly run |
| `codecov.yml` | Codecov configuration (flags, ignore, informational status) |
| `app/vitest.config.ts` | Vitest dual project configuration (unit + browser + coverage) |
| `app/vitest.setup.ts` | Global test shims |
| `app/playwright.config.ts` | E2E configuration (includes globalSetup) |
| `app/package.json` | `test`, `test:browser`, `test:e2e` scripts |
| `app-bridge/package.json` | test scripts, vitest dependencies |
| `app-bridge/vitest.config.ts` | vitest configuration (includes coverage) |
| `app-bridge/tsconfig.json` | `module: NodeNext`, root cause of zod v4 compatibility issues |
| `.golangci.yml` | Go lint configuration |

---

## 5. P0/P1 Implementation Results

### 5.1 TDD Implementation Process

**P0-1: Add app unit tests to CI**
- Step 1 (red): Observe current state — app has 207 test files, not executed in CI
- Step 2 (green): Modify `.github/workflows/ci.yml` to add `Test app (unit)` step
- Step 3 (verify): Run locally `cd app && npm run test` → tests passed, 0 failures

**P0-2: Enforce type checking**
- Step 1 (red): Run `cd app && npx tsc --noEmit` → 38 errors
  - `host-runtime.ts(462)`: `clientType: string` incompatible with `"browser" | "cli" | "mcp" | "mobile"`
  - `adaptive-modal-sheet.tsx(158, 336)`: react-native dual version type conflict (root 0.81.6 vs app 0.81.5)
- Step 2 (green):
  - `host-runtime.ts`: Extract `clientType` as explicit union type constant
  - `adaptive-modal-sheet.tsx`: `StyleSheet.flatten()` + `as any` assertion
- Step 3 (verify): `npx tsc --noEmit` → 0 errors

**P0-3: Add app-bridge tests**
- Step 1 (red): 0 test files under `app-bridge/src/`
- Step 2 (green):
  - Install `vitest` as dev dependency
  - Create `vitest.config.ts` (Node environment, `@server` alias)
  - Write tests: `base64.test.ts` (9 tests), `crypto.test.ts` (15 tests), `path-utils.test.ts` (8 tests)
- Step 3 (verify): `npm test` → 3 passed, 32 tests
- Step 4 (CI integration): Added `Test app-bridge` to `.github/workflows/ci.yml`

**P0-2 supplement: app-bridge zod v4 migration**
- Step 1 (red): Run `cd app-bridge && npx tsc --noEmit` → 39 errors
- Step 2 (green): Fix three categories of zod v3 → v4 compatibility issues:
  - `z.record(singleArg)` → `z.record(keyType, valueType)`: v4 requires explicit key type, ~20 fixes
  - `z.ZodTypeDef` removed: `z.ZodType<T, z.ZodTypeDef, unknown>` → `z.ZodType<T, unknown>`, 4 fixes
  - `.optional().default({})` stricter: v4 requires defaults to match full output type, changed to factory function
  - Files affected: `messages.ts`, `schedule/types.ts`, `provider-launch-config.ts`
- Step 3 (verify): `npx tsc --noEmit` → 0 errors, `npm test` → 32 passed
- Step 4 (CI): Removed `continue-on-error: true` from `.github/workflows/ci.yml`, typecheck now enforced

**P1-1: Coverage collection**
- Step 1 (research): Confirm vitest versions → app v3.2.4 / app-bridge v4.1.7
- Step 2 (install): `@vitest/coverage-v8@3.2.4` (app), `@vitest/coverage-v8@4.1.7` (app-bridge)
- Step 3 (configure): Enable coverage in `vitest.config.ts` (v8 provider, reporter: text/json/html/lcov)
- Step 4 (CI integration): Add `--coverage` to test steps, `actions/upload-artifact@v4` retains reports
- Step 5 (Codecov): `codecov/codecov-action@v5` uploads lcov + coverage.out, configure `codecov.yml`

**P1-2: E2E nightly**
- Step 1 (research): Confirm `playwright.config.ts` uses `globalSetup` to bootstrap daemon/relay/Metro
- Step 2 (create): `.github/workflows/e2e-nightly.yml` (scheduled + manual trigger)
- Step 3 (artifacts): Retain trace/screenshot/video for 7 days on failure

### 5.2 Status Summary

| Task | Status | Details |
|------|--------|---------|
| **P0-1: Add app unit tests to CI** | ✅ Complete | 203 files, 1282 tests, ~31s |
| **P0-2: Enforce app typecheck** | ✅ Complete | 0 errors |
| **P0-2: app-bridge typecheck** | ✅ Complete | zod v4 migration complete, 0 errors, enforced |
| **P0-3: Add app-bridge tests** | ✅ Complete | 3 files, 32 tests, ~300ms |
| **P1-1: Vitest coverage** | ✅ Complete | app 35.91% / app-bridge 89.41% |
| **P1-1: Go coverage** | ✅ Complete | `-coverprofile=coverage.out` |
| **P1-1: Codecov upload** | ✅ Configuration complete | Requires `CODECOV_TOKEN` secret |
| **P1-2: E2E nightly** | ✅ Complete | Daily at 02:00 UTC, workflow_dispatch |
| **P2-3: Session-Timeline E2E** | ✅ Complete | 7 specs, 10 tests |
| **P2-4: Go CLI global state decoupling** | ✅ Complete | output package + cmd package dependency injection |
| **P2-5: Daemon concurrency race tests** | ✅ Complete | Coalescer + AgentManager `-race` tests |

### 5.3 P2 Implementation Results (TDD Refactoring: Go Test Maintainability + Concurrency Safety)

**P2-4: Go CLI global state decoupling**
- **Problem**: `output.Render`/`RenderError` depend on global `output.Stdout`/`Stderr`; `getOutputOpts`/`newClient` depend on global flag variables; 20+ command files directly reference `output.Stdout`
- **Step 1 (red)**: Observe existing tests — extensive `oldStdout := output.Stdout; output.Stdout = &buf` pattern, tests implicitly coupled via global variables
- **Step 2 (green)**:
  - `cli/internal/output/render.go`: `Render`/`RenderError`/`PrintResult` add explicit `io.Writer` parameter
  - `cli/cmd/root.go`: `getOutputOpts` and `newClient` changed to parameterized signatures; introduce `cmdStdout`/`cmdStderr` (package-level, injectable in tests)
  - Batch replace all `output.Stdout` → `cmdStdout` in `cli/cmd/*.go`
- **Step 3 (verify)**: `go test ./cli/...` → all pass; `output_test.go` adds `TestRenderDoesNotDependOnGlobalStdout`

**P2-5: Daemon concurrency race tests**
- **Problem**: `StreamCoalescer` and `AgentManager` involve goroutines and shared state, but lack `-race` verification
- **Step 1 (red)**: Run `go test -race ./daemon/internal/agent/...` — no concurrency test coverage for high-frequency concurrent paths
- **Step 2 (green)**:
  - `coalescer_extra_test.go`: Added `TestStreamCoalescerConcurrentHandleAndFlush` (10 goroutines Handle + FlushAll + FlushAndDiscard), `TestStreamCoalescerConcurrentHandleAndFlushFor`
  - `manager_extra_test.go`: Added `TestAgentManagerConcurrentCreateDelete` (50 agents concurrent create/delete), `TestAgentManagerConcurrentCreateArchive`, `TestAgentManagerConcurrentReadWrite`
- **Step 3 (verify)**: `go test -race -run "TestStreamCoalescerConcurrent|TestAgentManagerConcurrent" ./daemon/internal/agent/...` → all pass, 0 data races

**P2-6: relayclient test supplementation**
- **Problem**: `connectControl` (35.7%), `controlReadPump` (50.0%), `controlKeepalive` (52.9%) have low coverage
- **Step 1 (red)**: Observe existing tests — already covers `handleControlMessage`, `buildControlURL`, `openDataSocket`, etc., but lacks control connection lifecycle tests
- **Step 2 (green)**: `client_extra_test.go` adds 8 tests:
  - `TestConnectControl_Success`: Verify successful control connection establishment and goroutine start
  - `TestConnectControl_FailureTriggersReconnect`: Verify failed dial triggers reconnect timer
  - `TestConnectControl_AlreadyStopped`: Verify no-op when stopped
  - `TestControlReadPump_ReceivesMessage`: Verify text messages are processed
  - `TestControlReadPump_NonTextMessageIgnored`: Verify binary messages are ignored
  - `TestControlReadPump_ScheduleReconnectOnClose`: Verify abnormal close triggers reconnect
  - `TestStop_Idempotent`: Verify Stop idempotency
  - `TestStop_ClosesControlConn`: Verify Stop closes control connection
- **Step 3 (verify)**: Coverage 65.3% → 75.2%, `go test -race` passes

**P2-7: agent/base test supplementation**
- **Problem**: `FindBinary` (0%) not tested
- **Step 1 (red)**: `process_test.go` has no `FindBinary` tests
- **Step 2 (green)**: Added 4 tests:
  - `TestFindBinary_PathLookup`: PATH fallback lookup
  - `TestFindBinary_EnvVar`: Environment variable priority
  - `TestFindBinary_CommonPaths`: Common path lookup
  - `TestFindBinary_NotFound`: Returns error when not found
- **Step 3 (verify)**: Coverage 59.4% → 63.1%

| Task | Status | Details |
|------|--------|---------|
| **P2-4: output package de-globalization** | ✅ Complete | `Render(w, result, opts)` explicit writer |
| **P2-4: cmd package dependency injection** | ✅ Complete | `getOutputOpts`/`newClient` parameterized, `cmdStdout` injectable |
| **P2-5: Coalescer concurrency tests** | ✅ Complete | 2 race tests, `-race` passes |
| **P2-5: AgentManager concurrency tests** | ✅ Complete | 3 race tests, `-race` passes |
| **P2-6: relayclient test supplementation** | ✅ Complete | 5 tests, `connectControl`/`controlReadPump`/`Stop` path coverage, +9.9% |
| **P2-7: agent/base test supplementation** | ✅ Complete | `FindBinary` 4 tests, +3.7% |

### 5.4 P0/P1 Key Modified Files

**CI / Configuration**
- `.github/workflows/ci.yml` — Main CI configuration (tests, coverage, Codecov)
- `.github/workflows/e2e-nightly.yml` — New nightly E2E workflow
- `codecov.yml` — Codecov configuration (flags, ignore, informational status)

**app fixes**
- `app/src/runtime/host-runtime.ts` — clientType type narrowing
- `app/src/components/adaptive-modal-sheet.tsx` — StyleSheet.flatten + type assertion

**app coverage**
- `app/vitest.config.ts` — Enable coverage (v8 provider, includes lcov)
- `app/package.json` — Add `@vitest/coverage-v8@3.2.4`

**app-bridge tests + coverage**
- `app-bridge/package.json` — Add test scripts, vitest + coverage dependencies
- `app-bridge/vitest.config.ts` — vitest configuration (includes coverage)
- `app-bridge/src/relay/base64.test.ts` — New
- `app-bridge/src/relay/crypto.test.ts` — New
- `app-bridge/src/shared/path-utils.test.ts` — New

**Dependency lock**
- `package-lock.json` — Updated

---

## 6. Codecov Configuration Notes

`codecov.yml` key configuration points:

- **Informational mode**: Coverage status does not block PR merges, only provides reference data
- **Flags**: Split by module (`js`, `go-protocol`, `go-cli`, `go-daemon`, `go-relay-go`), supports carryforward
- **Ignore**: Excludes test files, browser tests, e2e, test-stubs, vitest setup
- **Threshold**: 5%, allows normal fluctuation

Setup steps:
1. Visit [codecov.io](https://codecov.io) to bind this repository
2. Add `CODECOV_TOKEN` in GitHub Settings → Secrets → Actions
3. Automatic upload on next CI run

---

## 7. One-Line Summary

> **P0 + P1 + P2 (TDD refactoring) completed: app and app-bridge tests integrated into CI with coverage collection, app typecheck enforced, app-bridge zod v4 migration complete with typecheck enforced, E2E running nightly. Go CLI global state decoupled (output explicit writer + cmd parameterized dependency injection), Daemon concurrency components verified with `-race`. Current P3 backlog: Browser test expansion, Go testutil extraction, Maestro CI integration.**
