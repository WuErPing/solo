# Solo 深度安全分析报告

- **分析日期**：2026-07-25
- **分析范围**：全仓库（daemon, relay-go, cli, protocol, app, app-bridge）
- **分析方法**：4 路并行审计（认证与网络、注入与输入验证、密钥与供应链、DoS 与运行时）
- **关联评审**：`~/kso/review/github.com/wuerping/solo/2026-07-25_main_2032815_qodercli_qodercli/`

---

## 威胁模型前提

Solo 的安全边界是 **loopback 隔离**：daemon 绑定 `127.0.0.1:17612`，relay 是哑加密管道，信任通过 pairing offer（QR/链接）引导。因此，大多数 High 级发现的前提是攻击者已获得本地 WS 连接（恶意本地进程、CORS 配置错误的浏览器 JS、或被入侵的 relay 对端）。

---

## 发现汇总（按严重度）

### High（9 项）

| # | 发现 | 位置 | 影响 |
|---|------|------|------|
| H1 | **Daemon WS 零认证** — hello 握手无凭证，任何本地进程连接即获完全控制（agent/terminal/workspace/schedule） | `daemon/internal/server/daemon.go:664-677` | 本地提权：任意代码执行 |
| H2 | **Relay 零认证** — `serverId` 是唯一秘密，`GetOrCreate` 为任意 serverId 自动创建 session | `relay-go/internal/relay/server.go:104-148` | 知道 serverId 即可注入/中断流量 |
| H3 | **Control-socket 劫持** — 第二个 `role=server` 连接直接替换合法 daemon 的控制 socket | `relay-go/internal/relay/server.go:162-174` | 控制平面接管，DoS 所有客户端 |
| H4 | **Loop VerifyChecks 任意 shell 执行** — 客户端发送的 `verifyChecks` 字符串直接传入 `bash -c` | `daemon/internal/loop/engine.go:462` | 任意命令执行 |
| H5 | **Tmux SendKeys 任意击键注入** — 客户端可向任意 tmux pane 发送任意按键 | `daemon/internal/server/session_tmux_session.go:44` | 在目标 pane shell 中执行命令 |
| H6 | **Tmux NewSession 任意命令** — 客户端可创建运行任意命令的 tmux session | `daemon/internal/server/session_tmux_session.go:12-27` | 任意命令执行 |
| H7 | **Terminal Create 任意进程** — 客户端可指定任意 command + args + cwd 创建 PTY | `daemon/internal/server/session_terminal.go:32-53` | 任意进程启动 |
| H8 | **File Explorer 无目录限制** — 客户端可读取系统任意文件（`filepath.Clean` 不做边界限制） | `daemon/internal/server/session_fileexplorer.go:15-66` | 任意文件读取（10MB 上限） |
| H9 | **无 session/agent/terminal 数量上限** — 恶意客户端可循环创建，耗尽 goroutine 和 FD | `daemon/internal/server/daemon.go:432`, `agent/manager.go:133`, `terminal/manager.go:23` | 本地 DoS |

> **架构性观察**：H4-H8 是 **设计使然**（Solo 本质是远程终端/文件管理器）。核心缺陷不是"能执行命令"，而是 **无 per-session 授权或资源作用域**：任何已连接客户端可访问任意 terminal、任意 tmux pane、任意文件路径、执行任意命令。

### Medium（14 项）

| # | 发现 | 位置 |
|---|------|------|
| M1 | `serverId` 仅 32 位熵（4 字节随机），PID 回退可预测 | `daemon/internal/config/config.go:351-361` |
| M2 | 无前向保密 — 静态 daemon 密钥对，无轮换，泄露后所有历史/未来流量可解密 | `daemon/internal/relayclient/e2ee.go:30-60` |
| M3 | 无 daemon 身份密码学绑定 — relay 被入侵时可主动 MITM（替换握手公钥） | E2EE 握手流程 |
| M4 | Grace-period 会话劫持 — 90s（最长 15min）内，任何知道 `clientId` 的连接可恢复完整 session | `daemon/internal/server/daemon.go:720-740` |
| M5 | Origin 检查仅防浏览器 CSRF — 空 Origin 无条件接受，非浏览器客户端完全绕过 | `daemon.go:565-580`, `relay server.go:67-82` |
| M6 | API 密钥明文存储于 `~/.solo/config.json`（0600，但同用户进程可读） | `daemon/internal/config/config.go:84-91` |
| M7 | Memory turn 文件权限 0644（世界可读）— 对话内容（可能含代码/凭证）对同机所有用户可见 | `daemon/internal/memory/filebackend/recorder.go:18-21` |
| M8 | Push 通知泄露助手消息预览（220 字符）至 Expo Push API（无 E2EE） | `daemon/internal/push/notification.go` |
| M9 | `gosec` 安全 linter 未启用 — 无自动化检测硬编码凭证/弱加密/路径遍历 | `.golangci.yml` |
| M10 | GitHub Actions 未 SHA 固定 — `@v4`/`@v5` 标签可被仓库入侵后突变 | `.github/workflows/ci.yml` |
| M11 | SVG 预览正则消毒可绕过（`<foreignObject>`、CSS `url()`、无引号事件处理器），且 WebView 启用 JS + `originWhitelist=["*"]` | `app/src/components/svg-preview-utils.ts:7-15` |
| M12 | `agent_stream` 快速路径绕过 Zod 验证 — 仅 2 字段结构检查后直接 cast | `app-bridge/src/client/connection-manager.ts:943-955` |
| M13 | Relay 无 per-IP 连接限制 — 单 IP 可耗尽 10,000 全局配额；MaxConns 检查存在 TOCTOU 竞态 | `relay-go/internal/relay/server.go:121` |
| M14 | Pairing URL 无用户确认 — 恶意 deep link 可静默配对至流氓 daemon | `app/src/app/_layout.tsx:652-681` |

