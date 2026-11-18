import { useCallback, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { router } from "expo-router";
import { useHosts } from "@/runtime/host-runtime";
import { tmuxAgentsQueryKey, type TmuxAgent, type TmuxPane } from "@/hooks/use-tmux-agents";
import { useTmuxAgentStore } from "@/stores/tmux-agent-store";
import { withLiveTmuxClient } from "@/utils/tmux-rpc";
import {
  buildAgentLaunchCommand,
  buildTmuxSessionName,
  findSessionPane,
} from "@/utils/tmux-agent-launch";

const PANE_LOOKUP_ATTEMPTS = 6;
const PANE_LOOKUP_DELAY_MS = 500;

export interface TmuxAgentLaunchInput {
  serverId: string;
  agentName: string;
  launchCmd: string;
  filePath: string;
  workspaceRoot: string;
}

export type TmuxAgentLaunchResult = { ok: true } | { ok: false; error: string };

function delay(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

async function lookupSessionPane(
  serverId: string,
  serverLabel: string,
  sessionName: string,
): Promise<TmuxAgent | TmuxPane | null> {
  for (let attempt = 0; attempt < PANE_LOOKUP_ATTEMPTS; attempt++) {
    if (attempt > 0) {
      await delay(PANE_LOOKUP_DELAY_MS);
    }
    try {
      const payload = await withLiveTmuxClient(serverId, (client) => client.tmuxListAgents());
      const agents: TmuxAgent[] = (payload.agents ?? []).map((agent) => ({
        ...agent,
        serverId,
        serverLabel,
      }));
      const otherPanes: TmuxPane[] = (payload.otherPanes ?? []).map((pane) => ({
        ...pane,
        serverId,
        serverLabel,
      }));
      const found = findSessionPane(agents, otherPanes, sessionName);
      if (found) {
        return found;
      }
    } catch {
      // Transient disconnect — keep polling until attempts run out.
    }
  }
  return null;
}

export function useTmuxAgentLaunch(): {
  launch: (input: TmuxAgentLaunchInput) => Promise<TmuxAgentLaunchResult>;
  isLaunching: boolean;
} {
  const [isLaunching, setIsLaunching] = useState(false);
  const queryClient = useQueryClient();
  const hosts = useHosts();

  const launch = useCallback(
    async (input: TmuxAgentLaunchInput): Promise<TmuxAgentLaunchResult> => {
      setIsLaunching(true);
      try {
        const name = buildTmuxSessionName(input.agentName, Date.now());
        const command = buildAgentLaunchCommand({
          launchCmd: input.launchCmd,
          filePath: input.filePath,
        });

        let sessionName: string;
        try {
          const result = await withLiveTmuxClient(input.serverId, (client) =>
            client.tmuxNewSession(name, { workingDir: input.workspaceRoot, command }),
          );
          if (result.error || !result.sessionName) {
            return { ok: false, error: result.error ?? "Failed to create tmux session" };
          }
          sessionName = result.sessionName;
        } catch (err) {
          return {
            ok: false,
            error: err instanceof Error ? err.message : "Failed to create tmux session",
          };
        }

        const serverLabel =
          hosts.find((host) => host.serverId === input.serverId)?.label ?? input.serverId;
        const pane = await lookupSessionPane(input.serverId, serverLabel, sessionName);
        if (!pane) {
          // tmux kills the session when its launch command exits, so a missing
          // CLI binary surfaces here rather than at creation time.
          return {
            ok: false,
            error:
              "Session created but its pane was not found — the agent CLI may have exited immediately. Check that it is installed on the host.",
          };
        }

        useTmuxAgentStore.getState().setSelectedAgent(pane);
        void queryClient.invalidateQueries({ queryKey: tmuxAgentsQueryKey(input.serverId) });
        router.push("/tmux-pane");
        return { ok: true };
      } finally {
        setIsLaunching(false);
      }
    },
    [hosts, queryClient],
  );

  return { launch, isLaunching };
}