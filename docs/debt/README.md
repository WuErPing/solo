# Tech Debt Registry

> Every architectural compromise must be recorded here with a repayment window.
> See PADD §4: "短期迭代允许适度妥协，但所有妥协必须记录在 ADR，设定偿还技术债时间窗口。"

## Rules

1. **Every compromise gets an entry.** If an ADR or design decision accepts short-term debt, a corresponding `debt-NNN-*.md` file must exist here.
2. **Link to the ADR.** Each entry references the decision that introduced the debt.
3. **Set a repayment window.** No open-ended debt — every entry has a target release or date for resolution.
4. **Track status.** Entries move through: `Active` → `In Progress` → `Resolved`.
5. **Resolved entries stay.** Move to the Resolved table below for audit trail; do not delete.

## Active Debt

| ID | Title | Introduced | ADR / Source | Repayment Target | Status |
|----|-------|------------|--------------|------------------|--------|
| — | _(none yet)_ | — | — | — | — |

## Resolved Debt

| ID | Title | Introduced | Resolved | Resolution |
|----|-------|------------|----------|------------|
| — | _(none yet)_ | — | — | — |

## Entry Template

```markdown
# DEBT-NNN: <Short Title>

|              |                                        |
|--------------|----------------------------------------|
| **Status**   | Active / In Progress / Resolved        |
| **Introduced** | YYYY-MM-DD                           |
| **ADR**      | [ADR-NNN](../decisions/adr-nnn-*.md)   |
| **Repayment Target** | vX.Y.Z or YYYY-MM-DD           |
| **Owner**    | —                                      |

## What

One-paragraph description of the compromise.

## Why It Was Accepted

Business or schedule pressure that justified the trade-off.

## Impact

What degrades while this debt exists (performance, maintainability, coupling, etc.).

## Repayment Plan

Concrete steps to resolve, with estimated effort.

## Resolution (fill when done)

How it was actually resolved, link to PR/commit.
```
