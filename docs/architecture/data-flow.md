# 数据流说明

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

## Agent 消息流

### Agent 执行流程

```
User → App → App-Bridge → Relay → Daemon → Agent Manager → Agent Provider
                                                          │
                                                          ▼
User ← App ← App-Bridge ← Relay ← Daemon ← Agent Manager ← Agent
```

### 状态变更通知

```
Agent → Agent Manager → Daemon → Relay → App-Bridge → App → UI Update
```

## 推送通知流

```
Daemon → Expo Push Service → Apple/Google Push → Mobile App
```

## 文件操作流

### 文件浏览

```
App → App-Bridge → Relay → Daemon → File System → Response
```

### 文件编辑

```
App → App-Bridge → Relay → Daemon → Editor → File System → Response
```

## 终端会话流

```
App → App-Bridge → Relay → Daemon → Terminal Manager → Shell → Output
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

## 错误处理流

### 连接断开

```
1. 检测断开 (超时或网络错误)
2. 标记会话状态为断开
3. 尝试自动重连 (指数退避)
4. 通知用户 (如果重连失败)
```

### 消息丢失

```
1. Relay 缓冲消息 (默认 200 条)
2. 客户端重连后恢复会话
3. 重放缓冲消息
```
