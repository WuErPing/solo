# Solo 测试覆盖率报告

> 最后更新: 2026-06-16
> 本文档合并了此前分散的覆盖率文档，为项目测试覆盖率的唯一权威来源。

---

## 1. 总览

| 层 | 工具 | 覆盖率 | 状态 |
|----|------|--------|------|
| Go 后端 | `go test -short -race -coverprofile` | **~75%** (加权) | 大部分模块 >80%，2 个核心包 <70% |
| App 前端 | Vitest + v8 | **35.5%** (statements) | 工具层良好，UI 层薄弱 |
| App-Bridge | Vitest + v8 | **89.4%** | 优秀 |
| E2E | Playwright | 35 specs, ~6900 行 | 核心流程覆盖，nightly 运行 |
| CI | GitHub Actions | Go + JS + E2E nightly | Codecov 已集成 |

**关键指标**:
- Go 测试文件: 174 个 (158 源文件, 比例 1.10:1)
- App 测试文件: 235 个 (unit) + 35 个 (E2E)
- App 单元测试用例: 1,663 个
- CI 每次 push 运行: Go 测试 + App unit 测试 + App-Bridge 测试 + Lint + Typecheck
- E2E: 每日 02:00 UTC 自动运行 + 手动触发

---

## 2. Go 后端覆盖率

> 数据来源: `go test -short -race`，2026-06-12

### 2.1 模块覆盖率

| 模块 | 包 | 覆盖率 | 状态 |
|------|-----|--------|------|
| **Protocol** | `protocol` | 83.6% | OK |
| **CLI** | `cli/cmd` | 70.4% | 低 |
| | `cli/internal/client` | 87.4% | 良好 |
| | `cli/internal/output` | 92.0% | 优秀 |
| | `cli/internal/cliutil` | 91.8% | 优秀 |
| **Relay** | `relay/internal/config` | 97.0% | 优秀 |
| | `relay/internal/e2ee` | 83.2% | 良好 |
| | `relay/internal/metrics` | 100.0% | 完整 |
| | `relay/internal/relay` | 85.7% | 良好 |
| **Daemon** | `daemon/internal/agent` | 66.2% | 低 |
| | `daemon/internal/agent/base` | 89.0% | 良好 |
| | `daemon/internal/config` | 84.1% | 良好 |
| | `daemon/internal/memory` | 95.8% | 优秀 |
| | `daemon/internal/memory/bridge` | 90.9% | 优秀 |
| | `daemon/internal/memory/filebackend` | 85.0% | 良好 |
| | `daemon/internal/memory/redact` | 95.8% | 优秀 |
| | `daemon/internal/memorysetup` | 80.0% | 良好 |
| | `daemon/internal/metrics` | 100.0% | 完整 |
| | `daemon/internal/pidlock` | 87.5% | 良好 |
| | `daemon/internal/push` | 84.3% | 良好 |
| | `daemon/internal/relayclient` | 81.0% | 良好 |
| | `daemon/internal/schedule` | 81.6% | 良好 |
| | `daemon/internal/server` | 61.6% | 低 |
| | `daemon/internal/terminal` | 86.6% | 良好 |
| | `daemon/internal/workspace` | 86.0% | 良好 |

### 2.2 测试文件分布

| 模块 | 源文件 | 测试文件 | 比例 |
|------|--------|----------|------|
| daemon | 94 | 146 | 1.55:1 |
| relay-go | 10 | 8 | 0.80:1 |
| cli | 39 | 13 | 0.33:1 |
| protocol | 15 | 7 | 0.47:1 |
| **合计** | **158** | **174** | **1.10:1** |

### 2.3 高覆盖率包 (>= 90%)

- `daemon/internal/metrics` (100%), `relay/internal/metrics` (100%)
- `daemon/internal/memory` (95.8%), `daemon/internal/memory/redact` (95.8%)
- `daemon/internal/memory/bridge` (90.9%)
- `relay/internal/config` (97.0%)
- `cli/internal/output` (92.0%), `cli/internal/cliutil` (91.8%)
- `daemon/internal/agent/base` (89.0%)

### 2.4 低覆盖率包 (< 70%) — 最高风险

