import { useCallback } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import type { ScheduleSummary, ScheduleCadence, ScheduleTarget } from "@server/server/schedule/types";
import { useHostRuntimeClient } from "@/runtime/host-runtime";
import { schedulesQueryKey } from "./use-schedules";

export interface ScheduleMutationsInput {
  serverId: string;
}

export interface UpdateScheduleInput {
  scheduleId: string;
  prompt: string;
  name?: string | null;
  cwd?: string | null;
  cadence: ScheduleCadence;
  target: ScheduleTarget;
}

export interface ScheduleMutationsResult {
  pauseSchedule: (scheduleId: string) => Promise<ScheduleSummary>;
  resumeSchedule: (scheduleId: string) => Promise<ScheduleSummary>;
  deleteSchedule: (scheduleId: string) => Promise<string>;
  updateSchedule: (input: UpdateScheduleInput) => Promise<ScheduleSummary>;
  isPausing: (scheduleId: string) => boolean;
  isResuming: (scheduleId: string) => boolean;
  isDeleting: (scheduleId: string) => boolean;
  isUpdating: (scheduleId: string) => boolean;
}

export function useScheduleMutations({ serverId }: ScheduleMutationsInput): ScheduleMutationsResult {
  const client = useHostRuntimeClient(serverId);
  const queryClient = useQueryClient();

  const pauseMutation = useMutation({
    mutationFn: async (scheduleId: string): Promise<ScheduleSummary> => {
      if (!client) {
        throw new Error("Daemon client not available");
      }
      const payload = await client.schedulePause({ id: scheduleId });
      if (payload.error) {
        throw new Error(payload.error);
      }
      if (!payload.schedule) {
        throw new Error("Schedule not found");
      }
      return payload.schedule;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: schedulesQueryKey(serverId) });
    },
  });

  const resumeMutation = useMutation({
    mutationFn: async (scheduleId: string): Promise<ScheduleSummary> => {
      if (!client) {
        throw new Error("Daemon client not available");
      }
      const payload = await client.scheduleResume({ id: scheduleId });
      if (payload.error) {
        throw new Error(payload.error);
      }
      if (!payload.schedule) {
        throw new Error("Schedule not found");
      }
      return payload.schedule;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: schedulesQueryKey(serverId) });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: async (scheduleId: string): Promise<string> => {
      if (!client) {
        throw new Error("Daemon client not available");
      }
      const payload = await client.scheduleDelete({ id: scheduleId });
      if (payload.error) {
        throw new Error(payload.error);
      }
      return payload.scheduleId;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: schedulesQueryKey(serverId) });
    },
  });

  const updateMutation = useMutation({
    mutationFn: async (input: UpdateScheduleInput): Promise<ScheduleSummary> => {
      if (!client) {
        throw new Error("Daemon client not available");
      }
      const payload = await client.scheduleUpdate({
        id: input.scheduleId,
        prompt: input.prompt,
        cadence: input.cadence,
        target: input.target as Parameters<typeof client.scheduleUpdate>[0]["target"],
        ...(input.name ? { name: input.name } : {}),
        ...(input.cwd !== undefined && input.cwd !== null ? { cwd: input.cwd } : {}),
      });
      if (payload.error) {
        throw new Error(payload.error);
      }
      if (!payload.schedule) {
        throw new Error("Schedule not found");
      }
      return payload.schedule;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: schedulesQueryKey(serverId) });
    },
  });

  const pauseSchedule = useCallback(
    async (scheduleId: string): Promise<ScheduleSummary> => {
      return pauseMutation.mutateAsync(scheduleId);
    },
    [pauseMutation],
  );

  const resumeSchedule = useCallback(
    async (scheduleId: string): Promise<ScheduleSummary> => {
      return resumeMutation.mutateAsync(scheduleId);
    },
    [resumeMutation],
  );

  const deleteSchedule = useCallback(
    async (scheduleId: string): Promise<string> => {
      return deleteMutation.mutateAsync(scheduleId);
    },
    [deleteMutation],
  );

  const updateSchedule = useCallback(
    async (input: UpdateScheduleInput): Promise<ScheduleSummary> => {
      return updateMutation.mutateAsync(input);
    },
    [updateMutation],
  );

  const isPausing = useCallback(
    (scheduleId: string): boolean => {
      return pauseMutation.isPending && pauseMutation.variables === scheduleId;
    },
    [pauseMutation.isPending, pauseMutation.variables],
  );

  const isResuming = useCallback(
    (scheduleId: string): boolean => {
      return resumeMutation.isPending && resumeMutation.variables === scheduleId;
    },
    [resumeMutation.isPending, resumeMutation.variables],
  );

  const isDeleting = useCallback(
    (scheduleId: string): boolean => {
      return deleteMutation.isPending && deleteMutation.variables === scheduleId;
    },
    [deleteMutation.isPending, deleteMutation.variables],
  );

  const isUpdating = useCallback(
    (scheduleId: string): boolean => {
      return updateMutation.isPending && updateMutation.variables?.scheduleId === scheduleId;
    },
    [updateMutation.isPending, updateMutation.variables],
  );

  return {
    pauseSchedule,
    resumeSchedule,
    deleteSchedule,
    updateSchedule,
    isPausing,
    isResuming,
    isDeleting,
    isUpdating,
  };
}
