import { describe, expect, it } from "vitest";
import { fromByteArray } from "base64-js";

import {
  PAIR_WEB_QR_PREFIX,
  PAIR_WEB_REQUEST_TTL_SECONDS,
  PAIR_WEB_RELAY_CONNECTION_ID,
  PairWebOfferEnvelopeSchema,
  PairWebRequestSchema,
  base64UrlEncode,
  buildPairWebQrContent,
  decodePairWebRequestPayload,
  encodePairWebRequest,
  extractPairWebRequestFromQrContent,
  isPairWebRequestExpired,
  publicKeyFingerprint,
  type PairWebRequest,
} from "./pair-web-offer.js";

function sampleRequest(overrides?: Partial<PairWebRequest>): PairWebRequest {
  return {
    v: 1,
    kind: "pair-web",
    pairingId: "dGVzdC1wYWlyaW5nLWlkLTAw",
    webPublicKeyB64: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
    relay: { endpoint: "solo.up2ai.top:443" },
    exp: 1_800_000_000,
    ...overrides,
  };
}

describe("PairWebRequestSchema", () => {
  it("accepts a well-formed request", () => {
    expect(PairWebRequestSchema.parse(sampleRequest())).toEqual(sampleRequest());
  });

  it("rejects wrong version/kind and missing fields", () => {
    expect(() => PairWebRequestSchema.parse({ ...sampleRequest(), v: 2 })).toThrow();
    expect(() => PairWebRequestSchema.parse({ ...sampleRequest(), kind: "offer" })).toThrow();
    expect(() => PairWebRequestSchema.parse({ ...sampleRequest(), pairingId: "" })).toThrow();
    expect(() => PairWebRequestSchema.parse({ ...sampleRequest(), exp: "soon" })).toThrow();
    expect(() =>
      PairWebRequestSchema.parse({ ...sampleRequest(), relay: { endpoint: "" } }),
    ).toThrow();
  });
});

describe("pair-web request QR encoding", () => {
  it("roundtrips a request through base64url", () => {
    const request = sampleRequest();
    const encoded = encodePairWebRequest(request);
    expect(encoded).toMatch(/^[A-Za-z0-9_-]+$/);
    expect(PairWebRequestSchema.parse(decodePairWebRequestPayload(encoded))).toEqual(request);
  });

  it("builds QR content with the pair-web prefix and extracts it back", () => {
    const request = sampleRequest();
    const qr = buildPairWebQrContent(request);
    expect(qr.startsWith(PAIR_WEB_QR_PREFIX)).toBe(true);
    expect(extractPairWebRequestFromQrContent(qr)).toEqual(request);
  });

  it("returns null for non pair-web QR content", () => {
    expect(extractPairWebRequestFromQrContent("https://solo.up2ai.top/#offer=abc")).toBeNull();
    expect(extractPairWebRequestFromQrContent("hello world")).toBeNull();
  });

  it("throws for pair-web content with a malformed payload", () => {
    expect(() => extractPairWebRequestFromQrContent(PAIR_WEB_QR_PREFIX)).toThrow();
    expect(() => extractPairWebRequestFromQrContent(`${PAIR_WEB_QR_PREFIX}!!!`)).toThrow();
    const notARequest = base64UrlEncode(
      new TextEncoder().encode(JSON.stringify({ v: 1, kind: "pair-web" })),
    );
    expect(() => extractPairWebRequestFromQrContent(`${PAIR_WEB_QR_PREFIX}${notARequest}`)).toThrow();
  });
});

describe("isPairWebRequestExpired", () => {
  it("is expired once now reaches exp", () => {
    const request = sampleRequest({ exp: 1_000 });
    expect(isPairWebRequestExpired(request, 999_999)).toBe(false);
    expect(isPairWebRequestExpired(request, 1_000_000)).toBe(true);
  });
});

describe("publicKeyFingerprint", () => {
  it("is deterministic and grouped", () => {
    const fingerprint = publicKeyFingerprint("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=");
    expect(fingerprint).toBe("0000 0000 0000 0000");
    expect(publicKeyFingerprint("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")).toBe(fingerprint);
  });

  it("reflects the leading key bytes", () => {
    const keyB64 = fromByteArray(new Uint8Array([0x3f, 0x9a, 0x82, 0xc1, ...new Array(28).fill(0)]));
    expect(publicKeyFingerprint(keyB64, 4)).toBe("3f9a 82c1");
  });
});

describe("PairWebOfferEnvelopeSchema", () => {
  it("validates envelope shape", () => {
    const envelope = {
      v: 1,
      kind: "pair-web-offer",
      appPublicKeyB64: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
      ciphertextB64: "Zm9v",
    };
    expect(PairWebOfferEnvelopeSchema.parse(envelope)).toEqual(envelope);
    expect(() => PairWebOfferEnvelopeSchema.parse({ ...envelope, kind: "offer" })).toThrow();
    expect(() => PairWebOfferEnvelopeSchema.parse({ ...envelope, ciphertextB64: "" })).toThrow();
  });
});

describe("protocol constants", () => {
  it("keeps the documented TTL and relay routing id", () => {
    expect(PAIR_WEB_REQUEST_TTL_SECONDS).toBe(120);
    expect(PAIR_WEB_RELAY_CONNECTION_ID).toBe("pair");
  });
});
