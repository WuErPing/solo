import { describe, expect, it, vi } from "vitest";
import {
  findLatestAssistantMessageText,
  getLatestPermissionRequest,
  createNotifyAgentAttention,
} from "./session-notifications";
import type { StreamItem } from "@/types/stream";
import type { SessionState } from "@/stores/session-store";

describe("findLatestAssistantMessageText", () => {
  it("returns null for empty array", () => {
    expect(findLatestAssistantMessageText([])).toBeNull();
  });

  it("returns null when no assistant messages", () => {
    const items: StreamItem[] = [
      { kind: "user_message", id: "1", text: "hello", timestamp: new Date() },
      { kind: "thought", id: "2", text: "thinking", timestamp: new Date(), status: "ready" },
    ];
    expect(findLatestAssistantMessageText(items)).toBeNull();
  });

  it("returns the last assistant message text", () => {
    const items: StreamItem[] = [
      { kind: "assistant_message", id: "1", text: "first", timestamp: new Date() },
      { kind: "user_message", id: "2", text: "hello", timestamp: new Date() },
      { kind: "assistant_message", id: "3", text: "second", timestamp: new Date() },
    ];
    expect(findLatestAssistantMessageText(items)).toBe("second");
  });

  it("returns first assistant message when only one exists", () => {
    const items: StreamItem[] = [
      { kind: "user_message", id: "1", text: "hello", timestamp: new Date() },
      { kind: "assistant_message", id: "2", text: "response", timestamp: new Date() },
    ];
    expect(findLatestAssistantMessageText(items)).toBe("response");
  });
});

describe("getLatestPermissionRequest", () => {
  it("returns null when session is undefined", () => {
    expect(getLatestPermissionRequest(undefined, "agent-1")).toBeNull();
  });

  it("returns null when no pending permissions", () => {
    const session = {
      pendingPermissions: new Map(),
      agents: new Map(),
    } as unknown as SessionState;
    expect(getLatestPermissionRequest(session, "agent-1")).toBeNull();
  });

  it("returns latest pending permission for agent from session", () => {
    const session = {
      pendingPermissions: new Map([
        ["a1:1", { key: "a1:1", agentId: "a1", request: { id: "1", name: "tool1", kind: "tool", provider: "p" } }],
        ["a1:2", { key: "a1:2", agentId: "a1", request: { id: "2", name: "tool2", kind: "tool", provider: "p" } }],
        ["a2:1", { key: "a2:1", agentId: "a2", request: { id: "3", name: "tool3", kind: "tool", provider: "p" } }],
      ]),
      agents: new Map(),
    } as unknown as SessionState;
    const result = getLatestPermissionRequest(session, "a1");
    expect(result).not.toBeNull();
    expect(result!.id).toBe("2");
  });
});

describe("createNotifyAgentAttention", () => {
  function createHandler(overrides?: {
    appState?: string;
    isActivelyVisible?: boolean;
    session?: SessionState | undefined;
    attentionNotified?: Map<string, number>;
  }) {
    const sendNotification = vi.fn();
    const appStateRef = { current: overrides?.appState ?? "background" };
    const attentionNotifiedRef = { current: overrides?.attentionNotified ?? new Map<string, number>() };

    const handler = createNotifyAgentAttention({
      serverId: "server-1",
      appStateRef,
      attentionNotifiedRef,
      getSessionState: () => overrides?.session,
      isAppActivelyVisible: () => overrides?.isActivelyVisible ?? false,
      sendNotification,
    });

    return { handler, sendNotification, attentionNotifiedRef };
  }

  it("skips notification when reason is error", () => {
    const { handler, sendNotification } = createHandler();
    handler({
      agentId: "agent-1",
      reason: "error",
      timestamp: "2026-01-01T00:00:00Z",
    });
    expect(sendNotification).not.toHaveBeenCalled();
  });

  it("skips when app is actively visible and focused on the agent", () => {
    const { handler, sendNotification } = createHandler({
      isActivelyVisible: true,
      session: { focusedAgentId: "agent-1" } as unknown as SessionState,
    });
    handler({
      agentId: "agent-1",
      reason: "finished",
      timestamp: "2026-01-01T00:00:00Z",
    });
    expect(sendNotification).not.toHaveBeenCalled();
  });

  it("sends notification when app is in background", () => {
    const { handler, sendNotification } = createHandler({
      isActivelyVisible: false,
    });
    handler({
      agentId: "agent-1",
      reason: "finished",
      timestamp: "2026-01-01T00:00:00Z",
    });
    expect(sendNotification).toHaveBeenCalledTimes(1);
    expect(sendNotification.mock.calls[0][0]).toHaveProperty("title");
    expect(sendNotification.mock.calls[0][0]).toHaveProperty("body");
  });

  it("deduplicates by timestamp — skips if already notified at same or later time", () => {
    const ref = new Map<string, number>();
    ref.set("agent-1", new Date("2026-01-01T00:00:00Z").getTime());
    const { handler, sendNotification } = createHandler({
      attentionNotified: ref,
    });
    handler({
      agentId: "agent-1",
      reason: "finished",
      timestamp: "2026-01-01T00:00:00Z",
    });
    expect(sendNotification).not.toHaveBeenCalled();
  });

  it("sends notification for newer timestamp after previous notification", () => {
    const ref = new Map<string, number>();
    ref.set("agent-1", new Date("2026-01-01T00:00:00Z").getTime());
    const { handler, sendNotification } = createHandler({
      attentionNotified: ref,
    });
    handler({
      agentId: "agent-1",
      reason: "permission",
      timestamp: "2026-01-01T01:00:00Z",
    });
    expect(sendNotification).toHaveBeenCalledTimes(1);
  });
});