### Low（12 项）

| # | 发现 |
|---|------|
| L1 | Re-hello 重密钥接受任意新公钥；Go 与 TS 实现行为不一致 |
| L2 | 密钥在内存中无零化处理 |
| L3 | `/metrics` + `/api/status`（泄露 serverId）在 loopback 无认证 |
| L4 | Nginx 缺少 HSTS/CSP/X-Frame-Options 安全头 |
| L5 | 非 443 端口使用明文 `ws://`（E2EE 密文仍加密，但元数据/握手暴露） |
| L6 | Host-header script proxy 扩大 daemon HTTP 攻击面 |
| L7 | 共享/恢复的 `clientId` 静默合并 session |
| L8 | CLI 无 WS SetReadLimit — 恶意 daemon 可发超大帧致 OOM |
| L9 | Memory 模块无磁盘总量限制，无数据保留/清理机制 |
| L10 | Push token store / schedule / loop store 文件权限 0644 |
| L11 | 部署无制品签名/校验，`scp` 直接部署 |
| L12 | App 未使用 `expo-secure-store`，pairing 元数据存于普通存储；剪贴板无过期/敏感标记 |

---

## 正面发现（安全设计良好的部分）

| 领域 | 评估 |
|------|------|
| **E2EE 密码学原语** | X25519 ECDH + XSalsa20-Poly1305 实现正确；24 字节随机 nonce 每消息新生；AEAD 认证解密失败即拒绝；无时序侧信道 |
| **无自定义密码学** | 全部委托给 `golang.org/x/crypto/nacl/box`（Go）和 `tweetnacl`（TS），均为经审计的 NaCl 实现 |
| **安全随机数** | Go 用 `crypto/rand`，TS 用 `crypto.getRandomValues`（带 PRNG 可用性检查并 throw）；**全代码库零 `math/rand`** |
| **Provider CLI 参数构造** | 4 个 provider 均使用 `exec.Command(binary, args...)` 数组分离，用户 prompt 作为独立数组元素，**无 shell 注入** |
| **Git 操作** | 全部 `exec.Command("git", args...)` 数组传参，分支名/路径作为离散参数 |
| **Worktree 删除** | `filepath.Rel` + `..` 检查 + 恰好 2 段路径验证，正确限制在 `~/.solo/worktrees/` 内 |
| **项目配置写入** | `resolveKnownProjectRootForConfig` 验证路径 against 已注册项目 + `EvalSymlinks` 规范化 |
| **Mermaid 预览** | `escapeHtml()` + `textContent` 读取，XSS 安全 |
| **无 eval/Function/innerHTML** | app 应用代码中零危险 JS 执行（仅测试文件和受控 WebView） |
| **Panic 恢复** | 6 个恢复点覆盖 processLoop、agent startTurn、SafeBridge、ChannelDispatcher、sendProviderSnapshot、loop engine；SafeBridge 3 次连续 panic 后断路 30s |
| **资源清理** | WS 连接 defer Close；provider 子进程 SIGTERM→SIGKILL 升级 + Wait 回收；PTY fd 关闭 + KillAll；所有主要 map 有清理路径 |
| **无 `unsafe` 包** | 全 Go 代码库零 `unsafe.` 使用 |
| **所有 channel 有界** | Dispatcher 2560、sendQueue 10k、inboundQueue 64、turnCh 1024、CLI subscription 16 |
| **无 ReDoS** | Go 用 RE2（线性时间保证）；app 侧 `new RegExp()` 仅在测试文件 |
| **Memory 脱敏** | 默认 redactor 覆盖 OpenAI/GitHub/Anthropic/AWS token 模式 + env 文件 PASSWORD/SECRET |
| **无第三方遥测** | 零 analytics/crash-reporting SDK；唯一外部传输：Expo Push、relay（E2EE）、LLM API（用户发起） |
| **供应链** | lock file 完整；`go mod verify` 在 CI；npm overrides 固定 8 个安全关键传递依赖；CI 权限 `contents: read` 最小化 |

---

## 风险热图

