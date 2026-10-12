/**
 * One-time pairing channel used by Web QR login.
 *
 * The web client opens a relay session as `role=server` under a random
 * one-time pairingId; the approving app connects as `role=client` under the
 * same pairingId and pushes a single PairWebOfferEnvelope. The envelope
 * ciphertext is sealed to the web ephemeral public key, so the relay only
 * ever routes opaque frames — it never sees the ConnectionOffer plaintext.
 *
 * The channel delivers at most one valid offer: the web side closes it after
 * the first successful decrypt, and both the pairingId and the keypairs are
 * discarded.
 */

import { ConnectionOfferSchema, type ConnectionOffer } from "../shared/connection-offer.js";
import { buildRelayWebSocketUrl, normalizeHostPort } from "../shared/daemon-endpoints.js";
import {
  PAIR_WEB_RELAY_CONNECTION_ID,
  PAIR_WEB_REQUEST_TTL_SECONDS,
  PairWebOfferEnvelopeSchema,
  base64UrlEncode,
  buildPairWebQrContent,
  isPairWebRequestExpired,
  publicKeyFingerprint,
  type PairWebOfferEnvelope,
  type PairWebRequest,
} from "../shared/pair-web-offer.js";
import { arrayBufferToBase64, base64ToArrayBuffer } from "./base64.js";
import {
  deriveSharedKey,
  encrypt,
  decrypt,
  exportPublicKey,
  generateKeyPair,
  importPublicKey,
  randomBytes,
  type KeyPair,
} from "./crypto.js";

export class PairWebRequestExpiredError extends Error {
  constructor() {
    super("Pair-web request expired");
    this.name = "PairWebRequestExpiredError";
  }
}

export class PairWebChannelClosedError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "PairWebChannelClosedError";
  }
}

/** 128-bit one-time relay session identifier. */
export function generatePairingId(): string {
  return base64UrlEncode(randomBytes(16));
}

/**
 * Seals a ConnectionOffer to the web public key from the pairing request,
 * using a fresh ephemeral app keypair (injectable for tests).
 */
export function sealPairWebOffer(input: {
  request: PairWebRequest;
  offer: ConnectionOffer;
  appKeyPair?: KeyPair;
}): PairWebOfferEnvelope {
  const keyPair = input.appKeyPair ?? generateKeyPair();
  const webPublicKey = importPublicKey(input.request.webPublicKeyB64);
  const sharedKey = deriveSharedKey(keyPair.secretKey, webPublicKey);
  const ciphertext = encrypt(sharedKey, JSON.stringify(input.offer));
  return {
    v: 1,
    kind: "pair-web-offer",
    appPublicKeyB64: exportPublicKey(keyPair.publicKey),
    ciphertextB64: arrayBufferToBase64(ciphertext),
  };
}

/**
 * Opens a sealed offer envelope. Throws on any malformed envelope, decryption
 * failure, or an offer that fails ConnectionOfferSchema validation.
 */
export function openPairWebOffer(input: {
  webSecretKey: Uint8Array;
  envelope: unknown;
}): ConnectionOffer {
  const envelope = PairWebOfferEnvelopeSchema.parse(input.envelope);
  const appPublicKey = importPublicKey(envelope.appPublicKeyB64);
  const sharedKey = deriveSharedKey(input.webSecretKey, appPublicKey);
  const plaintext = decrypt(sharedKey, base64ToArrayBuffer(envelope.ciphertextB64));
  if (typeof plaintext !== "string") {
    throw new Error("Pair-web offer payload is not text");
  }
  return ConnectionOfferSchema.parse(JSON.parse(plaintext));
}

// ---------------------------------------------------------------------------
// Relay channel transport
// ---------------------------------------------------------------------------

export interface PairWebSocket {
  send(data: string): void;
  close(code?: number, reason?: string): void;
}

export interface PairWebSocketHandlers {
  onOpen: () => void;
  onMessage: (data: string) => void;
  onClose: (code: number, reason: string) => void;
  onError: (message: string) => void;
}

export type PairWebSocketFactory = (url: string, handlers: PairWebSocketHandlers) => PairWebSocket;

interface WsEventLike {
  data?: unknown;
  code?: number;
  reason?: string;
}

interface WsCandidate {
  send: (data: string) => void;
  close: (code?: number, reason?: string) => void;
  addEventListener?: (event: string, handler: (...args: unknown[]) => void) => void;
  on?: (event: string, handler: (...args: unknown[]) => void) => void;
}

function socketDataToText(data: unknown): string | null {
  if (typeof data === "string") {
    return data;
  }
  if (data instanceof ArrayBuffer) {
    return new TextDecoder().decode(data);
  }
  // Covers Node Buffer and other Uint8Array views.
  if (ArrayBuffer.isView(data)) {
    return new TextDecoder().decode(data as Uint8Array);
  }
  return null;
}