| 包 | 覆盖率 | 源文件 | 测试文件 | 风险 |
|----|--------|--------|----------|------|
| `daemon/internal/server` | 61.6% | 24 | 38 | **高** — WebSocket API, session 管理, 多客户端同步 |
| `daemon/internal/agent` | 66.2% | 20 | 58 | **高** — 核心业务: Provider 系统, AgentManager, Timeline |
| `cli/cmd` | 70.4% | 24 | 4 | 中 — 用户入口命令层 |

### 2.5 覆盖率提升历史

以下模块经过 TDD 专项提升 (2026-05-27 ~ 2026-06-12):

| 模块 | 提升前 | 提升后 | 增幅 |
|------|--------|--------|------|
| `daemon/internal/workspace` | 46.5% | 86.0% | +39.5pp |
| `daemon/internal/agent/base` | 63.1% | 89.0% | +25.9pp |
| `daemon/internal/terminal` | 76.4% | 86.6% | +10.2pp |
| `cli/internal/client` | 77.6% | 87.4% | +9.8pp |
| `daemon/internal/relayclient` | 75.2% | 81.0% | +5.8pp |
| `daemon/internal/agent` | 51.1% | 66.2% | +15.1pp |
| `cli/cmd` | 69.0% | 70.4% | +1.4pp |
| `daemon/internal/server` | 61.1% | 61.6% | +0.5pp |

---

## 3. App 前端覆盖率

> 数据来源: Vitest v8 provider, 2026-05-28
> 整体: **35.51%** (29,467 / 82,979 lines)

### 3.1 目录覆盖率

**高覆盖率 (>75%)**

| 目录 | 覆盖率 | 行数 | 说明 |
|------|--------|------|------|
| `src/query` | 100.0% | 12 | 完整 |
| `src/styles` | 91.8% | 861 | 主题和样式 |
| `src/keyboard` | 87.8% | 1,418 | 键盘处理 |
| `src/constants` | 79.3% | 82 | 常量定义 |
| `src/utils` | 79.3% | 5,241 | 工具函数 |
| `src/terminal` | 75.9% | 1,076 | 终端逻辑 |

**中覆盖率 (40-75%)**

| 目录 | 覆盖率 | 行数 | 说明 |
|------|--------|------|------|
| `src/stores` | 65.4% | 5,697 | Zustand stores |
| `src/runtime` | 65.0% | 1,835 | 运行时逻辑 |
| `src/panels` | 44.5% | 1,961 | 面板组件 |
| `src/attachments` | 44.1% | 696 | 附件处理 |
| `src/desktop` | 43.7% | 2,708 | 桌面端代码 |
| `src/hooks` | 43.3% | 7,594 | 自定义 hooks |

**低覆盖率 (<40%)**

| 目录 | 覆盖率 | 行数 | 说明 |
|------|--------|------|------|
| `src/screens` | 31.4% | 11,102 | 页面组件 |
| `src/contexts` | 25.7% | 2,707 | React contexts |
| `src/components` | 18.7% | 35,434 | UI 组件 |
| `src/app` | 7.3% | 1,676 | 入口/导航 |

### 3.2 App-Bridge 覆盖率

| 指标 | 值 |
|------|-----|
| Statements | 89.41% |
| Branches | 94.11% |
| 测试文件 | 3 (base64, crypto, path-utils) |
| 测试数量 | 32 |

### 3.3 前端覆盖率关键差距

1. **UI 组件** (18.7%): 35,434 行仅 6,621 行覆盖 — 回归风险高
2. **页面组件** (31.4%): agent、chat、settings 主要页面测试不足
3. **React Context** (25.7%): theme、auth、workspace 全局状态缺乏测试
4. **自定义 Hooks** (43.3%): useAgent、useWorkspace 等核心 hooks 覆盖不足

---

## 4. E2E 测试覆盖

> 35 个 Playwright spec 文件, ~6900 行代码
> 运行方式: 每日 02:00 UTC nightly + workflow_dispatch 手动触发

### 4.1 E2E Spec 清单

**核心流程**

