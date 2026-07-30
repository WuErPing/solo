# Deployment Topology

> Physical facts about where Solo runs. These constrain architecture decisions
> (e.g., single-relay bottleneck, no K8s orchestration, Tencent Cloud security groups).

## Production

```
Internet
  │
  ▼
solo.up2ai.top (DNS → 106.52.40.152)
  │
  ├── :443  Nginx (SSL termination, Let's Encrypt)
  │           └── proxy_pass → localhost:8081
  │
  ├── :80   Nginx (301 → 443)
  │
  └── :8081 Solo Relay (Go) — bound 0.0.0.0, security group blocks public ingress
              │
              ▼ (WebSocket, outbound from daemons)
        ┌──────────┬──────────┬──────────┐
        │ Daemon 1 │ Daemon 2 │ Daemon N │  (user machines, 127.0.0.1:17612)
        └──────────┴──────────┴──────────┘
```

## Server Inventory

| Role | Host | Provider | OS | Notes |
|------|------|----------|-----|-------|
| Relay + Nginx | tencent_gz_6 (106.52.40.152 / 172.16.0.2) | Tencent Cloud Guangzhou | Ubuntu 22.04 LTS | Single instance, no LB |

## Network Constraints

- Relay port 8081 is **not** publicly reachable (security group); all external traffic enters via Nginx :443.
- Daemon → Relay is an **outbound** WebSocket from user machines; no inbound ports required on daemon hosts.
- E2EE (X25519 + XSalsa20-Poly1305) means Relay cannot inspect payload.

## DNS

| Record | Value | TTL |
|--------|-------|-----|
| solo.up2ai.top | 106.52.40.152 | — |

## SSL

- Provider: Let's Encrypt (Certbot auto-renew)
- Protocols: TLSv1.2, TLSv1.3
- Cert path: `/etc/letsencrypt/live/solo.up2ai.top/`

## Daemon Hosts (user-side)

- Listen: `127.0.0.1:17612` (localhost only)
- Data dir: `~/.solo/`
- Service: user-level systemd (`~/.config/systemd/user/solo.service`) or foreground process
