// Pure zoom math for MermaidZoomView. Each function carries a 'worklet'
// directive: gesture callbacks run on the UI (worklet) runtime, and calling a
// non-worklet function from there crashes with "Tried to synchronously call a
// Remote Function". Keeping the functions in this plain module also lets unit
// tests run in node without react-native-gesture-handler/reanimated imports.

/** Hard zoom bounds relative to the contain-fit scale. */
export const MAX_ZOOM_FACTOR = 8;
export const DOUBLE_TAP_ZOOM_FACTOR = 2.5;

/** Scale that fits the whole content inside the viewport (upscales small content). */
export function fitContainScale(
  contentWidth: number,
  contentHeight: number,
  viewportWidth: number,
  viewportHeight: number,
): number {
  "worklet";
  if (contentWidth <= 0 || contentHeight <= 0 || viewportWidth <= 0 || viewportHeight <= 0) {
    return 1;
  }
  return Math.min(viewportWidth / contentWidth, viewportHeight / contentHeight);
}

export function clampScale(scale: number, minScale: number, maxScale: number): number {
  "worklet";
  return Math.min(maxScale, Math.max(minScale, scale));
}

/**
 * Clamp a center-based translation so the (scaled) content cannot be dragged
 * past the point where its edge crosses the viewport center. Content smaller
 * than the viewport on an axis stays centered (offset 0).
 */
export function clampOffset(
  offset: number,
  contentSize: number,
  viewportSize: number,
  scale: number,
): number {
  "worklet";
  const half = Math.max(0, (contentSize * scale - viewportSize) / 2);
  const clamped = Math.min(half, Math.max(-half, offset));
  // Normalize -0 so a centered diagram reports a clean 0.
  return clamped === 0 ? 0 : clamped;
}
