import { z } from "zod";
import {
  TerminalCellSchema as TerminalCellSchemaGenerated,
  TerminalCursorSchema as TerminalCursorSchemaGenerated,
  TerminalStateSchema as TerminalStateSchemaGenerated,
} from "../generated/protocol-schemas.js";

// ============================================================================
// Terminal Messages
// ============================================================================

export const ListTerminalsRequestSchema = z.object({
  type: z.literal("list_terminals_request"),
  cwd: z.string().optional(),
  requestId: z.string(),
});

export const SubscribeTerminalsRequestSchema = z.object({
  type: z.literal("subscribe_terminals_request"),
  cwd: z.string(),
});

export const UnsubscribeTerminalsRequestSchema = z.object({
  type: z.literal("unsubscribe_terminals_request"),
  cwd: z.string(),
});

export const CreateTerminalRequestSchema = z.object({
  type: z.literal("create_terminal_request"),
  cwd: z.string(),
  name: z.string().optional(),
  agentId: z.string().optional(),
  command: z.string().optional(),
  args: z.array(z.string()).optional(),
  requestId: z.string(),
});

export const SubscribeTerminalRequestSchema = z.object({
  type: z.literal("subscribe_terminal_request"),
  terminalId: z.string(),
  requestId: z.string(),
});

export const UnsubscribeTerminalRequestSchema = z.object({
  type: z.literal("unsubscribe_terminal_request"),
  terminalId: z.string(),
});

const TerminalClientMessageSchema = z.discriminatedUnion("type", [
  z.object({ type: z.literal("input"), data: z.string() }),
  z.object({ type: z.literal("resize"), rows: z.number(), cols: z.number() }),
  z.object({
    type: z.literal("mouse"),
    row: z.number(),
    col: z.number(),
    button: z.number(),
    action: z.enum(["down", "up", "move"]),
  }),
]);

export const TerminalInputSchema = z.object({
  type: z.literal("terminal_input"),
  terminalId: z.string(),
  message: TerminalClientMessageSchema,
});

export const KillTerminalRequestSchema = z.object({
  type: z.literal("kill_terminal_request"),
  terminalId: z.string(),
  requestId: z.string(),
});

export const CaptureTerminalRequestSchema = z.object({
  type: z.literal("capture_terminal_request"),
  terminalId: z.string(),
  start: z.number().int().optional(),
  end: z.number().int().optional(),
  stripAnsi: z.boolean().default(true),
  requestId: z.string(),
});

// ============================================================================
// Terminal Outbound Messages
// ============================================================================

const TerminalInfoSchema = z.object({
  id: z.string(),
  name: z.string(),
  cwd: z.string(),
  title: z.string().optional(),
});

export const TerminalCellSchema = TerminalCellSchemaGenerated;

export const TerminalCursorStyleSchema = z.enum(["block", "underline", "bar"]);

export const TerminalCursorSchema = TerminalCursorSchemaGenerated;

export const TerminalStateSchema = TerminalStateSchemaGenerated;

export const ListTerminalsResponseSchema = z.object({
  type: z.literal("list_terminals_response"),
  payload: z.object({
    cwd: z.string().optional(),
    terminals: z.array(TerminalInfoSchema.omit({ cwd: true })),
    requestId: z.string(),
  }),
});

export const TerminalsChangedSchema = z.object({
  type: z.literal("terminals_changed"),
  payload: z.object({
    cwd: z.string(),
    terminals: z.array(TerminalInfoSchema.omit({ cwd: true })),
  }),
});

export const CreateTerminalResponseSchema = z.object({
  type: z.literal("create_terminal_response"),
  payload: z.object({
    terminal: TerminalInfoSchema.nullable(),
    error: z.string().nullable(),
    requestId: z.string(),
  }),
});

export const SubscribeTerminalResponseSchema = z.object({
  type: z.literal("subscribe_terminal_response"),
  payload: z.union([
    z.object({
      terminalId: z.string(),
      slot: z.number().int().min(0).max(255),
      error: z.null(),
      requestId: z.string(),
    }),
    z.object({
      terminalId: z.string(),
      error: z.string(),
      requestId: z.string(),
    }),
  ]),
});

export const KillTerminalResponseSchema = z.object({
  type: z.literal("kill_terminal_response"),
  payload: z.object({
    terminalId: z.string(),
    success: z.boolean(),
    requestId: z.string(),
  }),
});

export const CaptureTerminalResponseSchema = z.object({
  type: z.literal("capture_terminal_response"),
  payload: z.object({
    terminalId: z.string(),
    lines: z.array(z.string()),
    totalLines: z.number().int().nonnegative(),
    requestId: z.string(),
  }),
});

export const TerminalStreamExitSchema = z.object({
  type: z.literal("terminal_stream_exit"),
  payload: z.object({
    terminalId: z.string(),
  }),
});

// Terminal message types
export type ListTerminalsRequest = z.infer<typeof ListTerminalsRequestSchema>;
export type ListTerminalsResponse = z.infer<typeof ListTerminalsResponseSchema>;
export type SubscribeTerminalsRequest = z.infer<typeof SubscribeTerminalsRequestSchema>;
export type UnsubscribeTerminalsRequest = z.infer<typeof UnsubscribeTerminalsRequestSchema>;
export type TerminalsChanged = z.infer<typeof TerminalsChangedSchema>;
export type CreateTerminalRequest = z.infer<typeof CreateTerminalRequestSchema>;
export type CreateTerminalResponse = z.infer<typeof CreateTerminalResponseSchema>;
export type SubscribeTerminalRequest = z.infer<typeof SubscribeTerminalRequestSchema>;
export type SubscribeTerminalResponse = z.infer<typeof SubscribeTerminalResponseSchema>;
export type UnsubscribeTerminalRequest = z.infer<typeof UnsubscribeTerminalRequestSchema>;
export type TerminalInput = z.infer<typeof TerminalInputSchema>;
export type TerminalCell = z.infer<typeof TerminalCellSchema>;
export type TerminalCursorStyle = z.infer<typeof TerminalCursorStyleSchema>;
export type TerminalCursor = z.infer<typeof TerminalCursorSchema>;
export type TerminalState = z.infer<typeof TerminalStateSchema>;
export type KillTerminalRequest = z.infer<typeof KillTerminalRequestSchema>;
export type KillTerminalResponse = z.infer<typeof KillTerminalResponseSchema>;
export type CaptureTerminalRequest = z.infer<typeof CaptureTerminalRequestSchema>;
export type CaptureTerminalResponse = z.infer<typeof CaptureTerminalResponseSchema>;
export type TerminalStreamExit = z.infer<typeof TerminalStreamExitSchema>;
