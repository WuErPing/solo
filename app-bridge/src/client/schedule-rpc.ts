import type {
  CreateScheduleOptions,
  DeleteLoopOptions,
  DeleteLoopTemplateOptions,
  GetLoopTemplateOptions,
  InspectLoopOptions,
  InspectScheduleOptions,
  LoopLogsOptions,
  RunLoopOptions,
  ScheduleAssistOptions,
  StopLoopOptions,
  UpdateLoopOptions,
  UpdateScheduleOptions,
} from "./daemon-client.js";
import type { ConnectionManager } from "./connection-manager.js";
import type { SessionOutboundMessage } from "../shared/messages.js";

type ScheduleCreatePayload = Extract<SessionOutboundMessage, { type: "schedule/create/response" }>["payload"];
type ScheduleListPayload = Extract<SessionOutboundMessage, { type: "schedule/list/response" }>["payload"];
type ScheduleInspectPayload = Extract<SessionOutboundMessage, { type: "schedule/inspect/response" }>["payload"];
type ScheduleLogsPayload = Extract<SessionOutboundMessage, { type: "schedule/logs/response" }>["payload"];
type SchedulePausePayload = Extract<SessionOutboundMessage, { type: "schedule/pause/response" }>["payload"];
type ScheduleResumePayload = Extract<SessionOutboundMessage, { type: "schedule/resume/response" }>["payload"];
type ScheduleDeletePayload = Extract<SessionOutboundMessage, { type: "schedule/delete/response" }>["payload"];
type ScheduleUpdatePayload = Extract<SessionOutboundMessage, { type: "schedule/update/response" }>["payload"];
type ScheduleAssistPayload = Extract<SessionOutboundMessage, { type: "schedule/assist/response" }>["payload"];
type LoopRunPayload = Extract<SessionOutboundMessage, { type: "loop/run/response" }>["payload"];
type LoopListPayload = Extract<SessionOutboundMessage, { type: "loop/list/response" }>["payload"];
type LoopInspectPayload = Extract<SessionOutboundMessage, { type: "loop/inspect/response" }>["payload"];
type LoopLogsPayload = Extract<SessionOutboundMessage, { type: "loop/logs/response" }>["payload"];
type LoopStopPayload = Extract<SessionOutboundMessage, { type: "loop/stop/response" }>["payload"];
type LoopUpdatePayload = Extract<SessionOutboundMessage, { type: "loop/update/response" }>["payload"];
type LoopDeletePayload = Extract<SessionOutboundMessage, { type: "loop/delete/response" }>["payload"];
type LoopTemplateListPayload = Extract<SessionOutboundMessage, { type: "loop/template/list/response" }>["payload"];
type LoopTemplateGetPayload = Extract<SessionOutboundMessage, { type: "loop/template/get/response" }>["payload"];
type LoopTemplateDeletePayload = Extract<SessionOutboundMessage, { type: "loop/template/delete/response" }>["payload"];

export class ScheduleRpc {
  constructor(private readonly client: ConnectionManager) {}

