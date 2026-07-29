# Tmux Pane Rendering — Decision Record

> **Status:** Decided & Implemented (Method B)
> **Date:** 2026-07-29
> **Supersedes:** `tmux-pane-analysis.md`, `tmux-pane-first-principles-2026-06-20.md`, `tmux-pane-client-emulator-first-principles.md`

---

## Decision

Render tmux pane content client-side using **xterm.js** (the existing `TerminalEmulator` component), replacing the former React Native `<Text>` tree renderer. The daemon continues to supply pane content via `tmux capture-pane`; the client feeds that ANSI output into a real terminal emulator grid rather than decomposing it into thousands of `<Text>` nodes.

This is implemented and in production.

---

## Why

The original architecture had a structural mismatch: tmux maintains a cell-based grid with incremental VT updates, but the app re-rendered the entire content string as a React Native `<Text>` tree on every poll cycle. This caused:

- **Jitter and flicker** — full-string replacement triggered full parse, full tree reconcile, ScrollView relayout, and scroll-position correction on every cycle, even when only one character changed.
- **Poor ANSI fidelity** — box-drawing characters were stripped, cursor was absent, CJK width was uncomputed, and true-color rendering was unstable.
- **No incremental repaint** — every poll produced new object references that defeated memoization, forcing O(n) work proportional to total scrollback rather than to the delta.

xterm.js solves all three at once: it maintains its own cell grid, applies only the VT sequences that arrive, handles Unicode/box-drawing/true-color natively, and keeps scroll position stable internally. Solo already had this component in production for the workspace terminal (xterm.js + WebGL renderer via Expo DOM), so the migration reused proven infrastructure rather than introducing new dependencies.

---

## Alternatives considered

### Method A — Incremental patches to the ANSI Text renderer

Patching the existing renderer (restore box drawing, add LRU cache, per-line diff). Low effort but hits a hard ceiling: the RN `<Text>` tree model is structurally incompatible with a cell-grid incremental update model. Useful only as a short-term stopgap (and was partially applied in v0.4.1 to stabilize the app before Method B landed).

### Method C — tmux Control Mode (`tmux -C`) PTY stream

A persistent `tmux -C` process on the daemon would stream real-time `%output` VT events directly to the client, eliminating polling entirely and achieving parity with a local terminal.

**Deferred because cost exceeds benefit:**

1. **Width conflict (blocker):** tmux pane dimensions are global. Resizing for a 40-column phone screen would disrupt a 200-column desktop session sharing the same pane. No clean mitigation exists.
2. **Engineering cost:** ~5–7 weeks for the Control Mode process manager, snapshot/stream race resolution, tmux 2.x–3.x version compatibility, crash recovery, and multi-client width arbitration.
3. **Marginal gain:** the primary use case (reading AI-agent output, occasional key input) is well served by snapshot + push-driven refresh. True real-time streaming matters only for interactive TUIs (vim, htop), a small fraction of usage.
4. **Ongoing maintenance:** tmux version upgrades, process-leak risk, and platform-specific behavior create a long tail of upkeep.

Method C may be revisited if real-time TUI interaction becomes a hard requirement, but the width-conflict blocker must be resolved first.

---

## Outcome / current state

- Method B is implemented. The living reference is **[docs/architecture/tmux-pane-content-loading.md](../architecture/tmux-pane-content-loading.md)**.
- Since the original analyses, the system has also gained **push-driven refresh** (daemon detects pane activity and pushes `tmux/pane_changed`, triggering an immediate refetch) and **snapshot diffing**, reducing both latency and unnecessary re-renders beyond what the original polling model achieved.
- The former `parseAnsi` / `AnsiTextContent` rendering path is no longer used for tmux panes.

---

## Superseded documents

The following analysis files are retained for historical reference but are superseded by this record:

1. `docs/analysis/tmux-pane-analysis.md` — original jitter root-cause analysis and four-option comparison (2026-06-09).
2. `docs/analysis/tmux-pane-first-principles-2026-06-20.md` — deep evaluation of Method C (Control Mode) obstacles and cost (2026-06-20).
3. `docs/analysis/tmux-pane-client-emulator-first-principles.md` — first-principles justification for client-side terminal emulator (2026-06-20).
