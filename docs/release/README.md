# 发版流程（Release Process）

> **定位**：本目录是 Solo「如何发版」的唯一权威入口，覆盖构建（build）与部署（deploy）。
> 运行时部署细节（Nginx / systemd / Docker / 端口 / 排障）见 [`../architecture/deployment.md`](../architecture/deployment.md)；版本号 bump 的自动化操作见 [`.agents/skills/update-version`](../../.agents/skills/update-version/SKILL.md)。

## 通用发版流水线

无论哪个模块，发版都遵循同一条主线：

```
改版本号 ──► 更新 CHANGELOG.md ──► 提交并打 tag ──► 构建产物 ──► 部署 ──► 验证
```

1. **改版本号**：按 [versioning.md](versioning.md) 的规则 bump 受影响模块。
2. **更新 CHANGELOG.md**：聚合上一个 tag 以来的提交，见 [versioning.md](versioning.md#changelog)。
3. **提交并打 tag**：release tag 锚定 app 版本，形如 `v<x.y.z>`。构建时 `Makefile` 会用 `git describe` 把 tag 注入二进制版本。
4. **构建产物**：见各模块文档的「构建」一节。
5. **部署**：见各模块文档的「部署」一节。
6. **验证**：健康检查 / 版本号回显 / 关键路径冒烟。

## 模块索引

| 模块 | 文档 | 构建命令 | 部署方式 |
|------|------|----------|----------|
| Relay（Go） | [relay.md](relay.md) | `make solo-relay-linux-amd64` | scp 二进制 + `systemctl restart`（加固迁移用 `make deploy-solo-relay`） |
| Daemon | [daemon.md](daemon.md) | `make solo` / `make solo-linux-amd64` | 用户态 systemd（`~/.config/systemd/user/solo.service`） |
| CLI | [cli.md](cli.md) | `make solo-cli` / `make solo-cli-linux-amd64` | 拷贝二进制 |
| Mobile App | [mobile-app.md](mobile-app.md) | `eas build --profile production` | EAS Submit / 本地 APK |
| 版本规范 | [versioning.md](versioning.md) | — | 各模块版本位置、bump 规则、CHANGELOG、tag |

## 与其它文档的分工

| 关注点 | 去哪里看 |
|--------|----------|
| 如何发版（构建 + 部署步骤） | **本目录** |
| 版本号怎么改、CHANGELOG 怎么写 | [versioning.md](versioning.md) |
| 运行时拓扑、Nginx、端口、监控、排障 | [`../architecture/deployment.md`](../architecture/deployment.md) |
| 部署配置模板（env / systemd / nginx） | [`deploy/`](../../deploy/) |
| 版本 bump 的脚本化操作 | [`.agents/skills/update-version`](../../.agents/skills/update-version/SKILL.md) |
| 构建 target 全量清单 | [`Makefile`](../../Makefile) |
