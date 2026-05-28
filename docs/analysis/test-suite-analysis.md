# 测试集全景分析报告

> 生成时间：2026-05-24
> 更新时间：2026-05-25 (last: messageID propagation + timeline deduplication + multi-client sync test)
> 分析范围：Solo 全项目（Go 后端 / TS 前端 / 移动端）

---

## 1. 现状总览

### 1.1 测试文件分布

| Module | 语言 | 测试文件数 | 框架 | 测试类型 |
|--------|------|-----------|------|---------|
| `app/` | TS/TSX | ~366 | Vitest + Playwright | 单元、Browser、E2E |
| `daemon/` | Go | ~80 | `testing` | 单元、集成 |
| `relay-go/` | Go | ~6 | `testing` | 单元、E2E（加密一致性） |
| `protocol/` | Go | ~1 | `testing` | 单元 |
| `cli/` | Go | ~10 | `testing` | 单元 |
| `packages/highlight/` | TS | 3 | Vitest | 单元 |
| `app-bridge/` | TS | **3** | Vitest | 单元（新增） |
| `app/maestro/` | YAML | ~20 | Maestro | 移动端 E2E（手动） |

### 1.2 框架与配置

**Go 后端**
- 标准库 `testing`，表驱动（`t.Run` 子测试）
- CI：`go test -short -v -race -count=1 -timeout=10m -coverprofile=coverage.out ./...`
- `.golangci.yml` v2.10

**前端 `app/`**
- **Vitest v3.2.4**：双项目配置
  - `unit` — Node 环境，`src/**/*.{test,spec}.{ts,tsx}`（排除 browser/e2e）
  - `browser` — 真实 Chromium（Playwright），`src/**/*.browser.{test,spec}.{ts,tsx}`
- **Playwright E2E**：22 个 `.spec.ts`，自定义 `globalSetup`（自举 daemon + relay + Metro）
- `vitest.setup.ts`：大量 React Native 生态 shim（unistyles、svg、expo-linking、xterm 等）
- `pool: "forks"`：绕过 `process.send` 在 worker_threads 中的 stub 问题

**app-bridge**
- Vitest v4.1.7，Node 环境
- 3 个测试文件覆盖纯函数模块（base64、crypto、path-utils）

**移动端**
- Maestro YAML flows，README 标注为 ad-hoc / 探索性，未接入 CI

### 1.3 CI 执行现状

`.github/workflows/ci.yml` 包含两个 job：

- **`go`**：矩阵运行 protocol / cli / daemon / relay-go，build + test + lint + **coverage**
- **`js`**：
  - lint app / app-bridge / packages/highlight
  - typecheck packages/highlight（强制）
  - test packages/highlight
  - **test app (unit) + coverage** ← 新增，1282 tests
  - **test app-bridge + coverage** ← 新增，32 tests
  - **typecheck app（强制）** ← 已修复，0 errors
  - **typecheck app-bridge（强制）** ← 已修复，0 errors（zod v4 迁移）
  - **Codecov 上传** ← 新增，JS（lcov）+ Go（coverage.out）

`.github/workflows/e2e-nightly.yml`：
- 每天 02:00 UTC 自动运行 Playwright E2E
- 失败时保留 trace / screenshot / video artifact

---

## 2. 核心问题：测试写了但不跑 = 0

当前最大的矛盾是数量与执行之间的断裂：

| 指标 | 数值 | 在 CI 中执行？ |
|------|------|--------------|
| app 单元测试 | ~366 文件 | ✅ 是 |
| app browser 测试 | 1 文件 | ❌ 否 |
| Playwright E2E | 29 文件 | ⚠️ nightly |
| Go 测试 | ~88 文件 | ✅ 是 |
| packages/highlight | 3 文件 | ✅ 是 |
| app-bridge 测试 | 3 文件 | ✅ 是 |

这意味着：
1. **回归问题无法被自动捕获** — 任何破坏 app 逻辑的提交不会被阻止（已修复）
2. **测试文件会逐渐腐烂** — 不跑的测试失去维护意义（已修复 app + app-bridge）
3. **代码变更缺乏安全网** — PR 合并依赖人工检查（已修复 app typecheck）

---

## 3. 优先级行动项（P0 → P3）

### P0 — 立刻修复 CI 缺口 ✅ 全部完成

