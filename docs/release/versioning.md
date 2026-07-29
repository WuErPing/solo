# 版本规范（Versioning）

> Solo 由多个**独立版本化**的模块组成。本文规定各模块版本号的存放位置、bump 规则、CHANGELOG 写法与 release tag 约定。
> 脚本化操作见 [`.agents/skills/update-version`](../../.agents/skills/update-version/SKILL.md)（含 `check-versions.sh`、`bump-version.sh`）。

## 约定

- 语义化版本 [SemVer](https://semver.org/lang/zh-CN/)：`MAJOR.MINOR.PATCH`。
- CHANGELOG 遵循 [Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/)。
- Release tag 锚定 **app** 版本，形如 `v0.11.0`。

## 各模块版本位置

| 模块 | 版本存放位置 | 当前值 | 说明 |
|------|--------------|--------|------|
| app | `app/package.json` → `version` | `0.11.0` | release tag 锚点 |
| app-bridge | `app-bridge/package.json` → `version` | `0.5.0` | |
| highlight | `packages/highlight/package.json` → `version` | `0.2.0` | |
| daemon | `daemon/internal/config/config.go` → `var Version` | `0.5.0` | 构建时经 `-ldflags` 覆盖 |
| cli | `cli/internal/config/version.go` → `var Version` | `dev` | 构建时经 `-ldflags` 覆盖 |
| relay-go | `relay-go/internal/relay/server.go` → `const version` | `relay-go-v1` | 字符串常量，非 SemVer |
| protocol | `protocol/protocol.go` → `WSProtocolVersion` / `RelayProtocolVersion` | `2` / `"2"` | 仅 wire 格式变更时递增 |

> **构建期版本注入**：`Makefile` 用 `git describe --tags` 得到 tag，dev 构建为 `{tag}-dev-{datetime}{-dirty}`，再经
> `-ldflags -X .../daemon/internal/config.Version=$(VERSION)`（cli 同理）注入。源码里的字面量只是 dev 回退值，**release 二进制的版本由 git tag 决定**。

## Bump 分级规则

对每个有改动的模块，取最高级别的变更：

| 变更类型 | Semver | 例子 |
|----------|--------|------|
| 破坏性 API / 协议变更 | **MAJOR** | 修改消息结构体、改 CLI flag、删除导出函数、wire 格式变更 |
| 新功能 / 增量变更 | **MINOR** | 新 provider、新 CLI 命令、新协议消息类型 |
| Bug 修复 / 内部改进 | **PATCH** | 竞态修复、错误处理修正、性能优化 |
| 仅文档 / 测试 / CI | **NONE** | 跳过 bump |

> **protocol 变更是特殊情况**：任何对 `protocol/` 消息结构体或常量的改动，对跨版本兼容而言都是 breaking，需要 `MAJOR`（或至少 `MINOR` 并同步递增协议版本常量）。

### 跨模块联动

当 `protocol/` 常量变更时，**所有消费方都要更新**：

1. 递增 `protocol/protocol.go` 的 `WSProtocolVersion` 或 `RelayProtocolVersion`
2. bump `daemon/internal/config/config.go`（daemon 是协议服务端）
3. bump `cli/internal/config/version.go`（CLI 与 daemon 配对）
4. bump `app/package.json`（app 是协议客户端）
5. 跑测试验证兼容：`make darwin && go test -short -race ./... && cd app && npm test`

## CHANGELOG

每次发版在 `CHANGELOG.md` 顶部新增一个 `## [x.y.z] - YYYY-MM-DD` 小节，聚合上一个 tag 以来的提交。CHANGELOG 改动与版本 bump **同一个 commit** 提交。

确定目标版本与上一个 tag：

```bash
PREV_TAG=$(git describe --tags --abbrev=0)
TARGET_VERSION=$(grep '"version"' app/package.json | head -1 | sed 's/.*"\([0-9.]*\)".*/\1/')
git log --pretty=format:"- %s" "$PREV_TAG"..HEAD
```

按 conventional-commit 类型归入 Keep a Changelog 小节：

| Commit 类型 | CHANGELOG 小节 |
|-------------|----------------|
| `feat` | **Added** |
| `fix` | **Fixed** |
| `refactor` / `perf` | **Changed** |
| `revert` / `remove` | **Removed** |
| `docs` / `test` / `chore` / `build` / `ci` / `style` | 省略（非用户可见） |

每条格式为 `- **<Scope>**: <描述>`（Scope 即括号内的范围，如 Daemon / Protocol / App）。仅保留有内容的小节。

## Release tag

```bash
git tag -a v0.11.0 -m "release: v0.11.0"
git push origin v0.11.0
```

打完 tag 后再执行各模块的构建/部署，`Makefile` 才能把正确版本注入二进制。

## 验证

```bash
# npm
grep -h '"version"' app/package.json app-bridge/package.json packages/highlight/package.json

# Go
grep -n 'var Version' daemon/internal/config/config.go cli/internal/config/version.go
grep -n 'const version' relay-go/internal/relay/server.go
grep -n 'ProtocolVersion' protocol/protocol.go
```
