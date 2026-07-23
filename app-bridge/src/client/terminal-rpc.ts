import { SessionInboundMessageSchema } from "../shared/messages.js";
import type {
  SessionOutboundMessage,
  TerminalInput,
  ListTerminalsResponse,
  CreateTerminalResponse,
  SubscribeTerminalResponse,
  CloseItemsResponse,
  KillTerminalResponse,
  CaptureTerminalResponse,
} from "../shared/messages.js";
import type { TerminalStreamEvent } from "./daemon-client.js";
import type { ConnectionManager } from "./connection-manager.js";
import { getDefaultLogger, toErrorInfo } from "../shared/logger.js";
import {
  decodeTerminalSnapshotPayload,
  decodeTerminalStreamFrame,
  encodeTerminalResizePayload,
  encodeTerminalStreamFrame,
  TerminalStreamOpcode,
  type TerminalStreamFrame,
} from "../shared/terminal-stream-protocol.js";
import { encodeUtf8String } from "./daemon-client-transport.js";

// ============================================================================
// Type aliases (not exported from daemon-client.ts)
// ============================================================================

type SetVoiceModePayload = Extract<
  SessionOutboundMessage,
  { type: "set_voice_mode_response" }
>["payload"];
type DictationFinishAcceptedPayload = Extract<
  SessionOutboundMessage,
  { type: "dictation_stream_finish_accepted" }
>["payload"];
type ListTerminalsPayload = ListTerminalsResponse["payload"];
type CreateTerminalPayload = CreateTerminalResponse["payload"];
type SubscribeTerminalPayload = SubscribeTerminalResponse["payload"];
type CloseItemsPayload = CloseItemsResponse["payload"];
type KillTerminalPayload = KillTerminalResponse["payload"];
type CaptureTerminalPayload = CaptureTerminalResponse["payload"];
type TmuxListAgentsPayload = Extract<
  SessionOutboundMessage,
  { type: "tmux/list_agents/response" }
>["payload"];
type TmuxCapturePanePayload = Extract<
  SessionOutboundMessage,
  { type: "tmux/capture_pane/response" }
>["payload"];
type TmuxSendKeysPayload = Extract<
  SessionOutboundMessage,
  { type: "tmux/send_keys/response" }
>["payload"];
type TmuxNewSessionPayload = Extract<
  SessionOutboundMessage,
  { type: "tmux/new_session/response" }
>["payload"];
type TmuxKillSessionPayload = Extract<
  SessionOutboundMessage,
  { type: "tmux/kill_session/response" }
>["payload"];
type TmuxDeleteCommandHistoryPayload = Extract<
  SessionOutboundMessage,
  { type: "tmux/delete_command_history/response" }
>["payload"];
type TmuxStatusLinePayload = Extract<
  SessionOutboundMessage,
  { type: "tmux/status_line/response" }
>["payload"];

// ============================================================================
// Constants (not exported from daemon-client.ts)
// ============================================================================

const DEFAULT_DICTATION_FINISH_ACCEPT_TIMEOUT_MS = 15000;
const DEFAULT_DICTATION_FINISH_FALLBACK_TIMEOUT_MS = 5 * 60 * 1000;
const DEFAULT_DICTATION_FINISH_TIMEOUT_GRACE_MS = 5000;

// ============================================================================

const perfNow: () => number =
  typeof performance !== "undefined" && typeof performance.now === "function"
    ? () => performance.now()
    : () => Date.now();

function isWaiterTimeoutError(error: unknown): boolean {
  return error instanceof Error && error.message.startsWith("Timeout waiting for message");
}

export class TerminalRpc {
  private terminalSlots = new Map<string, number>();
  private slotTerminals = new Map<number, string>();
  private terminalDirectorySubscriptions = new Set<string>();
  private readonly terminalStreamListeners = new Set<(event: TerminalStreamEvent) => void>();

  constructor(private readonly client: ConnectionManager) {}

  // ============================================================================
  // Audio / Voice
  // ============================================================================

