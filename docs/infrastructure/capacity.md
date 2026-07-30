# Capacity & Scaling Constraints

> Runtime limits that architecture decisions must respect.
> Update this file when infrastructure changes (new instances, config tuning, etc.).

## Relay

| Parameter | Current Value | Source |
|-----------|---------------|--------|
| Instances | 1 (single point of failure) | topology.md |
| Max message buffer per session | 200 (`MAX_BUFFER`) | systemd env |
| WebSocket idle timeout | 86400s (Nginx `proxy_read_timeout`) | nginx config |
| Nginx keepalive connections | 32 | nginx upstream |
| OS file descriptor limit | default (~1024) | not yet hardened |
| Concurrent sessions (observed) | 1–5 (single user product) | — |

## Daemon

| Parameter | Current Value | Source |
|-----------|---------------|--------|
| Listen address | 127.0.0.1:17612 | config.json |
| Max concurrent agents | unbounded (OS-limited) | — |
| Tmux pane snapshot interval | adaptive (2–30s) | host-status-check |

## Scaling Ceiling (current single-instance)

- **WebSocket connections**: bounded by OS fd limit (~1024 default). Hardened target: 65536 (`LimitNOFILE`).
- **Throughput**: no load-test data yet. Relay is stateless message-forward; CPU-bound on TLS at Nginx layer.
- **Horizontal scaling**: not yet supported. Relay holds in-memory session map; multi-instance requires sticky sessions or shared state.

## Known Bottlenecks

1. Single relay instance — no failover.
2. In-memory session map — lost on restart (clients auto-reconnect).
3. No rate limiting at Nginx or Relay layer.

## Future Constraints (from roadmap)

| Concern | Trigger | Planned Response |
|---------|---------|------------------|
| Multi-user growth | >50 concurrent daemons | Relay clustering or managed WS service |
| Rate limiting | Public abuse | Nginx `limit_req` + Relay token bucket |
| High availability | SLA requirement | Active-passive relay + health-check failover |
