# Tmux Pane Refresh Jitter Analysis

> Analysis of why the tmux pane viewer in the app jitters on refresh, while the host tmux window remains stable. Includes root causes, contributing factors, and proposed solutions.

- Status: **Analysis Complete**
- Author: Andy
- Created: 2026-06-07
- Related: [tmux-pane-content-loading.md](../architecture/tmux-pane-content-loading.md), [tmux-transport-disposed-race.md](tmux-transport-disposed-race.md)

## 1. Problem Statement

The tmux pane viewer in the Solo app exhibits visible "jitter" (抖动) on every 5-second polling refresh. The host tmux terminal does not jitter, even when displaying ASCII animations (spinners, progress bars) in the bottom status line.

## 2. Core Architecture Difference

The fundamental difference is the rendering model:

| | Host tmux | App tmux pane |
|---|---|---|
| Rendering model | Cell-based grid, incremental updates | Full string → full React tree re-render |
| Data flow | Real-time streaming | 5s polling with full content replacement |
| Scroll management | Terminal-internal cell grid | React Native ScrollView + setTimeout |
| Background color | Fixed | Dynamically detected from content |
| Box drawing | Rendered as-is | Stripped during ANSI parsing |

## 3. Root Causes

### 3.1 Full Content Replacement on Poll

**File**: `app/src/hooks/use-tmux-capture-pane.ts` (lines 50-66)

Every 5 seconds, `useTmuxCapturePane` fetches the entire pane content via `tmux capture-pane -t {paneId} -p -e -S -200`. While `keepPreviousData` prevents a blank flash, the entire content string is replaced on each poll.

Impact chain:
1. `parseAnsi(content)` re-parses the full string into new segment array
2. `AnsiTextContent` re-renders all `<Text>` children
3. React Native recalculates ScrollView content size
4. Any minor difference (trailing whitespace, cursor position, ANSI state) triggers full re-render

### 3.2 Auto-Scroll-to-Bottom on Every Content Update

**File**: `app/src/screens/tmux-pane-screen.tsx` (lines 121-126)

```typescript
useEffect(() => {
  if (content && !isLoadingMore && autoRefresh) {
    const id = setTimeout(() => scrollToBottom(), 50);
    return () => clearTimeout(id);
  }
}, [content, isLoadingMore, autoRefresh, scrollToBottom]);
```

Every 5 seconds when new content arrives:
1. Content string changes (even if visually identical)
2. `useEffect` fires because `content` dependency changed
3. After 50ms delay, `scrollToEnd({ animated: true })` is called
4. ScrollView animates to the bottom → visible "jump"

The 50ms `setTimeout` is fragile — it assumes React Native layout completes within 50ms. On slower devices or large content, layout may not be complete, causing `scrollToEnd` to scroll to the wrong position.

### 3.3 Background Color Derived from Content

**File**: `app/src/screens/tmux-pane-screen.tsx` (lines 86-94)

```typescript
const terminalColors = useMemo(() => {
  if (terminalThemeId !== "system") {
    return TERMINAL_THEME_PRESETS[terminalThemeId] ?? theme.colors.terminal;
  }
  const contentColors = content
    ? detectColorsFromAnsi(content)
    : { background: null, foreground: null };
  return mergeTerminalColors(theme.colors.terminal, contentColors);
}, [terminalThemeId, theme.colors.terminal, content]);
```

When `terminalThemeId === "system"`, background/foreground colors are derived from content via `detectColorsFromAnsi()`. If the ANSI color codes change between polls, `terminalColors` changes, causing the entire ScrollView background to shift — a direct visual flash.

### 3.4 ScrollView Content Size Changes

**File**: `app/src/components/ansi-text-renderer.tsx`

`AnsiTextContent` renders each ANSI segment as nested `<Text>` elements. When segments change (even if visual output is similar), React Native recalculates text layout. If total content height changes (even by a pixel), ScrollView's `contentSize` changes, causing scroll position to shift.

### 3.5 Box Drawing Characters Stripped

**File**: `app/src/utils/ansi-parser.ts` (lines 219-223)

```typescript
const cp = input.codePointAt(i)!;
if ((cp >= 0x2500 && cp <= 0x259f) || (cp >= 0x2800 && cp <= 0x28ff)) {
  i += cp > 0xffff ? 2 : 1;
  continue;
}
```