  async setVoiceMode(enabled: boolean, agentId?: string): Promise<SetVoiceModePayload> {
    const requestId = this.client.createRequestId();
    const message = SessionInboundMessageSchema.parse({
      type: "set_voice_mode",
      enabled,
      ...(agentId ? { agentId } : {}),
      requestId,
    });
    const response = await this.client.sendRequest({
      requestId,
      message,
      timeout: 10000,
      select: (msg) => {
        if (msg.type !== "set_voice_mode_response") {
          return null;
        }
        if (msg.payload.requestId !== requestId) {
          return null;
        }
        return msg.payload;
      },
    });
    if (!response.accepted) {
      const codeSuffix =
        typeof response.reasonCode === "string" && response.reasonCode.trim().length > 0
          ? ` (${response.reasonCode})`
          : "";
      throw new Error((response.error ?? "Failed to set voice mode") + codeSuffix);
    }
    return response;
  }

  async sendVoiceAudioChunk(audio: string, format: string, isLast = false): Promise<void> {
    this.client.sendSessionMessage({ type: "voice_audio_chunk", audio, format, isLast });
  }

  async startDictationStream(dictationId: string, format: string): Promise<void> {
    const ack = this.client.waitForWithCancel(
      (msg) => {
        if (msg.type !== "dictation_stream_ack") {
          return null;
        }
        if (msg.payload.dictationId !== dictationId) {
          return null;
        }
        if (msg.payload.ackSeq !== -1) {
          return null;
        }
        return msg.payload;
      },
      30000,
      { skipQueue: true },
    );
    const ackPromise = ack.promise.then(() => undefined);

    const streamError = this.client.waitForWithCancel(
      (msg) => {
        if (msg.type !== "dictation_stream_error") {
          return null;
        }
        if (msg.payload.dictationId !== dictationId) {
          return null;
        }
        return msg.payload;
      },
      30000,
      { skipQueue: true },
    );
    const errorPromise = streamError.promise.then((payload) => {
      throw new Error(payload.error);
    });

    const cleanupError = new Error("Cancelled dictation start waiter");
    try {
      this.client.sendSessionMessageStrict({ type: "dictation_stream_start", dictationId, format });
      await Promise.race([ackPromise, errorPromise]);
    } finally {
      ack.cancel(cleanupError);
      streamError.cancel(cleanupError);
      void ackPromise.catch(() => undefined);
      void errorPromise.catch(() => undefined);
    }
  }

  sendDictationStreamChunk(dictationId: string, seq: number, audio: string, format: string): void {
    this.client.sendSessionMessageStrict({
      type: "dictation_stream_chunk",
      dictationId,
      seq,
      audio,
      format,
    });
  }

