import { z } from "zod";
import {
  UsageQuotaListRequestSchema as GeneratedUsageQuotaListRequestSchema,
  UsageQuotaListResponseSchema as GeneratedUsageQuotaListResponseSchema,
} from "../../generated/protocol-schemas.js";

// The generated schemas type the wire `type` field as z.string(); the session
// discriminated unions need a literal discriminant, so extend the generated
// schemas here instead of hand-writing duplicates.

export const UsageQuotaListRequestSchema = GeneratedUsageQuotaListRequestSchema.extend({
  type: z.literal("usage/quota/list"),
});

export const UsageQuotaListResponseSchema = GeneratedUsageQuotaListResponseSchema.extend({
  type: z.literal("usage/quota/list/response"),
});
