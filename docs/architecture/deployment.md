# Deployment Architecture

> **Note**: 发版的构建与部署步骤（如何 cut a release）见 [`../release/`](../release/README.md)。本文聚焦**运行时部署与排障**：拓扑、Nginx、systemd、Docker、端口、监控、故障排除。
>
> Deployment configuration templates are tracked in [`deploy/`](../../deploy/) (Nginx: `deploy/nginx/solo-relay.conf`, Systemd: `deploy/systemd/solo-relay.service`, env: `deploy/solo-relay.env.example`). Relay is deployed via `make deploy-solo-relay` (scp + systemctl restart). Mobile builds use EAS (Expo Application Services).

## Table of Contents

- [Development Environment](#development-environment)
- [Production Environment](#production-environment)
- [Port Mapping](#port-mapping)
- [Relay Deployment](#relay-deployment)
- [Nginx Configuration](#nginx-configuration)
- [Systemd Service](#systemd-service)

## Development Environment

### Local Development

```
┌─────────────────────────────────────┐
│           Dev Machine                │
│  ┌─────────┐    ┌─────────────────┐ │
│  │ App     │───►│ Daemon:17612    │ │
│  │ (Web)   │    │ (Local Dev)     │ │
│  └─────────┘    └─────────────────┘ │
└─────────────────────────────────────┘
```

**Start Commands**:
```bash
# 1. Start Daemon
make run-daemon

# 2. Start Web App
make dev-web

# Or start all at once
make dev
```

### Configuration

**~/.solo/config.json**:
```json
{
  "daemon": {
    "listen": "127.0.0.1:17612",
    "cors": {
      "origins": ["https://solo.up2ai.top", "http://localhost:19000"]
    }
  }
}
```

## Production Environment

### Full Architecture

```
┌─────────────────────────────────────────────────────────┐
│                      Public Internet                     │
│  ┌─────────┐    ┌─────────┐    ┌─────────────────────┐ │
│  │ User    │───►│  Nginx  │───►│ Relay Server:8081   │ │
│  │         │    │ (SSL)   │    │ (Tencent Cloud)     │ │
│  └─────────┘    └─────────┘    └──────────┬──────────┘ │
│                                            │            │
└────────────────────────────────────────────┼────────────┘
                                             │
                              ┌──────────────┼──────────────┐
                              │              │              │
                              ▼              ▼              ▼
                        ┌─────────┐   ┌─────────┐   ┌─────────┐
                        │ Daemon 1│   │ Daemon 2│   │ Daemon N│
                        │ (User A)│   │ (User B)│   │ (User N)│
                        └─────────┘   └─────────┘   └─────────┘
```

### Actual Deployment Info

| Item | Value |
|------|-----|
| **Server** | Tencent Cloud Guangzhou |
| **Hostname** | tencent_gz_6 |
| **Public IP** | 106.52.40.152 |
| **Private IP** | 172.16.0.2 |
| **OS** | Ubuntu 22.04 LTS |
| **Domain** | solo.up2ai.top |
| **SSL Certificate** | Let's Encrypt (Certbot) |

## Port Mapping

| 端口 | 服务 | 监听地址 | 说明 | 访问控制 |
|------|------|----------|------|----------|
| 80 | Nginx | 0.0.0.0 | HTTP 重定向到 HTTPS | 公网开放 |
| 443 | Nginx | 0.0.0.0 | HTTPS + WebSocket 代理 | 公网开放 |
| 8081 | Solo Relay (Go) | 0.0.0.0 | Relay WebSocket 服务 | 腾讯云安全组放行内网/本机，公网不可直达 |
| 17612 | Daemon | 127.0.0.1 | Daemon 本地服务 | 仅本地 |

**⚠️ 重要**: 生产环境 Solo Relay 监听 `8081`。虽然进程绑定在 `0.0.0.0`，但腾讯云**安全组**未放行 8081 公网入站，外部无法直接访问，所有外部连接必须通过 Nginx (443端口) 反向代理。

> **现状 vs 加固目标**：上述为当前生产实况（8081、`User=ubuntu`、内联环境变量）。[`deploy/`](../../deploy/) 中的模板描述的是**尚未应用的加固目标**（`127.0.0.1:8080`、专用用户 `solo-relay`、`EnvironmentFile`、systemd 硬化项）。迁移步骤见 [`../release/relay.md`](../release/relay.md)。

对应的域名解析路径：

```
solo.up2ai.top ──► 106.52.40.152 (腾讯云)
                      │
                      ├──► :443 ──► Nginx (SSL) ──► Relay (localhost:8081)
                      │
                      └──► :80  ──► Nginx (重定向到 443)
```

## Relay 部署

### 二进制部署

> 完整的构建与部署步骤（含验证、回滚、加固迁移）见 [`../release/relay.md`](../release/relay.md)。此处仅说明运行时服务配置。

**构建与部署**（摘要）:
```bash
make solo-relay-linux-amd64   # 构建 → output/linux/solo-relay
make deploy-solo-relay        # scp 二进制 + systemd unit，daemon-reload，restart
```

> **⚠️ 注意**：`make deploy-solo-relay` 会推送 [`deploy/systemd/solo-relay.service`](../../deploy/systemd/solo-relay.service)（加固模板，依赖 `solo-relay` 用户与 `/opt/solo-relay/solo-relay.env`）。当前生产主机**尚未满足这些前置条件**，直接执行会导致服务起不来。迁移前先按 [`../release/relay.md`](../release/relay.md#加固迁移) 准备前置条件。

**当前生产 Systemd 服务**（`/etc/systemd/system/solo-relay.service`，实况）:
```ini
[Unit]
Description=Solo Relay Server (Go)
After=network.target

[Service]
Type=simple
User=ubuntu
Group=ubuntu
WorkingDirectory=/opt/solo-relay
ExecStart=/opt/solo-relay/solo-relay
Environment=PORT=8081
Environment=HOST=0.0.0.0
Environment=MAX_BUFFER=200
Environment=LOG_LEVEL=info
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
```

**加固目标模板**（[`deploy/systemd/solo-relay.service`](../../deploy/systemd/solo-relay.service)，尚未应用）：专用用户 `solo-relay`、`EnvironmentFile=/opt/solo-relay/solo-relay.env`（`PORT=8080`、`HOST=127.0.0.1`、`ALLOWED_ORIGINS=https://solo.up2ai.top`，模板见 [`deploy/solo-relay.env.example`](../../deploy/solo-relay.env.example)）、`Restart=on-failure`、`LimitNOFILE=65536`，并带一组安全硬化项（`NoNewPrivileges`、`ProtectSystem=strict`、`MemoryDenyWriteExecute` 等）。

**启动服务**:
```bash
sudo systemctl enable solo-relay
sudo systemctl start solo-relay
sudo systemctl status solo-relay
```

### Docker 部署

**Dockerfile**:
```dockerfile
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o solo-relay ./relay-go/cmd/relay

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/solo-relay .
CMD ["./solo-relay"]
```

**构建**:
```bash
docker build -t solo-relay:latest .
```

**运行**:
```bash
docker run -d \
  -p 8080:8080 \
  -e PORT=8080 \
  -e HOST=0.0.0.0 \
  -e MAX_BUFFER=200 \
  --name solo-relay \
  solo-relay:latest
```

## Nginx 配置

### 实际配置 (solo.up2ai.top)

```nginx
# /etc/nginx/sites-enabled/solo.up2ai.top

# HTTP 重定向到 HTTPS
server {
    listen 80;
    server_name solo.up2ai.top;
    return 301 https://$server_name$request_uri;
}

# HTTPS + WebSocket
server {
    listen 443 ssl;
    server_name solo.up2ai.top;
    
    # Let's Encrypt SSL
    ssl_certificate /etc/letsencrypt/live/solo.up2ai.top/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/solo.up2ai.top/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;

    # 所有请求代理到 Relay
    location / {
        proxy_pass http://localhost:8081;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

### 通用配置模板

```nginx
upstream solo_relay {
    server 127.0.0.1:8080;
    keepalive 32;
}

server {
    listen 80;
    server_name relay.example.com;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name relay.example.com;
    
    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    
    location / {
        proxy_pass http://solo_relay;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 86400;
        proxy_send_timeout 86400;
    }
    
    location /health {
        proxy_pass http://solo_relay/health;
        access_log off;
    }
}
```

### 负载均衡配置

```nginx
upstream solo_relay_cluster {
    least_conn;
    
    server 10.0.1.10:8080 weight=5;
    server 10.0.1.11:8080 weight=5;
    server 10.0.1.12:8080 backup;
    
    keepalive 32;
}

server {
    listen 443 ssl http2;
    server_name relay.example.com;
    
    location / {
        proxy_pass http://solo_relay_cluster;
        proxy_http_version 1.1;
        
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        
        proxy_read_timeout 86400;
        proxy_send_timeout 86400;
    }
}
```

## Daemon 部署

### 本地部署

**Systemd 服务** (`~/.config/systemd/user/solo.service`):
```ini
[Unit]
Description=Solo Daemon
After=network.target

[Service]
Type=simple
ExecStart=%h/.solo/bin/solo
Restart=always
RestartSec=5

[Install]
WantedBy=default.target
```

**启动**:
```bash
systemctl --user enable solo
systemctl --user start solo
systemctl --user status solo
```

### 配置 Relay 连接

```bash
# 使用 Makefile
make use-solo-relay

# 或手动配置
cat > ~/.solo/config.json << 'EOF'
{
  "daemon": {
    "relay": {
      "enabled": true,
      "endpoint": "solo.up2ai.top:443",
      "publicEndpoint": "solo.up2ai.top:443"
    }
  }
}
EOF
```

## Pairing Link 配置

### 完整配置示例

**~/.solo/config.json**:
```json
{
  "daemon": {
    "listen": "127.0.0.1:17612",
    "hostnames": ["localhost"],
    "cors": {
      "origins": ["http://localhost:19000"]
    },
    "relay": {
      "enabled": true,
      "endpoint": "solo.up2ai.top:443",
      "publicEndpoint": "solo.up2ai.top:443"
    }
  }
}
```

### 配置说明

| 配置项 | 示例值 | 说明 |
|--------|--------|------|
| `relay.enabled` | `true` | 是否启用 Relay |
| `relay.endpoint` | `solo.up2ai.top:443` | Relay 连接地址 (Daemon → Relay) |
| `relay.publicEndpoint` | `solo.up2ai.top:443` | Relay 公网地址 (用于 Pairing Link) |
| `app.baseUrl` | `https://solo.up2ai.top` | 前端应用地址 (用于生成 Pairing Link) |

### 配置注意事项

> **安全说明**: `cors.origins` 为空列表时，所有携带 Origin 头的 WebSocket 请求将被拒绝（fail-closed）。生产环境必须显式配置允许的来源。

**⚠️ 重要**: `relay.endpoint` 必须使用域名 + 443 端口 (HTTPS/WSS)，**不能**使用直接 IP + 8081 端口。原因：

- 生产环境 Relay 监听 `8081`，安全组未放行其公网入站
- Nginx 在 `0.0.0.0:443` 提供 SSL 终结并反向代理到 Relay
- 直接连接 `IP:8081` 会被安全组拦截（参见[故障排除](#app-扫码连接超时-handshaketimeout)）

### 生成 Pairing Link

```bash
# 确保 Relay 已启用
solo pair

# 输出 Pairing Link 和 QR Code
# https://solo.up2ai.top/#offer=eyJ2IjoyLCJzZXJ2ZXJJZCI6...
```

## 环境变量

### Relay

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `PORT` | 8080 | 监听端口 |
| `HOST` | 0.0.0.0 | 监听地址 |
| `MAX_BUFFER` | 200 | 最大消息缓冲数 |
| `LOG_LEVEL` | info | 日志级别 |
| `ALLOWED_ORIGINS` | `https://solo.up2ai.top,http://localhost:19000` | CORS 白名单（空值拒绝所有非空 Origin） |

### Daemon

| 变量 | 说明 |
|------|------|
| `SOLO_HOME` | Solo 数据目录 (~/.solo) |
| `APP_VARIANT` | 应用变体 (development/production) |
| `SOLO_ENABLE_MOCK_PROVIDER` | 启用 Mock Provider |

## 监控和日志

### Relay 监控

**指标端点**: `/metrics`

**Prometheus 配置**:
```yaml
scrape_configs:
  - job_name: 'solo-relay'
    static_configs:
      - targets: ['relay.example.com:8080']
```

### 日志查看

```bash
# Relay 日志
sudo journalctl -u solo-relay -f

# Daemon 日志
# 本地日志文件
tail -f ~/.solo/logs/daemon.log

# 或使用 systemd
journalctl --user -u solo -f
```

### 健康检查

```bash
# Relay 健康检查 (服务器本地)
curl http://localhost:8081/health

# Relay 健康检查 (公网, 通过 Nginx)
curl https://solo.up2ai.top/health
```

**响应示例**:
```json
{
  "status": "ok",
  "sessions": 1,
  "connections": 1,
  "version": "relay-go-v1"
}
```

判断 Daemon 是否在线：`sessions: 0` 表示 Daemon 未连接到 Relay，`sessions: 1` 表示已连接。

```bash
# Daemon 健康检查
curl http://localhost:17612/api/health
# {"status":"ok","timestamp":"2026-05-19T18:31:48Z"}

# 检查 Daemon 到 Relay 的网络连接
lsof -p $(pgrep solo) | grep -E 'solo.up2ai|443'
# 预期: ... TCP 192.168.x.x:xxxxx->solo.up2ai.top:https (ESTABLISHED)
```

## 备份和恢复

### 备份

```bash
# 备份 Solo 数据
tar czf solo-backup-$(date +%Y%m%d).tar.gz ~/.solo/
```

### 恢复

```bash
# 恢复 Solo 数据
tar xzf solo-backup-20240101.tar.gz -C ~/
```

## 故障排除

### Relay 无法启动

1. 检查端口占用
   ```bash
   sudo lsof -i :8081
   ```

2. 检查权限
   ```bash
   ls -la /opt/solo-relay/solo-relay
   ```

3. 查看日志
   ```bash
   sudo journalctl -u solo-relay --no-pager | tail -50
   ```

### Daemon 无法连接 Relay

1. 检查网络连通性
   ```bash
   telnet relay.example.com 8080
   ```

2. 检查配置
   ```bash
   cat ~/.solo/config.json
   ```

3. 查看 Daemon 日志
   ```bash
   tail -f /tmp/solo-daemon.log
   ```

### WebSocket 连接失败

1. 检查 Nginx 配置
   ```bash
   nginx -t
   ```

2. 检查防火墙
   ```bash
   sudo iptables -L | grep 8080
   ```

3. 测试 WebSocket
   ```bash
   wscat -c wss://relay.example.com/ws?serverId=test&role=client
   ```

### App 扫码连接超时 (HandshakeTimeout)

**症状**: App 扫描二维码后，连接约 10 秒后超时断开。

**原因**: Daemon 的 Relay 配置使用了直接 IP + 8081 端口。由于安全组未放行 8081 公网入站，外部连接 `IP:8081` 会被拦截。

**错误配置**:
```json
{
  "daemon": {
    "relay": {
      "endpoint": "106.52.40.152:8081"
    }
  }
}
```

**正确配置** (域名 + 443 端口):
```json
{
  "daemon": {
    "relay": {
      "endpoint": "solo.up2ai.top:443"
    }
  }
}
```

**修复步骤**:
1. 修改 `~/.solo/config.json`，将 `endpoint` 改为 `solo.up2ai.top:443`
2. 重启 Daemon: `pkill -f solo && ~/.solo/bin/solo`
3. 验证 Relay 连接状态:
   ```bash
   ssh tencent_gz_6 "curl -s http://localhost:8081/health"
   # {"connections":1,"sessions":1,...}  ✅ Daemon 已连接
   # {"connections":0,"sessions":0,...}  ❌ Daemon 未连接
   ```
