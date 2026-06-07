import type { AgentLifecycleStatus } from "@server/shared/agent-lifecycle";
import type { AgentStreamEventPayload } from "@server/shared/messages";

export function deriveOptimisticLifecycleStatus(
  currentStatus: AgentLifecycleStatus,
  event: AgentStreamEventPayload,
): AgentLifecycleStatus | null {
  switch (event.type) {
    case "turn_completed":
      // Allow transition from idle/initializing as well as running.
      // This fixes a race where the agent_update (running) is buffered
      // during history sync, so the reducer snapshot has stale status
      // when turn_completed arrives.
      if (currentStatus === "closed" || currentStatus === "error") {
        return null;
      }
      return "idle";
    case "turn_failed":
      if (currentStatus === "closed" || currentStatus === "error") {
        return null;
      }
      return "error";
    case "turn_canceled":
      // A canceled turn can be either a final user cancel or an interrupt before
      // a replacement turn starts. The daemon snapshot is authoritative here.
      return null;
    default:
      return null;
  }
}