  async finishDictationStream(
    dictationId: string,
    finalSeq: number,
  ): Promise<{ dictationId: string; text: string }> {
    const final = this.client.waitForWithCancel(
      (msg) => {
        if (msg.type !== "dictation_stream_final") {
          return null;
        }
        if (msg.payload.dictationId !== dictationId) {
          return null;
        }
        return msg.payload;
      },
      0,
      { skipQueue: true },
    );

    const streamError = this.client.waitForWithCancel(
      (msg) => {
        if (msg.type !== "dictation_stream_error") {
          return null;
        }
        if (msg.payload.dictationId !== dictationId) {
          return null;
        }
        return msg.payload;
      },
      0,
      { skipQueue: true },
    );

    const finishAccepted = this.client.waitForWithCancel<DictationFinishAcceptedPayload>(
      (msg) => {
        if (msg.type !== "dictation_stream_finish_accepted") {
          return null;
        }
        if (msg.payload.dictationId !== dictationId) {
          return null;
        }
        return msg.payload;
      },
      DEFAULT_DICTATION_FINISH_ACCEPT_TIMEOUT_MS,
      { skipQueue: true },
    );

    const finalPromise = final.promise;
    const errorPromise = streamError.promise.then((payload) => {
      throw new Error(payload.error);
    });
    const finishAcceptedPromise = finishAccepted.promise;

    const finalOutcomePromise = finalPromise.then((payload) => ({
      kind: "final" as const,
      payload,
    }));
    const errorOutcomePromise = errorPromise.then(
      () => ({
        kind: "error" as const,
        error: new Error("Unexpected dictation stream error state"),
      }),
      (error) => ({
        kind: "error" as const,
        error: error instanceof Error ? error : new Error(String(error)),
      }),
    );
    const finishAcceptedOutcomePromise = finishAcceptedPromise.then(
      (payload) => ({ kind: "accepted" as const, payload }),
      (error) => {
        if (isWaiterTimeoutError(error)) {
          return { kind: "accepted_timeout" as const };
        }
        return {
          kind: "accepted_error" as const,
          error: error instanceof Error ? error : new Error(String(error)),
        };
      },
    );

    const waitForFinalResult = async (
      timeoutMs: number,
    ): Promise<{ dictationId: string; text: string }> => {
      if (!Number.isFinite(timeoutMs) || timeoutMs <= 0) {
        const outcome = await Promise.race([finalOutcomePromise, errorOutcomePromise]);
        if (outcome.kind === "error") {
          throw outcome.error;
        }
        return outcome.payload;
      }

      let timeoutHandle: ReturnType<typeof setTimeout> | null = null;
      const timeoutPromise = new Promise<{ kind: "timeout" }>((resolve) => {
        timeoutHandle = setTimeout(() => resolve({ kind: "timeout" }), timeoutMs);
      });

      const outcome = await Promise.race([
        finalOutcomePromise,
        errorOutcomePromise,
        timeoutPromise,
      ]);

      if (timeoutHandle) {
        clearTimeout(timeoutHandle);
      }

      if (outcome.kind === "timeout") {
        throw new Error(`Timeout waiting for dictation finalization (${timeoutMs}ms)`);
      }
      if (outcome.kind === "error") {
        throw outcome.error;
      }
      return outcome.payload;
    };

    const cleanupError = new Error("Cancelled dictation finish waiter");
    try {
      this.client.sendSessionMessageStrict({ type: "dictation_stream_finish", dictationId, finalSeq });
      const firstOutcome = await Promise.race([
        finalOutcomePromise,
        errorOutcomePromise,
        finishAcceptedOutcomePromise,
      ]);

      if (firstOutcome.kind === "final") {
        return firstOutcome.payload;
      }
      if (firstOutcome.kind === "error") {
        throw firstOutcome.error;
      }

      if (firstOutcome.kind === "accepted") {
        return await waitForFinalResult(
          firstOutcome.payload.timeoutMs + DEFAULT_DICTATION_FINISH_TIMEOUT_GRACE_MS,
        );
      }

      return await waitForFinalResult(DEFAULT_DICTATION_FINISH_FALLBACK_TIMEOUT_MS);
    } finally {
      final.cancel(cleanupError);
      streamError.cancel(cleanupError);
      finishAccepted.cancel(cleanupError);
      void finalPromise.catch(() => undefined);
      void errorPromise.catch(() => undefined);
      void finishAcceptedPromise.catch(() => undefined);
    }
  }

  cancelDictationStream(dictationId: string): void {
    this.client.sendSessionMessageStrict({ type: "dictation_stream_cancel", dictationId });
  }

  async abortRequest(): Promise<void> {
    this.client.sendSessionMessage({ type: "abort_request" });
  }

  async audioPlayed(id: string): Promise<void> {
    this.client.sendSessionMessage({ type: "audio_played", id });
  }

  // ============================================================================
  // Terminals
  // ============================================================================

  subscribeTerminals(input: { cwd: string }): void {
    this.terminalDirectorySubscriptions.add(input.cwd);
    if (!this.client.isConnected) {
      return;
    }
    this.client.sendSessionMessage({
      type: "subscribe_terminals_request",
      cwd: input.cwd,
    });
  }

