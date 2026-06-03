import { z } from "zod";

export const TmuxAgentInfoSchema = z.object({
  sessionName: z.string(),
  windowName: z.string(),
  paneId: z.string(),
  paneIndex: z.number().int(),
  panePid: z.number().int(),
  agentName: z.string(),
  currentCmd: z.string(),
  workingDir: z.string(),
});

export const TmuxListAgentsRequestSchema = z.object({
  type: z.literal("tmux/list_agents"),
  requestId: z.string(),
});

export const TmuxListAgentsResponseSchema = z.object({
  type: z.literal("tmux/list_agents/response"),
  payload: z.object({
    requestId: z.string(),
    agents: z.array(TmuxAgentInfoSchema),
    error: z.string().nullable(),
  }),
});

export const TmuxCapturePaneRequestSchema = z.object({
  type: z.literal("tmux/capture_pane"),
  paneId: z.string(),
  requestId: z.string(),
});

export const TmuxCapturePaneResponseSchema = z.object({
  type: z.literal("tmux/capture_pane/response"),
  payload: z.object({
    requestId: z.string(),
    content: z.string(),
    error: z.string().nullable(),
  }),
});

export const TmuxSendKeysRequestSchema = z.object({
  type: z.literal("tmux/send_keys"),
  paneId: z.string(),
  keys: z.string(),
  sendEnter: z.boolean().optional(),
  requestId: z.string(),
});

export const TmuxSendKeysResponseSchema = z.object({
  type: z.literal("tmux/send_keys/response"),
  payload: z.object({
    requestId: z.string(),
    error: z.string().nullable(),
  }),
});
