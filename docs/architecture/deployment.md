# Deployment Architecture

> **Note**: Dockerfile, docker-compose.yml, nginx.conf, and systemd `.service` files are **not** currently tracked in this repository. Relay is deployed manually via `make deploy-solo-relay` (scp + systemctl restart). Mobile builds use EAS (Expo Application Services).

## Table of Contents

- [Development Environment](#development-environment)
- [Production Environment](#production-environment)
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

## Relay 部署

### 二进制部署

**构建**:
```bash
# Linux AMD64
make solo-relay-linux-amd64

# 或手动构建
GOOS=linux GOARCH=amd64 go build -o output/linux/solo-relay ./relay-go/cmd/relay
```

**部署**:
```bash
# 1. 复制二进制文件到服务器
scp output/linux/solo-relay solo.up2ai.top:/opt/solo-relay/solo-relay

# 或使用 Makefile
make deploy-solo-relay
```

**实际 Systemd 服务** (`/etc/systemd/system/solo-relay.service`):
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

**启动服务**:
```bash
sudo systemctl enable solo-relay
sudo systemctl start solo-relay
sudo systemctl status solo-relay
```

### Docker 部署

**Dockerfile**:
```dockerfile
FROM golang:1.23-alpine AS builder
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
    server 127.0.0.1:8081;
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
| `ALLOWED_ORIGINS` | `https://solo.up2ai.top,http://localhost:19000` | CORS 白名单 |

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
   sudo lsof -i :8080
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