  unsubscribeTerminals(input: { cwd: string }): void {
    this.terminalDirectorySubscriptions.delete(input.cwd);
    if (!this.client.isConnected) {
      return;
    }
    this.client.sendSessionMessage({
      type: "unsubscribe_terminals_request",
      cwd: input.cwd,
    });
  }

  async listTerminals(cwd?: string, requestId?: string): Promise<ListTerminalsPayload> {
    const resolvedRequestId = this.client.createRequestId(requestId);
    const message = SessionInboundMessageSchema.parse({
      type: "list_terminals_request",
      ...(cwd === undefined ? {} : { cwd }),
      requestId: resolvedRequestId,
    });
    return this.client.sendCorrelatedRequest({
      requestId: resolvedRequestId,
      message,
      responseType: "list_terminals_response",
      timeout: 10000,
      options: { skipQueue: true },
    });
  }

  async createTerminal(
    cwd: string,
    name?: string,
    requestId?: string,
    options?: { agentId?: string; command?: string; args?: string[] },
  ): Promise<CreateTerminalPayload> {
    const resolvedRequestId = this.client.createRequestId(requestId);
    const message = SessionInboundMessageSchema.parse({
      type: "create_terminal_request",
      cwd,
      name,
      agentId: options?.agentId,
      command: options?.command,
      args: options?.args,
      requestId: resolvedRequestId,
    });
    return this.client.sendCorrelatedRequest({
      requestId: resolvedRequestId,
      message,
      responseType: "create_terminal_response",
      timeout: 10000,
      options: { skipQueue: true },
    });
  }

  async subscribeTerminal(
    terminalId: string,
    requestId?: string,
  ): Promise<SubscribeTerminalPayload> {
    const resolvedRequestId = this.client.createRequestId(requestId);
    const message = SessionInboundMessageSchema.parse({
      type: "subscribe_terminal_request",
      terminalId,
      requestId: resolvedRequestId,
    });
    const payload = await this.client.sendCorrelatedRequest({
      requestId: resolvedRequestId,
      message,
      responseType: "subscribe_terminal_response",
      timeout: 10000,
      options: { skipQueue: true },
    });
    if (payload.error === null) {
      this.setTerminalSlot(terminalId, payload.slot);
    }
    return payload;
  }

  unsubscribeTerminal(terminalId: string): void {
    this.removeSlot(terminalId);
    this.client.sendSessionMessage({
      type: "unsubscribe_terminal_request",
      terminalId,
    });
  }

  sendTerminalInput(terminalId: string, message: TerminalInput["message"]): void {
    const slot = this.terminalSlots.get(terminalId);
    if (typeof slot === "number") {
      if (message.type === "input") {
        this.client.sendBinaryFrame(
          encodeTerminalStreamFrame({
            opcode: TerminalStreamOpcode.Input,
            slot,
            payload: encodeUtf8String(message.data),
          }),
        );
        return;
      }
      if (message.type === "resize") {
        this.client.sendBinaryFrame(
          encodeTerminalStreamFrame({
            opcode: TerminalStreamOpcode.Resize,
            slot,
            payload: encodeTerminalResizePayload({
              rows: message.rows,
              cols: message.cols,
            }),
          }),
        );
        return;
      }
    }
    this.client.sendSessionMessage({
      type: "terminal_input",
      terminalId,
      message,
    });
  }

  async killTerminal(terminalId: string, requestId?: string): Promise<KillTerminalPayload> {
    const resolvedRequestId = this.client.createRequestId(requestId);
    const message = SessionInboundMessageSchema.parse({
      type: "kill_terminal_request",
      terminalId,
      requestId: resolvedRequestId,
    });
    return this.client.sendCorrelatedRequest({
      requestId: resolvedRequestId,
      message,
      responseType: "kill_terminal_response",
      timeout: 10000,
      options: { skipQueue: true },
    });
  }

