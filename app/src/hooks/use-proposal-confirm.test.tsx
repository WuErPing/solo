/**
 * @vitest-environment jsdom
 */
import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { StoredSchedule } from "@server/server/schedule/types";

import { useProposalConfirm } from "./use-proposal-confirm";
import {
  useScheduleAssistantStore,
  type ScheduleAssistProposal,
} from "@/stores/schedule-assistant-store";

const { mockClient, mutations, createResult } = vi.hoisted(() => ({
  mockClient: {
    scheduleInspect: vi.fn(),
  },
  mutations: {
    pauseSchedule: vi.fn(),
    resumeSchedule: vi.fn(),
    deleteSchedule: vi.fn(),
    updateSchedule: vi.fn(),
  },
  createResult: {
    createSchedule: vi.fn(),
  },
}));

vi.mock("@/runtime/host-runtime", () => ({
  useHostRuntimeClient: () => mockClient,
  useHostRuntimeIsConnected: () => true,
}));

vi.mock("@/hooks/use-create-schedule", () => ({
  useCreateSchedule: () => ({
    createSchedule: createResult.createSchedule,
    isCreating: false,
  }),
}));

vi.mock("@/hooks/use-schedule-mutations", () => ({
  useScheduleMutations: () => ({
    ...mutations,
    isPausing: () => false,
    isResuming: () => false,
    isDeleting: () => false,
    isUpdating: () => false,
  }),
}));

vi.mock("@/utils/cron-timezone", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/utils/cron-timezone")>()),
  detectTimezone: () => "Asia/Shanghai",
}));

function makeStoredSchedule(overrides: Partial<StoredSchedule> = {}): StoredSchedule {
  return {
    id: "sched-1",
    name: "Nightly",
    prompt: "Summarize the nightly test runs",
    cadence: { type: "cron", expression: "0 1 * * 1-5", timezone: "Asia/Shanghai" },
    target: { type: "agent", agentId: "agent-1" },
    cwd: null,
    status: "active",
    createdAt: "2026-07-01T00:00:00.000Z",
    updatedAt: "2026-07-01T00:00:00.000Z",
    nextRunAt: "2026-07-19T01:00:00.000Z",
    lastRunAt: null,
    pausedAt: null,
    expiresAt: null,
    maxRuns: null,
    runs: [],
    ...overrides,
  };
}

function seedProposalMessage(proposal: ScheduleAssistProposal, id = "msg-1") {
  useScheduleAssistantStore.getState().addMessage("server-1", {
    id,
    role: "assistant",
    kind: "proposal",
    proposal,
    text: proposal.summary,
    createdAt: Date.now(),
  });
  return id;
}

function getMessage(id = "msg-1") {
  return useScheduleAssistantStore
    .getState()
    .threads["server-1"]?.messages.find((message) => message.id === id);
}

beforeEach(() => {
  useScheduleAssistantStore.setState({ threads: {} });
});

afterEach(() => {
  mockClient.scheduleInspect.mockReset();
  mutations.pauseSchedule.mockReset();
  mutations.resumeSchedule.mockReset();
  mutations.deleteSchedule.mockReset();
  mutations.updateSchedule.mockReset();
  createResult.createSchedule.mockReset();
});

function renderConfirm() {
  return renderHook(() => useProposalConfirm({ serverId: "server-1" }));
}