| Spec | 覆盖场景 |
|------|----------|
| `solo-local-core.spec.ts` | Mock provider, 验证 assistant 文本可见 |
| `multi-client-sync.spec.ts` | 两客户端同步, timeline 无重复 |
| `reconnect-resilience.spec.ts` | 断线重连后 timeline 完整 |
| `grace-period-recovery.spec.ts` | 断线期间消息, 重连后可见 |
| `rapid-fire-messages.spec.ts` | 20 条快速消息无丢失 |
| `optimistic-dedup.spec.ts` | 用户消息仅显示一次 |
| `message-ordering.spec.ts` | 消息严格按发送顺序 |
| `timeline-pagination.spec.ts` | tail/before/after 游标分页 |

**Workspace & 导航**

| Spec | 覆盖场景 |
|------|----------|
| `new-workspace.spec.ts` | 创建新 workspace |
| `workspace-lifecycle.spec.ts` | workspace 生命周期 |
| `workspace-cwd.spec.ts` | 工作目录切换 |
| `workspace-navigation-regression.spec.ts` | 导航回归 |
| `workspace-setup-runtime.spec.ts` | setup 脚本执行 |
| `workspace-setup-streaming.spec.ts` | setup 流式输出 |
| `sidebar-workspace.spec.ts` | 侧边栏 workspace 列表 |

**Terminal**

| Spec | 覆盖场景 |
|------|----------|
| `terminal-alternate-screen.spec.ts` | 终端备用屏幕 |
| `terminal-keystroke-stress.spec.ts` | 击键压力测试 |
| `terminal-performance.spec.ts` | 终端性能 |

**Settings & UI**

| Spec | 覆盖场景 |
|------|----------|
| `settings-navigation.spec.ts` | 设置页导航 |
| `settings-host-page.spec.ts` | Host 设置页 |
| `settings-toggle-tab-regression.spec.ts` | Tab 切换回归 |
| `projects-settings.spec.ts` | 项目设置 |
| `archive-tab.spec.ts` | 归档标签 |
| `launcher-tab.spec.ts` | 启动器标签 |
| `svg-preview.spec.ts` | SVG 预览 |
| `file-explorer-collapse.spec.ts` | 文件浏览器折叠 |

**网络 & 基础设施**

| Spec | 覆盖场景 |
|------|----------|
| `startup-loading.spec.ts` | 启动加载 |
| `startup-wire-metrics.spec.ts` | 启动指标 |
| `web-direct-tcp-reconnect.spec.ts` | TCP 重连 |
| `relay-data-socket-timeout-regression.spec.ts` | Relay 超时回归 |
| `pi-provider-tool-use.spec.ts` | Pi provider 工具调用 |

### 4.2 E2E 已识别但未覆盖的场景

| 场景 | 优先级 | 阻塞因素 |
|------|--------|----------|
| 跨 provider 格式一致性 (Claude/OpenCode/Kimi) | P1 | 需要真实 provider 环境 |
| messageID 传播验证 | P1 | 需要真实 provider |
| 入队队列溢出恢复 (100+ 快速消息) | P2 | Mock provider 串行化限制 |
| App 后台 2 分钟后恢复 | P2 | 需要 mobile E2E |
| 真实 provider 的多客户端同步 | P2 | 需要真实 provider |

---

## 5. CI/CD & Codecov 集成

### 5.1 CI 流水线

**Go 流水线** (matrix: protocol, cli, daemon, relay-go):
- `go mod verify` → `go build ./...` → `go test -short -race -count=1 -timeout=10m -coverprofile=coverage.out`
- `golangci-lint` v2.10

**JS 流水线**:
- Lint: `expo lint --max-warnings 0` (app), `eslint src/` (app-bridge, highlight)
- Typecheck: `tsc --noEmit` (app, app-bridge, highlight) — 强制执行
- Test: `npm test` (packages/highlight); `npm test -- --coverage` (app, app-bridge)
- Coverage 上传: `codecov/codecov-action@v5`

**E2E 流水线** (nightly):
- `.github/workflows/e2e-nightly.yml`
- 每日 02:00 UTC + 手动触发
- 失败时保留 trace/screenshot/video 7 天

### 5.2 Codecov 配置

