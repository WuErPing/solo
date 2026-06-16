import { z } from "zod";

export const TmuxPaneInfoSchema = z.object({
  sessionName: z.string(),
  windowName: z.string(),
  paneId: z.string(),
  paneIndex: z.number().int(),
  panePid: z.number().int(),
  currentCmd: z.string(),
  workingDir: z.string(),
  title: z.string().optional(),
  lastContentChange: z.number().int().optional(),
});

export const TmuxAgentInfoSchema = z.object({
  sessionName: z.string(),
  windowName: z.string(),
  paneId: z.string(),
  paneIndex: z.number().int(),
  panePid: z.number().int(),
  agentName: z.string(),
  currentCmd: z.string(),
  workingDir: z.string(),
  status: z.string().optional(),
  activity: z.string().optional(),
  launchCmd: z.string().optional(),
  lastContentChange: z.number().int().optional(),
});

export const AgentCommandEntrySchema = z.object({
  agentName: z.string(),
  launchCmd: z.string(),
  lastSeen: z.string(),
});

export const TmuxListAgentsRequestSchema = z.object({
  type: z.literal("tmux/list_agents"),
  requestId: z.string(),
});

export const TmuxListAgentsResponseSchema = z.object({
  type: z.literal("tmux/list_agents/response"),
  payload: z.object({
    requestId: z.string(),
    agents: z.array(TmuxAgentInfoSchema).nullish().default([]),
    otherPanes: z.array(TmuxPaneInfoSchema).nullish().default([]),
    commandHistory: z.array(AgentCommandEntrySchema).nullish().default([]),
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
    error: z.string().nullable(),
  }),
});

export const TmuxNewSessionRequestSchema = z.object({
  type: z.literal("tmux/new_session"),
  name: z.string(),
  workingDir: z.string().optional(),
  command: z.string().optional(),
  requestId: z.string(),
});

export const TmuxNewSessionResponseSchema = z.object({
  type: z.literal("tmux/new_session/response"),
  payload: z.object({
    requestId: z.string(),
    sessionName: z.string(),
    error: z.string().nullable(),
  }),
});

export const TmuxKillSessionRequestSchema = z.object({
  type: z.literal("tmux/kill_session"),
  sessionName: z.string(),
  requestId: z.string(),
});

export const TmuxKillSessionResponseSchema = z.object({
  type: z.literal("tmux/kill_session/response"),
  payload: z.object({
    requestId: z.string(),
    error: z.string().nullable(),
  }),
});

export const TmuxDeleteCommandHistoryRequestSchema = z.object({
  type: z.literal("tmux/delete_command_history"),
  launchCmd: z.string(),
  requestId: z.string(),
});

export const TmuxDeleteCommandHistoryResponseSchema = z.object({
  type: z.literal("tmux/delete_command_history/response"),
  payload: z.object({
    requestId: z.string(),
    error: z.string().nullable(),
  }),
});
