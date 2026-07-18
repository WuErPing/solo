/**
 * @vitest-environment jsdom
 */
import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useScheduleAssist } from "./use-schedule-assist";
import { useScheduleAssistantStore } from "@/stores/schedule-assistant-store";

const { mockClient, clientRef } = vi.hoisted(() => {
  const hoistedClient = {
    scheduleAssist: vi.fn(),
  };
  return {
    mockClient: hoistedClient,
    clientRef: { current: hoistedClient as typeof hoistedClient | null },
  };
});

vi.mock("@/runtime/host-runtime", () => ({
  useHostRuntimeClient: () => clientRef.current,
  useHostRuntimeIsConnected: () => true,
}));

vi.mock("@/utils/cron-timezone", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/utils/cron-timezone")>()),
  detectTimezone: () => "Asia/Shanghai",
}));

function getThread(serverId: string) {
  return useScheduleAssistantStore.getState().threads[serverId];
}

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(new Date("2026-07-18T10:00:00.000Z"));
  useScheduleAssistantStore.setState({ threads: {} });
  clientRef.current = mockClient;
});

afterEach(() => {
  mockClient.scheduleAssist.mockReset();
  vi.useRealTimers();
});

function renderAssist(contextScheduleId?: string) {
  return renderHook(() => useScheduleAssist({ serverId: "server-1", contextScheduleId }));
}