function createDefaultPairWebSocket(url: string, handlers: PairWebSocketHandlers): PairWebSocket {
  const WsImpl = (globalThis as { WebSocket?: new (url: string) => WsCandidate }).WebSocket;
  if (!WsImpl) {
    throw new Error("WebSocket is not available in this runtime");
  }
  const ws = new WsImpl(url);

  const handleMessageArg = (arg: unknown) => {
    // Browser-style runtimes pass a MessageEvent; ws-style pass the data.
    const data =
      arg !== null && typeof arg === "object" && "data" in (arg as WsEventLike)
        ? (arg as WsEventLike).data
        : arg;
    const text = socketDataToText(data);
    if (text !== null) {
      handlers.onMessage(text);
    }
  };

  if (typeof ws.addEventListener === "function") {
    ws.addEventListener("open", () => handlers.onOpen());
    ws.addEventListener("message", (event: unknown) => handleMessageArg(event));
    ws.addEventListener("close", (event: unknown) => {
      const closeEvent = (event ?? {}) as WsEventLike;
      handlers.onClose(closeEvent.code ?? 1000, closeEvent.reason ?? "");
    });
    ws.addEventListener("error", () => handlers.onError("WebSocket error"));
  } else if (typeof ws.on === "function") {
    ws.on("open", () => handlers.onOpen());
    ws.on("message", (data: unknown) => handleMessageArg(data));
    ws.on("close", (code: unknown, reason: unknown) =>
      handlers.onClose(typeof code === "number" ? code : 1000, String(reason ?? "")),
    );
    ws.on("error", (error: unknown) =>
      handlers.onError(error instanceof Error ? error.message : String(error)),
    );
  } else {
    throw new Error("WebSocket implementation exposes neither addEventListener nor on");
  }

  return {
    send: (data) => ws.send(data),
    close: (code, reason) => ws.close(code, reason),
  };
}

export interface PairWebChannel {
  sendJson(payload: unknown): void;
  close(code?: number, reason?: string): void;
  /** Subscribes to JSON-decoded inbound frames; non-JSON frames are dropped. */
  onJsonMessage(callback: (payload: unknown) => void): () => void;
  onClose(callback: (info: { code: number; reason: string }) => void): () => void;
}

const DEFAULT_CONNECT_TIMEOUT_MS = 10_000;

export function connectPairWebChannel(input: {
  relayEndpoint: string;
  pairingId: string;
  role: "server" | "client";
  socketFactory?: PairWebSocketFactory;
  timeoutMs?: number;
}): Promise<PairWebChannel> {
  const endpoint = normalizeHostPort(input.relayEndpoint);
  const url = buildRelayWebSocketUrl({
    endpoint,
    serverId: input.pairingId,
    role: input.role,
    connectionId: PAIR_WEB_RELAY_CONNECTION_ID,
  });
  const factory = input.socketFactory ?? createDefaultPairWebSocket;
  const timeoutMs = input.timeoutMs ?? DEFAULT_CONNECT_TIMEOUT_MS;

  return new Promise<PairWebChannel>((resolve, reject) => {
    const messageCallbacks = new Set<(payload: unknown) => void>();
    const closeCallbacks = new Set<(info: { code: number; reason: string }) => void>();
    let open = false;
    let settled = false;
    let socket: PairWebSocket | null = null;

    const channel: PairWebChannel = {
      sendJson: (payload) => {
        if (!open || !socket) {
          throw new Error("Pair-web channel is not open");
        }
        socket.send(JSON.stringify(payload));
      },
      close: (code, reason) => {
        socket?.close(code, reason);
      },
      onJsonMessage: (callback) => {
        messageCallbacks.add(callback);
        return () => messageCallbacks.delete(callback);
      },
      onClose: (callback) => {
        closeCallbacks.add(callback);
        return () => closeCallbacks.delete(callback);
      },
    };

    const timer = setTimeout(() => {
      if (settled) return;
      settled = true;
      try {
        socket?.close();
      } catch {
        // best effort
      }
      reject(new Error("Pair-web channel connect timed out"));
    }, timeoutMs);
    (timer as unknown as { unref?: () => void }).unref?.();

    const failBeforeOpen = (error: Error) => {
      if (settled) return;
      settled = true;
      clearTimeout(timer);
      reject(error);
    };

    socket = factory(url, {
      onOpen: () => {
        if (settled) return;
        settled = true;
        open = true;
        clearTimeout(timer);
        resolve(channel);
      },
      onMessage: (data) => {
        let parsed: unknown;
        try {
          parsed = JSON.parse(data);
        } catch {
          return;
        }
        for (const callback of messageCallbacks) {
          callback(parsed);
        }
      },
      onClose: (code, reason) => {
        if (!settled) {
          failBeforeOpen(
            new PairWebChannelClosedError(
              `Pair-web channel closed before it opened (code=${code} ${reason})`,
            ),
          );
          return;
        }
        for (const callback of closeCallbacks) {
          callback({ code, reason });
        }
      },
      onError: (message) => {
        failBeforeOpen(new Error(message));
      },
    });
  });
}

