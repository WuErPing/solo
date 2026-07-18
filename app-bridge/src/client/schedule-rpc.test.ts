import { describe, expect, it, vi, afterEach } from "vitest";
import {
  createConnectedClient,
  simulateServerResponse,
} from "./daemon-client-test-harness.js";

afterEach(() => {
  vi.useRealTimers();
});

function findSentMessage(
  transport: { sentMessages: Array<{ parsed: { type: string; message?: unknown } }> },
  messageType: string,
) {
  return transport.sentMessages.find(
    (m) =>
      m.parsed.type === "session" &&
      (m.parsed as { message?: { type?: string } }).message?.type === messageType,
  );
}

function getRequestId(sent: { parsed: { type: string; message?: unknown } } | undefined): string {
  return (sent?.parsed as { message?: { requestId?: string } }).message?.requestId ?? "";
}

const mockSchedule = {
  id: "sched-1",
  name: null,
  prompt: "Run the daily report",
  cadence: { type: "cron" as const, expression: "0 9 * * *", timezone: "UTC" },
  target: { type: "agent" as const, agentId: "agent-1" },
  status: "active" as const,
  createdAt: "2026-06-22T00:00:00Z",
  updatedAt: "2026-06-22T00:00:00Z",
  nextRunAt: "2026-06-22T09:00:00Z",
  lastRunAt: null,
  pausedAt: null,
  expiresAt: null,
  maxRuns: null,
};

const mockLoop = {
  id: "loop-1",
  name: "test-loop",
  prompt: "Fix lint errors",
  cwd: "/test/project",
  provider: "claude-code",
  model: null,
  workerProvider: null,
  workerModel: null,
  verifierProvider: null,
  verifierModel: null,
  verifyPrompt: null,
  verifyChecks: [],
  archive: false,
  sleepMs: 5000,
  maxIterations: null,
  maxTimeMs: null,
  status: "running" as const,
  createdAt: "2026-06-22T00:00:00Z",
  updatedAt: "2026-06-22T00:00:00Z",
  startedAt: "2026-06-22T00:00:00Z",
  completedAt: null,
  stopRequestedAt: null,
  iterations: [],
  logs: [],
  nextLogSeq: 1,
  activeIteration: null,
  activeWorkerAgentId: null,
  activeVerifierAgentId: null,
};

const mockLoopItem = {
  id: "loop-1",
  name: "test-loop",
  status: "running" as const,
  cwd: "/test/project",
  provider: "claude",
  model: null,
  createdAt: "2026-06-22T00:00:00Z",
  updatedAt: "2026-06-22T00:00:00Z",
  activeIteration: null,
};

describe("ScheduleRpc — schedule", () => {
  it("scheduleCreate sends schedule/create and resolves with response", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const promise = client.scheduleCreate({
      requestId: "req-sched-create",
      prompt: "Run the daily report",
      cadence: { type: "cron", expression: "0 9 * * *" },
      target: { type: "agent", agentId: "agent-1" },
    });

    const sent = findSentMessage(transport, "schedule/create");
    expect(sent).toBeDefined();

    simulateServerResponse(transport, {
      type: "schedule/create/response",
      payload: { requestId: "req-sched-create", schedule: mockSchedule, error: null },
    });

    const result = await promise;
    expect(result.schedule?.id).toBe("sched-1");
    await cleanup();
  });

  it("scheduleList sends schedule/list and resolves with array", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const promise = client.scheduleList("req-sched-list");

    simulateServerResponse(transport, {
      type: "schedule/list/response",
      payload: { requestId: "req-sched-list", schedules: [mockSchedule], error: null },
    });

    const result = await promise;
    expect(result.schedules).toHaveLength(1);
    await cleanup();
  });

  it("scheduleDelete sends schedule/delete", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const promise = client.scheduleDelete({ id: "sched-1", requestId: "req-sched-del" });

    simulateServerResponse(transport, {
      type: "schedule/delete/response",
      payload: { requestId: "req-sched-del", scheduleId: "sched-1", error: null },
    });

    await promise;
    await cleanup();
  });

  it("schedulePause sends schedule/pause", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const promise = client.schedulePause({ id: "sched-1", requestId: "req-sched-pause" });

    simulateServerResponse(transport, {
      type: "schedule/pause/response",
      payload: { requestId: "req-sched-pause", schedule: { ...mockSchedule, status: "paused" as const }, error: null },
    });

    const result = await promise;
    expect(result.schedule?.status).toBe("paused");
    await cleanup();
  });
});

