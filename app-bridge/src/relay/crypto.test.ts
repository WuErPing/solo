import { describe, it, expect } from "vitest";
import {
  generateKeyPair,
  exportPublicKey,
  importPublicKey,
  exportSecretKey,
  importSecretKey,
  deriveSharedKey,
  encrypt,
  decrypt,
} from "./crypto.js";

describe("generateKeyPair", () => {
  it("returns distinct keys", () => {
    const kp = generateKeyPair();
    expect(kp.publicKey).toBeInstanceOf(Uint8Array);
    expect(kp.secretKey).toBeInstanceOf(Uint8Array);
    expect(kp.publicKey.byteLength).toBe(32);
    expect(kp.secretKey.byteLength).toBe(32);
    expect(kp.publicKey).not.toEqual(kp.secretKey);
  });

  it("produces different keys on repeated calls", () => {
    const a = generateKeyPair();
    const b = generateKeyPair();
    expect(a.publicKey).not.toEqual(b.publicKey);
    expect(a.secretKey).not.toEqual(b.secretKey);
  });
});

describe("exportPublicKey / importPublicKey", () => {
  it("roundtrips a public key", () => {
    const kp = generateKeyPair();
    const exported = exportPublicKey(kp.publicKey);
    expect(typeof exported).toBe("string");
    const imported = importPublicKey(exported);
    expect(imported).toEqual(kp.publicKey);
  });

  it("throws on wrong length", () => {
    expect(() => exportPublicKey(new Uint8Array(31))).toThrow("Invalid public key length");
    expect(() => importPublicKey("aGVsbG8=")).toThrow("Invalid public key length");
  });
});

describe("exportSecretKey / importSecretKey", () => {
  it("roundtrips a secret key", () => {
    const kp = generateKeyPair();
    const exported = exportSecretKey(kp.secretKey);
    expect(typeof exported).toBe("string");
    const imported = importSecretKey(exported);
    expect(imported).toEqual(kp.secretKey);
  });

  it("throws on wrong length", () => {
    expect(() => exportSecretKey(new Uint8Array(31))).toThrow("Invalid secret key length");
    expect(() => importSecretKey("aGVsbG8=")).toThrow("Invalid secret key length");
  });
});

describe("deriveSharedKey", () => {
  it("derives identical shared keys for two parties", () => {
    const alice = generateKeyPair();
    const bob = generateKeyPair();
    const aliceShared = deriveSharedKey(alice.secretKey, bob.publicKey);
    const bobShared = deriveSharedKey(bob.secretKey, alice.publicKey);
    expect(aliceShared).toEqual(bobShared);
    expect(aliceShared.byteLength).toBe(32);
  });

  it("throws on invalid secret key length", () => {
    const kp = generateKeyPair();
    expect(() => deriveSharedKey(new Uint8Array(31), kp.publicKey)).toThrow(
      "Invalid secret key length"
    );
  });

  it("throws on invalid public key length", () => {
    const kp = generateKeyPair();
    expect(() => deriveSharedKey(kp.secretKey, new Uint8Array(31))).toThrow(
      "Invalid peer public key length"
    );
  });
});

describe("encrypt / decrypt", () => {
  it("roundtrips a string message", () => {
    const shared = deriveSharedKey(generateKeyPair().secretKey, generateKeyPair().publicKey);
    const plaintext = "hello, world! 🌍";
    const ciphertext = encrypt(shared, plaintext);
    expect(ciphertext.byteLength).toBeGreaterThan(24); // nonce + ciphertext
    const decrypted = decrypt(shared, ciphertext);
    expect(decrypted).toBe(plaintext);
  });

  it("roundtrips ArrayBuffer data", () => {
    const shared = deriveSharedKey(generateKeyPair().secretKey, generateKeyPair().publicKey);
    const plaintext = new Uint8Array([0, 1, 255, 254]).buffer;
    const ciphertext = encrypt(shared, plaintext);
    const decrypted = decrypt(shared, ciphertext);
    expect(decrypted).toBeInstanceOf(ArrayBuffer);
    expect(new Uint8Array(decrypted as ArrayBuffer)).toEqual(new Uint8Array(plaintext));
  });

  it("produces different ciphertexts for same plaintext", () => {
    const shared = deriveSharedKey(generateKeyPair().secretKey, generateKeyPair().publicKey);
    const plaintext = "same";
    const a = new Uint8Array(encrypt(shared, plaintext));
    const b = new Uint8Array(encrypt(shared, plaintext));
    expect(a).not.toEqual(b);
  });

  it("throws on tampered ciphertext", () => {
    const shared = deriveSharedKey(generateKeyPair().secretKey, generateKeyPair().publicKey);
    const ciphertext = new Uint8Array(encrypt(shared, "secret"));
    ciphertext[ciphertext.byteLength - 1] ^= 0xff;
    expect(() => decrypt(shared, ciphertext.buffer)).toThrow("Decryption failed");
  });

  it("throws on short input", () => {
    const shared = deriveSharedKey(generateKeyPair().secretKey, generateKeyPair().publicKey);
    expect(() => decrypt(shared, new ArrayBuffer(23))).toThrow("Ciphertext bundle too short");
  });

  it("throws on wrong shared key", () => {
    const alice = generateKeyPair();
    const bob = generateKeyPair();
    const sharedAB = deriveSharedKey(alice.secretKey, bob.publicKey);
    const sharedAC = deriveSharedKey(alice.secretKey, generateKeyPair().publicKey);
    const ciphertext = encrypt(sharedAB, "secret");
    expect(() => decrypt(sharedAC, ciphertext)).toThrow("Decryption failed");
  });
});