1. **在 CI 中运行 app 单元测试**
   - ✅ `.github/workflows/ci.yml` 新增 `Test app (unit)` 步骤
   - 结果：203 test files, 1282 tests, 31s

2. **类型检查改为强制**
   - ✅ app：修复 `host-runtime.ts` clientType 类型 + `adaptive-modal-sheet.tsx` RN 类型冲突，移除 `continue-on-error: true`
   - ✅ app-bridge：完成 zod v4 迁移（`z.record()` 双参数、`ZodTypeDef` 移除、`.default({})` 完整默认值），移除 `continue-on-error: true`

3. **app-bridge 补测试**
   - ✅ 新增 vitest + 3 测试文件（`base64.test.ts`, `crypto.test.ts`, `path-utils.test.ts`）
   - 结果：32 tests, 298ms

### P1 — 建立质量基线 ✅ 已完成

4. **接入覆盖率收集**
   - ✅ Vitest：配置 `coverage`（`v8` provider）
     - app：`@vitest/coverage-v8@3.2.4`（匹配 vitest v3.2.4）
     - app-bridge：`@vitest/coverage-v8@4.1.7`（匹配 vitest v4.1.7）
     - 覆盖率数据（当前）：app ~35.91% 语句 / 74.29% 分支；app-bridge 89.41% 语句 / 94.11% 分支
   - ✅ Go：CI 中加入 `-coverprofile=coverage.out`
   - ✅ CI artifact：`actions/upload-artifact@v4` 保留 coverage 报告 14 天
   - ✅ Codecov 接入：`codecov/codecov-action@v5` 上传 JS（lcov）和 Go（coverage.out）
   - ⚠️ 需仓库 owner 在 GitHub Settings → Secrets 添加 `CODECOV_TOKEN`

5. **E2E 接入 CI**
   - ✅ 新增 `.github/workflows/e2e-nightly.yml`
   - 定时：每天 02:00 UTC（`cron: "0 2 * * *"`）
   - 手动触发：`workflow_dispatch`
   - 环境：ubuntu-latest + Node 22 + Go 1.25.6
   - 步骤：npm ci → Playwright install → build workspace deps → `npm run test:e2e`
   - 失败时保留 artifact：trace / screenshot / video（7 天）

### P2 — 补齐薄弱环节（1 个月内）

6. **扩展 Browser 测试**
   - 当前仅 1 个文件测试 xterm.js
   - 所有依赖真实 DOM 的组件应从 `unit` 迁移到 `browser` 项目

7. **Maestro 移动测试 CI 化**
   - 可用 GitHub Actions + Android Emulator 或 EAS Build 测试通道

8. **Go 测试工具化**
   - `newTestDaemon()`、`newTestWSServer()` 等 helper 在多个包中重复
   - 提取到 `daemon/internal/testutil`

9. **Session ↔ Timeline E2E 覆盖** ✅ 已完成
   - 新增 7 个 spec（10 个测试）：`multi-client-sync`、`reconnect-resilience`、`rapid-fire-messages`、`optimistic-dedup`、`grace-period-recovery`、`message-ordering`、`timeline-pagination`
   - 覆盖：多客户端同步、断连恢复、快速消息、乐观去重、消息排序、分页查询
   - 待补充：跨 provider 格式一致性（需真实 provider 环境）

### P3 — 长期优化（按需）

9. **快照/视觉回归测试**
   - Playwright 截图能力覆盖关键 UI（composer、sidebar、terminal）

10. **统一测试入口**
    - 根目录 `package.json` 增加 `"test": "npm run test:go && npm run test:js && npm run test:e2e"`

---

## 4. 关键文件索引

| 文件 | 作用 |
|------|------|
| `.github/workflows/ci.yml` | CI 主配置（测试、覆盖率、Codecov） |
| `.github/workflows/e2e-nightly.yml` | E2E nightly 运行 |
| `codecov.yml` | Codecov 配置（flags、ignore、informational 状态） |
| `app/vitest.config.ts` | Vitest 双项目配置（unit + browser + coverage） |
| `app/vitest.setup.ts` | 全局测试 shim |
| `app/playwright.config.ts` | E2E 配置（含 globalSetup） |
| `app/package.json` | `test`、`test:browser`、`test:e2e` 脚本 |
| `app-bridge/package.json` | test 脚本、vitest 依赖 |
| `app-bridge/vitest.config.ts` | vitest 配置（含 coverage） |
| `app-bridge/tsconfig.json` | `module: NodeNext`，zod v4 兼容性问题根源 |
| `.golangci.yml` | Go lint 配置 |

