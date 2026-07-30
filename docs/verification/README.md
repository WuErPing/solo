# Verification Strategy

> PADD §6: "PADD 的双驱动能否真正闭环，取决于每一层决策都有对应的验证手段。"
> This directory maps the six-layer verification model to Solo's concrete tooling.

## Six-Layer Verification Mapping

| Layer | What It Validates | Solo Tooling | Entry Point |
|-------|-------------------|--------------|-------------|
| **Static** | Syntax, types, lint | `go vet`, `golangci-lint v2`, `tsc --noEmit`, ESLint | `make ci` |
| **Test** | Unit + integration behaviour | Go `go test -short -race`, Jest (app + app-bridge) | `make ci` |
| **Runtime (E2E)** | Real user scenarios | Playwright (43 specs), daemon+relay+Metro globalSetup | `.github/workflows/e2e-nightly.yml` |
| **Non-functional** | Resilience, security, performance | Manual chaos (kill relay, network partition), security-deep-analysis findings | Ad-hoc / per-release |
| **Semantic** | Design quality, intent alignment | LLM ADR-consistency check (advisory), code review | `.github/workflows/semantic-check.yml` |
| **Value** | Solves real user problem | Product metrics, user feedback, roadmap KPIs | `product/roadmap-2026.md` §KPIs |

## Coverage Goals

| Module | Unit | Integration | E2E |
|--------|------|-------------|-----|
| protocol/ | High (serialization round-trips) | — | — |
| daemon/ | High (core logic) | Medium (store + runner) | Via Playwright |
| relay-go/ | Medium (session mgmt) | Low | Via Playwright |
| app/ | Medium (components, stores) | — | Via Playwright |
| app-bridge/ | High (RPC schemas, transforms) | — | — |

## Reports

Audit and coverage reports live in [`reports/`](reports/):

| Report | Date | Summary |
|--------|------|---------|
| [Test Coverage](reports/test-coverage.md) | 2026-06-16 | Unified coverage: Go + App + E2E + CI/Codecov |
| [Test Quality Audit](reports/test-quality-audit-2026-07.md) | 2026-07-27 | Unit-test quality issues + prioritized fix tracking |

## Principles

1. **Verification consumes Context**: validators read PRD/ADR from this repo to judge correctness.
2. **Cost-aware layering**: run cheap layers (static, unit) on every push; expensive layers (E2E, non-functional) nightly or per-release.
3. **Findings graduate**: non-functional findings that require design changes become ADRs or debt entries.
