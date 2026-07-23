import { useCallback } from "react";
import type { ScheduleCadence, StoredSchedule } from "@server/server/schedule/types";
import { useHostRuntimeClient } from "@/runtime/host-runtime";
import {
  useCreateSchedule,
  type CreateScheduleResult,
} from "@/hooks/use-create-schedule";
import {
  useScheduleMutations,
  type ScheduleMutationsResult,
  type UpdateScheduleInput,
} from "@/hooks/use-schedule-mutations";
import { cronToUTC, detectTimezone } from "@/utils/cron-timezone";
import {
  useScheduleAssistantStore,
  type ScheduleAssistProposal,
} from "@/stores/schedule-assistant-store";

export interface UseProposalConfirmResult {
  confirmProposal: (messageId: string, proposal: ScheduleAssistProposal) => Promise<void>;
}

type HostClient = NonNullable<ReturnType<typeof useHostRuntimeClient>>;

/** Fields needed to render a success receipt after a proposal is applied. */
interface ProposalReceipt {
  text: string;
  receiptScheduleId?: string;
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

/** Merge the proposed update fields over the current schedule into an update payload. */
export function mergeUpdateProposal(
  proposal: ScheduleAssistProposal,
  current: StoredSchedule,
  timezone: string,
): UpdateScheduleInput {
  return {
    scheduleId: current.id,
    prompt: proposal.prompt ?? current.prompt,
    name: proposal.name ?? current.name,
    cwd: proposal.cwd ?? current.cwd ?? null,
    cadence: proposal.cadence ? toStorageCadence(proposal.cadence, timezone) : current.cadence,
    target: proposal.target ?? current.target,
  };
}

async function applyCreateProposal(
  proposal: ScheduleAssistProposal,
  createSchedule: CreateScheduleResult["createSchedule"],
  timezone: string,
): Promise<ProposalReceipt> {
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
  return {
    text: receiptLabel("Created", schedule.name ?? proposal.name),
    receiptScheduleId: schedule.id,
  };
}

async function applyUpdateProposal(
  proposal: ScheduleAssistProposal,
  deps: {
    client: HostClient;
    updateSchedule: ScheduleMutationsResult["updateSchedule"];
  },
  timezone: string,
): Promise<ProposalReceipt> {
  if (!proposal.scheduleId) {
    throw new Error("Proposal is missing the schedule to update");
  }
  const inspected = await deps.client.scheduleInspect({ id: proposal.scheduleId });
  const current = inspected.schedule;
  if (!current) {
    throw new Error(inspected.error ?? "Schedule not found");
  }
  await deps.updateSchedule(mergeUpdateProposal(proposal, current, timezone));
  return {
    text: receiptLabel("Updated", proposal.name ?? current.name),
    receiptScheduleId: current.id,
  };
}

async function applyLifecycleProposal(
  proposal: ScheduleAssistProposal,
  deps: {
    pauseSchedule: ScheduleMutationsResult["pauseSchedule"];
    resumeSchedule: ScheduleMutationsResult["resumeSchedule"];
    deleteSchedule: ScheduleMutationsResult["deleteSchedule"];
  },
): Promise<ProposalReceipt> {
  if (!proposal.scheduleId) {
    throw new Error(`Proposal is missing the schedule to ${proposal.op}`);
  }
  const scheduleId = proposal.scheduleId;
  if (proposal.op === "pause") {
    await deps.pauseSchedule(scheduleId);
  } else if (proposal.op === "resume") {
    await deps.resumeSchedule(scheduleId);
  } else {
    await deps.deleteSchedule(scheduleId);
  }
  const verb =
    proposal.op === "pause" ? "Paused" : proposal.op === "resume" ? "Resumed" : "Deleted";
  return {
    text: receiptLabel(verb, proposal.name),
    // Deleted schedules no longer exist — nothing to link to.
    ...(proposal.op === "delete" ? {} : { receiptScheduleId: scheduleId }),
  };
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
        let receipt: ProposalReceipt;
        if (proposal.op === "create") {
          receipt = await applyCreateProposal(proposal, createSchedule, timezone);
        } else if (proposal.op === "update") {
          if (!client) {
            throw new Error("Daemon client not available");
          }
          receipt = await applyUpdateProposal(proposal, { client, updateSchedule }, timezone);
        } else {
          receipt = await applyLifecycleProposal(proposal, {
            pauseSchedule,
            resumeSchedule,
            deleteSchedule,
          });
        }
        updateMessage(serverId, messageId, {
          applying: false,
          kind: "receipt",
          text: receipt.text,
          ...(receipt.receiptScheduleId
            ? { receiptScheduleId: receipt.receiptScheduleId }
            : {}),
        });
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
