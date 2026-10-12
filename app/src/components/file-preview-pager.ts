export interface FilePreviewPagerMetrics {
  offset: number;
  viewportHeight: number;
  contentHeight: number;
}

export interface FilePreviewPagerState {
  canPageUp: boolean;
  canPageDown: boolean;
  pageUpOffset: number;
  pageDownOffset: number;
}

// Distance from a boundary within which the pager treats the view as "at the edge".
export const PAGER_EDGE_EPSILON = 8;
// A "page" keeps 10% of the viewport as context overlap, matching reader conventions.
export const PAGER_PAGE_FRACTION = 0.9;

export function getFilePreviewPagerState(metrics: FilePreviewPagerMetrics): FilePreviewPagerState {
  const maxOffset = Math.max(0, metrics.contentHeight - metrics.viewportHeight);
  const offset = Math.min(Math.max(metrics.offset, 0), maxOffset);
  const pageStep = metrics.viewportHeight * PAGER_PAGE_FRACTION;

  const canPageUp = offset > PAGER_EDGE_EPSILON;
  const canPageDown = offset < maxOffset - PAGER_EDGE_EPSILON;

  return {
    canPageUp,
    canPageDown,
    pageUpOffset: Math.max(0, offset - pageStep),
    pageDownOffset: Math.min(maxOffset, offset + pageStep),
  };
}
