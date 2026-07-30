# Dual-Impact Review Checklist

> PADD §3.4: "需求变更同时评审产品影响 + 架构影响。"
> Use this checklist for any change that touches user-facing behaviour OR system structure.

## When to Use

- New feature or significant feature change
- Protocol/wire format change
- New dependency or infrastructure change
- Any change that modifies an ADR's assumptions

## Checklist

### Product Impact

- [ ] Does this change align with the current roadmap pillar? (`product/roadmap-2026.md`)
- [ ] Is there a PRD / user story that motivates this? (`product/prd/`)
- [ ] Are user-facing behaviours documented (feature spec, UI mockup)?
- [ ] Does this affect existing features? List them.
- [ ] Are acceptance criteria defined and testable?

### Architecture Impact

- [ ] Does this change respect existing module boundaries? (`architecture/components.md`)
- [ ] Does it introduce new coupling between modules?
- [ ] Are quality attributes affected (performance, security, scalability)?
- [ ] Does it contradict any accepted ADR? If so, propose a superseding ADR.
- [ ] Are interface contracts updated (protocol/, app-bridge/ schemas)?

### Tech Debt Assessment

- [ ] Does this change introduce any compromise? (shortcut, duplication, deferred refactor)
- [ ] If yes: is there a `debt/debt-NNN-*.md` entry with a repayment window?
- [ ] Is the debt linked to the relevant ADR?

### Verification Plan

- [ ] Which verification layers apply? (static / test / E2E / non-functional / semantic / value)
- [ ] Are new tests added at the appropriate layer?
- [ ] For non-functional changes: is there a manual or automated resilience check?

## Output

After review, record the conclusion:
- **No concerns** → proceed.
- **Concerns resolved** → note resolution in PR description.
- **Debt accepted** → create debt entry + ADR (if architectural) before merge.
