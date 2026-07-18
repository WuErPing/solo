import { beforeEach, describe, expect, it } from "vitest";

import {
  buildTranscript,
  useScheduleAssistantStore,
  type AssistantMessage,
} from "./schedule-assistant-store";

function makeMessage(overrides: Partial<AssistantMessage> = {}): AssistantMessage {
  return {
    id: overrides.id ?? `msg-${Math.random().toString(36).slice(2)}`,
    role: "user",
    kind: "text",
    text: "hello",
    createdAt: Date.now(),
    ...overrides,
  };
}

function getThread(serverId: string) {
  return useScheduleAssistantStore.getState().threads[serverId];
}

beforeEach(() => {
  useScheduleAssistantStore.setState({ threads: {} });
});

describe("schedule-assistant-store", () => {
  it("addMessage appends messages per serverId", () => {
    const { addMessage } = useScheduleAssistantStore.getState();

    addMessage("server-1", makeMessage({ id: "m1" }));
    addMessage("server-1", makeMessage({ id: "m2", role: "assistant" }));
    addMessage("server-2", makeMessage({ id: "m3" }));

    expect(getThread("server-1")?.messages.map((m) => m.id)).toEqual(["m1", "m2"]);
    expect(getThread("server-2")?.messages.map((m) => m.id)).toEqual(["m3"]);
  });

  it("updateMessage patches only the matching message", () => {
    const { addMessage, updateMessage } = useScheduleAssistantStore.getState();
    addMessage("server-1", makeMessage({ id: "m1", text: "first" }));
    addMessage("server-1", makeMessage({ id: "m2", text: "second" }));

    updateMessage("server-1", "m1", { text: "patched", applying: true });

    const messages = getThread("server-1")?.messages ?? [];
    expect(messages[0]?.text).toBe("patched");
    expect(messages[0]?.applying).toBe(true);
    expect(messages[1]?.text).toBe("second");
  });

  it("updateMessage is scoped per serverId", () => {
    const { addMessage, updateMessage } = useScheduleAssistantStore.getState();
    addMessage("server-1", makeMessage({ id: "m1" }));
    addMessage("server-2", makeMessage({ id: "m1" }));

    updateMessage("server-1", "m1", { text: "only-server-1" });

    expect(getThread("server-1")?.messages[0]?.text).toBe("only-server-1");
    expect(getThread("server-2")?.messages[0]?.text).toBe("hello");
  });

  it("setSending toggles per-server sending state", () => {
    const { setSending } = useScheduleAssistantStore.getState();

    setSending("server-1", true);
    setSending("server-2", false);

    expect(getThread("server-1")?.isSending).toBe(true);
    expect(getThread("server-2")?.isSending).toBe(false);
  });

  it("setProviderInfo stores the latest llmProvider/model", () => {
    const { setProviderInfo } = useScheduleAssistantStore.getState();

    setProviderInfo("server-1", { llmProvider: "openai", model: "gpt-5" });

    expect(getThread("server-1")?.llmProvider).toBe("openai");
    expect(getThread("server-1")?.model).toBe("gpt-5");
  });

  it("clearThread removes messages but keeps provider info", () => {
    const { addMessage, setProviderInfo, clearThread } = useScheduleAssistantStore.getState();
    addMessage("server-1", makeMessage({ id: "m1" }));
    setProviderInfo("server-1", { llmProvider: "openai", model: "gpt-5" });

    clearThread("server-1");

    expect(getThread("server-1")?.messages).toEqual([]);
    expect(getThread("server-1")?.llmProvider).toBe("openai");
  });
});

describe("buildTranscript", () => {
  it("maps user and assistant text messages to turns", () => {
    const turns = buildTranscript([
      makeMessage({ id: "1", role: "user", text: "hi" }),
      makeMessage({ id: "2", role: "assistant", kind: "text", text: "hello back" }),
    ]);

    expect(turns).toEqual([
      { role: "user", content: "hi" },
      { role: "assistant", content: "hello back" },
    ]);
  });

  it("caps the transcript at the last 10 turns", () => {
    const messages = Array.from({ length: 14 }, (_, index) =>
      makeMessage({ id: `m${index}`, role: "user", text: `msg-${index}` }),
    );

    const turns = buildTranscript(messages);

    expect(turns).toHaveLength(10);
    expect(turns[0]?.content).toBe("msg-4");
    expect(turns[9]?.content).toBe("msg-13");
  });

  it("summarizes proposals to a single line", () => {
    const turns = buildTranscript([
      makeMessage({
        id: "p1",
        role: "assistant",
        kind: "proposal",
        proposal: {
          op: "update",
          scheduleId: "sched-1",
          name: "Nightly",
          summary: "Move cadence to 07:30",
        },
        text: "Move cadence to 07:30",
      }),
    ]);

    expect(turns).toHaveLength(1);
    expect(turns[0]?.role).toBe("assistant");
    expect(turns[0]?.content).toBe('[proposal] update "Nightly": Move cadence to 07:30');
    expect(turns[0]?.content).not.toContain("\n");
  });

  it("includes receipts and errors as plain text lines", () => {
    const turns = buildTranscript([
      makeMessage({
        id: "r1",
        role: "assistant",
        kind: "receipt",
        text: 'Created ✓ "Nightly"',
      }),
      makeMessage({
        id: "e1",
        role: "assistant",
        kind: "error",
        error: "no_llm_provider",
        text: "No LLM provider configured",
      }),
    ]);

    expect(turns[0]?.content).toBe('Created ✓ "Nightly"');
    expect(turns[1]?.content).toBe("No LLM provider configured");
  });
});
