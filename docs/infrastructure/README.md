# Infrastructure Context

> Independent of any single product. This layer captures the physical runtime environment
> shared by all services — deployment topology, capacity constraints, and operational limits.
> See PADD §5: "基础设施事实独立于产品，作为所有产品共享的环境约束层。"

## Contents

| Document | Summary |
|----------|---------|
| [topology.md](topology.md) | Physical deployment topology, servers, DNS, network path |
| [capacity.md](capacity.md) | QPS limits, connection limits, resource quotas, scaling constraints |

## Relationship to Other Docs

- **`architecture/deployment.md`** focuses on _how to deploy_ (commands, configs, troubleshooting).
- **This directory** focuses on _what the environment is_ (facts that constrain design decisions).
- Service-level ADRs and designs should reference these facts when making capacity or scaling claims.
