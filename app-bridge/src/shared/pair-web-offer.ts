import { z } from "zod";
import { fromByteArray, toByteArray } from "base64-js";

/**
 * Web QR login ("pair-web") protocol payloads.
 *
 * Flow: the web client renders a QR code containing a PairWebRequest
 * (`solo://pair-web#<base64url(JSON)>`), the mobile app scans it, and returns
 * a ConnectionOffer over a one-time relay channel, encrypted to the web
 * client's ephemeral public key (PairWebOfferEnvelope).
 *
 * Everything here is one-time: a fresh pairingId and X25519 keypair per QR,
 * and a short TTL so a leaked QR code expires quickly.
 */

/** QR content prefix scanned by the app: `solo://pair-web#<base64url(JSON)>`. */
export const PAIR_WEB_QR_PREFIX = "solo://pair-web#";

/** Lifetime of a pairing request, baked into `exp` at creation time. */
export const PAIR_WEB_REQUEST_TTL_SECONDS = 120;

/**
 * Fixed relay routing identifier for the pairing channel. The pairingId
 * already scopes the relay session, so a constant connectionId keeps the
 * wire format trivial without weakening isolation.
 */
export const PAIR_WEB_RELAY_CONNECTION_ID = "pair";

export const PairWebRequestSchema = z.object({
  v: z.literal(1),
  kind: z.literal("pair-web"),
  pairingId: z.string().min(1),
  webPublicKeyB64: z.string().min(1),
  relay: z.object({
    endpoint: z.string().min(1),
  }),
  /** Unix seconds; the request is dead once `now >= exp`. */
  exp: z.number().int().positive(),
});

export type PairWebRequest = z.infer<typeof PairWebRequestSchema>;

export const PairWebOfferEnvelopeSchema = z.object({
  v: z.literal(1),
  kind: z.literal("pair-web-offer"),
  /** Ephemeral X25519 public key of the approving app. */
  appPublicKeyB64: z.string().min(1),
  /**
   * base64 of the standard crypto bundle ([nonce(24)][ciphertext]) holding the
   * JSON-encoded ConnectionOffer, sealed to the web public key.
   */
  ciphertextB64: z.string().min(1),
});

export type PairWebOfferEnvelope = z.infer<typeof PairWebOfferEnvelopeSchema>;

export function base64UrlEncode(bytes: Uint8Array): string {
  return fromByteArray(bytes).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/g, "");
}

export function base64UrlDecodeToBytes(value: string): Uint8Array {
  const normalized = value.trim().replace(/-/g, "+").replace(/_/g, "/");
  const padded = normalized.padEnd(normalized.length + ((4 - (normalized.length % 4)) % 4), "=");
  return toByteArray(padded);
}

export function encodePairWebRequest(request: PairWebRequest): string {
  return base64UrlEncode(new TextEncoder().encode(JSON.stringify(request)));
}

export function decodePairWebRequestPayload(encoded: string): unknown {
  return JSON.parse(new TextDecoder().decode(base64UrlDecodeToBytes(encoded))) as unknown;
}

export function buildPairWebQrContent(request: PairWebRequest): string {
  return `${PAIR_WEB_QR_PREFIX}${encodePairWebRequest(request)}`;
}

/**
 * Returns the validated request when `raw` is a pair-web QR, `null` when the
 * content belongs to a different flow (e.g. a daemon `#offer=` link), and
 * throws when it is a pair-web code but malformed.
 */
export function extractPairWebRequestFromQrContent(raw: string): PairWebRequest | null {
  const trimmed = raw.trim();
  if (!trimmed.startsWith(PAIR_WEB_QR_PREFIX)) {
    return null;
  }
  const encoded = trimmed.slice(PAIR_WEB_QR_PREFIX.length).trim();
  if (!encoded) {
    throw new Error("Pair-web QR payload is empty");
  }
  return PairWebRequestSchema.parse(decodePairWebRequestPayload(encoded));
}

export function isPairWebRequestExpired(request: PairWebRequest, nowMs: number = Date.now()): boolean {
  return nowMs >= request.exp * 1000;
}

/**
 * Display fingerprint of a base64 public key: first bytes as grouped hex.
 * Both sides render it so the user can eyeball that the phone and the
 * browser are talking about the same ephemeral key.
 */
export function publicKeyFingerprint(publicKeyB64: string, byteCount = 8): string {
  const bytes = toByteArray(publicKeyB64);
  const groups: string[] = [];
  for (let i = 0; i < Math.min(byteCount, bytes.byteLength); i += 2) {
    const hi = bytes[i]!.toString(16).padStart(2, "0");
    const lo = i + 1 < bytes.byteLength ? bytes[i + 1]!.toString(16).padStart(2, "0") : "";
    groups.push(`${hi}${lo}`);
  }
  return groups.join(" ");
}
