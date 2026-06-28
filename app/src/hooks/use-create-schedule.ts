import { useCallback } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import type { ScheduleSummary, ScheduleCadence, ScheduleTarget } from "@server/server/schedule/types";
import { useHostRuntimeClient } from "@/runtime/host-runtime";
import { schedulesQueryKey } from "./use-schedules";

interface SchedulesQueryData {
  schedules: ScheduleSummary[];
  error: string | null;
}

export interface CreateScheduleInput {
  name?: string | null;
  prompt: string;
  cadence: ScheduleCadence;
  target: ScheduleTarget;
  cwd?: string | null;
  maxRuns?: number;
  expiresAt?: string;
}

export interface CreateScheduleResult {
  createSchedule: (input: CreateScheduleInput) => Promise<ScheduleSummary>;
  isCreating: boolean;
}

export function useCreateSchedule({ serverId }: { serverId: string }): CreateScheduleResult {
  const client = useHostRuntimeClient(serverId);
  const queryClient = useQueryClient();

  const mutation = useMutation({
    mutationFn: async (input: CreateScheduleInput): Promise<ScheduleSummary> => {
      if (!client) {
        throw new Error("Daemon client not available");
      }
      const payload = await client.scheduleCreate({
        prompt: input.prompt,
        cadence: input.cadence,
        target: input.target as Parameters<typeof client.scheduleCreate>[0]["target"],
        ...(input.name !== undefined ? { name: input.name } : {}),
        ...(input.cwd !== undefined && input.cwd !== null ? { cwd: input.cwd } : {}),
        ...(typeof input.maxRuns === "number" ? { maxRuns: input.maxRuns } : {}),
        ...(input.expiresAt ? { expiresAt: input.expiresAt } : {}),
      });
      if (payload.error) {
        throw new Error(payload.error);
      }
      if (!payload.schedule) {
        throw new Error("Schedule creation failed");
      }
      return payload.schedule;
    },
    onSuccess: (schedule) => {
      const key = schedulesQueryKey(serverId);
      queryClient.setQueryData<SchedulesQueryData>(key, (old) => {
        if (!old) {
          return { schedules: [schedule], error: null };
        }
        if (old.schedules.some((s) => s.id === schedule.id)) {
          return old;
        }
        return { schedules: [schedule, ...old.schedules], error: old.error };
      });
      void queryClient.invalidateQueries({
        queryKey: key,
        refetchType: "active",
      });
    },
  });

  const createSchedule = useCallback(
    async (input: CreateScheduleInput): Promise<ScheduleSummary> => {
      return mutation.mutateAsync(input);
    },
    [mutation],
  );

  return {
    createSchedule,
    isCreating: mutation.isPending,
  };
}