```yaml
# codecov.yml 关键配置
flags:
  js:           # app/src/ + app-bridge/src/
  go-protocol:  # protocol/
  go-cli:       # cli/
  go-daemon:    # daemon/
  go-relay-go:  # relay-go/

coverage:
  status:
    project:
      default:
        target: auto
        threshold: 5%
        informational: true   # 不阻塞 PR 合并
```

**设置步骤**:
1. 在 [codecov.io](https://codecov.io) 绑定仓库
2. GitHub Settings → Secrets → Actions 添加 `CODECOV_TOKEN`
3. 下次 CI 运行自动上传

### 5.3 关键配置文件

| 文件 | 用途 |
|------|------|
| `.github/workflows/ci.yml` | 主 CI (测试 + 覆盖率 + Codecov) |
| `.github/workflows/e2e-nightly.yml` | E2E nightly |
| `codecov.yml` | Codecov 配置 |
| `app/vitest.config.ts` | Vitest dual project (unit + browser + coverage) |
| `app/vitest.setup.ts` | 全局测试 shims |
| `app/playwright.config.ts` | E2E 配置 |
| `app-bridge/vitest.config.ts` | app-bridge vitest 配置 |
| `.golangci.yml` | Go lint 配置 |

---

## 6. 覆盖率差距根因分析

低覆盖率模块 (< 90%) 共享四个根因模式:

### 6.1 缺少 exec/PTY 抽象

没有 `interface` 包装 `os/exec` 或 PTY 操作，这些路径只能在完整集成环境中测试。

| 受影响模块 | 覆盖率 |
|-----------|--------|
| `daemon/internal/workspace` | 86.0% (已通过 GitCommander 接口改善) |
| `daemon/internal/agent/base` | 89.0% |
| `daemon/internal/terminal` | 86.6% |

**解决方案**: 引入 `exec.Runner` / `PTYStarter` 接口; 测试中用 stub 替代。

### 6.2 缺少 WebSocket/Transport 抽象

WebSocket 调用直接通过 `gorilla/websocket`，无接口层，需要真实服务器测试。

| 受影响模块 | 覆盖率 |
|-----------|--------|
| `daemon/internal/relayclient` | 81.0% |
| `cli/internal/client` | 87.4% |
| `relay-go/internal/relay` | 85.7% |

**解决方案**: 提取 `Transport` 接口; 使用 `httptest.NewServer` + gorilla upgrader 做集成测试。

### 6.3 缺少 Clock 抽象

`time.AfterFunc`、`time.NewTicker` 直接调用，定时器相关路径无法快速单元测试。

| 受影响模块 | 覆盖率 |
|-----------|--------|
| `daemon/internal/agent` | 66.2% (coalescer 已通过 Clock 接口改善至 ~97%) |
| `relay-go/internal/e2ee` | 83.2% |
| `daemon/internal/push` | 84.3% |

**解决方案**: 注入 `Clock` 接口; 测试中使用 fake clock 即时推进时间。

### 6.4 长集成依赖链

这些模块组合了多个服务，下层缺口向上级联。

| 受影响模块 | 覆盖率 |
|-----------|--------|
| `daemon/internal/agent` | 66.2% |
| `daemon/internal/server` | 61.6% |
| `cli/cmd` | 70.4% |

**解决方案**: 使用 `httptest` mock daemon 测试 CLI; 扩展 mockPusher 风格的 mock 测试 server; 使用 `tmpdir` 测试存储和 pidlock。

---

## 7. 历次 TDD 专项摘要

### P0 — workspace + agent/coalescer (2026-05-28)

- **workspace**: 引入 `GitCommander` 接口, 消除 `exec.Command` 直接调用, 46.5% → 86.0%
- **coalescer**: 引入 `Clock` 接口, 12 个确定性测试 (零 sleep), ~70% → ~97%
- **provider_mock**: 30 个测试覆盖所有方法, ~30% → ~85%

### P1 — agent/base + server handlers (2026-05-28)

- **agent/base**: 25 个测试覆盖 CallbackDispatcher, PermissionManager, EventPump 等, 63.1% → 88.7%
- **server**: 22 个 WebSocket handler 集成测试, 61.1% → 63.7%

### P2 — 多模块补充 (2026-05-28)

- **cli/internal/client**: 密码学 & 配对测试, 77.6% → 87.4%
- **terminal**: No-PTY 路径测试, 76.4% → 86.6%
- **relayclient**: E2EEConn 包装方法, 75.2% → 81.0%
- **cli/cmd**: onboard 纯函数, 69.0% → 71.5%
- **server**: CORS & Status, 63.7% → 64.2%
- **agent**: 事件优先级 & 持久化元数据, 51.1% → 56.7%

### P2 — 并发安全 & 可维护性 (2026-05-25)

- CLI 全局状态解耦: `output.Render` 显式 `io.Writer` 参数, `cmd` 包参数化依赖注入
- Daemon 并发 race 测试: Coalescer + AgentManager `-race` 测试
- relayclient 补充: 8 个测试, `connectControl`/`controlReadPump`/`Stop` 路径, 65.3% → 75.2%

### Session-Timeline E2E (2026-06-08)

- 7 个 E2E spec (10 个测试): 多客户端同步, 断线恢复, 快速消息, 乐观去重, 消息排序, 分页
- 后端 `AppendFromHistory()` 全行扫描修复非连续重复

---

## 8. 优先级路线图

### 当前待办

| 优先级 | 目标 | 当前 | 状态 |
|--------|------|------|------|
| P1 | `daemon/internal/server` > 75% | 61.6% | 进行中 |
| P1 | `daemon/internal/agent` > 75% | 66.2% | 进行中 |
| P2 | `cli/cmd` > 80% | 70.4% | 未开始 |
| P2 | `protocol` > 90% | 83.6% | 未开始 |
| P2 | App 整体 > 50% | 35.5% | 未开始 |

### 最高风险文件 (0% 或极低覆盖)

| 文件 | 覆盖率 | 函数数 | 说明 |
|------|--------|--------|------|
| `schedule_runner.go` | 0% | 4 | 定时任务执行器 |
| `session_workspace.go` | 10% | 18 | session workspace 操作 |
| `session_agent_stream.go` | 13% | 7 | agent 流处理 |
| `provider_opencode.go` | — | 11 untested | OpenCode provider 客户端 |

### 长期目标

| 目标 | 时间线 |
|------|--------|
| Go 后端整体 > 80% | Q3 2026 |
| App 前端整体 > 50% | Q3 2026 |
| 核心路径 (agent lifecycle, session) > 90% | Q3 2026 |
| Browser test 扩展 (xterm.js 等 DOM 组件) | 按需 |
| Maestro mobile E2E 集成到 CI | 按需 |
| Visual regression testing (Playwright screenshot) | 按需 |

---

## 9. 已知问题

### Flaky 测试

- `TestBuildOpenCodeModelDefinition`: map 迭代顺序不确定, 已通过 `sort.Strings` 修复 (2026-06-12)
- `TestPiIntegration_FullPath`: 等待外部进程, 不被 `-short` 跳过, 导致完整套件超时
- `optimistic-dedup.spec.ts`: Metro 首次打包延迟偶尔导致超时, CI 配置 `retries: 1`

### 外部依赖测试

以下测试需要真实 AI 服务, CI 中跳过:
- `TestOpenCodeReasoningE2E` — 需要 `xiaomi-token-plan-cn/mimo-v2`
- `TestOpenCodeReasoningDedupE2E` — 同上
- `pi-provider-tool-use.spec.ts` — 需要本地 Pi 二进制

---

## 10. 相关文档

本文档合并了以下历史文档 (已归档):
- `docs/analysis/go-coverage-report.md` — Go 后端 TDD 详细记录 (2026-05-27/28)
- `docs/analysis/app-coverage-analysis.md` — App 前端覆盖率分析 (2026-05-28)
- `docs/architecture/backend-code-coverage.md` — Go 后端最新快照 (2026-06-12)
- `docs/analysis/test-suite-analysis.md` — 测试套件清单与 CI 集成 (2026-05-24/25)
- `docs/analysis/session-timeline-e2e-gaps.md` — Session-Timeline E2E 差距分析 (2026-05-26)
