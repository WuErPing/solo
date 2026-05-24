import { describe, it, expect } from "vitest";
import { arrayBufferToBase64, base64ToArrayBuffer } from "./base64.js";

function strToBuffer(str: string): ArrayBuffer {
  return new TextEncoder().encode(str).buffer;
}

function bufferToStr(buffer: ArrayBuffer): string {
  return new TextDecoder().decode(buffer);
}

describe("arrayBufferToBase64", () => {
  it("encodes empty buffer", () => {
    const buffer = new ArrayBuffer(0);
    expect(arrayBufferToBase64(buffer)).toBe("");
  });

  it("encodes simple ASCII", () => {
    const buffer = strToBuffer("hello");
    expect(arrayBufferToBase64(buffer)).toBe("aGVsbG8=");
  });

  it("encodes binary data", () => {
    const bytes = new Uint8Array([0, 1, 255, 254]);
    expect(arrayBufferToBase64(bytes.buffer)).toBe("AAH//g==");
  });
});

describe("base64ToArrayBuffer", () => {
  it("decodes empty string", () => {
    const result = base64ToArrayBuffer("");
    expect(result.byteLength).toBe(0);
  });

  it("decodes simple ASCII", () => {
    const result = base64ToArrayBuffer("aGVsbG8=");
    expect(bufferToStr(result)).toBe("hello");
  });

  it("decodes binary data", () => {
    const result = base64ToArrayBuffer("AAH//g==");
    expect(new Uint8Array(result)).toEqual(new Uint8Array([0, 1, 255, 254]));
  });

  it("handles base64url encoding", () => {
    const result = base64ToArrayBuffer("aGVsbG8");
    expect(bufferToStr(result)).toBe("hello");
  });

  it("handles whitespace padding", () => {
    const result = base64ToArrayBuffer("  aGVsbG8=  ");
    expect(bufferToStr(result)).toBe("hello");
  });
});

describe("roundtrip", () => {
  it("preserves random bytes", () => {
    const data = crypto.getRandomValues(new Uint8Array(128));
    const encoded = arrayBufferToBase64(data.buffer);
    const decoded = new Uint8Array(base64ToArrayBuffer(encoded));
    expect(decoded).toEqual(data);
  });
});