  async closeItems(
    input: { agentIds?: string[]; terminalIds?: string[] },
    requestId?: string,
  ): Promise<CloseItemsPayload> {
    const resolvedRequestId = this.client.createRequestId(requestId);
    const message = SessionInboundMessageSchema.parse({
      type: "close_items_request",
      agentIds: input.agentIds ?? [],
      terminalIds: input.terminalIds ?? [],
      requestId: resolvedRequestId,
    });
    return this.client.sendCorrelatedRequest({
      requestId: resolvedRequestId,
      message,
      responseType: "close_items_response",
      timeout: 10000,
      options: { skipQueue: true },
    });
  }

  async captureTerminal(
    terminalId: string,
    options?: { start?: number; end?: number; stripAnsi?: boolean },
    requestId?: string,
  ): Promise<CaptureTerminalPayload> {
    const resolvedRequestId = this.client.createRequestId(requestId);
    const message = SessionInboundMessageSchema.parse({
      type: "capture_terminal_request",
      terminalId,
      ...(options?.start === undefined ? {} : { start: options.start }),
      ...(options?.end === undefined ? {} : { end: options.end }),
      ...(options?.stripAnsi === undefined ? {} : { stripAnsi: options.stripAnsi }),
      requestId: resolvedRequestId,
    });
    return this.client.sendCorrelatedRequest({
      requestId: resolvedRequestId,
      message,
      responseType: "capture_terminal_response",
      timeout: 10000,
      options: { skipQueue: true },
    });
  }

  // ============================================================================
  // Tmux
  // ============================================================================