All Box Drawing (`U+2500`–`U+259F`) and Braille (`U+2800`–`U+28FF`) characters are stripped. This was done because tmux capture returns content at the host's terminal width, which doesn't match the app's display width, causing box drawing frames to misalign.

Impact: TUI applications (htop, lazygit, claude-code) display without frames/borders, degrading user experience.

### 3.6 No Width Adaptation

The tmux capture pipeline has no width adaptation:
- `TmuxCapturePaneRequestSchema` sends only `paneId` and `startLine`, no width parameter
- Daemon executes `tmux capture-pane` without `-C` (crop) flag
- No reflow logic exists in the app to re-wrap lines to fit display width

### 3.7 `isLoadingMore` Flicker on Every Poll (Root Cause of Height Pulse)

**File**: `app/src/hooks/use-tmux-capture-pane.ts` (original formula)

The original hook derived `isLoadingMore` as:

```typescript
const isLoadingMore = useMemo(() => isFetching && !!data, [isFetching, data]);
```

During every 5-second poll, React Query sets `isFetching` to `true` while the fetch is in flight, then `false` when it completes. After the initial load (`!!data` is true), this makes `isLoadingMore` toggle `true → false` every 5 seconds.

**File**: `app/src/screens/tmux-pane-screen.tsx` (ScrollView children)

```typescript
{isLoadingMore && (
  <Text style={styles.loadingMoreText}>Loading more history...</Text>
)}
```

The screen renders a `<Text>` row conditioned on `isLoadingMore`. When `isLoadingMore` flashes true, the row is mounted; when it flashes false, it is unmounted. Each mount/unmount changes the ScrollView's `contentSize.height` by the height of that row, which fires `onContentSizeChange` and triggers a `scrollToEnd` adjustment. The net result is a visible **content-height pulse** every 5 seconds — even when the `AnsiTextContent` subtree is completely stable.

> **Why tests missed it**: the mock client resolves `tmuxCapturePane` synchronously, so `isFetching` transitions `true → false` inside the same tick and no intermediate render is observable. Only a real async fetch (production behavior) produces the flicker.

### 3.8 Missing `React.memo` on `AnsiTextContent` (Root Cause of Text-Tree Reconciliation)

**File**: `app/src/components/ansi-text-renderer.tsx` (original)

`AnsiTextContent` was exported as a plain function component. Even though the screen's `segments` memo produced a stable reference across polls, any re-render of the screen (e.g. caused by `isFetching` state transitions, `isLoadingMore` flashes, or parent component updates) would cascade into `AnsiTextContent`, forcing React Native to re-reconcile the entire nested `<Text>` tree. On Android, this reconciliation is expensive and produces visible jitter.

Without `React.memo`, **stable props do not prevent re-rendering** — the component re-renders whenever its parent does. The `<Text>` tree contains one nested `<Text>` per ANSI segment (dozens to hundreds of nodes), so the reconciliation cost is proportional to content length.

## 4. Why Host tmux Does Not Jitter

Host tmux uses **cell-based rendering** — it maintains an internal grid of characters with their styles, and only redraws cells that have actually changed. ASCII animations (spinners, progress bars) update specific cells without affecting layout or scroll position.

- Host tmux: incremental cell update → only changed pixels redraw
- App: full content string → full parse → full React tree re-render → ScrollView relayout → scroll position adjustment

## 5. Contributing Code Locations

| Factor | File | Lines | Issue |
|---|---|---|---|
| Full content replacement on poll | `use-tmux-capture-pane.ts` | 50-66 | 5s polling replaces entire content string |
| Auto-scroll on every content change | `tmux-pane-screen.tsx` | 121-126 | `scrollToEnd({ animated: true })` fires on every `content` change |
| Color detection from content | `tmux-pane-screen.tsx` | 86-94 | Background color derived from content, changes on poll |
| Background style bound to content colors | `tmux-pane-screen.tsx` | ~307 | `backgroundColor: terminalColors.background` on ScrollView |
| 50ms scroll delay | `tmux-pane-screen.tsx` | 123 | Fragile timing assumption for layout completion |
| ANSI re-parse on every content change | `tmux-pane-screen.tsx` | 95-98 | `parseAnsi(content)` produces new segment array each time |
| React Native Text re-render | `ansi-text-renderer.tsx` | 17-28 | All `<Text>` children recreated when segments reference changes |
| **Missing `React.memo` on `AnsiTextContent`** | `ansi-text-renderer.tsx` | 13 | No bail-out on stable props → entire `<Text>` tree reconciled every parent re-render |
| **`isLoadingMore` flicker on poll** | `use-tmux-capture-pane.ts` + `tmux-pane-screen.tsx` | 79 / 347 | `isFetching && !!data` toggles every poll → "Loading more..." row mount/unmount → content-height pulse |
| Box drawing stripped | `ansi-parser.ts` | 219-223 | All box drawing/braille characters removed |
| No width parameter in capture | `rpc-schemas.ts` | 517-522 | `TmuxCapturePaneRequestSchema` has no width field |

