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