// ---------------------------------------------------------------------------
// High-level flows
// ---------------------------------------------------------------------------

export interface PairWebHostSession {
  request: PairWebRequest;
  qrContent: string;
  fingerprint: string;
  expiresAtMs: number;
  /**
   * Resolves with the first valid ConnectionOffer and closes the channel.
   * Rejects with PairWebRequestExpiredError on TTL expiry, or
   * PairWebChannelClosedError when the channel dies first.
   */
  waitForOffer(): Promise<ConnectionOffer>;
  close(): void;
}

/**
 * Web side: generates the one-time keypair + pairingId, connects to the relay
 * as role=server, and returns the QR content to render.
 */
export async function createPairWebHostSession(input: {
  relayEndpoint: string;
  socketFactory?: PairWebSocketFactory;
  connectTimeoutMs?: number;
  nowMs?: number;
}): Promise<PairWebHostSession> {
  const nowMs = input.nowMs ?? Date.now();
  const keyPair = generateKeyPair();
  const pairingId = generatePairingId();
  const request: PairWebRequest = {
    v: 1,
    kind: "pair-web",
    pairingId,
    webPublicKeyB64: exportPublicKey(keyPair.publicKey),
    relay: { endpoint: normalizeHostPort(input.relayEndpoint) },
    exp: Math.floor(nowMs / 1000) + PAIR_WEB_REQUEST_TTL_SECONDS,
  };
  const channel = await connectPairWebChannel({
    relayEndpoint: request.relay.endpoint,
    pairingId,
    role: "server",
    socketFactory: input.socketFactory,
    timeoutMs: input.connectTimeoutMs,
  });

  const session: PairWebHostSession = {
    request,
    qrContent: buildPairWebQrContent(request),
    fingerprint: publicKeyFingerprint(request.webPublicKeyB64),
    expiresAtMs: request.exp * 1000,
    waitForOffer: () =>
      new Promise<ConnectionOffer>((resolve, reject) => {
        const cleanup = () => {
          clearTimeout(expiryTimer);
          unsubscribeMessage();
          unsubscribeClose();
        };
        const settle = (offer?: ConnectionOffer, error?: Error) => {
          cleanup();
          channel.close(1000, offer ? "offer received" : "pair-web wait aborted");
          if (offer) {
            resolve(offer);
          } else {
            reject(error ?? new PairWebChannelClosedError("Pair-web wait aborted"));
          }
        };
        const expiryTimer = setTimeout(
          () => settle(undefined, new PairWebRequestExpiredError()),
          Math.max(0, session.expiresAtMs - Date.now()),
        );
        (expiryTimer as unknown as { unref?: () => void }).unref?.();
        const unsubscribeMessage = channel.onJsonMessage((payload) => {
          let offer: ConnectionOffer;
          try {
            offer = openPairWebOffer({ webSecretKey: keyPair.secretKey, envelope: payload });
          } catch {
            // Not a valid offer for this channel — ignore and keep waiting.
            return;
          }
          settle(offer);
        });
        const unsubscribeClose = channel.onClose(() =>
          settle(
            undefined,
            new PairWebChannelClosedError("Pair-web channel closed before an offer arrived"),
          ),
        );
      }),
    close: () => channel.close(),
  };
  return session;
}

/**
 * App side: seals the offer to the web public key and pushes it through the
 * pairing channel. Resolves once the frame is sent and the peer has had a
 * chance to confirm (the web side closes the channel on receipt); absence of
 * confirmation within `confirmTimeoutMs` is not treated as fatal.
 */
export async function sendPairWebOffer(input: {
  request: PairWebRequest;
  offer: ConnectionOffer;
  socketFactory?: PairWebSocketFactory;
  connectTimeoutMs?: number;
  confirmTimeoutMs?: number;
}): Promise<void> {
  if (isPairWebRequestExpired(input.request)) {
    throw new PairWebRequestExpiredError();
  }
  const offer = ConnectionOfferSchema.parse(input.offer);
  const channel = await connectPairWebChannel({
    relayEndpoint: input.request.relay.endpoint,
    pairingId: input.request.pairingId,
    role: "client",
    socketFactory: input.socketFactory,
    timeoutMs: input.connectTimeoutMs,
  });
  try {
    channel.sendJson(sealPairWebOffer({ request: input.request, offer }));
    await new Promise<void>((resolve) => {
      const timer = setTimeout(() => {
        unsubscribeClose();
        resolve();
      }, input.confirmTimeoutMs ?? 3_000);
      (timer as unknown as { unref?: () => void }).unref?.();
      const unsubscribeClose = channel.onClose(() => {
        clearTimeout(timer);
        resolve();
      });
    });
  } finally {
    channel.close();
  }
}