describe("useScheduleAssist", () => {
  it("pushes the user message and calls the bridge with wire options", async () => {
    mockClient.scheduleAssist.mockResolvedValueOnce({
      requestId: "req-1",
      kind: "answer",
      message: "3 schedules run today",
      error: null,
      llmProvider: "openai",
      model: "gpt-5",
    });

    const { result } = renderAssist("sched-42");

    await act(async () => {
      await result.current.send("  What runs today?  ");
    });

    expect(mockClient.scheduleAssist).toHaveBeenCalledTimes(1);
    expect(mockClient.scheduleAssist).toHaveBeenCalledWith({
      message: "What runs today?",
      timezone: "Asia/Shanghai",
      clientNow: "2026-07-18T10:00:00.000Z",
      contextScheduleId: "sched-42",
    });

    const thread = getThread("server-1");
    expect(thread?.messages[0]).toMatchObject({
      role: "user",
      kind: "text",
      text: "What runs today?",
    });
    expect(thread?.isSending).toBe(false);
    expect(thread?.llmProvider).toBe("openai");
    expect(thread?.model).toBe("gpt-5");
  });

  it("omits contextScheduleId and transcript when not set", async () => {
    mockClient.scheduleAssist.mockResolvedValueOnce({
      requestId: "req-1",
      kind: "clarify",
      message: "Which one?",
      error: null,
    });

    const { result } = renderAssist();

    await act(async () => {
      await result.current.send("pause it");
    });

    expect(mockClient.scheduleAssist).toHaveBeenCalledWith({
      message: "pause it",
      timezone: "Asia/Shanghai",
      clientNow: "2026-07-18T10:00:00.000Z",
    });
  });

  it("sends the prior thread as transcript on the next turn", async () => {
    mockClient.scheduleAssist.mockResolvedValue({
      requestId: "req-1",
      kind: "clarify",
      message: "Which schedule?",
      error: null,
    });

    const { result } = renderAssist();

    await act(async () => {
      await result.current.send("pause it");
    });
    await act(async () => {
      await result.current.send("the nightly one");
    });

    expect(mockClient.scheduleAssist).toHaveBeenLastCalledWith({
      message: "the nightly one",
      timezone: "Asia/Shanghai",
      clientNow: "2026-07-18T10:00:00.000Z",
      transcript: [
        { role: "user", content: "pause it" },
        { role: "assistant", content: "Which schedule?" },
      ],
    });
  });

  it("maps a proposal payload to a proposal message", async () => {
    const proposal = {
      op: "create" as const,
      name: "Nightly",
      prompt: "Summarize tests",
      cadence: { type: "cron" as const, expression: "0 9 * * *" },
      target: { type: "agent" as const, agentId: "agent-1" },
      summary: "Create nightly summary at 09:00",
      nextRunAt: "2026-07-19T01:00:00.000Z",
    };
    mockClient.scheduleAssist.mockResolvedValueOnce({
      requestId: "req-1",
      kind: "proposal",
      proposal,
      error: null,
    });

    const { result } = renderAssist();

    await act(async () => {
      await result.current.send("every day at 9 summarize tests");
    });

    const message = getThread("server-1")?.messages.at(-1);
    expect(message).toMatchObject({
      role: "assistant",
      kind: "proposal",
      text: "Create nightly summary at 09:00",
    });
    expect(message?.proposal).toEqual(proposal);
  });

  it("maps clarify and answer payloads to text messages", async () => {
    mockClient.scheduleAssist
      .mockResolvedValueOnce({
        requestId: "req-1",
        kind: "clarify",
        message: "Which schedule do you mean?",
        error: null,
      })
      .mockResolvedValueOnce({
        requestId: "req-2",
        kind: "answer",
        message: "Nightly runs at 09:00",
        error: null,
      });

    const { result } = renderAssist();

    await act(async () => {
      await result.current.send("change it");
      await result.current.send("what does nightly do?");
    });

    const messages = getThread("server-1")?.messages ?? [];
    expect(messages[1]).toMatchObject({ kind: "text", text: "Which schedule do you mean?" });
    expect(messages[3]).toMatchObject({ kind: "text", text: "Nightly runs at 09:00" });
  });

  it("maps a kind:error payload to an error message with the error code", async () => {
    mockClient.scheduleAssist.mockResolvedValueOnce({
      requestId: "req-1",
      kind: "error",
      message: "Add an LLM provider in Settings",
      error: "no_llm_provider",
    });

    const { result } = renderAssist();

    await act(async () => {
      await result.current.send("every hour ping");
    });

    const message = getThread("server-1")?.messages.at(-1);
    expect(message).toMatchObject({
      role: "assistant",
      kind: "error",
      error: "no_llm_provider",
      text: "Add an LLM provider in Settings",
    });
  });

  it("maps a transport timeout to an error message with code timeout", async () => {
    mockClient.scheduleAssist.mockRejectedValueOnce(
      new Error("Timeout waiting for message (120000ms)"),
    );

    const { result } = renderAssist();

    await act(async () => {
      await result.current.send("every hour ping");
    });

    const message = getThread("server-1")?.messages.at(-1);
    expect(message).toMatchObject({ kind: "error", error: "timeout" });
  });

  it("maps other transport failures to an error message with code unknown", async () => {
    mockClient.scheduleAssist.mockRejectedValueOnce(new Error("socket closed"));

    const { result } = renderAssist();

    await act(async () => {
      await result.current.send("every hour ping");
    });

    const message = getThread("server-1")?.messages.at(-1);
    expect(message).toMatchObject({
      kind: "error",
      error: "unknown",
      text: "socket closed",
    });
  });

  it("adds a not_connected error message when the client is missing", async () => {
    clientRef.current = null;

    const { result } = renderAssist();

    await act(async () => {
      await result.current.send("hello");
    });

    const messages = getThread("server-1")?.messages ?? [];
    expect(messages).toHaveLength(2);
    expect(messages[1]).toMatchObject({ kind: "error", error: "not_connected" });
    expect(mockClient.scheduleAssist).not.toHaveBeenCalled();
  });

  it("exposes isSending while the request is in flight", async () => {
    let resolveAssist: ((value: unknown) => void) | undefined;
    mockClient.scheduleAssist.mockReturnValueOnce(
      new Promise((resolve) => {
        resolveAssist = resolve;
      }),
    );

    const { result } = renderAssist();

    let sendPromise: Promise<void> | undefined;
    act(() => {
      sendPromise = result.current.send("hello");
    });

    expect(result.current.isSending).toBe(true);

    await act(async () => {
      resolveAssist?.({ requestId: "req-1", kind: "answer", message: "ok", error: null });
      await sendPromise;
    });

    expect(result.current.isSending).toBe(false);
  });

  it("ignores empty input", async () => {
    const { result } = renderAssist();

    await act(async () => {
      await result.current.send("   ");
    });

    expect(mockClient.scheduleAssist).not.toHaveBeenCalled();
    expect(getThread("server-1")?.messages ?? []).toHaveLength(0);
  });
});