## 6. Proposed Solutions

### 6.1 Priority Matrix (Revised After Implementation)

| Priority | Solution | Effect | Effort |
|---|---|---|---|
| P0 | `AnsiTextContent` wrapped in `React.memo` | Block `<Text>` tree reconciliation when props are reference-stable | Small |
| P0 | Pagination-only `isLoadingMore` (`isPaginating`) | Eliminate 5s content-height pulse from "Loading more..." row | Medium |
| P0 | Query result deduplication | Stabilize `content` string reference → downstream memos skip | Small |
| P1 | `onContentSizeChange` + `animated: false` | Precise scroll timing, no animation jitter | Medium |
| P1 | Stable background color (cache detected colors per pane) | Eliminate background flash | Small |
| P2 | `maintainVisibleContentPosition` | Native-level scroll stability | Small |
| P2 | Daemon-side `-C` width crop | Proper width adaptation, preserve box drawing | Medium |
| P3 | Daemon strip trailing empty lines | Reduce capture diffs | Small |
| P4 | xterm.js renderer for tmux | Full cell-based rendering, no jitter | Large |

> **Implementation note**: the original matrix ranked `AnsiTextContent` memo and `isLoadingMore` separation as P3. On-device debugging (§9) revealed these were the dominant visible-jitter sources after content dedup, so they were promoted to P0 and implemented in the same release.

### 6.2 P0: Query Result Deduplication

**File**: `app/src/hooks/use-tmux-capture-pane.ts`

```typescript
const prevResultRef = useRef<{ content: string } | null>(null);

queryFn: async () => {
  const payload = await withLiveTmuxClient(serverId, (c) =>
    c.tmuxCapturePane(paneId, -scrollbackLines),
  );
  const newContent = payload.content ?? "";
  // Return same reference if content unchanged
  if (prevResultRef.current && prevResultRef.current.content === newContent) {
    return prevResultRef.current;
  }
  const result = { content: newContent, error: payload.error ?? null };
  prevResultRef.current = result;
  return result;
},
```

When tmux content is unchanged (common during idle), `data` reference stays stable, preventing all downstream `useMemo` recalculations.

> **Caveat**: This only eliminates jitter for truly idle panes. If the pane has a spinner, progress bar, or any ASCII animation (the exact scenario described in Section 1), content changes every frame and this fix has no effect. See 6.8 (xterm.js) for that case.

### 6.3 P0: Compare Content Before Scroll

**File**: `app/src/screens/tmux-pane-screen.tsx`

```typescript
const prevContentRef = useRef<string | null>(null);
useEffect(() => {
  if (content && !isLoadingMore && autoRefresh) {
    if (content === prevContentRef.current) return;
    prevContentRef.current = content;
    const id = setTimeout(() => scrollToBottom(), 50);
    return () => clearTimeout(id);
  }
}, [content, isLoadingMore, autoRefresh, scrollToBottom]);
```

### 6.4 P1: Replace setTimeout with onContentSizeChange

**File**: `app/src/screens/tmux-pane-screen.tsx`

```typescript
const contentHeightRef = useRef(0);
const isAtBottomRef = useRef(true);
const scrollThrottleRef = useRef(false);

const handleContentSizeChange = useCallback((_w: number, h: number) => {
  if (h !== contentHeightRef.current && isAtBottomRef.current && autoRefresh) {
    contentHeightRef.current = h;
    if (scrollThrottleRef.current) return;
    scrollThrottleRef.current = true;
    setTimeout(() => {
      scrollRef.current?.scrollToEnd({ animated: false });
      scrollThrottleRef.current = false;
    }, 100);
  }
}, [autoRefresh]);

<ScrollView
  onContentSizeChange={handleContentSizeChange}
  onScroll={handleScroll}
  scrollEventThrottle={16}
>
```

