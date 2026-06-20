# Handoff: Tmux Pane Original-Width Horizontal Scroll

## Goal
Add a toggle on the `/tmux-pane-xterm` screen that switches between:
1. **Fit mode** (default): content is rewrapped to the phone's visible column width.
2. **Original-width mode**: xterm renders at the tmux pane's native column width and the user can pan left/right to see the full pane.

## What Changed

### Daemon / protocol
- `TmuxCapturePaneResponsePayload` now includes `paneCols` (pane native width).
- `captureTmuxPane` returns `(content, paneCols, error)`.
- When the app requests `cols`, the daemon rewraps output to that width; when `cols` is omitted, raw pane output is returned.

### App types
- `app-bridge/src/server/tmux/rpc-schemas.ts`: `paneCols` added to `TmuxCapturePaneResponseSchema`.
- `app/src/hooks/use-tmux-capture-pane.ts`: result now exposes `paneCols`; query omits `cols` in original mode.

### Tmux pane screen
- Added `viewMode: "fit" | "original"` state.
- Added **Width** toggle button in the bottom key bar (next to Refresh).
- In original mode the terminal is wrapped in a horizontal `ScrollView` sized by `contentContainerStyle={{ minWidth: estimatedOriginalWidth, alignItems: 'flex-start' }}`.
- Estimated original width is computed from `paneCols / measuredCols × screenWidth`.

### Terminal emulator
- New props: `forceCols?: number` and `allowHorizontalScroll?: boolean`.
- Runtime reacts to prop changes via `setForceCols` / `setAllowHorizontalScroll` without remounting.
- When `forceCols` is set, the runtime first fits to the container to measure the real cell width, then expands `host` and `root` to `cellWidth × forcedCols` and resizes xterm.
- `.xterm-viewport` gets `overflow-x: auto` / `touch-action: pan-x pan-y` and `.xterm-screen` gets `min-width: max-content` in original mode.

## Verification

### Automated checks
- `cd app && npx tsc --noEmit` ✅
- `cd app && npx expo lint --max-warnings 0 src/screens/tmux-pane-xterm-screen.tsx src/components/terminal-emulator.tsx src/terminal/runtime/terminal-emulator-runtime.ts` ✅
- `cd app && npm test -- --run src/screens/tmux-pane-xterm-screen.test.tsx` ✅ 13/13
- `cd app-bridge && npx tsc --noEmit && npx eslint src/` ✅

### Web E2E check
A temporary Playwright harness confirmed that the horizontal `ScrollView` pattern produces:
- scrollable viewport width = 400 px
- content width = 1200 px
- `overflow-x: auto`
- programmatic scroll from `scrollLeft = 0` to `scrollLeft = 800` succeeds.

### Android deploy
- Release APK built with `APP_VARIANT=production npx expo run:android --variant=release`.
- APK installed and launched on connected device `PLT140`.

## Known Risks / Follow-ups
1. **Android gesture interaction**: Web E2E confirms the RN-level scroller layout is correct, but the physical-device test is the final proof. If horizontal panning still does not work on Android, the next thing to inspect is whether the Expo DOM component WebView is intercepting horizontal gestures. Remedies to try:
   - Set `dom.scrollEnabled: false` in original mode and let the RN `ScrollView` own all scrolling (vertical history may need an alternative).
   - Or wrap the WebView in a native `HorizontalScrollView` and size the DOM component to `estimatedOriginalWidth`.
2. **Refresh frequency**: `useTmuxCapturePane` refetches every 500 ms–5 s depending on activity. The horizontal scroller should preserve its scroll offset across content updates because only the snapshot text changes; the component does not remount. If the offset resets, investigate whether snapshot writes trigger a full xterm reset/resize.
3. **Estimated width accuracy**: `estimatedOriginalWidth` uses the fit-mode cell width as a proxy. On devices with unusual font metrics this may be slightly off, causing minor right-padding or minor clipping. The fix is to measure the actual xterm cell width after render and feed it back, but that requires a native↔web round trip.

## How to Test Manually
1. Connect an Android device or emulator.
2. `cd app && APP_VARIANT=production npx expo run:android --variant=release`
3. Open `/tmux-pane-xterm`.
4. Tap **Width** in the bottom key bar (should highlight and label flips to **Fit**).
5. Swipe left/right; the full tmux pane width should be reachable.
6. Tap **Fit** to return to phone-width wrapping.

## Files Touched
- `protocol/message_tmux.go`
- `daemon/internal/server/session_tmux.go`
- `app-bridge/src/server/tmux/rpc-schemas.ts`
- `app/src/hooks/use-tmux-capture-pane.ts`
- `app/src/screens/tmux-pane-xterm-screen.tsx`
- `app/src/components/terminal-emulator.tsx`
- `app/src/terminal/runtime/terminal-emulator-runtime.ts`
