import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useCallback } from "react";
import type { LoopRecord } from "@server/server/loop/rpc-schemas";
import type { AgentSessionConfig } from "@server/shared/agent-session-config";

import { useHostRuntimeClient } from "@/runtime/host-runtime";
import { loopsQueryKey } from "./use-loops";

export interface CreateLoopInput {
  prompt: string;
  cwd: string;
  name?: string | null;
  verifyPrompt?: string | null;
  verifyChecks?: string[];
  sleepMs?: number;
  maxIterations?: number;
  maxTimeMs?: number;
  agentTemplate?: AgentSessionConfig | null;
  workerAgentTemplate?: AgentSessionConfig | null;
  verifierAgentTemplate?: AgentSessionConfig | null;
}

export interface CreateLoopResult {
  createLoop: (input: CreateLoopInput) => Promise<LoopRecord>;
  isCreating: boolean;
}

export function useCreateLoop({ serverId }: { serverId: string }): CreateLoopResult {
  const client = useHostRuntimeClient(serverId);
  const queryClient = useQueryClient();

  const mutation = useMutation({
    mutationFn: async (input: CreateLoopInput): Promise<LoopRecord> => {
      if (!client) {
        throw new Error("Daemon client not available");
      }
      const payload = await client.loopRun({
        prompt: input.prompt,
        cwd: input.cwd,
        name: input.name ?? null,
        verifyPrompt: input.verifyPrompt ?? null,
        verifyChecks: input.verifyChecks,
        sleepMs: input.sleepMs,
        maxIterations: input.maxIterations,
        maxTimeMs: input.maxTimeMs,
        agentTemplate: input.agentTemplate ?? null,
        workerAgentTemplate: input.workerAgentTemplate ?? null,
        verifierAgentTemplate: input.verifierAgentTemplate ?? null,
      });
      if (payload.error) {
        throw new Error(payload.error);
      }
      if (!payload.loop) {
        throw new Error("Loop creation failed");
      }
      return payload.loop;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: loopsQueryKey(serverId) });
    },
  });

  const createLoop = useCallback(
    async (input: CreateLoopInput): Promise<LoopRecord> => {
      return mutation.mutateAsync(input);
    },
    [mutation],
  );

  return { createLoop, isCreating: mutation.isPending };
}