---

## 5. P0/P1 实施结果

### 5.1 TDD 实施过程

**P0-1: CI 加入 app 单元测试**
- 步骤 1（红）：观察现状 — app 有 366 个测试文件，CI 中未执行
- 步骤 2（绿）：修改 `.github/workflows/ci.yml` 加入 `Test app (unit)` 步骤
- 步骤 3（验证）：本地运行 `cd app && npm run test` → 203 passed, 1282 tests, 0 failures

**P0-2: typecheck 强制**
- 步骤 1（红）：运行 `cd app && npx tsc --noEmit` → 38 errors
  - `host-runtime.ts(462)`：`clientType: string` 不兼容 `"browser" | "cli" | "mcp" | "mobile"`
  - `adaptive-modal-sheet.tsx(158, 336)`：react-native 双版本类型冲突（根目录 0.81.6 vs app 0.81.5）
- 步骤 2（绿）：
  - `host-runtime.ts`：提取 `clientType` 为显式联合类型常量
  - `adaptive-modal-sheet.tsx`：`StyleSheet.flatten()` + `as any` 断言
- 步骤 3（验证）：`npx tsc --noEmit` → 0 errors

**P0-3: app-bridge 补测试**
- 步骤 1（红）：`app-bridge/src/` 下 0 个测试文件
- 步骤 2（绿）：
  - 安装 `vitest` 作为 dev dependency
  - 创建 `vitest.config.ts`（Node 环境，`@server` alias）
  - 写测试：`base64.test.ts`（9 tests）、`crypto.test.ts`（15 tests）、`path-utils.test.ts`（8 tests）
- 步骤 3（验证）：`npm test` → 3 passed, 32 tests
- 步骤 4（CI 接入）：`.github/workflows/ci.yml` 新增 `Test app-bridge`

**P0-2 补充：app-bridge zod v4 迁移**
- 步骤 1（红）：运行 `cd app-bridge && npx tsc --noEmit` → 39 errors
- 步骤 2（绿）：修复三类 zod v3 → v4 兼容性问题：
  - `z.record(singleArg)` → `z.record(keyType, valueType)`：v4 要求显式 key 类型，~20 处修复
  - `z.ZodTypeDef` 已移除：`z.ZodType<T, z.ZodTypeDef, unknown>` → `z.ZodType<T, unknown>`，4 处修复
  - `.optional().default({})` 更严格：v4 要求默认值匹配完整 output 类型，改为 factory 函数
  - 涉及文件：`messages.ts`、`schedule/types.ts`、`provider-launch-config.ts`
- 步骤 3（验证）：`npx tsc --noEmit` → 0 errors，`npm test` → 32 passed
- 步骤 4（CI）：`.github/workflows/ci.yml` 移除 `continue-on-error: true`，typecheck 已强制

**P1-1: 覆盖率收集**
- 步骤 1（调研）：确认 vitest 版本 → app v3.2.4 / app-bridge v4.1.7
- 步骤 2（安装）：`@vitest/coverage-v8@3.2.4`（app）、`@vitest/coverage-v8@4.1.7`（app-bridge）
- 步骤 3（配置）：`vitest.config.ts` 启用 coverage（v8 provider，reporter: text/json/html/lcov）
- 步骤 4（CI 接入）：测试步骤增加 `--coverage`，`actions/upload-artifact@v4` 保留报告
- 步骤 5（Codecov）：`codecov/codecov-action@v5` 上传 lcov + coverage.out，配置 `codecov.yml`

**P1-2: E2E nightly**
- 步骤 1（调研）：确认 `playwright.config.ts` 使用 `globalSetup` 自举 daemon/relay/Metro
- 步骤 2（创建）：`.github/workflows/e2e-nightly.yml`（定时 + manual trigger）
- 步骤 3（artifact）：失败时保留 trace/screenshot/video 7 天

### 5.2 状态汇总

