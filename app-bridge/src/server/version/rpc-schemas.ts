import { z } from "zod";
import {
  DaemonVersionSwitchRequestedStatusPayloadSchema as GeneratedDaemonVersionSwitchRequestedStatusPayloadSchema,
  ListDaemonVersionsRequestSchema as GeneratedListDaemonVersionsRequestSchema,
  ListDaemonVersionsResponseSchema as GeneratedListDaemonVersionsResponseSchema,
  SwitchDaemonVersionRequestSchema as GeneratedSwitchDaemonVersionRequestSchema,
} from "../../generated/protocol-schemas.js";

// The generated schemas type the wire `type`/`status` fields as z.string();
// the session discriminated unions need literal discriminants, so extend the
// generated schemas here instead of hand-writing duplicates.

export const ListDaemonVersionsRequestSchema = GeneratedListDaemonVersionsRequestSchema.extend({
  type: z.literal("list_daemon_versions_request"),
});

export const ListDaemonVersionsResponseSchema = GeneratedListDaemonVersionsResponseSchema.extend({
  type: z.literal("list_daemon_versions_response"),
});

export const SwitchDaemonVersionRequestSchema = GeneratedSwitchDaemonVersionRequestSchema.extend({
  type: z.literal("switch_daemon_version_request"),
});

export const DaemonVersionSwitchRequestedStatusPayloadSchema =
  GeneratedDaemonVersionSwitchRequestedStatusPayloadSchema.extend({
    status: z.literal("daemon_version_switch_requested"),
  });

export type DaemonVersionSwitchRequestedStatusPayload = z.infer<
  typeof DaemonVersionSwitchRequestedStatusPayloadSchema
>;
