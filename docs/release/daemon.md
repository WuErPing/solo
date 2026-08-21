# Daemon 发版（solo）

> Daemon 是运行在用户主机上的本地服务（默认监听 `127.0.0.1:17612`），通常以用户态 systemd 常驻。

## 构建

```bash
# Darwin ARM64（本地）
make solo            # → output/darwin/solo

# Linux AMD64
make solo-linux-amd64  # → output/linux/solo
```

版本由 `Makefile` 经 `-ldflags -X .../daemon/internal/config.Version=$(VERSION)` 注入，`$(VERSION)` 取自 `git describe`（dev 构建为 `{tag}-dev-{datetime}{-dirty}`）。源码回退值在 `daemon/internal/config/config.go` 的 `var Version`。因此**发版前先打 tag**（见 [versioning.md](versioning.md)），二进制才会带上正确版本。

验证注入结果：

```bash
./output/darwin/solo --version
```

## 部署（用户态 systemd）

安装二进制并启用用户服务（`~/.config/systemd/user/solo.service`）：

```bash
mkdir -p ~/.solo/bin
cp output/darwin/solo ~/.solo/bin/solo

systemctl --user daemon-reload
systemctl --user enable solo
systemctl --user restart solo
systemctl --user status solo
```

unit 示例见 [`../architecture/deployment.md`](../architecture/deployment.md#daemon-部署)。

## 部署（supervised 模式）

Daemon 的首选运行方式是由 **solo-supervisor**（`supervisor/`）拉起并看护：

- **看门狗**：supervisor 负责 spawn / respawn daemon，按退出码决定行为——`42` = 主动重启、`0` = 干净停止、其它 = 崩溃（退避 + 熔断）。
- **版本目录**：可切换的 daemon 构建放在 `~/.solo/versions/solo-*`，`~/.solo/versions/current` 指针指向当前激活的构建。
- **开发环境**：`make restart` 构建并把 `solo-$(VERSION)` 发布进 `~/.solo/versions/`、更新 `current` 指针，然后启动 solo-supervisor。
- **App 触发**：host 页面的 Operations（Restart daemon / Daemon version picker）通过退出码契约让 supervisor 完成重启或版本切换；daemon 直接运行（非 supervised）时这些操作返回 `NOT_SUPERVISED`。
- **崩溃兜底**：若某个构建反复崩溃，crash-breaker 会将其拉黑并把 `current` 指针改写回可用的旧版本。
- **所有权**：supervised 模式下 `~/.solo/solo.pid` 与 `~/.solo/logs/daemon.log` 由 supervisor 管理。

细节见 [`../architecture/daemon-supervision.md`](../architecture/daemon-supervision.md) 与 [ADR-003](../decisions/adr-003-supervisor-exit-code-restart-contract.md) / [ADR-004](../decisions/adr-004-daemon-version-switching.md) / [ADR-005](../decisions/adr-005-supervisor-crash-fallback.md)。

> systemd 仍可直跑 `solo`（见上一节），也可以把 unit 的 `ExecStart` 指向 **solo-supervisor**，由 systemd 管 supervisor、supervisor 管 daemon。

## 配置 Relay 连接

```bash
make use-solo-relay
# 或手动编辑 ~/.solo/config.json：
# {"daemon":{"relay":{"enabled":true,"endpoint":"solo.up2ai.top:443","publicEndpoint":"solo.up2ai.top:443"}}}
```

> `relay.endpoint` 必须用**域名 + 443**，不能用裸 IP + 8081/8080（Relay 仅监听本地，外网走 Nginx 反代）。详见 [`../architecture/deployment.md`](../architecture/deployment.md)。

## 验证

```bash
curl http://localhost:17612/api/health
# {"status":"ok","timestamp":"..."}

# 确认已连上 Relay
ssh tencent_gz_6 "curl -s http://localhost:8080/health"   # sessions >= 1
```
