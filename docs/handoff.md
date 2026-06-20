# Handoff: Tmux Pane Full-Content Fit

## Goal
The `/tmux-pane-xterm` screen must render the **full, native-width content of the host tmux pane** — not a rewrapped approximation. A single toggle selects the view naturally:
1. **Fit mode** (default): the native grid is rendered at the pane's real column count and CSS-scaled down so the **whole pane is visible** at once (sharp downscale; more rows shown because each is shorter). If the pane already fits the screen, scale is 1 (natural 1:1).
2. **1:1 mode** (toggle): the native grid is rendered at the base font size and the user pans left/right to read it. **The horizontal scroller is the terminal's root `div`, which lives inside the DOM component** (iframe on web, WebView on native), so pointer events reach it.

Both modes render the identical native grid — no content is rewrapped or lost.

## Root causes that were fixed
1. **Lossy rewrap.** The previous "Fit" mode requested `cols = measuredCols` from the daemon, which ran `wrapContentToCols(content, phoneCols)` — an ANSI-aware rewrapper that approximates wide chars (CJK, box drawing) as one column. This broke ASCII box-drawing tables/TUIs and discarded the pane's native layout.
2. **Outer RN `ScrollView` cannot be scrolled from inside the DOM component.** Expo `"use dom"` components run in a separate document context (iframe on web, WebView on native). Pointer/touch/wheel events inside that context do not propagate to the RN `ScrollView` wrapping it — so the horizontal scrollbar was visible but dragging the terminal content never panned. The scroller must live **inside** the DOM component.

## What Changed

### Screen (`app/src/screens/tmux-pane-xterm-screen.tsx`)
- **Always fetches native-width content** by omitting `cols` in `useTmuxCapturePane` — no daemon rewrapping. The daemon's `wrapContentToCols` path is now unused by this screen (still available in the protocol for other consumers).
- **Always renders at `forceCols = paneCols`** in both modes, so the full native grid is shown.
- **Both modes render `TerminalEmulator` directly** (same `flex:1` DOM props). There is **no RN `ScrollView` wrapper** — the horizontal scroller is the terminal root `div` inside the DOM component.
- Fit mode: `fitToWidth` true (runtime scales the host to fit). 1:1 mode: `fitToWidth` false, `allowHorizontalScroll` true (root div is the horizontal scroller).
- The toggle button is a natural **Fit / 1:1 zoom control**: label is `1:1` in fit mode (tap to zoom to native) and `Fit` in 1:1 mode (tap to fit the whole pane).
- Removed `measuredCols` / `estimateInitialCols` / `estimatedOriginalWidth` / `onInitialColsEstimated` / `measuredHostWidth` / `handleTerminalResize` / the `HorizontalTerminalScroller` / `makeOriginalDomProps` (no longer needed).

### Terminal emulator (`app/src/components/terminal-emulator.tsx`)
- New prop `fitToWidth?: boolean`, forwarded to the runtime via `setFitToWidth` (reacts without remounting).
- The root `div` is the horizontal scroller in 1:1 mode: `width: 100%`, `overflow-x: auto`, `overflow-y: hidden`, `touch-action: pan-x pan-y`. In fit mode: `overflow: hidden`, `touch-action: pan-y`.

### Runtime (`app/src/terminal/runtime/terminal-emulator-runtime.ts`)
- New `fitToWidth` mount option + `setFitToWidth` setter.
- `fitAndEmitResize` force-cols branch now:
  - Measures the real cell width **and** cell height (using `offsetWidth`/`offsetHeight`, which are layout sizes unaffected by the CSS transform — `getBoundingClientRect` would return the scaled rect and corrupt the measurement).
  - When `fitToWidth` and the native pane is wider than the container: applies `transform: scale(s)` (`s = containerWidth / (cellWidth × paneCols)`, clamped ≤ 1) to the host with `transform-origin: top left`, and grows rows to fill the scaled height. Root stays at container size, overflow hidden.
  - Otherwise (1:1, or pane already fits): lays the host out at the full native width and makes the **root** the scroller (`width: 100%`, `overflow-x: auto`, `overflow-y: hidden`) so it pans to reveal the wide host.