  async scheduleCreate(options: CreateScheduleOptions): Promise<ScheduleCreatePayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId: options.requestId,
      message: {
        type: "schedule/create",
        prompt: options.prompt,
        cadence: options.cadence,
        target: options.target,
        ...(options.name ? { name: options.name } : {}),
        ...(options.cwd !== undefined && options.cwd !== null ? { cwd: options.cwd } : {}),
        ...(typeof options.maxRuns === "number" ? { maxRuns: options.maxRuns } : {}),
        ...(options.expiresAt ? { expiresAt: options.expiresAt } : {}),
      },
      responseType: "schedule/create/response",
      timeout: 10000,
    });
  }

  async scheduleList(requestId?: string): Promise<ScheduleListPayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId,
      message: {
        type: "schedule/list",
      },
      responseType: "schedule/list/response",
      timeout: 10000,
    });
  }

  async scheduleInspect(options: InspectScheduleOptions): Promise<ScheduleInspectPayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId: options.requestId,
      message: {
        type: "schedule/inspect",
        scheduleId: options.id,
      },
      responseType: "schedule/inspect/response",
      timeout: 10000,
    });
  }

  async scheduleLogs(options: InspectScheduleOptions): Promise<ScheduleLogsPayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId: options.requestId,
      message: {
        type: "schedule/logs",
        scheduleId: options.id,
      },
      responseType: "schedule/logs/response",
      timeout: 10000,
    });
  }

  async schedulePause(options: InspectScheduleOptions): Promise<SchedulePausePayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId: options.requestId,
      message: {
        type: "schedule/pause",
        scheduleId: options.id,
      },
      responseType: "schedule/pause/response",
      timeout: 10000,
    });
  }

  async scheduleResume(options: InspectScheduleOptions): Promise<ScheduleResumePayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId: options.requestId,
      message: {
        type: "schedule/resume",
        scheduleId: options.id,
      },
      responseType: "schedule/resume/response",
      timeout: 10000,
    });
  }

  async scheduleDelete(options: InspectScheduleOptions): Promise<ScheduleDeletePayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId: options.requestId,
      message: {
        type: "schedule/delete",
        scheduleId: options.id,
      },
      responseType: "schedule/delete/response",
      timeout: 10000,
    });
  }

  async scheduleUpdate(options: UpdateScheduleOptions): Promise<ScheduleUpdatePayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId: options.requestId,
      message: {
        type: "schedule/update",
        scheduleId: options.id,
        prompt: options.prompt,
        cadence: options.cadence,
        target: options.target,
        ...(options.name ? { name: options.name } : {}),
        ...(options.cwd !== undefined && options.cwd !== null ? { cwd: options.cwd } : {}),
        ...(typeof options.maxRuns === "number" ? { maxRuns: options.maxRuns } : {}),
        ...(options.expiresAt ? { expiresAt: options.expiresAt } : {}),
      },
      responseType: "schedule/update/response",
      timeout: 10000,
    });
  }

  async scheduleAssist(options: ScheduleAssistOptions): Promise<ScheduleAssistPayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId: options.requestId,
      message: {
        type: "schedule/assist",
        message: options.message,
        timezone: options.timezone,
        clientNow: options.clientNow,
        ...(options.contextScheduleId ? { contextScheduleId: options.contextScheduleId } : {}),
        ...(options.transcript && options.transcript.length > 0
          ? { transcript: options.transcript }
          : {}),
      },
      responseType: "schedule/assist/response",
      timeout: 120000,
    });
  }

  async loopRun(options: RunLoopOptions): Promise<LoopRunPayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId: options.requestId,
      message: {
        type: "loop/run",
        prompt: options.prompt,
        cwd: options.cwd,
        ...(options.verifyPrompt ? { verifyPrompt: options.verifyPrompt } : {}),
        ...(options.verifyChecks && options.verifyChecks.length > 0
          ? { verifyChecks: options.verifyChecks }
          : {}),
        ...(options.name ? { name: options.name } : {}),
        ...(typeof options.sleepMs === "number" ? { sleepMs: options.sleepMs } : {}),
        ...(typeof options.maxIterations === "number"
          ? { maxIterations: options.maxIterations }
          : {}),
        ...(typeof options.maxTimeMs === "number" ? { maxTimeMs: options.maxTimeMs } : {}),
        ...(options.agentTemplate ? { agentTemplate: options.agentTemplate } : {}),
        ...(options.workerAgentTemplate
          ? { workerAgentTemplate: options.workerAgentTemplate }
          : {}),
        ...(options.verifierAgentTemplate
          ? { verifierAgentTemplate: options.verifierAgentTemplate }
          : {}),
      },
      responseType: "loop/run/response",
      timeout: 15000,
    });
  }

  async loopList(requestId?: string): Promise<LoopListPayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId,
      message: {
        type: "loop/list",
      },
      responseType: "loop/list/response",
      timeout: 10000,
    });
  }

  async loopInspect(options: string | InspectLoopOptions): Promise<LoopInspectPayload> {
    const normalized = typeof options === "string" ? { id: options } : options;
    return this.client.sendCorrelatedSessionRequest({
      requestId: normalized.requestId,
      message: {
        type: "loop/inspect",
        id: normalized.id,
      },
      responseType: "loop/inspect/response",
      timeout: 10000,
    });
  }

  async loopLogs(options: string | LoopLogsOptions, afterSeq?: number): Promise<LoopLogsPayload> {
    const normalized = typeof options === "string" ? { id: options, afterSeq } : options;
    return this.client.sendCorrelatedSessionRequest({
      requestId: normalized.requestId,
      message: {
        type: "loop/logs",
        id: normalized.id,
        ...(typeof normalized.afterSeq === "number" ? { afterSeq: normalized.afterSeq } : {}),
      },
      responseType: "loop/logs/response",
      timeout: 10000,
    });
  }

  async loopStop(options: string | StopLoopOptions): Promise<LoopStopPayload> {
    const normalized = typeof options === "string" ? { id: options } : options;
    return this.client.sendCorrelatedSessionRequest({
      requestId: normalized.requestId,
      message: {
        type: "loop/stop",
        id: normalized.id,
      },
      responseType: "loop/stop/response",
      timeout: 10000,
    });
  }

  async loopUpdate(options: UpdateLoopOptions): Promise<LoopUpdatePayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId: options.requestId,
      message: {
        type: "loop/update",
        id: options.id,
        ...(options.name ? { name: options.name } : {}),
        ...(typeof options.archive === "boolean" ? { archive: options.archive } : {}),
        ...(options.prompt ? { prompt: options.prompt } : {}),
        ...(options.cwd ? { cwd: options.cwd } : {}),
        ...(options.verifyChecks ? { verifyChecks: options.verifyChecks } : {}),
        ...(options.maxIterations != null ? { maxIterations: options.maxIterations } : {}),
        ...(options.agentTemplate ? { agentTemplate: options.agentTemplate } : {}),
        ...(options.workerAgentTemplate
          ? { workerAgentTemplate: options.workerAgentTemplate }
          : {}),
        ...(options.verifierAgentTemplate
          ? { verifierAgentTemplate: options.verifierAgentTemplate }
          : {}),
      },
      responseType: "loop/update/response",
      timeout: 10000,
    });
  }

  async loopDelete(options: string | DeleteLoopOptions): Promise<LoopDeletePayload> {
    const normalized = typeof options === "string" ? { id: options } : options;
    return this.client.sendCorrelatedSessionRequest({
      requestId: normalized.requestId,
      message: {
        type: "loop/delete",
        id: normalized.id,
      },
      responseType: "loop/delete/response",
      timeout: 10000,
    });
  }

  async loopTemplateList(requestId?: string): Promise<LoopTemplateListPayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId,
      message: {
        type: "loop/template/list",
      },
      responseType: "loop/template/list/response",
      timeout: 10000,
    });
  }

  async loopTemplateGet(options: string | GetLoopTemplateOptions): Promise<LoopTemplateGetPayload> {
    const normalized = typeof options === "string" ? { templateID: options } : options;
    return this.client.sendCorrelatedSessionRequest({
      requestId: normalized.requestId,
      message: {
        type: "loop/template/get",
        templateID: normalized.templateID,
      },
      responseType: "loop/template/get/response",
      timeout: 10000,
    });
  }

  async loopTemplateDelete(options: string | DeleteLoopTemplateOptions): Promise<LoopTemplateDeletePayload> {
    const normalized = typeof options === "string" ? { templateID: options } : options;
    return this.client.sendCorrelatedSessionRequest({
      requestId: normalized.requestId,
      message: {
        type: "loop/template/delete",
        templateID: normalized.templateID,
      },
      responseType: "loop/template/delete/response",
      timeout: 10000,
    });
  }
}
