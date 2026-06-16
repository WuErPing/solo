import { useCallback } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import type { LoopRecord } from "@server/server/loop/rpc-schemas";
import { useHostRuntimeClient } from "@/runtime/host-runtime";
import { loopsQueryKey } from "./use-loops";
import { loopInspectQueryKey } from "./use-loop-inspect";

export interface LoopMutationsInput {
  serverId: string;
}

export interface UpdateLoopInput {
  loopId: string;
  name?: string | null;
  archive?: boolean | null;
}

export interface LoopMutationsResult {
  updateLoop: (input: UpdateLoopInput) => Promise<LoopRecord>;
  deleteLoop: (loopId: string) => Promise<string>;
  stopLoop: (loopId: string) => Promise<LoopRecord>;
  isUpdating: boolean;
  isDeleting: boolean;
  isStopping: boolean;
}

export function useLoopMutations({ serverId }: LoopMutationsInput): LoopMutationsResult {
  const client = useHostRuntimeClient(serverId);
  const queryClient = useQueryClient();

  const updateMutation = useMutation({
    mutationFn: async (input: UpdateLoopInput): Promise<LoopRecord> => {
      if (!client) {
        throw new Error("Daemon client not available");
      }
      const payload = await client.loopUpdate({
        id: input.loopId,
        ...(input.name ? { name: input.name } : {}),
        ...(input.archive != null ? { archive: input.archive } : {}),
      });
      if (payload.error) {
        throw new Error(payload.error);
      }
      if (!payload.loop) {
        throw new Error("Loop not found");
      }
      return payload.loop;
    },
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({
        queryKey: loopsQueryKey(serverId),
        refetchType: "all",
      });
      void queryClient.invalidateQueries({
        queryKey: loopInspectQueryKey(serverId, variables.loopId),
        refetchType: "all",
      });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: async (loopId: string): Promise<string> => {
      if (!client) {
        throw new Error("Daemon client not available");
      }
      const payload = await client.loopDelete({ id: loopId });
      if (payload.error) {
        throw new Error(payload.error);
      }
      return payload.id;
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        queryKey: loopsQueryKey(serverId),
        refetchType: "all",
      });
    },
  });

  const stopMutation = useMutation({
    mutationFn: async (loopId: string): Promise<LoopRecord> => {
      if (!client) {
        throw new Error("Daemon client not available");
      }
      const payload = await client.loopStop({ id: loopId });
      if (payload.error) {
        throw new Error(payload.error);
      }
      if (!payload.loop) {
        throw new Error("Loop not found");
      }
      return payload.loop;
    },
    onSuccess: (_data, loopId) => {
      void queryClient.invalidateQueries({
        queryKey: loopsQueryKey(serverId),
        refetchType: "all",
      });
      void queryClient.invalidateQueries({
        queryKey: loopInspectQueryKey(serverId, loopId),
        refetchType: "all",
      });
    },
  });

  const updateLoop = useCallback(
    async (input: UpdateLoopInput): Promise<LoopRecord> => {
      return updateMutation.mutateAsync(input);
    },
    [updateMutation],
  );

  const deleteLoop = useCallback(
    async (loopId: string): Promise<string> => {
      return deleteMutation.mutateAsync(loopId);
    },
    [deleteMutation],
  );

  const stopLoop = useCallback(
    async (loopId: string): Promise<LoopRecord> => {
      return stopMutation.mutateAsync(loopId);
    },
    [stopMutation],
  );

  return {
    updateLoop,
    deleteLoop,
    stopLoop,
    isUpdating: updateMutation.isPending,
    isDeleting: deleteMutation.isPending,
    isStopping: stopMutation.isPending,
  };
}
