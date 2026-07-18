import { useCallback } from "react";
import type { ScheduleCadence } from "@server/server/schedule/types";
import { useHostRuntimeClient } from "@/runtime/host-runtime";
import { useCreateSchedule } from "@/hooks/use-create-schedule";
import { useScheduleMutations } from "@/hooks/use-schedule-mutations";
import { cronToUTC, detectTimezone } from "@/utils/cron-timezone";
import {
  useScheduleAssistantStore,
  type ScheduleAssistProposal,
} from "@/stores/schedule-assistant-store";

export interface UseProposalConfirmResult {
  confirmProposal: (messageId: string, proposal: ScheduleAssistProposal) => Promise<void>;
}

/** Proposal cadences are local; storage keeps cron in UTC (existing convention). */
function toStorageCadence(cadence: ScheduleCadence, timezone: string): ScheduleCadence {
  if (cadence.type === "cron") {
    return { type: "cron", expression: cronToUTC(cadence.expression, timezone), timezone };
  }
  return cadence;
}

function receiptLabel(verb: string, name?: string | null): string {
  return name ? `${verb} ✓ "${name}"` : `${verb} ✓`;
}

export function useProposalConfirm({ serverId }: { serverId: string }): UseProposalConfirmResult {
  const client = useHostRuntimeClient(serverId);
  const { createSchedule } = useCreateSchedule({ serverId });
  const { pauseSchedule, resumeSchedule, deleteSchedule, updateSchedule } = useScheduleMutations({
    serverId,
  });
  const updateMessage = useScheduleAssistantStore((state) => state.updateMessage);

  const confirmProposal = useCallback(
    async (messageId: string, proposal: ScheduleAssistProposal) => {
      const timezone = detectTimezone();
      updateMessage(serverId, messageId, { applying: true, applyError: undefined });
      try {
        switch (proposal.op) {
          case "create": {
            if (!proposal.prompt || !proposal.cadence || !proposal.target) {
              throw new Error("Proposal is missing prompt, cadence, or target");
            }
            const schedule = await createSchedule({
              name: proposal.name ?? null,
              prompt: proposal.prompt,
              cadence: toStorageCadence(proposal.cadence, timezone),
              target: proposal.target,
              cwd: proposal.cwd ?? null,
              ...(typeof proposal.maxRuns === "number" ? { maxRuns: proposal.maxRuns } : {}),
              ...(proposal.expiresAt ? { expiresAt: proposal.expiresAt } : {}),
            });
            updateMessage(serverId, messageId, {
              applying: false,
              kind: "receipt",
              text: receiptLabel("Created", schedule.name ?? proposal.name),
              receiptScheduleId: schedule.id,
            });
            break;
          }
          case "update": {
            if (!proposal.scheduleId) {
              throw new Error("Proposal is missing the schedule to update");
            }
            if (!client) {
              throw new Error("Daemon client not available");
            }
            const inspected = await client.scheduleInspect({ id: proposal.scheduleId });
            const current = inspected.schedule;
            if (!current) {
              throw new Error(inspected.error ?? "Schedule not found");
            }
            await updateSchedule({
              scheduleId: current.id,
              prompt: proposal.prompt ?? current.prompt,
              name: proposal.name ?? current.name,
              cwd: proposal.cwd ?? current.cwd ?? null,
              cadence: proposal.cadence
                ? toStorageCadence(proposal.cadence, timezone)
                : current.cadence,
              target: proposal.target ?? current.target,
            });
            updateMessage(serverId, messageId, {
              applying: false,
              kind: "receipt",
              text: receiptLabel("Updated", proposal.name ?? current.name),
              receiptScheduleId: current.id,
            });
            break;
          }
          case "pause":
          case "resume":
          case "delete": {
            if (!proposal.scheduleId) {
              throw new Error(`Proposal is missing the schedule to ${proposal.op}`);
            }
            const scheduleId = proposal.scheduleId;
            if (proposal.op === "pause") {
              await pauseSchedule(scheduleId);
            } else if (proposal.op === "resume") {
              await resumeSchedule(scheduleId);
            } else {
              await deleteSchedule(scheduleId);
            }
            const verb = proposal.op === "pause" ? "Paused" : proposal.op === "resume" ? "Resumed" : "Deleted";
            updateMessage(serverId, messageId, {
              applying: false,
              kind: "receipt",
              text: receiptLabel(verb, proposal.name),
              // Deleted schedules no longer exist — nothing to link to.
              ...(proposal.op === "delete" ? {} : { receiptScheduleId: scheduleId }),
            });
            break;
          }
        }
      } catch (error) {
        updateMessage(serverId, messageId, {
          applying: false,
          applyError: error instanceof Error ? error.message : "Failed to apply the proposal",
        });
      }
    },
    [
      client,
      createSchedule,
      deleteSchedule,
      pauseSchedule,
      resumeSchedule,
      serverId,
      updateMessage,
      updateSchedule,
    ],
  );

  return { confirmProposal };
}
