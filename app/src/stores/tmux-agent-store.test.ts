import { describe, expect, it } from "vitest";
import { useTmuxAgentStore } from "./tmux-agent-store";
import type { TmuxAgent } from "@/hooks/use-tmux-agents";

const mockAgent: TmuxAgent = {
  serverId: "server1",
  paneId: "%0",
  agentName: "claude",
  sessionName: "dev",
  windowName: "main",
  paneIndex: 0,
  panePid: 1234,
  currentCmd: "claude",
  workingDir: "/home/user",
  serverLabel: "local",
};

describe("tmuxAgentStore", () => {
  it("starts with no selected agent", () => {
    const { selectedAgent } = useTmuxAgentStore.getState();
    expect(selectedAgent).toBeNull();
  });

  it("sets and clears the selected agent", () => {
    const { setSelectedAgent } = useTmuxAgentStore.getState();
    setSelectedAgent(mockAgent);
    expect(useTmuxAgentStore.getState().selectedAgent).toEqual(mockAgent);

    setSelectedAgent(null);
    expect(useTmuxAgentStore.getState().selectedAgent).toBeNull();
  });
});
