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
| Box drawing stripped | `ansi-parser.ts` | 219-223 | All box drawing/braille characters removed |
| No width parameter in capture | `rpc-schemas.ts` | 517-522 | `TmuxCapturePaneRequestSchema` has no width field |

## 6. Proposed Solutions

### 6.1 Priority Matrix

| Priority | Solution | Effect | Effort |
|---|---|---|---|
| P0 | Query result deduplication | Eliminate idle re-renders when content unchanged | Small |
| P0 | Compare content before scroll | Skip scroll when content identical | Small |
| P1 | `onContentSizeChange` + `animated: false` | Precise scroll timing, no animation jitter | Medium |
| P1 | Stable background color | Eliminate background flash | Small |
| P2 | `maintainVisibleContentPosition` | Native-level scroll stability | Small |
| P2 | Daemon-side `-C` width crop | Proper width adaptation, preserve box drawing | Medium |
| P3 | `AnsiTextContent` memo | Reduce text tree re-renders | Medium |
| P3 | Daemon strip trailing empty lines | Reduce capture diffs | Small |
| P4 | xterm.js renderer for tmux | Full cell-based rendering, no jitter | Large |

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

**Note**: This solution does NOT help when ASCII animations are present (content changes every frame). See 6.6 for that case.

### 6.3 P0: Compare Content Before Scroll

**File**: `app/src/screens/tmux-pane-screen.tsx`

```typescript
const prevContentRef = useRef(content);
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

const handleContentSizeChange = useCallback((_w: number, h: number) => {
  if (h !== contentHeightRef.current && isAtBottomRef.current && autoRefresh) {
    contentHeightRef.current = h;
    requestAnimationFrame(() => {
      scrollRef.current?.scrollToEnd({ animated: false });
    });
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
- `onContentSizeChange` ensures layout is complete before scrolling
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
  if (!cachedColorsRef.current && content) {
    cachedColorsRef.current = detectColorsFromAnsi(content);
  }
  return mergeTerminalColors(theme.colors.terminal, cachedColorsRef.current ?? { background: null, foreground: null });
}, [terminalThemeId, theme.colors.terminal, paneId]); // content removed from deps
```

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

### 6.7 P2: maintainVisibleContentPosition

```typescript
<ScrollView
  maintainVisibleContentPosition={{ minIndexForVisible: 0 }}
>
```

React Native's native solution for "content changes but visible area should stay stable." Automatically adjusts offset when content is inserted/removed.

### 6.8 P4: xterm.js Renderer (Long-term)

Replace the `ScrollView` + `AnsiTextContent` approach with xterm.js, similar to `terminal-emulator.tsx`. xterm.js maintains a cell grid internally, supports incremental updates, and renders box drawing/braille correctly.

This is the only solution that fully eliminates jitter for ASCII animations, since xterm.js only redraws changed cells.

## 7. Recommended Execution Order

1. **P0** — Query dedup + content comparison before scroll → eliminates most visible jitter for static content
2. **P1** — `onContentSizeChange` + `animated: false` + stable background → eliminates animation jitter and background flash
3. **P2** — Daemon width crop + `maintainVisibleContentPosition` → proper width adaptation, native scroll stability
4. **P3** — Memo + strip trailing lines → reduce unnecessary re-renders
5. **P4** — xterm.js renderer → complete solution matching host tmux behavior

## 8. Open Questions

1. **CJK character width**: How to calculate `CHARACTER_WIDTH` for mixed CJK/ASCII content? May need `wcwidth` or equivalent.
2. **Dynamic width changes**: When device rotates or window resizes, should we re-capture with new width?
3. **Box drawing + reflow**: If using daemon-side `-C` crop, will box drawing frames still break when the crop splits a frame line?
4. **Performance**: Will `onContentSizeChange` fire too frequently during ASCII animations?
