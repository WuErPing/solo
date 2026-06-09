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
  startLine: z.number().int().optional(),
  lastContentHash: z.string().optional(),
  requestId: z.string(),
});

export const TmuxCapturePaneResponseSchema = z.object({
  type: z.literal("tmux/capture_pane/response"),
  payload: z.object({
    requestId: z.string(),
    content: z.string(),
    changed: z.boolean().optional(),
    contentHash: z.string().optional(),
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

export const TmuxThemeColorsSchema = z.object({
  background: z.string(),
  foreground: z.string(),
  paneActiveBorder: z.string().optional(),
  paneInactiveBorder: z.string().optional(),
  statusBackground: z.string().optional(),
  statusForeground: z.string().optional(),
  messageBackground: z.string().optional(),
  messageForeground: z.string().optional(),
  windowStatusCurrentBg: z.string().optional(),
  windowStatusCurrentFg: z.string().optional(),
});

export const TmuxGetThemeRequestSchema = z.object({
  type: z.literal("tmux/get_theme"),
  sessionId: z.string(),
  requestId: z.string(),
});

export const TmuxGetThemeResponseSchema = z.object({
  type: z.literal("tmux/get_theme/response"),
  payload: z.object({
    requestId: z.string(),
    theme: TmuxThemeColorsSchema,
    error: z.string().nullable(),
  }),
});

export const TmuxStatusLineRequestSchema = z.object({
  type: z.literal("tmux/status_line"),
  sessionId: z.string(),
  requestId: z.string(),
});

export const TmuxStatusLineResponseSchema = z.object({
  type: z.literal("tmux/status_line/response"),
  payload: z.object({
    requestId: z.string(),
    statusLeft: z.string(),
    statusCenter: z.string(),
    statusRight: z.string(),
    paneBackground: z.string().optional(),
    paneForeground: z.string().optional(),
    error: z.string().nullable(),
  }),
});