```
影响 ^
  高 | [H1,H4-H8]  [H2,H3,H9]
     | (本地提权)   (relay 接管/DoS)
  中 | [M1,M4,M5]  [M2,M3,M6-M8,M11-M14]
     | (会话/认证)  (密钥/隐私/供应链)
  低 | [L1-L12]
     +------------------------>
        低          中          高   概率
```

---

## 优先整改建议（按优先级）

### L0 — 立即（下次发布前）

| # | 行动 | 工作量 | 对应发现 |
|---|------|--------|----------|
| 1 | **Daemon 增加可选 auth token** — `--auth-token` flag，WS hello 携带 Bearer token 校验；非 loopback 绑定时强制启用并 Warn 日志 | 1-2 天 | H1, M5 |
| 2 | **Relay 控制平面签名** — daemon 用 E2EE 密钥对 `serverId+role+nonce` 做 HMAC/签名注册，relay 验证；消除 serverId 知识即劫持 | 2-3 天 | H2, H3 |
| 3 | **serverId 熵提升至 ≥128 位**，移除 PID 回退 | 0.5 天 | M1 |
| 4 | **CLI 设置 `conn.SetReadLimit(16 << 20)`** | 5 分钟 | L8 |
| 5 | **Memory/push/schedule/loop 文件权限改为 0600** | 0.5 天 | M7, L10 |

### L1 — 本月

| # | 行动 | 工作量 | 对应发现 |
|---|------|--------|----------|
| 6 | **Daemon 资源上限** — maxSessions=50, maxAgents=20, maxTerminals=30，超限拒绝并返回错误 | 1 天 | H9 |
| 7 | **Relay per-IP 限流**（50/IP）+ 修复 MaxConns TOCTOU（先 Add 再检查，拒绝时 Decrement） | 1 天 | M13 |
| 8 | **启用 `gosec` linter** 并修复初始告警 | 1 天 | M9 |
| 9 | **GitHub Actions SHA 固定** | 0.5 天 | M10 |
| 10 | **Pairing 用户确认对话框** — 显示 serverId + relay endpoint，用户确认后才导入 | 0.5 天 | M14 |

### L2 — 本季度

| # | 行动 | 工作量 | 对应发现 |
|---|------|--------|----------|
| 11 | **Daemon 身份绑定（TOFU）** — daemon 在握手时签名挑战，客户端在 pairing 时固定公钥；将 E2EE 从防被动 MITM 升级为防主动 MITM | 3-5 天 | M3 |
| 12 | **SVG 消毒改用 DOMPurify**（或等效白名单库），替换正则；WebView 收紧 `originWhitelist` | 1 天 | M11 |
| 13 | **agent_stream 快速路径增加轻量 schema 校验**（关键字段类型检查，非完整 Zod） | 0.5 天 | M12 |
| 14 | **Push 通知预览改为 opt-in**，或仅发送 "Agent finished" 等无内容摘要 | 0.5 天 | M8 |
| 15 | **Memory 模块磁盘配额** — 可配置 maxTotalSize / maxTurnsPerSession，超限清理最旧 session | 1-2 天 | L9 |
| 16 | **App 采用 `expo-secure-store`** 存储 pairing 元数据和连接凭证 | 1 天 | L12 |

### L3 — 半年

| # | 行动 | 工作量 | 对应发现 |
|---|------|--------|----------|
| 17 | **前向保密 / 密钥轮换** — 每 session 临时签名密钥，或定期轮换 daemon 密钥对并通知客户端 | 5-10 天 | M2 |
| 18 | **API 密钥迁移至 OS keychain**（macOS Keychain / Linux secret-service） | 2-3 天 | M6 |
| 19 | **制品签名 + 部署校验** — CI 生成 checksum/signature，deploy 脚本验证 | 2 天 | L11 |
| 20 | **Nginx 安全头** — HSTS, X-Frame-Options, X-Content-Type-Options, CSP | 0.5 天 | L4 |

---

## 总结

Solo 的 **密码学原语实现质量高**（正确的 NaCl box 构造、安全随机数、无自定义密码学、无时序侧信道），**资源清理和 panic 隔离做得扎实**（6 个恢复点、有界 channel、子进程升级 kill），**供应链卫生良好**（lock file、go mod verify、npm overrides、最小 CI 权限）。

系统性安全差距集中在 **认证与授权层**：

1. **零认证模型** — daemon WS 和 relay 均无凭证，安全完全依赖 loopback 绑定 + Origin 检查（仅防浏览器）+ pairing QR 的真实性。
2. **无资源作用域** — 任何已连接客户端可访问任意 terminal/tmux/文件/命令，无 per-session 权限边界。
3. **密钥生命周期** — 静态 daemon 密钥无前向保密、无轮换、无身份绑定。

对于 **单用户本地工具** 的定位，这些风险在当前威胁模型下是可接受的（攻击者需已在本机）。但若未来支持多用户、企业部署、或 daemon 暴露至非 loopback 地址，L0/L1 整改是 **硬性前提**。
