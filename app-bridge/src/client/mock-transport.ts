import type { DaemonTransport } from "./daemon-client-transport-types.js";

type Handler = () => void;
type MessageHandler = (data: unknown) => void;
type CloseHandler = (event?: unknown) => void;
type ErrorHandler = (event?: unknown) => void;

export interface CapturedMessage {
  raw: string;
  parsed: { type: string; message?: unknown } & Record<string, unknown>;
}

export class MockTransport implements DaemonTransport {
  private openHandlers: Set<Handler> = new Set();
  private closeHandlers: Set<CloseHandler> = new Set();
  private errorHandlers: Set<ErrorHandler> = new Set();
  private messageHandlers: Set<MessageHandler> = new Set();

  readonly sentMessages: CapturedMessage[] = [];
  closed = false;
  closeCode?: number;
  closeReason?: string;

  send(data: string | Uint8Array | ArrayBuffer): void {
    if (typeof data === "string") {
      try {
        this.sentMessages.push({ raw: data, parsed: JSON.parse(data) });
      } catch {
        this.sentMessages.push({ raw: data, parsed: { type: "raw", raw: data } });
      }
    } else {
      this.sentMessages.push({
        raw: "[binary]",
        parsed: { type: "binary", byteLength: data.byteLength },
      });
    }
  }

  close(code?: number, reason?: string): void {
    this.closed = true;
    this.closeCode = code;
    this.closeReason = reason;
  }

  onOpen(handler: Handler): () => void {
    this.openHandlers.add(handler);
    return () => this.openHandlers.delete(handler);
  }

  onClose(handler: CloseHandler): () => void {
    this.closeHandlers.add(handler);
    return () => this.closeHandlers.delete(handler);
  }

  onError(handler: ErrorHandler): () => void {
    this.errorHandlers.add(handler);
    return () => this.errorHandlers.delete(handler);
  }

  onMessage(handler: MessageHandler): () => void {
    this.messageHandlers.add(handler);
    return () => this.messageHandlers.delete(handler);
  }

  simulateOpen(): void {
    for (const handler of this.openHandlers) handler();
  }

  simulateClose(event?: unknown): void {
    for (const handler of this.closeHandlers) handler(event);
  }

  simulateError(event?: unknown): void {
    for (const handler of this.errorHandlers) handler(event);
  }

  simulateMessage(data: unknown): void {
    for (const handler of this.messageHandlers) handler(data);
  }

  get lastSentMessage(): CapturedMessage | undefined {
    return this.sentMessages[this.sentMessages.length - 1];
  }

  findSentMessage(type: string): CapturedMessage | undefined {
    return this.sentMessages.find((m) => m.parsed.type === type);
  }

  reset(): void {
    this.sentMessages.length = 0;
    this.closed = false;
    this.closeCode = undefined;
    this.closeReason = undefined;
  }
}

export function createMockTransportFactory(
  transport: MockTransport,
): (options: { url: string; headers?: Record<string, string> }) => DaemonTransport {
  return () => transport;
}