Key points:
- `animated: false` eliminates scroll animation jitter
- `onContentSizeChange` fires after layout, so scroll position is accurate
- 100ms throttle prevents scroll churn during rapid content updates (e.g. ASCII animations)
- Only auto-scrolls when user is already at bottom

### 6.5 P1: Stable Background Color

**File**: `app/src/screens/tmux-pane-screen.tsx`

Cache detected colors with `useRef`, reset only when `paneId` changes:

```typescript
const cachedColorsRef = useRef<...>(null);
const prevPaneIdRef = useRef(paneId);

const terminalColors = useMemo(() => {
  if (terminalThemeId !== "system") {
    return TERMINAL_THEME_PRESETS[terminalThemeId] ?? theme.colors.terminal;
  }
  // Reset cache on pane change
  if (paneId !== prevPaneIdRef.current) {
    prevPaneIdRef.current = paneId;
    cachedColorsRef.current = null;
  }
  // Read content via ref to avoid re-triggering memo on every poll
  if (!cachedColorsRef.current && contentRef.current) {
    cachedColorsRef.current = detectColorsFromAnsi(contentRef.current);
  }
  return mergeTerminalColors(theme.colors.terminal, cachedColorsRef.current ?? { background: null, foreground: null });
  // eslint-disable-next-line react-hooks/exhaustive-deps -- content read via ref; re-detect only on pane change
}, [terminalThemeId, theme.colors.terminal, paneId]);
```

A `contentRef` is kept in sync via a separate `useEffect`, so `detectColorsFromAnsi` runs only once per pane, not on every poll.

### 6.6 P2: Daemon-Side Width Crop (Preserve Box Drawing)

**Go — `daemon/internal/server/session_tmux.go`**:

```go
func captureTmuxPane(paneID string, startLine int, width int) (string, error) {
    args := []string{"capture-pane", "-t", paneID, "-p", "-e", "-S", strconv.Itoa(startLine)}
    if width > 0 {
        args = append(args, "-C", strconv.Itoa(width))
    }
    out, err := exec.Command("tmux", args...).Output()
    ...
}
```

**TypeScript — `rpc-schemas.ts`**:

```typescript
export const TmuxCapturePaneRequestSchema = z.object({
  type: z.literal("tmux/capture_pane"),
  paneId: z.string(),
  startLine: z.number().int().optional(),
  width: z.number().int().optional(),  // NEW: target columns
  requestId: z.string(),
});
```

**App — `use-tmux-capture-pane.ts`**:

```typescript
const { width } = useWindowDimensions();
const cols = Math.floor(width / CHARACTER_WIDTH);

const payload = await withLiveTmuxClient(serverId, (c) =>
  c.tmuxCapturePane(paneId, -scrollbackLines, cols),
);
```

This allows removing the box-drawing strip logic in `ansi-parser.ts` since tmux will handle width-adapted output.

> **Caveat**: `tmux capture-pane -C` truncates to the specified column width — it does not reflow lines. This only solves the case where the app's display is narrower than the host terminal. If the app is wider, lines will still be short and box frames will not extend to fill the display.

### 6.7 P2: maintainVisibleContentPosition

```typescript
<ScrollView
  maintainVisibleContentPosition={{ minIndexForVisible: 0 }}
>
```

React Native's native solution for "content changes but visible area should stay stable." Automatically adjusts offset when content is inserted/removed.

> **Platform note**: `maintainVisibleContentPosition` is well-supported on iOS. Android support was added in later RN versions and behavior can be inconsistent — test on both platforms.

### 6.8 P4: xterm.js Renderer (Long-term)

Replace the `ScrollView` + `AnsiTextContent` approach with xterm.js, similar to `terminal-emulator.tsx`. xterm.js maintains a cell grid internally, supports incremental updates, and renders box drawing/braille correctly.

This is the only solution that fully eliminates jitter for ASCII animations, since xterm.js only redraws changed cells.

## 7. Recommended Execution Order (Revised)

