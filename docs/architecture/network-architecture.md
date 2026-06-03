# Solo Network Architecture

## Overview

Solo is an AI-powered development assistant platform that uses a layered network architecture, supporting both local development and remote collaboration modes. This document is written based on actual deployment environments and details the network paths and data flows between components.

## System Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         Client Layer                            │
├─────────────────────────────────────────────────────────────────┤
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐         │
│  │   Web App    │  │  Mobile App  │  │    CLI       │         │
│  │  (Browser)   │  │  (iOS/Android)│  │  (Command)   │         │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘         │
└─────────┼─────────────────┼─────────────────┼──────────────────┘
          │                 │                 │
          └─────────────────┴─────────────────┘
                            │
                    ┌───────▼───────┐
                    │   App-Bridge   │
                    │  (TypeScript)  │
                    └───────┬───────┘
                            │ WebSocket
┌───────────────────────────▼─────────────────────────────────────┐
│                      Network Transport Layer                    │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                    Nginx (Reverse Proxy)                │   │
│  │              solo.up2ai.top:443                          │   │
│  │  - SSL Termination (Let's Encrypt)                      │   │
│  │  - Reverse proxy to localhost:8081                     │   │
│  └─────────────────────────┬───────────────────────────────┘   │
│                            │                                    │
│  ┌─────────────────────────▼───────────────────────────────┐   │
│  │              Solo Relay Server (Go)                      │   │
│  │              localhost:8081                              │   │
│  │                                                         │   │
│  │  ┌─────────────────┐    ┌─────────────────┐            │   │
│  │  │  Control Socket │    │   Data Socket   │            │   │
│  │  │  (Control Conn) │    │   (Data Conn)   │            │   │
│  │  └────────┬────────┘    └────────┬────────┘            │   │
│  │           │                      │                     │   │
│  │           └──────────────────────┘                     │   │
│  │                      │                                  │   │
│  │           ┌──────────▼──────────┐                      │   │
│  │           │   Session Store     │                      │   │
│  │           │   (Session Mgmt)    │                      │   │
│  │           └─────────────────────┘                      │   │
│  └─────────────────────────┬───────────────────────────────┘   │
└────────────────────────────┼────────────────────────────────────┘
                             │
┌────────────────────────────▼────────────────────────────────────┐
│                      Service Layer (User Machine)               │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────────┐   │
│  │              Solo Daemon (Go)                            │   │
│  │              127.0.0.1:17612                             │   │
│  │                                                         │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────┐        │   │
│  │  │   Agent     │  │  Workspace  │  │ Terminal│        │   │
│  │  │  Manager    │  │   Store     │  │ Manager │        │   │
│  │  └─────────────┘  └─────────────┘  └─────────┘        │   │
│  │                                                         │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────┐        │   │
│  │  │   Script    │  │     Git     │  │  Push   │        │   │
│  │  │   Proxy     │  │   Service   │  │ Service │        │   │
│  │  └─────────────┘  └─────────────┘  └─────────┘        │   │
│  │                                                         │   │
│  │  ┌─────────────────────────────────────────────────┐   │   │
│  │  │           Relay Client                           │   │   │
│  │  │  Control Conn ──► Relay Server (solo.up2ai.top)  │   │   │
│  │  │  Data Conn ◄── Relay Server                      │   │   │
│  │  └─────────────────────────────────────────────────┘   │   │
│  └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

## 核心组件

### 1. App (客户端应用)

**目录**: `app/`

**技术栈**: React Native + Expo

**职责**:
- 提供用户界面（Web、iOS、Android）
- 通过 App-Bridge 与 Daemon 通信
- 管理用户会话和工作区
- 支持扫描二维码配对

**关键目录**:
- `src/screens/` - 页面组件
- `src/components/` - 可复用组件
- `src/app/` - Expo Router 路由

### 2. App-Bridge (客户端通信库)

**目录**: `app-bridge/`

**技术栈**: TypeScript

**职责**:
- 封装 WebSocket 通信细节
- 支持直接连接和 Relay 连接两种方式
- 实现端到端加密 (E2EE)

**关键模块**:

| 文件 | 职责 |
|------|------|
| `src/client/daemon-client.ts` | 主客户端，管理连接状态 |
| `src/client/daemon-client-websocket-transport.ts` | WebSocket 传输实现 |
| `src/client/daemon-client-relay-e2ee-transport.ts` | Relay E2EE 传输 |
| `src/relay/e2ee.ts` | 端到端加密实现 |
| `src/shared/connection-offer.ts` | Pairing Link 类型定义 |

### 3. Daemon (守护进程)

**目录**: `daemon/`

**技术栈**: Go

**职责**:
- 核心服务，管理所有业务逻辑
- WebSocket 服务器 (端口 17612)
- Agent 生命周期管理
- 工作区和项目管理

**关键目录**:
- `internal/server/` - WebSocket 服务器
- `internal/relayclient/` - Relay 客户端
- `internal/agent/` - Agent 管理
- `internal/workspace/` - 工作区管理

### 4. Relay (中继服务器)

**目录**: `relay-go/`

**技术栈**: Go

**职责**:
- WebSocket 连接中继 (端口 8081)
- 会话管理
- 消息缓冲
- NAT 穿透支持

**关键文件**:
- `internal/relay/server.go` - HTTP/WebSocket 服务器
- `internal/relay/session.go` - 会话管理
- `internal/relay/control.go` - 控制连接逻辑

## 实际部署架构

### 服务器信息

| 项目 | 值 |
|------|-----|
| **服务器** | Tencent Cloud Guangzhou |
| **主机名** | tencent_gz_6 |
| **公网 IP** | 106.52.40.152 |
| **内网 IP** | 172.16.0.2 |
| **操作系统** | Ubuntu 22.04 LTS |
| **域名** | solo.up2ai.top |
| **SSL 证书** | Let's Encrypt (Certbot) |

### 实际网络路径

```
Mobile App / Web App
    │
    │ WSS (WebSocket over TLS)
    │ solo.up2ai.top:443
    ▼
┌─────────────────────────────┐
│      Nginx (443端口)         │
│  - SSL 终结 (Let's Encrypt)  │
│  - 反向代理到 localhost:8081 │
└─────────────┬───────────────┘
              │
              │ WS (WebSocket)
              │ localhost:8081
              ▼
┌─────────────────────────────┐
│      Solo Relay (Go)        │
│      端口: 8081              │
│  - 会话管理                  │
│  - 消息路由                  │
│  - 连接配对                  │
└─────────────┬───────────────┘
              │
              │ WS (WebSocket)
              │ 公网/内网
              ▼
┌─────────────────────────────┐
│      Daemon (用户机器)        │
│      端口: 17612             │
│  - Agent / Workspace         │
│  - Terminal / Git            │
└─────────────────────────────┘
```

### Nginx 配置

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

### Solo Relay 配置

**Systemd Service** (`/etc/systemd/system/solo-relay.service`):
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

### 端口映射

| 端口 | 服务 | 监听地址 | 说明 | 访问控制 |
|------|------|----------|------|----------|
| 80 | Nginx | 0.0.0.0 | HTTP 重定向到 HTTPS | 公网开放 |
| 443 | Nginx | 0.0.0.0 | HTTPS + WebSocket 代理 | 公网开放 |
| 8081 | Solo Relay | **127.0.0.1** | Relay WebSocket 服务 | **仅本地** |
| 17612 | Daemon | 127.0.0.1 | Daemon 本地服务 | 仅本地 |

**⚠️ 重要**: 端口 8081 仅监听在 `127.0.0.1`，外部无法直接访问。所有外部连接必须通过 Nginx (443端口) 反向代理。

## 网络路径详解

### 路径 1: 本地开发模式

```
App (Web/Mobile) 
    │
    │ WebSocket
    │ localhost:17612
    ▼
Daemon (127.0.0.1:17612)
```

**特点**:
- 直接连接，无 Relay 参与
- 低延迟，适合本地开发
- 无法从公网访问

**配置**:
```json
{
  "daemon": {
    "listen": "127.0.0.1:17612"
  }
}
```

### 路径 2: Relay 模式 (生产环境)

```
App (Web/Mobile)
    │
    │ WSS (WebSocket over TLS)
    │ solo.up2ai.top:443
    ▼
Nginx (443端口)
    │
    │ WS (WebSocket)
    │ localhost:8081
    ▼
Relay Server (8081端口)
    │
    │ WS (WebSocket)
    ▼
Daemon (通过 Relay Client 连接)
```

**特点**:
- 支持公网访问
- 解决 NAT 穿透问题
- 可选 E2EE 加密
- 适合远程协作

**配置**:
```json
{
  "daemon": {
    "relay": {
      "enabled": true,
      "endpoint": "solo.up2ai.top:443",
      "publicEndpoint": "solo.up2ai.top:443"
    }
  }
}
```

**⚠️ 常见错误**: 使用直接 IP 连接
```json
{
  "daemon": {
    "relay": {
      "enabled": true,
      "endpoint": "106.52.40.152:8081",
      "publicEndpoint": "solo.up2ai.top:443"
    }
  }
}
```
**问题**: `106.52.40.152:8081` 会被防火墙拦截，因为 8081 端口仅监听在 `127.0.0.1`。
**症状**: App 扫码连接后 10 秒超时 (HandshakeTimeout)。

### 路径 3: 移动端通过 Relay

```
Mobile App (iOS/Android)
    │
    │ WSS (WebSocket over TLS)
    │ solo.up2ai.top:443
    ▼
Relay Server (公网)
    │
    │ WS (WebSocket)
    ▼
Daemon (本地或远程)
```

**特点**:
- 移动端通常通过 Relay 连接
- 使用 E2EE 确保安全性
- 支持后台推送通知

## Pairing Link (配对链接) 流程

Pairing Link 是移动端/桌面端 App 连接到 Daemon 的关键机制，它将复杂的网络配置封装为一个简单的 URL。

### 生成流程

```
┌─────────────────────────────────────────────────────────────────┐
│                    Pairing Link 生成                             │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────┐    ┌─────────────┐    ┌─────────────────────────┐ │
│  │  User   │───►│  solo pair  │───►│  GeneratePairingOffer   │ │
│  │         │    │  (CLI 命令)  │    │  (cli/internal/client)  │ │
│  └─────────┘    └─────────────┘    └─────────────────────────┘ │
│                                             │                   │
│                                             ▼                   │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │              ConnectionOfferV2 (JSON)                    │   │
│  │  {                                                       │   │
│  │    "v": 2,                                               │   │
│  │    "serverId": "75df32ee",                               │   │
│  │    "daemonPublicKeyB64": "LbDipkESA0+8Mzs57k0EnIW8...",  │   │
│  │    "relay": {                                            │   │
│  │      "endpoint": "solo.up2ai.top:443"                    │   │
│  │    }                                                     │   │
│  │  }                                                       │   │
│  └─────────────────────────┬───────────────────────────────┘   │
│                            │                                    │
│                            ▼ Base64URL 编码                      │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │  Pairing Link URL                                        │   │
│  │  https://solo.up2ai.top/#offer=eyJ2IjoyLCJzZXJ2ZXJJZCI6...  │   │
│  └─────────────────────────────────────────────────────────┘   │
│                            │                                    │
│                            ▼                                    │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │  QR Code (终端显示)                                       │   │
│  │  ┌─────┐                                                │   │
│  │  │ ▄▄▄ │                                                │   │
│  │  │ ███ │                                                │   │
│  │  │ ▀▀▀ │                                                │   │
│  │  └─────┘                                                │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 关键组件

| 组件 | 位置 | 职责 |
|------|------|------|
| **CLI `pair` 命令** | `cli/cmd/daemon_pair.go` | 用户入口，读取配置 |
| **Pairing 逻辑** | `cli/internal/client/pairing.go` | 生成/解析配对链接 |
| **密钥管理** | `cli/internal/client/pairing.go` | Curve25519 密钥对 |
| **Offer Schema** | `app-bridge/src/shared/connection-offer.ts` | 类型定义和验证 |

### 配对链接结构

```
https://solo.up2ai.top/#offer={base64url-encoded-json}
```

**解码后的 JSON 结构**:
```json
{
  "v": 2,
  "serverId": "75df32ee",
  "daemonPublicKeyB64": "LbDipkESA0+8Mzs57k0EnIW8wvFLaZ95MxhOHEqWNXs=",
  "relay": {
    "endpoint": "solo.up2ai.top:443"
  }
}
```

### 使用流程

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Daemon    │     │   用户操作   │     │  Mobile App │
│  (服务器端)  │     │             │     │  (客户端)    │
└──────┬──────┘     └──────┬──────┘     └──────┬──────┘
       │                   │                   │
       │  1. solo pair     │                   │
       │◄──────────────────│                   │
       │                   │                   │
       │  2. 生成 Pairing  │                   │
       │     Link + QR     │                   │
       │──────────────────►│                   │
       │                   │                   │
       │  3. 扫描二维码或   │                   │
       │     粘贴链接      │                   │
       │──────────────────────────────────────►│
       │                   │                   │
       │                   │     4. 解析 Offer │
       │                   │                   │
       │                   │     5. 连接 Relay │
       │                   │                   │
       │                   │     6. E2EE 握手  │
       │◄──────────────────────────────────────│
       │                   │                   │
       │  7. 建立数据通道   │◄─────────────────│
       │                   │                   │
       │  8. 正常通信       │◄────────────────►│
```

### 与部署架构的衔接

#### 实际部署中的网络路径

**App → Relay → Daemon 路径** (通过 Nginx 反向代理):

```
Mobile App (iOS/Android)
    │
    │ 1. 扫描二维码获取 Pairing Link
    │    https://solo.up2ai.top/#offer=...
    │
    │ 2. 解析出 Relay Endpoint
    │    solo.up2ai.top:443
    │
    │ 3. WebSocket over TLS (WSS)
    ▼
┌─────────────────────────────┐
│      Nginx (反向代理)        │
│      solo.up2ai.top:443     │
│                             │
│  SSL 终结 / 负载均衡 / 静态资源 │
└─────────────┬───────────────┘
              │
              │ 4. WebSocket (HTTP)
              ▼
┌─────────────────────────────┐
│      Relay Server            │
│      127.0.0.1:8081          │
│                             │
│  会话管理 / 消息路由 / 连接配对  │
└─────────────┬───────────────┘
              │
              │ 5. WebSocket (WSS)
              ▼
┌─────────────────────────────┐
│      Daemon (用户机器)        │
│      127.0.0.1:17612         │
│                             │
│  Agent / Workspace / Terminal │
└─────────────────────────────┘
```

**关键说明**:
- **App → Nginx**: 使用 WSS (WebSocket over TLS)，端口 443
- **Nginx → Relay**: 使用 WS (WebSocket)，端口 8081，仅本地访问
- **Daemon → Nginx**: 使用 WSS (WebSocket over TLS)，端口 443
- **Nginx → Relay**: 反向代理到 `localhost:8081`

**⚠️ 防火墙/安全组规则**:
- 端口 443: 对外开放 (App 和 Daemon 连接)
- 端口 8081: 仅本地访问 (Nginx 反向代理)
- 端口 80: 对外开放 (HTTP 重定向到 HTTPS)

#### 配置映射

| 配置项 | 值 | 说明 |
|--------|-----|------|
| **Relay 公网地址** | `solo.up2ai.top:443` | Nginx 反向代理入口 (HTTPS/WSS) |
| **Relay 本地地址** | `127.0.0.1:8081` | Relay Server 实际监听 (WS) |
| **Daemon 监听地址** | `127.0.0.1:17612` | Daemon 本地监听 |
| **App Base URL** | `https://solo.up2ai.top` | 前端应用地址 |

**Daemon 连接配置**:
```json
{
  "relay": {
    "enabled": true,
    "endpoint": "solo.up2ai.top:443",
    "publicEndpoint": "solo.up2ai.top:443"
  }
}
```

**连接协议对比**:

| 路径 | 协议 | 端口 | 说明 |
|------|------|------|------|
| App → Nginx | WSS | 443 | WebSocket over TLS |
| Nginx → Relay | WS | 8081 | 本地反向代理 |
| Daemon → Nginx | WSS | 443 | WebSocket over TLS |
| Nginx → Relay | WS | 8081 | 本地反向代理 |

#### 域名解析与端口映射

```
solo.up2ai.top ──► 106.52.40.152 (腾讯云)
                      │
                      ├──► :443 ──► Nginx (SSL) ──► Relay (localhost:8081)
                      │
                      └──► :80  ──► Nginx (重定向到 443)
```

**端口访问控制**:

| 端口 | 监听地址 | 服务 | 访问控制 |
|------|----------|------|----------|
| 80 | 0.0.0.0 | Nginx HTTP | 公网开放 (重定向到 443) |
| 443 | 0.0.0.0 | Nginx HTTPS | 公网开放 (SSL + WebSocket 代理) |
| 8081 | 127.0.0.1 | Relay WS | **仅本地访问** (Nginx 反向代理) |
| 17612 | 127.0.0.1 | Daemon HTTP/WS | 仅本地访问 |

**⚠️ 重要**: 端口 8081 仅监听在 `127.0.0.1`，外部无法直接访问。所有外部连接必须通过 Nginx (443端口)。

### 安全机制

1. **E2EE 加密**: Pairing Link 包含 Daemon 公钥，用于端到端加密
2. **ServerID 验证**: 唯一标识 Daemon，防止中间人攻击
3. **TLS 传输**: Relay 通信使用 WSS (WebSocket over TLS)
4. **密钥持久化**: Daemon 密钥对保存在 `~/.solo/daemon-keypair.json`

### 代码示例

**生成 Pairing Link (CLI)**:
```bash
# 默认配置
solo pair

# 输出:
# Scan to pair:
# [QR Code]
# https://solo.up2ai.top/#offer=eyJ2IjoyLCJzZXJ2ZXJJZCI6...
```

**解析 Pairing Link (App)**:
```typescript
// app/src/components/pair-link-modal.tsx
const parsedOffer = ConnectionOfferSchema.parse(
  decodeOfferFragmentPayload(encoded)
);

// 连接到 Daemon
const { client, hostname } = await connectToDaemon(
  {
    type: "relay",
    relayEndpoint: normalizeHostPort(parsedOffer.relay.endpoint),
    daemonPublicKeyB64: parsedOffer.daemonPublicKeyB64,
  },
  { serverId: parsedOffer.serverId }
);
```

## WebSocket 消息流

### 连接建立流程

```
Client                              Relay                              Daemon
  │                                  │                                  │
  │  1. WebSocket 连接                │                                  │
  │─────────────────────────────────►│                                  │
  │                                  │                                  │
  │                                  │  2. 验证参数                      │
  │                                  │  (serverId, role, connectionId)   │
  │                                  │                                  │
  │  3. 连接确认                      │                                  │
  │◄─────────────────────────────────│                                  │
  │                                  │                                  │
  │                                  │  4. 如果是 Server 角色            │
  │                                  │     等待 Client 连接              │
  │                                  │                                  │
  │                                  │  5. 如果是 Client 角色            │
  │                                  │     匹配到 Server 会话            │
  │                                  │                                  │
  │                                  │  6. 建立数据通道                   │
  │                                  │◄────────────────────────────────►│
```

### 消息传输流程

```
┌─────────┐     ┌─────────────┐     ┌─────────┐
│  Client │◄───►│    Relay    │◄───►│  Daemon │
│         │     │             │     │         │
└────┬────┘     └──────┬──────┘     └────┬────┘
     │                 │                 │
     │  1. Send(msg)   │                 │
     │────────────────►│                 │
     │                 │  2. Forward     │
     │                 │────────────────►│
     │                 │                 │
     │                 │  3. Process     │
     │                 │                 │
     │                 │  4. Response    │
     │                 │◄────────────────│
     │                 │                 │
     │  5. Receive     │                 │
     │◄────────────────│                 │
```

## 消息类型

### 控制消息 (Control Messages)

**方向**: Daemon ↔ Relay

| 消息 | 说明 |
|------|------|
| `hello` | 握手，交换协议版本和认证信息 |
| `ping` | 心跳保活 |
| `pong` | 心跳响应 |
| `attach` | 请求建立数据连接 |
| `detach` | 断开数据连接 |

### 数据消息 (Data Messages)

**方向**: Client ↔ Daemon (通过 Relay)

| 消息 | 说明 |
|------|------|
| `auth` | 认证 |
| `request` | 请求 |
| `response` | 响应 |
| `event` | 事件通知 |
| `error` | 错误 |

## 会话生命周期

### 1. 创建会话

```
Client          Relay           Daemon
  │              │               │
  │── connect ──►│               │
  │              │── attach ────►│
  │              │               │
  │              │◄── accept ────│
  │◄─ connected ─│               │
```

### 2. 数据传输

```
Client          Relay           Daemon
  │              │               │
  │── message ──►│── forward ───►│
  │              │               │
  │◄─ response ──│◄── result ────│
```

### 3. 关闭会话

```
Client          Relay           Daemon
  │              │               │
  │── close ────►│── detach ────►│
  │              │               │
  │◄─ closed ────│◄── ack ───────│
```

## 端到端加密 (E2EE) 流程

### 密钥交换

```
Client                                          Daemon
  │                                              │
  │  1. 生成临时密钥对 (X25519)                    │
  │                                              │
  │  2. 发送公钥 (通过 Relay 控制连接)              │
  │─────────────────────────────────────────────►│
  │                                              │
  │                                              │  3. 生成临时密钥对
  │                                              │
  │                                              │  4. 发送公钥
  │◄─────────────────────────────────────────────│
  │                                              │
  │  5. 计算共享密钥                               │
  │     (X25519 密钥交换)                          │
  │                                              │
  │                                              │  6. 计算共享密钥
  │                                              │
  │  7. 派生加密密钥 (XSalsa20-Poly1305)           │
  │                                              │
  │                                              │  8. 派生加密密钥
```

### 加密传输

```
Client                      Relay                      Daemon
  │                          │                          │
  │  1. 加密消息              │                          │
  │     (XSalsa20-Poly1305)  │                          │
  │                          │                          │
  │── ciphertext ───────────►│── forward ──────────────►│
  │                          │                          │
  │                          │                          │  2. 解密消息
  │                          │                          │
  │                          │                          │  3. 处理请求
  │                          │                          │
  │                          │                          │  4. 加密响应
  │                          │                          │
  │◄── ciphertext ──────────│◄── forward ──────────────│
  │                          │                          │
  │  5. 解密响应              │                          │
```

## 心跳机制

### 控制连接心跳

```
Daemon          Relay
  │              │
  │── ping ────►│
  │              │
  │◄── pong ────│
  │              │
  │  (每 10 秒)  │
```

### 数据连接心跳

```
Client          Relay          Daemon
  │              │              │
  │── ping ────►│── forward ──►│
  │              │              │
  │◄── pong ─────│◄── forward ──│
  │              │              │
  │  (每 30 秒)  │              │
```

## 配置文件

### Daemon 配置 (~/.solo/config.json)

**生产环境配置** (Daemon 通过 HTTPS/WSS 连接 Relay):

```json
{
  "daemon": {
    "listen": "127.0.0.1:17612",
    "hostnames": ["localhost"],
    "cors": {
      "origins": ["https://solo.up2ai.top", "http://localhost:19000"]
    },
    "relay": {
      "enabled": true,
      "endpoint": "solo.up2ai.top:443",
      "publicEndpoint": "solo.up2ai.top:443"
    }
  }
}
```

**⚠️ 重要**: `endpoint` 必须使用域名 + 443 端口 (HTTPS/WSS)，**不能**使用直接 IP + 8081 端口。原因：
- 生产环境 Relay 仅监听 `localhost:8081`
- Nginx 在 `0.0.0.0:443` 提供 SSL 终结并反向代理到 Relay
- 直接连接 `IP:8081` 会被防火墙/安全组拦截

**错误配置示例** (会导致连接超时):
```json
{
  "daemon": {
    "relay": {
      "enabled": true,
      "endpoint": "106.52.40.152:8081",
      "publicEndpoint": "solo.up2ai.top:443"
    }
  }
}
```

### Relay 环境变量

```bash
PORT=8081
HOST=0.0.0.0
MAX_BUFFER=200
LOG_LEVEL=info
```

## 协议常量

**位置**: `protocol/protocol.go`

```go
const (
    WSProtocolVersion        = 1
    HelloTimeoutMs           = 15000
    SessionDisconnectGraceMs = 90000
    
    WSEndpoint           = "/ws"
    RelayProtocolVersion = "2"
)
```

## 健康检查与故障排查

### Relay 健康检查

```bash
# 服务器本地检查
curl http://localhost:8081/health

# 公网检查 (通过 Nginx)
curl https://solo.up2ai.top/health
```

**响应示例** (Daemon 未连接):
```json
{
  "status": "ok",
  "sessions": 0,
  "connections": 0,
  "version": "relay-go-v1"
}
```

**响应示例** (Daemon 已连接):
```json
{
  "status": "ok",
  "sessions": 1,
  "connections": 1,
  "version": "relay-go-v1"
}
```

**判断 Daemon 是否在线**:
- `sessions: 0` - Daemon 未连接到 Relay
- `sessions: 1` - Daemon 已连接到 Relay

### Daemon 健康检查

```bash
curl http://localhost:17612/api/health
```

**响应**:
```json
{
  "status": "ok",
  "timestamp": "2026-05-19T18:31:48Z"
}
```

### 连接状态检查

**检查 Daemon 是否连接到 Relay**:
```bash
# 查看 Daemon 网络连接
lsof -p $(pgrep solo) | grep -E 'solo.up2ai|443'

# 预期输出 (已连接):
# solo  1234  user  7u  IPv4  ...  TCP 192.168.x.x:xxxxx->solo.up2ai.top:https (ESTABLISHED)
```

**检查 Relay 端连接**:
```bash
ssh tencent_gz_6 "curl -s http://localhost:8081/health"
# {"connections":1,"sessions":1,...}  ✅ Daemon 已连接
# {"connections":0,"sessions":0,...}  ❌ Daemon 未连接
```

### 常见问题：App 扫码连接超时

**症状**: App 扫描二维码后，连接 10 秒后断开

**原因**: Daemon 配置错误，使用直接 IP 连接

**检查配置**:
```bash
cat ~/.solo/config.json
```

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

**正确配置**:
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
3. 验证: `ssh tencent_gz_6 "curl -s http://localhost:8081/health"`

## 总结

Solo 的网络架构采用分层设计：

1. **客户端层**: Web、Mobile、CLI 通过 App-Bridge 统一接入
2. **网络层**: Nginx (SSL) + Relay Server 提供公网接入能力
3. **服务层**: Daemon 提供核心功能

关键特点：
- 支持本地直连和远程 Relay 两种模式
- WebSocket 全双工通信
- 可选的端到端加密 (E2EE)
- Pairing Link 机制简化移动端连接
- 实际部署使用 Nginx + Let's Encrypt SSL