  async tmuxListAgents(requestId?: string): Promise<TmuxListAgentsPayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId,
      message: {
        type: "tmux/list_agents",
      },
      responseType: "tmux/list_agents/response",
      timeout: 10000,
    });
  }

  async tmuxCapturePane(
    paneId: string,
    startLine?: number,
    lastContentHash?: string,
    cols?: number,
    requestId?: string,
  ): Promise<TmuxCapturePanePayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId,
      message: {
        type: "tmux/capture_pane",
        paneId,
        ...(startLine === undefined ? {} : { startLine }),
        ...(lastContentHash === undefined ? {} : { lastContentHash }),
        ...(cols === undefined ? {} : { cols }),
      },
      responseType: "tmux/capture_pane/response",
      timeout: 10000,
    });
  }

  async tmuxSendKeys(paneId: string, keys: string, sendEnter?: boolean, requestId?: string): Promise<TmuxSendKeysPayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId,
      message: {
        type: "tmux/send_keys",
        paneId,
        keys,
        ...(sendEnter === undefined ? {} : { sendEnter }),
      },
      responseType: "tmux/send_keys/response",
      timeout: 10000,
    });
  }

  async tmuxStatusLine(sessionId: string, requestId?: string): Promise<TmuxStatusLinePayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId,
      message: {
        type: "tmux/status_line",
        sessionId,
      },
      responseType: "tmux/status_line/response",
      timeout: 10000,
    });
  }

  async tmuxNewSession(name: string, options?: { workingDir?: string; command?: string }, requestId?: string): Promise<TmuxNewSessionPayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId,
      message: {
        type: "tmux/new_session",
        name,
        ...(options?.workingDir ? { workingDir: options.workingDir } : {}),
        ...(options?.command ? { command: options.command } : {}),
      },
      responseType: "tmux/new_session/response",
      timeout: 10000,
    });
  }

  async tmuxKillSession(sessionName: string, requestId?: string): Promise<TmuxKillSessionPayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId,
      message: {
        type: "tmux/kill_session",
        sessionName,
      },
      responseType: "tmux/kill_session/response",
      timeout: 10000,
    });
  }

  async tmuxDeleteCommandHistory(launchCmd: string, requestId?: string): Promise<TmuxDeleteCommandHistoryPayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId,
      message: {
        type: "tmux/delete_command_history",
        launchCmd,
      },
      responseType: "tmux/delete_command_history/response",
      timeout: 10000,
    });
  }

  // ============================================================================
  // Terminal Stream Events
  // ============================================================================

  onTerminalStreamEvent(handler: (event: TerminalStreamEvent) => void): () => void {
    this.terminalStreamListeners.add(handler);
    return () => {
      this.terminalStreamListeners.delete(handler);
    };
  }

  async waitForTerminalStreamEvent(
    predicate: (event: TerminalStreamEvent) => boolean,
    timeout = 5000,
  ): Promise<TerminalStreamEvent> {
    return new Promise<TerminalStreamEvent>((resolve, reject) => {
      const timeoutHandle = setTimeout(() => {
        unsubscribe();
        reject(new Error(`Timeout waiting for terminal stream event (${timeout}ms)`));
      }, timeout);

      const unsubscribe = this.onTerminalStreamEvent((event) => {
        if (!predicate(event)) {
          return;
        }
        clearTimeout(timeoutHandle);
        unsubscribe();
        resolve(event);
      });
    });
  }

  // ============================================================================
  // Slot Management (internal helpers exposed as needed)
  // ============================================================================

  private setTerminalSlot(terminalId: string, slot: number): void {
    const existingTerminalId = this.slotTerminals.get(slot);
    if (existingTerminalId && existingTerminalId !== terminalId) {
      this.terminalSlots.delete(existingTerminalId);
    }

    const existingSlot = this.terminalSlots.get(terminalId);
    if (typeof existingSlot === "number" && existingSlot !== slot) {
      this.slotTerminals.delete(existingSlot);
    }

    this.terminalSlots.set(terminalId, slot);
    this.slotTerminals.set(slot, terminalId);
  }

  removeSlot(terminalId: string): void {
    const slot = this.terminalSlots.get(terminalId);
    if (typeof slot !== "number") {
      return;
    }
    this.terminalSlots.delete(terminalId);
    if (this.slotTerminals.get(slot) === terminalId) {
      this.slotTerminals.delete(slot);
    }
  }

  clearSlots(): void {
    this.terminalSlots.clear();
    this.slotTerminals.clear();
  }

  // ============================================================================
  // Binary Frame Handling
  // ============================================================================

  tryHandleBinaryFrame(rawBytes: Uint8Array): boolean {
    const frame = decodeTerminalStreamFrame(rawBytes);
    if (!frame) {
      return false;
    }
    const binaryStartMs = perfNow();
    this.handleBinaryFrame(frame);
    let frameKind: "output" | "snapshot" | "other" = "other";
    if (frame.opcode === TerminalStreamOpcode.Output) {
      frameKind = "output";
    } else if (frame.opcode === TerminalStreamOpcode.Snapshot) {
      frameKind = "snapshot";
    }
    this.client.recordBinaryFrame(
      frameKind,
      rawBytes.byteLength,
      perfNow() - binaryStartMs,
    );
    return true;
  }

  private handleBinaryFrame(frame: TerminalStreamFrame): void {
    const terminalId = this.slotTerminals.get(frame.slot);
    if (!terminalId) {
      return;
    }

    if (frame.opcode === TerminalStreamOpcode.Output) {
      this.emitTerminalStreamEvent({
        terminalId,
        type: "output",
        data: frame.payload,
      });
      return;
    }

    if (frame.opcode === TerminalStreamOpcode.Snapshot) {
      const state = decodeTerminalSnapshotPayload(frame.payload);
      if (!state) {
        return;
      }
      this.emitTerminalStreamEvent({
        terminalId,
        type: "snapshot",
        state,
      });
    }
  }

  private emitTerminalStreamEvent(event: TerminalStreamEvent): void {
    for (const listener of this.terminalStreamListeners) {
      try {
        listener(event);
      } catch (error) {
        getDefaultLogger().warn(
          { err: toErrorInfo(error) },
          "terminal_stream_listener_failed",
        );
      }
    }
  }

  // ============================================================================
  // Resubscribe (called on reconnect)
  // ============================================================================

  resubscribe(): void {
    if (this.terminalDirectorySubscriptions.size === 0) {
      return;
    }
    for (const cwd of this.terminalDirectorySubscriptions) {
      this.client.sendSessionMessage({
        type: "subscribe_terminals_request",
        cwd,
      });
    }
  }
}