1. **Layer 1 (P0)** — Content reference dedup + per-pane reset → stabilizes `content` and all downstream memos
2. **Layer 2 (P0)** — Pagination-only `isLoadingMore` → eliminates the 5s height pulse
3. **Layer 3 (P0)** — `React.memo` on `AnsiTextContent` → blocks Text-tree reconciliation
4. **P1** — `onContentSizeChange` + `animated: false` + color cache → polish
5. **P2** — Daemon `-C` width crop + `maintainVisibleContentPosition` → future work
6. **P4** — xterm.js renderer → long-term replacement matching host tmux cell-based model

## 8. Open Questions

1. **CJK character width**: How to calculate `CHARACTER_WIDTH` for mixed CJK/ASCII content? May need `wcwidth` or equivalent.
2. **Dynamic width changes**: When device rotates or window resizes, should we re-capture with new width?
3. **Box drawing + reflow**: If using daemon-side `-C` crop, will box drawing frames still break when the crop splits a frame line?
4. **Performance**: Will `onContentSizeChange` fire too frequently during ASCII animations?

## 9. On-Device Debug Overlay Methodology

Unit tests alone were not sufficient to localize the jitter. Tests use a synchronous mock client, so `isFetching` transitions `true → false` inside the same React tick and no intermediate render is observable. The visible on-device symptom ("every 5 seconds something pulses") required a runtime probe.

### 9.1 What Was Measured

A temporary fixed-position overlay was added to `TmuxPaneScreenInner` that rendered, once per second, a set of counters maintained in a module-level `debugRef` object:

| Counter | Purpose |
|---|---|
| `scr` (screenRenders) | Total screen re-renders (expected to rise with each poll) |
| `cΔ` (contentChanges) | Number of times the `content` string reference changed |
| `col` (terminalColorsChanges) | Number of times the `terminalColors` memo produced a new reference |
| `load+` (isLoadingMoreFlashes) | Number of `false → true` transitions of `isLoadingMore` |
| `s2b` (scrollToEndCalls) | Number of `scrollToEnd` invocations |
| `csz` (handleContentSizeFires) | Number of `onContentSizeChange` callbacks |
| `ansi` | Number of times the `AnsiTextContent` subtree actually re-rendered |

### 9.2 First Reading — Misleading Signal

With the dedup + pagination-only `isLoadingMore` fixes already in place, the overlay showed:

- `scr` and `ansi`: **rising continuously**, in lockstep
- All other counters: stable

This *looked* like `AnsiTextContent` was re-rendering on every screen render, which would mean `React.memo` was not bailing out. But that contradicted the unit test `is wrapped in React.memo so stable props do not cause re-renders`, which passed.

### 9.3 The Trap — Debug Wrapper Defined Inside the Component

The counter was implemented as:

```typescript
function TmuxPaneScreenInner() {
  ...
  function AnsiTextContentDebug(props) {
    ansiRenderCount.n += 1;
    return <AnsiTextContent {...props} />;
  }
  ...
  <AnsiTextContentDebug segments={segments} ... />
}
```

Defining `AnsiTextContentDebug` **inside** `TmuxPaneScreenInner` made it a brand-new component type on every screen render. React treats each new function identity as a distinct component and unmounts the previous instance before mounting the new one. The `<Text>` subtree was being destroyed and rebuilt every 5 seconds — **the debug wrapper itself was the dominant jitter source**.

### 9.4 Corrected Reading — Confirming the Fix

Moving `AnsiTextContentDebug` to module scope and wrapping it in `React.memo`:

```typescript
const AnsiTextContentDebug = React.memo(function AnsiTextContentDebug(props) {
  ansiRenderCount.n += 1;
  return <AnsiTextContent {...props} />;
});
```

Result on device:

- `ansi`: stabilized at `1` (initial mount only)
- `cΔ`, `col`, `load+`, `s2b`, `csz`: stable at `0` after initial load
- `scr`: continued rising (expected — React Query state transitions still re-render the screen)
- **Visible jitter disappeared entirely**

This confirmed that the production `AnsiTextContent` (already `React.memo`-wrapped in `ansi-text-renderer.tsx`) was correctly bailing out. The remaining screen re-renders did not propagate into the `<Text>` subtree.

### 9.5 Lesson

When instrumenting a React component tree for runtime diagnosis, always:
1. Define probe wrappers at **module scope** so React sees them as stable component types.
2. Wrap them in `React.memo` to match the optimization path of the component under test.
3. Otherwise, the probe itself becomes the dominant source of the behavior being measured.

## 10. Final Resolution (Shipped in `@getsolo/app` 0.4.1)

