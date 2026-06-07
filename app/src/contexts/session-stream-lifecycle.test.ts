import { describe, expect, it } from "vitest";
import type { AgentStreamEventPayload } from "@server/shared/messages";
import { deriveOptimisticLifecycleStatus } from "./session-stream-lifecycle";

const turnCompletedEvent: AgentStreamEventPayload = {
  type: "turn_completed",
  provider: "claude",
};

const turnFailedEvent: AgentStreamEventPayload = {
  type: "turn_failed",
  provider: "claude",
  error: "failed",
};

const turnCanceledEvent: AgentStreamEventPayload = {
  type: "turn_canceled",
  provider: "codex",
  reason: "interrupted",
};

describe("session stream lifecycle helpers", () => {
  it("derives optimistic terminal lifecycle for running, idle, and initializing", () => {
    expect(deriveOptimisticLifecycleStatus("running", turnCompletedEvent)).toBe("idle");
    expect(deriveOptimisticLifecycleStatus("running", turnFailedEvent)).toBe("error");
    expect(deriveOptimisticLifecycleStatus("running", turnCanceledEvent)).toBe(null);
    expect(deriveOptimisticLifecycleStatus("idle", turnCompletedEvent)).toBe("idle");
    expect(deriveOptimisticLifecycleStatus("idle", turnFailedEvent)).toBe("error");
    expect(deriveOptimisticLifecycleStatus("initializing", turnCompletedEvent)).toBe("idle");
    expect(deriveOptimisticLifecycleStatus("initializing", turnFailedEvent)).toBe("error");
    expect(deriveOptimisticLifecycleStatus("closed", turnCompletedEvent)).toBe(null);
    expect(deriveOptimisticLifecycleStatus("closed", turnFailedEvent)).toBe(null);
    expect(deriveOptimisticLifecycleStatus("error", turnCompletedEvent)).toBe(null);
    expect(deriveOptimisticLifecycleStatus("error", turnFailedEvent)).toBe(null);
  });
});
