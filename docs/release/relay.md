# Relay 发版（solo-relay）

> Relay 是部署在腾讯云（`tencent_gz_6` / `solo.up2ai.top`）上的 Go WebSocket 中继服务。
> 运行时拓扑、Nginx、端口、监控与排障见 [`../architecture/deployment.md`](../architecture/deployment.md)；加固配置模板见 [`deploy/`](../../deploy/)。

## 生产现状（务必先读）

当前生产主机跑的是**旧版部署**，与 [`deploy/`](../../deploy/) 里的加固模板**尚不一致**：

| 项目 | 生产实况 | `deploy/` 加固目标（尚未应用） |
|------|----------|-------------------------------|
| 监听端口 | `8081`（`HOST=0.0.0.0`，安全组挡公网） | `8080`（`HOST=127.0.0.1`） |
| 运行用户 | `ubuntu` | 专用用户 `solo-relay` |
| 环境配置 | unit 内联 `Environment=` | `EnvironmentFile=/opt/solo-relay/solo-relay.env` |
| systemd 硬化 | 无 | `NoNewPrivileges` / `ProtectSystem=strict` 等 |
| Nginx 上游 | `proxy_pass http://localhost:8081` | `127.0.0.1:8080` |

> **⚠️ 关键警告**：`make deploy-solo-relay` 会把 [`deploy/systemd/solo-relay.service`](../../deploy/systemd/solo-relay.service)（加固模板）推送到生产并重启。该模板依赖 `solo-relay` 用户与 `/opt/solo-relay/solo-relay.env`，而**当前生产主机两者都不存在**，直接执行会导致 relay 起不来、Daemon 断连。
>
> 因此在完成下面的[加固迁移](#加固迁移)之前，**更新生产二进制请只 scp + restart，不要套用 deploy/ 的 unit**：
> ```bash
> make solo-relay-linux-amd64
> scp output/linux/solo-relay tencent_gz_6:/opt/solo-relay/solo-relay
> ssh tencent_gz_6 "chmod +x /opt/solo-relay/solo-relay && sudo systemctl restart solo-relay"
> ```

## 构建

```bash
# Linux AMD64（生产）
make solo-relay-linux-amd64
# 等价于：GOOS=linux GOARCH=amd64 go build -ldflags "$(GO_LDFLAGS)" -o output/linux/solo-relay ./relay-go/cmd/relay

# Darwin ARM64（本地）
make solo-relay
```

产物：`output/linux/solo-relay`。

> Relay 的版本是 `relay-go/internal/relay/server.go` 里的 `const version`（当前 `relay-go-v1`），与 daemon/cli 的 `-ldflags` 注入不同，它是源码常量，发版前按需手动更新（见 [versioning.md](versioning.md)）。

## 部署（当前生产方式）

只更新二进制并重启，沿用线上已有的旧版 unit：

```bash
make solo-relay-linux-amd64
scp output/linux/solo-relay tencent_gz_6:/opt/solo-relay/solo-relay
ssh tencent_gz_6 "chmod +x /opt/solo-relay/solo-relay && sudo systemctl restart solo-relay"
```

## 验证

```bash
# 服务状态
make relay-status

# 健康检查（服务器本地，生产端口 8081）
ssh tencent_gz_6 "curl -s http://localhost:8081/health"
# {"status":"ok","sessions":1,"connections":3,"version":"relay-go-v1"}

# 公网（经 Nginx 443）
curl -s -H "Host: solo.up2ai.top" https://solo.up2ai.top/health
```

`sessions: 1` 表示有 Daemon 已连接；`sessions: 0` 表示尚无 Daemon 连接。

## 回滚

部署前备份上一版二进制，出问题即回滚：

```bash
ssh tencent_gz_6 "cp /opt/solo-relay/solo-relay /opt/solo-relay/solo-relay.prev"
# 回滚：
ssh tencent_gz_6 "cp /opt/solo-relay/solo-relay.prev /opt/solo-relay/solo-relay && sudo systemctl restart solo-relay"
```

## 加固迁移

把生产从旧版（8081/ubuntu）迁移到 [`deploy/`](../../deploy/) 的加固模板（8080/solo-relay）。**有短暂停机**，需 Daemon 随后重连。按顺序执行：

1. 创建专用用户与目录：
   ```bash
   ssh tencent_gz_6 "sudo useradd --system --home /opt/solo-relay --shell /usr/sbin/nologin solo-relay && sudo mkdir -p /opt/solo-relay && sudo chown solo-relay:solo-relay /opt/solo-relay"
   ```
2. 安装环境文件（模板 [`deploy/solo-relay.env.example`](../../deploy/solo-relay.env.example)，`PORT=8080`、`HOST=127.0.0.1`、`ALLOWED_ORIGINS=https://solo.up2ai.top`）：
   ```bash
   scp deploy/solo-relay.env.example tencent_gz_6:/tmp/solo-relay.env
   ssh tencent_gz_6 "sudo mv /tmp/solo-relay.env /opt/solo-relay/solo-relay.env && sudo chown solo-relay:solo-relay /opt/solo-relay/solo-relay.env"
   ```
3. 改 Nginx 上游到 8080（模板 [`deploy/nginx/solo-relay.conf`](../../deploy/nginx/solo-relay.conf)）并 `sudo nginx -t && sudo systemctl reload nginx`。
4. 用 `make deploy-solo-relay` 推送加固版二进制 + unit 并重启（此时前置条件已满足）。
5. 验证：`ssh tencent_gz_6 "curl -s http://localhost:8080/health"`，并确认 Daemon 重连（`sessions >= 1`）。

> 迁移完成后，请同步更新 [`../architecture/deployment.md`](../architecture/deployment.md) 的「生产现状」描述，并把 `Makefile` 的 `SOLO_RELAY_PORT/SOLO_RELAY_NGINX_PORT` 默认值与实际端口统一。

## 相关链接

- 版本与 CHANGELOG：[versioning.md](versioning.md)
- 运行时部署 / Nginx / 排障：[`../architecture/deployment.md`](../architecture/deployment.md)
- Node.js 版 relay（paseo-relay，**已弃用并下线**）：`make solo-relay-nodejs` / `make solo-relay-nodejs-docker`
