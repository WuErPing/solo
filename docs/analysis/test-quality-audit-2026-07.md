# 单元测试质量审计报告（2026-07-27）

> 范围：protocol、daemon、cli、relay-go、usage(Go）与 app、app-bridge、packages/highlight(JS）全部单元测试。
> 方法：每模块深读 6-10 个代表性测试文件并核对源码，问题均带 file:line。
> 总体评级：**B+**。修复进度见文末追踪表。

## 总体结论

测试规模大、核心路径有真正的行为级测试（WS 双客户端同步、E2EE 握手、provider 真实响应 fixture），约定遵守良好（表驱动、`t.TempDir`/`t.Setenv`、store 隔离）。问题集中在四个跨模块主题：wall-clock sleep（含"永绿"用例）、空断言/调试型测试、测试基建重复、关键覆盖缺口。

## 分模块评级

| 模块 | 评级 | 规模 | 主要短板 |
|---|---|---|---|
| protocol + daemon | B+ | 176 测试文件 / 1352 测试函数；coverage protocol ≈80%、daemon ≈67% | sleep 时序耦合（173 处）、空断言、provider `!short` 误伤 |
| cli + relay-go + usage | B+ | — | e2ee sleep、mock daemon 三份重复、loop_* 零覆盖 |
| app (vitest) | B+ | 269 测试文件（hooks 43、stores 16、utils 80、screens 29、components 46） | 永绿用例、巨型组件覆盖空洞、基建重复 |
| app-bridge + highlight | B+ | 21 测试文件 / 193 用例 | connection-manager 盲区、CRLF/Unicode 边界缺失 |

## 亮点（值得作为范本）

- `daemon/internal/schedule/cron_test.go` — 表驱动 + 固定时钟 + 区间断言。
- `daemon/internal/agent/coalescer_clock_test.go` — fakeClock 注入 `AfterFunc`，定时器逻辑确定性测试。
- `daemon/internal/server/handler_integration_test.go` / `multi_client_sync_test.go` — httptest + 真实 WS 双客户端验证行为。
- `app-bridge/src/relay/encrypted-channel.test.ts` — 全仓库最佳：真实密钥派生握手、fake timers、缓冲重放、表驱动非法输入。
- `app-bridge/src/client/speedtest.test.ts` — 分段延迟数学断言精确。
- `app/src/hooks/use-tmux-capture-pane.test.ts:725-762` — 自适应轮询抽纯函数 `computeAdaptiveInterval(now)` 断言 2000/2001ms 边界。
- `app/src/components/svg-preview.test.ts` — 安全逻辑正反向断言（XSS 移除 + 合法保留）。
- `usage/provider/qoder/qoder_test.go` — 生产真实响应 fixture + 请求头/path/分页断言。
- `usage/cmd/init_test.go` — 副作用断言严谨（0600 权限、`--print` 不落盘）。

## 问题清单

### 主题 1：wall-clock sleep（flaky / 永绿）

| 严重度 | 位置 | 问题 |
|---|---|---|
| high | `app/src/hooks/use-tmux-capture-pane.test.ts:418-422` | **无法失败的测试**：暂停轮询用 100ms sleep 断言，轮询周期 200ms，逻辑失效也不红 |
| high | `daemon/internal/agent/manager_subscribe_test.go:101` | 5s sleep 等 350 个异步事件 |
| high | `relay-go/internal/e2ee/channel_test.go:164,225,288,349,427` | sleep 20–50ms 后直接断言异步投递 |
| high | `relay-go/internal/e2ee/channel_test.go:357,560` | 真实 sleep 1.1s/1.5s 验证 hello 重试 |
| medium | `daemon/internal/agent/coalescer_window_test.go:26`、`coalescer_reasoning_window_test.go:33`、`coalescer_reasoning_test.go:30` | 已有 fakeClock 仍用 2.2–2.5s 真实 sleep（共 ~7s） |
| medium | `app/src/hooks/use-tmux-capture-pane.test.ts:284,303,322,388,568` | 多处真实 10–100ms sleep（同文件后半已用 fake timers） |
| medium | `app/src/runtime/host-runtime.test.ts:485-500,1498,1546,1620,1641,1689,1783,1823` | 手写 `Date.now()` 自旋轮询，上限 200–300ms，高负载 CI 假失败 |
| medium | `app-bridge/src/client/speedtest.test.ts:76-85` | 真实 15ms sleep + `>=10` 断言 |
| low | `daemon/internal/loop/engine_test.go:145`、`schedule/executor_test.go:46` | `EveryMs:1` + 真实 ticker |
| low | `relay-go/internal/relay/server_extra_test.go:136,161,235,503` | 负向断言固定 400ms 窗口 |

### 主题 2：空断言 / 调试型测试（虚假安全感）

| 严重度 | 位置 | 问题 |
|---|---|---|
| high | `daemon/internal/server/session_tmux_debug_test.go:10-44` | printf 调试伪装成测试，依赖宿主机 tmux，无 `LookPath` 守卫 |
| high | `daemon/internal/server/session_grace_test.go:181-186` | 算出 `msgCount` 后 `_ = msgCount`，零断言 |
| high | `daemon/internal/server/session_grace_test.go:256-301` | 注释自认"may be a no-op"，订阅保留未真正断言 |
| high | `cli/cmd/root_test.go:420-424` | `_ *testing.T`，只 SetArgs 不执行，永久绿 |
| medium | `cli/cmd/cmd_extra_test.go:1463-1501` | `err == nil` 时仅 `t.Logf`，行为对错都通过 |
| medium | `relay-go/internal/config/config_test.go:151-159` | 测试名说 trim，断言空格被保留——把疑似 bug 固化成契约（实现 `splitAndTrim` 名不副实） |
| medium | `daemon/internal/relayclient/client_extra_test.go:92-114` | 多个 `_ *testing.T` 零断言用例 |
| medium | `daemon/internal/memory/redactor_test.go:13-16` | 编译期断言冒充运行时测试；测试体内 `t.Helper()` 无效 |
| medium | `app/src/screens/workspace/workspace-screen.test.tsx:1-395` | 350 行 mock（连纯函数都 mock）换一个 `renders without crashing` |
| low | `app/src/voice-removal-absence.test.ts:38-40` | 文件不存在时 `expect(true).toBe(true)` 空断言 |
| low | `relay-go/internal/e2ee/parity_test.go:13-51` | 读源码做字符串检查，`:45-50` 条件自相矛盾恒为假 |

### 主题 3：测试基建重复

| 严重度 | 位置 | 问题 |
|---|---|---|
| medium | `cli/cmd/root_test.go:22-114`、`cmd_extra_test.go:225-415`、`cli/internal/client/client_test.go:105-202` | mock daemon 三份重复，协议变更改三处 |
| medium | `cli/cmd` ~20 个测试 | 全局 flag 样板重写且不还原（`cmd_extra_test.go:947` 设后不恢复），顺序依赖 |
| medium | app 25 个测试文件 | 各自复制 50+ 行 theme mock；16 个文件手写 JSDOM+createRoot setup |
| medium | app-bridge 7 个 RPC 测试文件 | `findSentMessage` 逐字重复；harness 的 `extractRequestId` 无人引用 |
| low | `daemon/internal/agent/manager*_test.go` | 各自重复 ~20 个方法的 AgentSession no-op mock |

### 主题 4：环境/隔离

| 严重度 | 位置 | 问题 |
|---|---|---|
| high | `daemon/internal/agent/providers/opencode/server_test.go:41-50` | `os.Setenv` + 手工备份恢复整个 `os.Environ()`，与 -race/parallel 不兼容 |
| medium | `cli/internal/client/host_test.go:23,51,310` | `os.Unsetenv("SOLO_LISTEN")` 不还原，泄漏到后续测试（全仓 `os.Setenv` 84 处 vs `t.Setenv` 9 处） |
| low | `relay-go/internal/e2ee/channel_test.go:26-39` | mockTransport 投递 goroutine 永不退出 |

## 覆盖缺口（按重要性）

1. **provider 客户端 short 模式近零覆盖**：kimi/pi 纯单测被 `//go:build !short` 误伤（kimi/client.go 0%、pi 0%、opencode 0%、codex 13.5%、claude 34.5%）；契约套件 `contracttest.RunProviderContractSuite` short 不执行。合计 ~2500 行。
2. **`app-bridge/src/client/connection-manager.ts`（1295 行）**：重连/退避/错误处理零测试（所有用例 `reconnect: disabled`）。
3. **loops 功能整链无测试**：`use-loops.ts`、`use-loop-mutations.ts`、`use-create-loop.ts`、`use-loop-inspect.ts`、`use-loop-templates.ts` + 4 个 screen；schedules 同构功能测试完整，明显不对称。
4. **`app/src/hooks/use-git-actions.ts`（575 行）无测试**。
5. **`cli/cmd/loop_*.go` 全部命令零测试**（enhancedMockDaemon 可复用）。
6. **tmux 扫描/解析缺纯单测**：`session_tmux_scan.go`（509 行）等主要靠宿主机集成测试（session_tmux.go 12.3%）；应用 fixture 测解析。
7. **`relay-go/internal/relay/buffer.go` FrameBuffer 溢出淘汰/重缓冲分支未测**（防内存膨胀关键逻辑）；连接上限 maxConns 路径未触发。
8. **大型组件接线层无测试**：`git-diff-pane.tsx`（2722 行）、`agent-stream-view.tsx`（1368 行）、`file-explorer-pane.tsx`（1218 行）。
9. 小缺口：`usage/cmd/fetch.go` 并发聚合未测；`app-bridge` 的 `daemon-endpoints.ts`、`tool-name-normalization.ts`、`terminal-stream-protocol.ts`、`daemon-client-runtime-metrics.ts` 零测试；highlight 缺 CRLF/Unicode 用例、parsers 全映射遍历；protocol `MsgType` 方法群 0%（可反射遍历兜底）。

## 修复建议（按收益排序，即本轮修复顺序）

1. 解开 provider 测试的 `!short` 误伤（消灭最大盲区）。
2. 消灭 wall-clock sleep：长 sleep 改条件轮询/channel/fake timers/`vi.waitFor`，修掉永绿用例。
3. 清理空断言与调试测试（含 relay-go config trim 名不副实的实现/测试修正）。
4. 为 connection-manager 补重连测试（MockTransport + fake timers）。
5. 合并测试基建：mock daemon 单点化、`resetFlags(t)`、app `src/test/` 共享 mock、app-bridge harness 统一导出。
6. 补 loops 对称覆盖（基建先行，按 schedules 结构）。

## 修复追踪

| # | 项 | 状态 | 备注 |
|---|---|---|---|
| 1 | provider `!short` 误伤 | ✅ | 移除 `daemon/internal/agent/providers/kimi/client_test.go` 与 `pi/client_test.go` 的文件级 `//go:build !short && !external_api`；契约套件仍由 `testing.Short()` 跳过；顺手将 `opencode/server_test.go` 的 `os.Setenv` 备份恢复改为 `t.Setenv` |
| 2 | wall-clock sleep | ✅ | 已处理：coalescer 三文件改用 fakeClock；manager_subscribe 移除 5s 固定 sleep 并降 worker delay 至 2ms；relay-go e2ee channel 注入 retry interval 并改同步 WaitGroup/轮询；use-tmux-capture-pane 改用 fake timers + cleanup() 解决隔离，消除永绿用例与 10–100ms sleep；speedtest 改用 fake timers；loop engine 与 schedule executor 移除创建/到期等待 sleep。剩余低优先级：host-runtime.test.ts 自旋轮询、relay-go server_extra_test.go 400ms nudge 窗口 |
| 3 | 空断言/调试测试 | ✅ | 已处理：删除 `session_tmux_debug_test.go` 调试测试；修复 `session_grace_test.go` 零断言；`root_test.go` 真正执行命令；`cmd_extra_test.go` 两个 follow 测试改为明确断言；`relay-go/config` 修复 `splitAndTrim` 实现与测试；`relayclient/client_extra_test.go` 多个 `_ *testing.T` 改为实际断言并迁移 `E2EEConn` 编译期断言到 `e2ee.go`；`memory/redactor_test.go` 编译期断言迁移到 `redactor.go`；`e2ee/parity_test.go` 移除恒为假的 legacy 检查。剩余低优先级：workspace-screen.test.tsx 350 行 mock、voice-removal-absence.test.ts 空 true-true 分支 |
| 4 | connection-manager 重连测试 | ✅ | 新增 `app-bridge/src/client/connection-manager.test.ts`：验证 reconnect 定时触发、退避递增、关闭 reconnect 后不再重连；使用 MockTransport + fake timers + Math.random mock |
| 5 | 测试基建合并 | ✅ | CLI: 新增 `cli/cmd/testutil_test.go` 的 `resetFlags(t)` 与共享 `setupTestCLI/setupEnhancedCLI`；新建 `cli/internal/clienttest/mock_daemon.go` 统一 mock daemon，替换 `root_test.go`、`cmd_extra_test.go`、`cli/internal/client/client_test.go` 三处重复实现。剩余低优先级：app theme mock 重复、app-bridge `findSentMessage` 重复 |
| 6 | loops 覆盖 | ✅ | 按 schedules 结构新增 hooks 测试：`use-loops.test.ts`、`use-loop-inspect.test.ts`、`use-loop-mutations.test.ts`、`use-create-loop.test.ts`、`use-loop-templates.test.ts`，合计 33 个用例 |