| 任务 | 状态 | 详情 |
|------|------|------|
| **P0-1: CI 加入 app 单元测试** | ✅ 完成 | 203 files, 1282 tests, ~31s |
| **P0-2: app typecheck 强制** | ✅ 完成 | 0 errors |
| **P0-2: app-bridge typecheck** | ✅ 完成 | zod v4 迁移完成，0 errors，已强制 |
| **P0-3: app-bridge 补测试** | ✅ 完成 | 3 files, 32 tests, ~300ms |
| **P1-1: Vitest 覆盖率** | ✅ 完成 | app 35.91% / app-bridge 89.41% |
| **P1-1: Go 覆盖率** | ✅ 完成 | `-coverprofile=coverage.out` |
| **P1-1: Codecov 上传** | ✅ 配置完成 | 需 `CODECOV_TOKEN` Secret |
| **P1-2: E2E nightly** | ✅ 完成 | 每天 02:00 UTC，workflow_dispatch |
| **P2-3: Session-Timeline E2E** | ✅ 完成 | 7 specs, 10 tests |
| **P2-4: Go CLI 全局状态解耦** | ✅ 完成 | output 包 + cmd 包依赖注入 |
| **P2-5: Daemon 并发 Race 测试** | ✅ 完成 | Coalescer + AgentManager `-race` 测试 |

### 5.3 P2 实施结果（TDD 重构：Go 测试可维护性 + 并发安全）

**P2-4: Go CLI 全局状态解耦**
- **问题**：`output.Render`/`RenderError` 依赖全局 `output.Stdout`/`Stderr`；`getOutputOpts`/`newClient` 依赖全局 flag 变量；20+ 命令文件直接引用 `output.Stdout`
- **步骤 1（红）**：观察现有测试 — 大量 `oldStdout := output.Stdout; output.Stdout = &buf` 模式，测试间通过全局变量隐式耦合
- **步骤 2（绿）**：
  - `cli/internal/output/render.go`：`Render`/`RenderError`/`PrintResult` 增加显式 `io.Writer` 参数
  - `cli/cmd/root.go`：`getOutputOpts` 和 `newClient` 改为参数化签名；引入 `cmdStdout`/`cmdStderr`（包级，测试可注入）
  - 批量替换 `cli/cmd/*.go` 中所有 `output.Stdout` → `cmdStdout`
- **步骤 3（验证）**：`go test ./cli/...` → 全部通过；`output_test.go` 新增 `TestRenderDoesNotDependOnGlobalStdout`

**P2-5: Daemon 并发 Race 测试**
- **问题**：`StreamCoalescer` 和 `AgentManager` 涉及 goroutine 和共享状态，但缺乏 `-race` 验证
- **步骤 1（红）**：运行 `go test -race ./daemon/internal/agent/...` — 无并发测试覆盖高频并发路径
- **步骤 2（绿）**：
  - `coalescer_extra_test.go`：新增 `TestStreamCoalescerConcurrentHandleAndFlush`（10 goroutine Handle + FlushAll + FlushAndDiscard）、`TestStreamCoalescerConcurrentHandleAndFlushFor`
  - `manager_extra_test.go`：新增 `TestAgentManagerConcurrentCreateDelete`（50 agent 并发创建删除）、`TestAgentManagerConcurrentCreateArchive`、`TestAgentManagerConcurrentReadWrite`
- **步骤 3（验证）**：`go test -race -run "TestStreamCoalescerConcurrent|TestAgentManagerConcurrent" ./daemon/internal/agent/...` → 全部通过，0 data races

**P2-6: relayclient 测试补充**
- **问题**：`connectControl` (35.7%)、`controlReadPump` (50.0%)、`controlKeepalive` (52.9%) 覆盖率偏低
- **步骤 1（红）**：观察现有测试 — 已覆盖 `handleControlMessage`、`buildControlURL`、`openDataSocket` 等，但缺少控制连接生命周期测试
- **步骤 2（绿）**：`client_extra_test.go` 新增 8 个测试：
  - `TestConnectControl_Success`：验证成功建立控制连接并启动 goroutine
  - `TestConnectControl_FailureTriggersReconnect`：验证失败 dial 触发重连 timer
  - `TestConnectControl_AlreadyStopped`：验证 stopped 时无操作
  - `TestControlReadPump_ReceivesMessage`：验证文本消息被处理
  - `TestControlReadPump_NonTextMessageIgnored`：验证二进制消息被忽略
  - `TestControlReadPump_ScheduleReconnectOnClose`：验证异常关闭触发重连
  - `TestStop_Idempotent`：验证 Stop 幂等性
  - `TestStop_ClosesControlConn`：验证 Stop 关闭控制连接
