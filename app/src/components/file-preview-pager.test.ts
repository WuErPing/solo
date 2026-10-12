import { describe, expect, it } from "vitest";
import {
  getFilePreviewPagerState,
  PAGER_EDGE_EPSILON,
  PAGER_PAGE_FRACTION,
} from "./file-preview-pager";

describe("getFilePreviewPagerState", () => {
  it("disables both directions when content fits the viewport", () => {
    const state = getFilePreviewPagerState({ offset: 0, viewportHeight: 600, contentHeight: 400 });

    expect(state.canPageUp).toBe(false);
    expect(state.canPageDown).toBe(false);
    expect(state.pageUpOffset).toBe(0);
    expect(state.pageDownOffset).toBe(0);
  });

  it("only pages down from the top of a long document", () => {
    const state = getFilePreviewPagerState({ offset: 0, viewportHeight: 600, contentHeight: 2000 });

    expect(state.canPageUp).toBe(false);
    expect(state.canPageDown).toBe(true);
    expect(state.pageDownOffset).toBe(600 * PAGER_PAGE_FRACTION);
  });

  it("only pages up from the bottom of a long document", () => {
    const state = getFilePreviewPagerState({
      offset: 1400,
      viewportHeight: 600,
      contentHeight: 2000,
    });

    expect(state.canPageUp).toBe(true);
    expect(state.canPageDown).toBe(false);
    expect(state.pageUpOffset).toBe(1400 - 600 * PAGER_PAGE_FRACTION);
  });

  it("pages both ways in the middle of a long document", () => {
    const state = getFilePreviewPagerState({ offset: 700, viewportHeight: 600, contentHeight: 2000 });

    expect(state.canPageUp).toBe(true);
    expect(state.canPageDown).toBe(true);
    expect(state.pageUpOffset).toBe(160);
    expect(state.pageDownOffset).toBe(1240);
  });

  it("clamps the page-down target to the last full viewport", () => {
    const state = getFilePreviewPagerState({
      offset: 1200,
      viewportHeight: 600,
      contentHeight: 2000,
    });

    expect(state.canPageDown).toBe(true);
    expect(state.pageDownOffset).toBe(1400);
  });

  it("clamps the page-up target to the top", () => {
    const state = getFilePreviewPagerState({ offset: 200, viewportHeight: 600, contentHeight: 2000 });

    expect(state.canPageUp).toBe(true);
    expect(state.pageUpOffset).toBe(0);
  });

  it("treats offsets within the edge epsilon as boundaries", () => {
    const top = getFilePreviewPagerState({
      offset: PAGER_EDGE_EPSILON - 1,
      viewportHeight: 600,
      contentHeight: 2000,
    });
    const bottom = getFilePreviewPagerState({
      offset: 1400 - PAGER_EDGE_EPSILON + 1,
      viewportHeight: 600,
      contentHeight: 2000,
    });

    expect(top.canPageUp).toBe(false);
    expect(bottom.canPageDown).toBe(false);
  });

  it("clamps out-of-range offsets before evaluating boundaries", () => {
    const state = getFilePreviewPagerState({
      offset: 9999,
      viewportHeight: 600,
      contentHeight: 2000,
    });

    expect(state.canPageDown).toBe(false);
    expect(state.pageUpOffset).toBe(1400 - 600 * PAGER_PAGE_FRACTION);
  });

  it("stays disabled before layout metrics arrive", () => {
    const state = getFilePreviewPagerState({ offset: 0, viewportHeight: 0, contentHeight: 0 });

    expect(state.canPageUp).toBe(false);
    expect(state.canPageDown).toBe(false);
    expect(state.pageUpOffset).toBe(0);
    expect(state.pageDownOffset).toBe(0);
  });
});