- The `.xterm-viewport` is always `overflow-x: hidden` (never an inner horizontal scroller). Its `touch-action` is `pan-x pan-y` in 1:1 mode (forward horizontal touch drags to the root scroller) and `pan-y` in fit mode. Because `overflow-x` is hidden, `pan-x` never causes inner horizontal scroll — it only lets the root scroller pan. `allowHorizontalScroll` was removed from the runtime (the component still uses the prop for root layout).
- Transform is cleared on unmount and in the non-force-cols branch.


## Verification

### Automated checks
- `cd app && npx tsc --noEmit` ✅
- `cd app && npx expo lint --max-warnings 0` on all changed files ✅
- `cd app && npm test -- --run src/screens/tmux-pane-xterm-screen.test.tsx` ✅ 22/22
- `cd app && npm test -- --run src/terminal/runtime/terminal-emulator-runtime.browser.test.ts` ✅ 19/20 — the one failure (`reports a larger PTY size when the terminal container grows`) is a **pre-existing cold-start flake** that also fails on the unmodified baseline (verified by stashing).
- Related tmux tests (`tmux-pane-screen`, `tmux-dashboard`, `use-tmux-capture-pane`) ✅ 104/104

### Browser regression tests added
- `scales the host down to fit the container when fitToWidth is set` — asserts `transform: scale(0.xx)` and that the terminal still reports the forced column count.
- `makes the root div the horizontal scroller in 1:1 mode (no outer RN ScrollView)` — asserts root `width: 100%` / `overflow-x: auto` and host width > container.
- `does not make the root scrollable in fit mode` — asserts root `overflow-x: hidden`.
- `never lets the xterm viewport scroll horizontally, even when forceCols expands the host` — guards the single-scroller invariant (viewport `overflow-x: hidden`).

## Known Risks / Follow-ups
1. **Fit-mode legibility.** Scale-to-fit shows the whole pane but text can be small when the pane is much wider than the phone (e.g. 200 cols → ~0.4× scale). This is intentional ("see everything"); the 1:1 toggle exists for reading. Pinch-zoom is not implemented (hard to coordinate with the Expo DOM WebView); the toggle is the zoom control.
2. **Refresh / scroll-offset preservation.** Horizontal pan offset is the root div's `scrollLeft` (inside the DOM component) and survives xterm `clear()`/`write()` refreshes (no remount; stable `streamKey` per mode). Vertical xterm scroll still resets to the bottom on a content change (pre-existing, same as before).
3. **Custom vertical scrollbar in 1:1 mode.** The terminal's custom vertical scrollbar overlay is absolutely positioned within the root, so when the root is scrolled horizontally it sits at the far right of the wide content (may be off-screen until panned). Vertical scrolling still works via touch/wheel; only the drag-handle affordance is misplaced in 1:1 mode. Fix would require `position: sticky` or hoisting the handle out of the scroll container.
4. **Dead daemon rewrap path.** `wrapContentToCols` / the `cols` request path in `captureTmuxPane` is now unused by the app. Left in place (protocol still allows `cols`); remove if no other consumer needs it.
5. **Physical-device verification** remains the final proof (Android/iOS WebView horizontal scroll, fit-mode scale rendering).

## How to Test Manually
1. Connect an Android device or emulator.
2. `cd app && APP_VARIANT=production npx expo run:android --variant=release`
3. Open `/tmux-pane-xterm` on a pane wider than the phone (e.g. an agent TUI or a wide table).
4. **Fit mode (default)**: the entire pane width should be visible (scaled down), with box-drawing/tables intact (not rewrapped/broken).
5. Tap **1:1**: terminal renders at native size; swipe left/right to read the full width.
6. Tap **Fit** to return to the scaled full view.

## Files Touched
- `app/src/screens/tmux-pane-xterm-screen.tsx`
- `app/src/screens/tmux-pane-xterm-screen.test.tsx`
- `app/src/components/terminal-emulator.tsx`
- `app/src/terminal/runtime/terminal-emulator-runtime.ts`
- `app/src/terminal/runtime/terminal-emulator-runtime.browser.test.ts`
- `docs/handoff.md`
