# 测试集全景分析报告

> 生成时间：2026-05-24
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
| `cli/` | Go | ~1 | `testing` | 单元 |
| `packages/highlight/` | TS | 3 | Vitest | 单元 |
| `app-bridge/` | TS | **3** | Vitest | 单元（新增） |
| `app/maestro/` | YAML | ~20 | Maestro | 移动端 E2E（手动） |

### 1.2 框架与配置

**Go 后端**
- 标准库 `testing`，表驱动（`t.Run` 子测试）
- CI：`go test -short -v -race -count=1 -timeout=10m ./...`
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

- **`go`**：矩阵运行 protocol / cli / daemon / relay-go，build + test + lint
- **`js`**：
  - lint app / app-bridge / packages/highlight
  - typecheck packages/highlight（强制）
  - test packages/highlight
  - **test app (unit)** ← 新增，1282 tests
  - **test app-bridge** ← 新增，32 tests
  - **typecheck app（强制）** ← 已修复，0 errors
  - typecheck app-bridge（optional）← 39 zod v4 迁移错误

---

## 2. 核心问题：测试写了但不跑 = 0

当前最大的矛盾是数量与执行之间的断裂：

| 指标 | 数值 | 在 CI 中执行？ |
|------|------|--------------|
| app 单元测试 | ~366 文件 | ✅ 是 |
| app browser 测试 | 1 文件 | ❌ 否 |
| Playwright E2E | 22 文件 | ❌ 否 |
| Go 测试 | ~88 文件 | ✅ 是 |
| packages/highlight | 3 文件 | ✅ 是 |
| app-bridge 测试 | 3 文件 | ✅ 是 |

这意味着：
1. **回归问题无法被自动捕获** — 任何破坏 app 逻辑的提交不会被阻止（已修复）
2. **测试文件会逐渐腐烂** — 不跑的测试失去维护意义（已修复 app + app-bridge）
3. **代码变更缺乏安全网** — PR 合并依赖人工检查（已修复 app typecheck）

---

## 3. 优先级行动项（P0 → P3）

### P0 — 立刻修复 CI 缺口（本周）✅ 已完成

1. **在 CI 中运行 app 单元测试**
   - ✅ `.github/workflows/ci.yml` 新增 `Test app (unit)` 步骤
   - 结果：203 test files, 1282 tests, 31s

2. **类型检查改为强制**
   - ✅ app：修复 `host-runtime.ts` clientType 类型 + `adaptive-modal-sheet.tsx` RN 类型冲突，移除 `continue-on-error: true`
   - ⚠️ app-bridge：39 个 zod v4 兼容性错误，需专门迁移任务

3. **app-bridge 补测试**
   - ✅ 新增 vitest + 3 测试文件（`base64.test.ts`, `crypto.test.ts`, `path-utils.test.ts`）
   - 结果：32 tests, 298ms

### P1 — 建立质量基线（进行中）

4. **接入覆盖率收集**
   - ✅ Vitest：配置 `coverage`（`v8` provider）
     - app：`@vitest/coverage-v8@3.2.4`（匹配 vitest v3.2.4），`vitest.config.ts` 配置 reporter: text/json/html
     - app-bridge：`@vitest/coverage-v8@4.1.7`（匹配 vitest v4.1.7）
     - 覆盖率数据：app 约 35.91% 语句 / 74.29% 分支；app-bridge 89.41% 语句 / 94.11% 分支
   - ✅ Go：CI 中加入 `-coverprofile=coverage.out`
   - ✅ CI artifact：使用 `actions/upload-artifact@v4` 保留 coverage 报告 14 天
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

### P3 — 长期优化（按需）

9. **快照/视觉回归测试**
   - Playwright 截图能力覆盖关键 UI（composer、sidebar、terminal）

10. **统一测试入口**
    - 根目录 `package.json` 增加 `"test": "npm run test:go && npm run test:js && npm run test:e2e"`

---

## 4. 关键文件索引

| 文件 | 作用 |
|------|------|
| `.github/workflows/ci.yml` | CI 主配置 |
| `app/vitest.config.ts` | Vitest 双项目配置（unit + browser） |
| `app/vitest.setup.ts` | 全局测试 shim |
| `app/playwright.config.ts` | E2E 配置（含 globalSetup） |
| `app/package.json` | `test`、`test:browser`、`test:e2e` 脚本 |
| `app-bridge/package.json` | test 脚本、vitest 依赖 |
| `app-bridge/vitest.config.ts` | vitest 配置 |
| `app-bridge/tsconfig.json` | `module: NodeNext`，zod v4 兼容性问题根源 |
| `.golangci.yml` | Go lint 配置 |

---

## 5. P0 实施结果（2026-05-24）

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

### 5.2 状态汇总

| 任务 | 状态 | 详情 |
|------|------|------|
| **P0-1: CI 加入 app 单元测试** | ✅ 完成 | 203 files, 1282 tests, ~31s |
| **P0-2: app typecheck 强制** | ✅ 完成 | 0 errors |
| **P0-2: app-bridge typecheck** | ⚠️ 阻塞中 | 39 个 zod v4 兼容性错误，保留 optional |
| **P0-3: app-bridge 补测试** | ✅ 完成 | 3 files, 32 tests, ~300ms |

### 5.3 P0 关键修改文件
- `.github/workflows/ci.yml` — CI 配置
- `app/src/runtime/host-runtime.ts` — clientType 类型收窄
- `app/src/components/adaptive-modal-sheet.tsx` — StyleSheet.flatten + 类型断言
- `app-bridge/package.json` — 添加 test 脚本、vitest 依赖
- `app-bridge/vitest.config.ts` — vitest 配置
- `app-bridge/src/relay/base64.test.ts` — 新增
- `app-bridge/src/relay/crypto.test.ts` — 新增
- `app-bridge/src/shared/path-utils.test.ts` — 新增

### 5.4 P1 关键修改文件
- `app/vitest.config.ts` — 启用 coverage（v8 provider，含 lcov）
- `app-bridge/vitest.config.ts` — 启用 coverage（v8 provider，含 lcov）
- `.github/workflows/ci.yml` — 测试步骤增加 `--coverage`，新增 Codecov 上传 + artifact 保留
- `.github/workflows/e2e-nightly.yml` — 新增 nightly E2E workflow
- `codecov.yml` — Codecov 配置（flags、ignore、informational 状态）

---

## 6. 一句话总结

> **P0 已完成：app 单元测试和 app-bridge 测试已接入 CI，app typecheck 已强制。当前核心缺口是覆盖率（零配置）、E2E 未跑 CI、以及 app-bridge 的 zod v4 迁移。**