The jitter was eliminated by three layered fixes, each blocking a distinct re-render path. Any single fix alone was insufficient — removing one of them re-introduced visible jitter on device.

### 10.1 Layer 1 — Content Reference Stabilization

**File**: `app/src/hooks/use-tmux-capture-pane.ts`

`queryFn` returns the same object reference when the payload is byte-identical to the previous fetch. React Query's `structuralSharing` then preserves `data`, so the downstream `useMemo` in the screen short-circuits `parseAnsi` and `terminalColors`.

```typescript
const prevResultRef = useRef<{ content: string; error: string | null } | null>(null);

useEffect(() => {
  setScrollbackLines(DEFAULT_SCROLLBACK_LINES);
  prevResultRef.current = null; // scope dedup per pane
}, [paneId]);

queryFn: async () => {
  const payload = await withLiveTmuxClient(serverId, (c) =>
    c.tmuxCapturePane(paneId, -scrollbackLines),
  );
  const newContent = payload.content ?? "";
  if (prevResultRef.current && prevResultRef.current.content === newContent) {
    return prevResultRef.current;
  }
  const result = { content: newContent, error: payload.error ?? null };
  prevResultRef.current = result;
  return result;
},
```

### 10.2 Layer 2 — Pagination-Only `isLoadingMore`

**File**: `app/src/hooks/use-tmux-capture-pane.ts`

`isLoadingMore` was rewritten from `isFetching && !!data` to an `isPaginating` state that is only set when `loadMoreHistory()` changes `scrollbackLines`. Poll refetches (which don't change `scrollbackLines`) leave `isLoadingMore === false`, so the "Loading more history..." row is never mounted/unmounted on the 5s cycle.

```typescript
const isPaginatingRef = useRef(false);
const prevScrollbackRef = useRef(scrollbackLines);
const [isPaginating, setIsPaginating] = useState(false);
if (scrollbackLines !== prevScrollbackRef.current) {
  prevScrollbackRef.current = scrollbackLines;
  isPaginatingRef.current = true;
  setIsPaginating(true);
}
useEffect(() => {
  if (isPaginatingRef.current && !isFetching) {
    isPaginatingRef.current = false;
    setIsPaginating(false);
  }
}, [isFetching]);

const isLoadingMore = isPaginating;
```

### 10.3 Layer 3 — `React.memo` on `AnsiTextContent`

**File**: `app/src/components/ansi-text-renderer.tsx`

Wrapping `AnsiTextContent` in `React.memo` makes stable `segments` / `terminalColors` / `style` props short-circuit the entire `<Text>` subtree reconciliation on Android. This is the last line of defense: even when the screen re-renders for unrelated state transitions, the `<Text>` tree is not touched.

```typescript
export const AnsiTextContent = React.memo(function AnsiTextContent({
  segments, style, terminalColors,
}: AnsiTextContentProps) {
  ...
});
```

### 10.4 TDD Contracts

Each layer has a corresponding regression test that fails without the fix:

| Test | File | Layer it protects |
|---|---|---|
| `preserves content STRING identity across many polls with identical payload` | `use-tmux-capture-pane.test.ts` | Layer 1 — content reference stability |
| `resets dedup cache when paneId changes so new pane gets fresh content` | `use-tmux-capture-pane.test.ts` | Layer 1 — per-pane scoping of dedup cache |
| `does NOT set isLoadingMore during a poll refetch with unchanged scrollback` | `use-tmux-capture-pane.test.ts` | Layer 2 — pagination-only loading flag |
| `does NOT re-parse ANSI content when content reference is stable across re-renders` | `tmux-pane-screen.test.tsx` | Layer 1+2 — screen-level memo short-circuit |
| `is wrapped in React.memo so stable props do not cause re-renders` | `ansi-text-renderer.test.tsx` | Layer 3 — component bail-out |

All 1553 unit tests pass in the final tree.

### 10.5 Verification

- Device: PLT140 (Android)
- APK: `app-release.apk` built via `APP_VARIANT=production ./gradlew assembleRelease`
- Scenario: tmux pane with static shell prompt, no ASCII animation, auto-refresh on, 5s poll interval
- Observation over 60 seconds: no visible pulse, no scroll shift, no background flash
- Debug overlay confirmed: `ansi=1` (stable), all other counters stable after initial load
