/**
 * @vitest-environment jsdom
 */
import React from "react";
import { act, renderHook } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useTmuxAgentStore } from "@/stores/tmux-agent-store";
import { useTmuxAgentLaunch, type TmuxAgentLaunchInput, type TmuxAgentLaunchResult } from "./use-tmux-agent-launch";

const { mockClient, mockRouter, mockWithLiveTmuxClient } = vi.hoisted(() => ({
  mockClient: {
    tmuxNewSession: vi.fn(),
    tmuxListAgents: vi.fn(),
  },
  mockRouter: { push: vi.fn() },
  mockWithLiveTmuxClient: vi.fn(),
}));

vi.mock("@/utils/tmux-rpc", () => ({
  withLiveTmuxClient: mockWithLiveTmuxClient,
}));

vi.mock("@/runtime/host-runtime", () => ({
  useHosts: () => [{ serverId: "server-1", label: "Host 1" }],
}));

vi.mock("expo-router", () => ({
  router: mockRouter,
}));

vi.mock("@/hooks/use-tmux-agents", () => ({
  tmuxAgentsQueryKey: (serverId: string) => ["tmux-agents", serverId],
}));

const LAUNCH_INPUT: TmuxAgentLaunchInput = {
  serverId: "server-1",
  agentName: "claude",
  launchCmd: "claude",
  filePath: "docs/guide.md",
  workspaceRoot: "/tmp/project",
};

const AGENT_INFO = {
  sessionName: "claude-123",
  windowName: "main",
  paneId: "%1",
  paneIndex: 0,
  panePid: 100,
  agentName: "claude",
  currentCmd: "claude",
  workingDir: "/tmp/project",
};

const PANE_INFO = {
  sessionName: "claude-123",
  windowName: "main",
  paneId: "%2",
  paneIndex: 0,
  panePid: 200,
  currentCmd: "claude",
  workingDir: "/tmp/project",
};

function renderLaunchHook() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const wrapper = ({ children }: { children: React.ReactNode }) =>
    React.createElement(QueryClientProvider, { client: queryClient }, children);
  return renderHook(() => useTmuxAgentLaunch(), { wrapper });
}

beforeEach(() => {
  mockWithLiveTmuxClient.mockImplementation((_serverId: string, op: (client: unknown) => unknown) =>
    op(mockClient),
  );
  mockClient.tmuxNewSession.mockResolvedValue({ sessionName: "claude-123", error: null });
  mockClient.tmuxListAgents.mockResolvedValue({ agents: [AGENT_INFO], otherPanes: [] });
});

afterEach(() => {
  vi.clearAllMocks();
  vi.useRealTimers();
  useTmuxAgentStore.getState().setSelectedAgent(null);
});

describe("useTmuxAgentLaunch", () => {
  it("creates a session with cwd and prompt, selects the pane and navigates", async () => {
    const { result } = renderLaunchHook();

    let outcome: Awaited<ReturnType<typeof result.current.launch>> | undefined;
    await act(async () => {
      outcome = await result.current.launch(LAUNCH_INPUT);
    });

    expect(outcome).toEqual({ ok: true });
    expect(mockClient.tmuxNewSession).toHaveBeenCalledWith(
      expect.stringMatching(/^claude-\d+$/),
      {
        workingDir: "/tmp/project",
        command: "claude 'Read docs/guide.md and follow the instructions in it.'",
      },
    );
    expect(useTmuxAgentStore.getState().selectedAgent).toMatchObject({
      sessionName: "claude-123",
      paneId: "%1",
      serverId: "server-1",
      serverLabel: "Host 1",
    });
    expect(mockRouter.push).toHaveBeenCalledWith("/tmux-pane");
    expect(result.current.isLaunching).toBe(false);
  });

  it("accepts a plain pane while agent detection lags", async () => {
    mockClient.tmuxListAgents.mockResolvedValue({ agents: [], otherPanes: [PANE_INFO] });
    const { result } = renderLaunchHook();

    let outcome: Awaited<ReturnType<typeof result.current.launch>> | undefined;
    await act(async () => {
      outcome = await result.current.launch(LAUNCH_INPUT);
    });

    expect(outcome).toEqual({ ok: true });
    expect(useTmuxAgentStore.getState().selectedAgent).toMatchObject({ paneId: "%2" });
    expect(mockRouter.push).toHaveBeenCalledWith("/tmux-pane");
  });

  it("surfaces the daemon error when creation fails", async () => {
    mockClient.tmuxNewSession.mockResolvedValue({
      sessionName: "",
      error: "tmux is not installed or not in PATH",
    });
    const { result } = renderLaunchHook();

    let outcome: Awaited<ReturnType<typeof result.current.launch>> | undefined;
    await act(async () => {
      outcome = await result.current.launch(LAUNCH_INPUT);
    });

    expect(outcome).toEqual({ ok: false, error: "tmux is not installed or not in PATH" });
    expect(mockRouter.push).not.toHaveBeenCalled();
    expect(useTmuxAgentStore.getState().selectedAgent).toBeNull();
  });

  it("surfaces transport errors when creation throws", async () => {
    mockClient.tmuxNewSession.mockRejectedValue(new Error("Daemon client not available"));
    const { result } = renderLaunchHook();

    let outcome: Awaited<ReturnType<typeof result.current.launch>> | undefined;
    await act(async () => {
      outcome = await result.current.launch(LAUNCH_INPUT);
    });

    expect(outcome).toEqual({ ok: false, error: "Daemon client not available" });
    expect(mockRouter.push).not.toHaveBeenCalled();
  });

  it("reports a timeout when the pane never appears", async () => {
    vi.useFakeTimers();
    mockClient.tmuxListAgents.mockResolvedValue({ agents: [], otherPanes: [] });
    const { result } = renderLaunchHook();

    let outcome: TmuxAgentLaunchResult | undefined;
    let pending: Promise<TmuxAgentLaunchResult> | undefined;
    act(() => {
      pending = result.current.launch(LAUNCH_INPUT);
    });
    for (let i = 0; i < 5; i++) {
      await act(async () => {
        await vi.advanceTimersByTimeAsync(500);
      });
    }
    await act(async () => {
      outcome = await pending;
    });

    expect(outcome).toMatchObject({ ok: false });
    expect(outcome && !outcome.ok && outcome.error).toContain("pane was not found");
    expect(mockClient.tmuxListAgents).toHaveBeenCalledTimes(6);
    expect(mockRouter.push).not.toHaveBeenCalled();
  });
});