# AGENTS.md

Behavioral guidelines and project rules for AI coding agents and human contributors.

**Pre-load context:** Before starting any analysis or implementation, load the skill at `.agents/skills/solo-dev-base` to get the full architecture overview, repo map, tech stack, and build commands.

**Tradeoff:** These guidelines bias toward caution over speed. For trivial tasks, use judgment.

## 1. Think Before Coding

**Don't assume. Don't hide confusion. Surface tradeoffs.**

Before implementing:
- State your assumptions explicitly. If uncertain, ask.
- If multiple interpretations exist, present them - don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- If something is unclear, stop. Name what's confusing. Ask.

## 2. Simplicity First

**Minimum code that solves the problem. Nothing speculative.**

- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" or "configurability" that wasn't requested.
- No error handling for impossible scenarios.
- If you write 200 lines and it could be 50, rewrite it.

Ask yourself: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

## 3. Surgical Changes

**Touch only what you must. Clean up only your own mess.**

When editing existing code:
- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If you notice unrelated dead code, mention it - don't delete it.

When your changes create orphans:
- Remove imports/variables/functions that YOUR changes made unused.
- Don't remove pre-existing dead code unless asked.

The test: Every changed line should trace directly to the user's request.

## 4. Goal-Driven Execution

**Define success criteria. Loop until verified.**

Transform tasks into verifiable goals:
- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"

For multi-step tasks, state a brief plan:
```
1. [Step] → verify: [check]
2. [Step] → verify: [check]
3. [Step] → verify: [check]
```

Strong success criteria let you loop independently. Weak criteria ("make it work") require constant clarification.

---

**These guidelines are working if:** fewer unnecessary changes in diffs, fewer rewrites due to overcomplication, and clarifying questions come before implementation rather than after mistakes.

## Project Rules

- [Go Backend](.agents/rules/go-backend.md) — context, errors, concurrency, logging, naming
- [TypeScript & React Native](.agents/rules/ts-frontend.md) — type safety, components, state, styling
- [Testing](.agents/rules/testing.md) — Go table-driven tests, Vitest, CI requirements
- [Security](.agents/rules/security.md) — E2EE, credentials, input validation, network
- [Architecture](.agents/rules/architecture.md) — module boundaries, layers, protocol, data flow

## Quick Reference

```
make darwin          # build all Go binaries
make dev             # daemon :17612 + Expo web :19000
go test -short -race ./...   # Go tests
npm test             # JS tests (from app/)
```