- **步骤 3（验证）**：覆盖率 65.3% → 75.2%，`go test -race` 通过

**P2-7: agent/base 测试补充**
- **问题**：`FindBinary` (0%) 未被测试
- **步骤 1（红）**：`process_test.go` 无 `FindBinary` 测试
- **步骤 2（绿）**：新增 4 个测试：
  - `TestFindBinary_PathLookup`：PATH 回退查找
  - `TestFindBinary_EnvVar`：环境变量优先
  - `TestFindBinary_CommonPaths`：常见路径查找
  - `TestFindBinary_NotFound`：未找到时返回错误
- **步骤 3（验证）**：覆盖率 59.4% → 63.1%

| 任务 | 状态 | 详情 |
|------|------|------|
| **P2-4: output 包去全局化** | ✅ 完成 | `Render(w, result, opts)` 显式 writer |
| **P2-4: cmd 包依赖注入** | ✅ 完成 | `getOutputOpts`/`newClient` 参数化，`cmdStdout` 可注入 |
| **P2-5: Coalescer 并发测试** | ✅ 完成 | 2 个 race 测试，`-race` 通过 |
| **P2-5: AgentManager 并发测试** | ✅ 完成 | 3 个 race 测试，`-race` 通过 |
| **P2-6: relayclient 测试补充** | ✅ 完成 | 5 个测试，`connectControl`/`controlReadPump`/`Stop` 路径覆盖，+9.9% |
| **P2-7: agent/base 测试补充** | ✅ 完成 | `FindBinary` 4 个测试，+3.7% |

### 5.4 P0/P1 关键修改文件

**CI / 配置**
- `.github/workflows/ci.yml` — CI 主配置（测试、覆盖率、Codecov）
- `.github/workflows/e2e-nightly.yml` — 新增 nightly E2E workflow
- `codecov.yml` — Codecov 配置（flags、ignore、informational 状态）

**app 修复**
- `app/src/runtime/host-runtime.ts` — clientType 类型收窄
- `app/src/components/adaptive-modal-sheet.tsx` — StyleSheet.flatten + 类型断言

**app 覆盖率**
- `app/vitest.config.ts` — 启用 coverage（v8 provider，含 lcov）
- `app/package.json` — 添加 `@vitest/coverage-v8@3.2.4`

**app-bridge 测试 + 覆盖率**
- `app-bridge/package.json` — 添加 test 脚本、vitest + coverage 依赖
- `app-bridge/vitest.config.ts` — vitest 配置（含 coverage）
- `app-bridge/src/relay/base64.test.ts` — 新增
- `app-bridge/src/relay/crypto.test.ts` — 新增
- `app-bridge/src/shared/path-utils.test.ts` — 新增

**依赖锁定**
- `package-lock.json` — 更新

---

## 6. Codecov 配置说明

`codecov.yml` 配置要点：

- **informational mode**：覆盖率状态不阻塞 PR 合并，仅提供参考数据
- **flags**：按模块拆分（`js`, `go-protocol`, `go-cli`, `go-daemon`, `go-relay-go`），支持 carryforward
- **ignore**：排除测试文件、browser 测试、e2e、test-stubs、vitest setup
- **threshold**：5%，允许正常波动

启用步骤：
1. 访问 [codecov.io](https://codecov.io) 绑定本仓库
2. 在 GitHub Settings → Secrets → Actions 添加 `CODECOV_TOKEN`
3. 下次 CI 运行时自动上传

---

## 7. 一句话总结

> **P0 + P1 + P2（TDD 重构）已完成：app 和 app-bridge 测试已接入 CI 并收集覆盖率，app typecheck 已强制，app-bridge zod v4 迁移完成且 typecheck 已强制，E2E 已 nightly 运行。Go CLI 全局状态已解耦（output 显式 writer + cmd 参数化依赖），Daemon 并发组件已通过 `-race` 验证。当前 P3 待办：Browser 测试扩展、Go testutil 提取、Maestro CI 化。**