describe("useProposalConfirm", () => {
  it("create op converts cron cadence to UTC and maps all fields", async () => {
    createResult.createSchedule.mockResolvedValueOnce(makeStoredSchedule({ id: "sched-new" }));
    const proposal: ScheduleAssistProposal = {
      op: "create",
      name: "Nightly",
      prompt: "Summarize the nightly test runs",
      cadence: { type: "cron", expression: "0 9 * * 1-5" },
      target: { type: "agent", agentId: "agent-1" },
      cwd: "~/work/backend",
      maxRuns: 30,
      expiresAt: "2026-08-31T00:00:00.000Z",
      summary: "Create nightly summary",
    };
    const messageId = seedProposalMessage(proposal);
    const { result } = renderConfirm();

    await act(async () => {
      await result.current.confirmProposal(messageId, proposal);
    });

    expect(createResult.createSchedule).toHaveBeenCalledWith({
      name: "Nightly",
      prompt: "Summarize the nightly test runs",
      cadence: { type: "cron", expression: "0 1 * * 1-5", timezone: "Asia/Shanghai" },
      target: { type: "agent", agentId: "agent-1" },
      cwd: "~/work/backend",
      maxRuns: 30,
      expiresAt: "2026-08-31T00:00:00.000Z",
    });

    const message = getMessage(messageId);
    expect(message).toMatchObject({
      kind: "receipt",
      applying: false,
      receiptScheduleId: "sched-new",
    });
    expect(message?.text).toContain("Created");
    expect(message?.applyError).toBeUndefined();
  });

  it("create op passes every-cadence through unchanged", async () => {
    createResult.createSchedule.mockResolvedValueOnce(makeStoredSchedule({ id: "sched-new" }));
    const proposal: ScheduleAssistProposal = {
      op: "create",
      prompt: "Ping health check",
      cadence: { type: "every", everyMs: 900000 },
      target: { type: "provider", providerId: "claude" },
      summary: "Ping every 15 min",
    };
    const messageId = seedProposalMessage(proposal);
    const { result } = renderConfirm();

    await act(async () => {
      await result.current.confirmProposal(messageId, proposal);
    });

    expect(createResult.createSchedule).toHaveBeenCalledWith({
      name: null,
      prompt: "Ping health check",
      cadence: { type: "every", everyMs: 900000 },
      target: { type: "provider", providerId: "claude" },
      cwd: null,
    });
  });

  it("update op inspects, merges proposal fields, and full-replaces", async () => {
    mockClient.scheduleInspect.mockResolvedValueOnce({
      requestId: "req-1",
      schedule: makeStoredSchedule(),
      error: null,
    });
    mutations.updateSchedule.mockResolvedValueOnce(makeStoredSchedule());
    const proposal: ScheduleAssistProposal = {
      op: "update",
      scheduleId: "sched-1",
      name: "Nightly",
      cadence: { type: "cron", expression: "30 7 * * 1-5" },
      summary: "Move to 07:30",
    };
    const messageId = seedProposalMessage(proposal);
    const { result } = renderConfirm();

    await act(async () => {
      await result.current.confirmProposal(messageId, proposal);
    });

    expect(mockClient.scheduleInspect).toHaveBeenCalledWith({ id: "sched-1" });
    expect(mutations.updateSchedule).toHaveBeenCalledWith({
      scheduleId: "sched-1",
      prompt: "Summarize the nightly test runs",
      name: "Nightly",
      cwd: null,
      cadence: { type: "cron", expression: "30 23 * * 1-5", timezone: "Asia/Shanghai" },
      target: { type: "agent", agentId: "agent-1" },
    });

    const message = getMessage(messageId);
    expect(message).toMatchObject({ kind: "receipt", receiptScheduleId: "sched-1" });
    expect(message?.text).toContain("Updated");
  });

  it("pause op calls pauseSchedule and records a receipt", async () => {
    mutations.pauseSchedule.mockResolvedValueOnce(makeStoredSchedule({ status: "paused" }));
    const proposal: ScheduleAssistProposal = {
      op: "pause",
      scheduleId: "sched-1",
      name: "Nightly",
      summary: "Pause nightly",
    };
    const messageId = seedProposalMessage(proposal);
    const { result } = renderConfirm();

    await act(async () => {
      await result.current.confirmProposal(messageId, proposal);
    });

    expect(mutations.pauseSchedule).toHaveBeenCalledWith("sched-1");
    expect(getMessage(messageId)).toMatchObject({
      kind: "receipt",
      receiptScheduleId: "sched-1",
    });
    expect(getMessage(messageId)?.text).toContain("Paused");
  });

  it("resume op calls resumeSchedule and records a receipt", async () => {
    mutations.resumeSchedule.mockResolvedValueOnce(makeStoredSchedule());
    const proposal: ScheduleAssistProposal = {
      op: "resume",
      scheduleId: "sched-1",
      name: "Nightly",
      summary: "Resume nightly",
    };
    const messageId = seedProposalMessage(proposal);
    const { result } = renderConfirm();

    await act(async () => {
      await result.current.confirmProposal(messageId, proposal);
    });

    expect(mutations.resumeSchedule).toHaveBeenCalledWith("sched-1");
    expect(getMessage(messageId)?.text).toContain("Resumed");
  });

  it("delete op calls deleteSchedule and records a receipt without a link", async () => {
    mutations.deleteSchedule.mockResolvedValueOnce("sched-1");
    const proposal: ScheduleAssistProposal = {
      op: "delete",
      scheduleId: "sched-1",
      name: "Nightly",
      summary: "Delete nightly",
    };
    const messageId = seedProposalMessage(proposal);
    const { result } = renderConfirm();

    await act(async () => {
      await result.current.confirmProposal(messageId, proposal);
    });

    expect(mutations.deleteSchedule).toHaveBeenCalledWith("sched-1");
    const message = getMessage(messageId);
    expect(message?.kind).toBe("receipt");
    expect(message?.text).toContain("Deleted");
    expect(message?.receiptScheduleId).toBeUndefined();
  });

  it("keeps the proposal and records an inline error when the mutation fails", async () => {
    createResult.createSchedule.mockRejectedValueOnce(new Error("invalid cadence"));
    const proposal: ScheduleAssistProposal = {
      op: "create",
      prompt: "Summarize",
      cadence: { type: "cron", expression: "0 9 * * *" },
      target: { type: "agent", agentId: "agent-1" },
      summary: "Create",
    };
    const messageId = seedProposalMessage(proposal);
    const { result } = renderConfirm();

    await act(async () => {
      await result.current.confirmProposal(messageId, proposal);
    });

    const message = getMessage(messageId);
    expect(message?.kind).toBe("proposal");
    expect(message?.proposal).toEqual(proposal);
    expect(message?.applying).toBe(false);
    expect(message?.applyError).toBe("invalid cadence");
  });
});