describe("ScheduleRpc — loop", () => {
  it("loopRun sends loop/run and resolves with response", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const promise = client.loopRun({
      requestId: "req-loop-run",
      prompt: "Fix lint errors",
      cwd: "/test/project",
    });

    const sent = findSentMessage(transport, "loop/run");
    expect(sent).toBeDefined();

    simulateServerResponse(transport, {
      type: "loop/run/response",
      payload: { requestId: "req-loop-run", loop: mockLoop, error: null },
    });

    const result = await promise;
    expect(result.loop?.id).toBe("loop-1");
    await cleanup();
  });

  it("loopList sends loop/list", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const promise = client.loopList("req-loop-list");

    simulateServerResponse(transport, {
      type: "loop/list/response",
      payload: { requestId: "req-loop-list", loops: [mockLoopItem], error: null },
    });

    const result = await promise;
    expect(result.loops).toHaveLength(1);
    await cleanup();
  });

  it("loopInspect accepts string ID", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const promise = client.loopInspect("loop-1");

    const sent = findSentMessage(transport, "loop/inspect");
    expect(sent).toBeDefined();
    expect((sent!.parsed as { message?: { id?: string } }).message?.id).toBe("loop-1");

    const requestId = getRequestId(sent);
    simulateServerResponse(transport, {
      type: "loop/inspect/response",
      payload: { requestId, loop: mockLoop, error: null },
    });

    const result = await promise;
    expect(result.loop?.id).toBe("loop-1");
    await cleanup();
  });

  it("loopDelete accepts string ID", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const promise = client.loopDelete("loop-1");

    const sent = findSentMessage(transport, "loop/delete");
    const requestId = getRequestId(sent);

    simulateServerResponse(transport, {
      type: "loop/delete/response",
      payload: { requestId, id: "loop-1", error: null },
    });

    await promise;
    await cleanup();
  });
});

describe("ScheduleRpc — schedule assist", () => {
  it("scheduleAssist sends schedule/assist and resolves with the payload", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const promise = client.scheduleAssist({
      requestId: "req-sched-assist",
      message: "Every weekday at 9am, summarize the nightly test runs",
      timezone: "Asia/Shanghai",
      clientNow: "2026-07-18T09:00:00+08:00",
    });

    const sent = findSentMessage(transport, "schedule/assist");
    expect(sent).toBeDefined();
    const wireMessage = (sent!.parsed as { message?: Record<string, unknown> }).message;
    expect(wireMessage).toMatchObject({
      type: "schedule/assist",
      requestId: "req-sched-assist",
      message: "Every weekday at 9am, summarize the nightly test runs",
      timezone: "Asia/Shanghai",
      clientNow: "2026-07-18T09:00:00+08:00",
    });
    expect(wireMessage).not.toHaveProperty("provider");
    expect(wireMessage).not.toHaveProperty("llmProvider");
    expect(wireMessage).not.toHaveProperty("model");

    simulateServerResponse(transport, {
      type: "schedule/assist/response",
      payload: {
        requestId: "req-sched-assist",
        kind: "proposal",
        proposal: {
          op: "create",
          name: "Nightly test summary",
          prompt: "Summarize the nightly test runs",
          cadence: { type: "cron", expression: "0 9 * * 1-5", timezone: "Asia/Shanghai" },
          target: { type: "agent", agentId: "agent-1" },
          summary: "Create \"Nightly test summary\" every weekday at 09:00",
          nextRunAt: "2026-07-21T09:00:00+08:00",
        },
        error: null,
        llmProvider: "openai",
        model: "gpt-5",
      },
    });

    const result = await promise;
    expect(result.kind).toBe("proposal");
    expect(result.proposal?.op).toBe("create");
    expect(result.llmProvider).toBe("openai");
    await cleanup();
  });

  it("scheduleAssist resolves (not rejects) a kind:error payload", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const promise = client.scheduleAssist({
      requestId: "req-sched-assist-err",
      message: "Pause everything",
      timezone: "Asia/Shanghai",
      clientNow: "2026-07-18T09:00:00+08:00",
    });

    simulateServerResponse(transport, {
      type: "schedule/assist/response",
      payload: {
        requestId: "req-sched-assist-err",
        kind: "error",
        message: "No LLM provider configured on this host.",
        error: "no_llm_provider",
      },
    });

    const result = await promise;
    expect(result.kind).toBe("error");
    expect(result.error).toBe("no_llm_provider");
    await cleanup();
  });
});
