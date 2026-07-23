import { describe, expect, it, vi } from "vitest";
import { createSendAgentMessage } from "./agent-operations";
import type { DaemonClient } from "@server/client/daemon-client";
import type { StreamItem } from "@/types/stream";

function createMockDeps(overrides?: {
  client?: Partial<DaemonClient> | null;
  currentHead?: StreamItem[];
  agent?: { status: string; persistence: { provider: string; sessionId: string } | null };
}) {
  const setAgentStreamHead = vi.fn();
  const setAgentStreamTail = vi.fn();
  const sendAgentMessage = vi.fn().mockResolvedValue(undefined);
  const resumeAgent = vi.fn().mockResolvedValue(undefined);
  const onError = vi.fn();

  const client = overrides?.client === null
    ? null
    : ({
        sendAgentMessage,
        resumeAgent,
        ...overrides?.client,
      } as unknown as DaemonClient);

  return {
    deps: {
      serverId: "server-1",
      client,
      setAgentStreamHead,
      setAgentStreamTail,
      getCurrentHead: () => overrides?.currentHead,
      getAgent: () => overrides?.agent,
      onError,
    },
    setAgentStreamHead,
    setAgentStreamTail,
    sendAgentMessage,
    resumeAgent,
    onError,
  };
}

describe("createSendAgentMessage", () => {
  it("appends user message to tail when no head exists", async () => {
    const { deps, setAgentStreamTail, setAgentStreamHead } = createMockDeps();
    const send = createSendAgentMessage(deps);

    await send("agent-1", "Hello");

    expect(setAgentStreamTail).toHaveBeenCalledTimes(1);
    expect(setAgentStreamHead).not.toHaveBeenCalled();

    const updater = setAgentStreamTail.mock.calls[0][1];
    const prev = new Map<string, StreamItem[]>();
    const result = updater(prev);
    expect(result.get("agent-1")).toHaveLength(1);
    expect(result.get("agent-1")![0].kind).toBe("user_message");
    expect(result.get("agent-1")![0].text).toBe("Hello");
  });

  it("appends user message to head when head has items", async () => {
    const { deps, setAgentStreamHead, setAgentStreamTail } = createMockDeps({
      currentHead: [{ kind: "assistant_message", id: "1", text: "prev", timestamp: new Date() }],
    });
    const send = createSendAgentMessage(deps);

    await send("agent-1", "Hello");

    expect(setAgentStreamHead).toHaveBeenCalledTimes(1);
    expect(setAgentStreamTail).not.toHaveBeenCalled();
  });

  it("skips sending when client is null", async () => {
    const { deps, sendAgentMessage } = createMockDeps({ client: null });
    const send = createSendAgentMessage(deps);

    await send("agent-1", "Hello");

    expect(sendAgentMessage).not.toHaveBeenCalled();
  });

  it("auto-resumes closed agents before sending", async () => {
    const { deps, resumeAgent, sendAgentMessage } = createMockDeps({
      agent: { status: "closed", persistence: { provider: "claude", sessionId: "s1" } },
    });
    const send = createSendAgentMessage(deps);

    await send("agent-1", "Hello");

    expect(resumeAgent).toHaveBeenCalledTimes(1);
    expect(resumeAgent).toHaveBeenCalledWith({ provider: "claude", sessionId: "s1" });
    expect(sendAgentMessage).toHaveBeenCalledTimes(1);
  });

  it("does not auto-resume idle agents", async () => {
    const { deps, resumeAgent } = createMockDeps({
      agent: { status: "idle", persistence: null },
    });
    const send = createSendAgentMessage(deps);

    await send("agent-1", "Hello");

    expect(resumeAgent).not.toHaveBeenCalled();
  });

  it("sends message with messageId via client", async () => {
    const { deps, sendAgentMessage } = createMockDeps();
    const send = createSendAgentMessage(deps);

    await send("agent-1", "Hello world");

    expect(sendAgentMessage).toHaveBeenCalledTimes(1);
    const [agentId, text, options] = sendAgentMessage.mock.calls[0];
    expect(agentId).toBe("agent-1");
    expect(text).toBe("Hello world");
    expect(options).toHaveProperty("messageId");
  });

  it("rolls back the optimistic message and reports an error when client is null", async () => {
    const { deps, setAgentStreamTail, onError } = createMockDeps({ client: null });
    const send = createSendAgentMessage(deps);

    await send("agent-1", "Hello");

    expect(onError).toHaveBeenCalledTimes(1);
    expect(onError.mock.calls[0][0]).toBeInstanceOf(Error);

    // First call appends the optimistic message, second rolls it back.
    expect(setAgentStreamTail).toHaveBeenCalledTimes(2);
    const appendUpdater = setAgentStreamTail.mock.calls[0][1];
    const rollbackUpdater = setAgentStreamTail.mock.calls[1][1];
    const appended = appendUpdater(new Map());
    expect(appended.get("agent-1")).toHaveLength(1);
    const rolledBack = rollbackUpdater(appended);
    expect(rolledBack.get("agent-1")).toHaveLength(0);
  });

  it("rolls back the optimistic message and reports an error when the send fails", async () => {
    const sendError = new Error("network down");
    const { deps, setAgentStreamTail, onError } = createMockDeps({
      client: { sendAgentMessage: vi.fn().mockRejectedValue(sendError) },
    });
    const send = createSendAgentMessage(deps);

    await send("agent-1", "Hello");
    await vi.waitFor(() => expect(onError).toHaveBeenCalledTimes(1));
    expect(onError).toHaveBeenCalledWith(sendError);

    const appendUpdater = setAgentStreamTail.mock.calls[0][1];
    const rollbackUpdater = setAgentStreamTail.mock.calls[1][1];
    const appended = appendUpdater(new Map());
    expect(appended.get("agent-1")).toHaveLength(1);
    const rolledBack = rollbackUpdater(appended);
    expect(rolledBack.get("agent-1")).toHaveLength(0);
  });
});
