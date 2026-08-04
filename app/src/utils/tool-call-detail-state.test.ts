import { describe, expect, it } from "vitest";

import type { ToolCallDetail } from "@server/server/agent/agent-sdk-types";
import { hasMeaningfulToolCallDetail, isPendingToolCallDetail } from "./tool-call-detail-state";

describe("tool-call detail state", () => {
  it("treats empty unknown payloads as not meaningful", () => {
    const detail: ToolCallDetail = {
      type: "unknown",
      input: {},
      output: null,
    };

    expect(hasMeaningfulToolCallDetail(detail)).toBe(false);
  });

  it("treats partial unknown payloads with real values as meaningful", () => {
    const detail: ToolCallDetail = {
      type: "unknown",
      input: { path: "src/index.ts" },
      output: null,
    };

    expect(hasMeaningfulToolCallDetail(detail)).toBe(true);
  });

  it("marks running calls with no meaningful detail as pending", () => {
    expect(
      isPendingToolCallDetail({
        detail: {
          type: "unknown",
          input: {},
          output: null,
        },
        status: "running",
        error: null,
      }),
    ).toBe(true);
  });

  it("does not mark completed calls as pending", () => {
    expect(
      isPendingToolCallDetail({
        detail: {
          type: "unknown",
          input: {},
          output: null,
        },
        status: "completed",
        error: null,
      }),
    ).toBe(false);
  });

  it("treats enriched search detail as meaningful", () => {
    const detail: ToolCallDetail = {
      type: "search",
      query: "needle",
      toolName: "grep",
      content: "12:needle",
      filePaths: ["src/index.ts"],
    };

    expect(hasMeaningfulToolCallDetail(detail)).toBe(true);
  });

  it("treats fetch detail as meaningful", () => {
    const detail: ToolCallDetail = {
      type: "fetch",
      url: "https://example.com",
      result: "Fetched summary",
    };

    expect(hasMeaningfulToolCallDetail(detail)).toBe(true);
  });
});
