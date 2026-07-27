import type { UsageQuotaListOptions } from "./daemon-client.js";
import type { ConnectionManager } from "./connection-manager.js";
import type { SessionOutboundMessage } from "../shared/messages.js";
import type {
  UsagePlan,
  UsageQuota,
  UsageQuotaSnapshot,
} from "../generated/protocol-schemas.js";

export type UsageQuotaListPayload = Extract<
  SessionOutboundMessage,
  { type: "usage/quota/list/response" }
>["payload"];

export type { UsagePlan, UsageQuota, UsageQuotaSnapshot };

export class UsageRpc {
  constructor(private readonly client: ConnectionManager) {}

  async usageQuotaList(opts?: UsageQuotaListOptions): Promise<UsageQuotaListPayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId: opts?.requestId,
      message: {
        type: "usage/quota/list",
        ...(typeof opts?.forceRefresh === "boolean" ? { forceRefresh: opts.forceRefresh } : {}),
      },
      responseType: "usage/quota/list/response",
      // A cold/forced refresh may block on provider fetches (per-provider
      // timeout ~15s on the daemon), so allow more headroom than the 10s
      // default used by the schedule RPCs.
      timeout: 20000,
    });
  }
}
