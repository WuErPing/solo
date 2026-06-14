import { describe, expect, it } from "vitest";
import { parseOscDefaultColors } from "./parse-osc-default-colors";

const ESC = "\x1b";
const BEL = "\x07";
const ST = `${ESC}\\`;

describe("parseOscDefaultColors", () => {
  describe("OSC 11 (default background)", () => {
    it("extracts 16-bit-per-channel rgb reply terminated by BEL", () => {
      const content = `some text${ESC}]11;rgb:1e1e/2e2e/3e3e${BEL}more text`;
      expect(parseOscDefaultColors(content)).toEqual({
        background: "#1e2e3e",
        foreground: null,
      });
    });

    it("extracts 8-bit rgb reply (legacy terminals)", () => {
      const content = `${ESC}]11;rgb:1e/2e/3e${BEL}`;
      expect(parseOscDefaultColors(content).background).toBe("#1e2e3e");
    });

    it("extracts 4-bit rgb reply", () => {
      const content = `${ESC}]11;rgb:1/2/3${BEL}`;
      expect(parseOscDefaultColors(content).background).toBe("#112233");
    });

    it("normalizes mixed-width rgb (8/16/4) to 8-bit per channel", () => {
      const content = `${ESC}]11;rgb:1e/2e2e/3${BEL}`;
      expect(parseOscDefaultColors(content).background).toBe("#1e2e33");
    });

    it("accepts ST terminator (ESC \\\\)", () => {
      const content = `${ESC}]11;rgb:aa11/bb22/cc33${ST}`;
      expect(parseOscDefaultColors(content).background).toBe("#aa11bb22cc33".length > 7 ? "#aabbcc" : "#aabbcc");
      // More precisely: each 16-bit component truncates to 8-bit
      expect(parseOscDefaultColors(content).background).toBe("#aabbcc");
    });

    it("is case-insensitive on the rgb: prefix and hex digits", () => {
      expect(parseOscDefaultColors(`${ESC}]11;RGB:AA/BB/CC${BEL}`).background).toBe("#aabbcc");
      expect(parseOscDefaultColors(`${ESC}]11;rgb:aa/bb/cc${BEL}`).background).toBe("#aabbcc");
    });
  });

  describe("OSC 10 (default foreground)", () => {
    it("extracts foreground rgb", () => {
      const content = `${ESC}]10;rgb:cdcd/d6d6/f4f4${BEL}`;
      expect(parseOscDefaultColors(content)).toEqual({
        background: null,
        foreground: "#cdd6f4",
      });
    });
  });

  describe("combined replies", () => {
    it("extracts both OSC 10 and OSC 11 from the same content", () => {
      const content =
        `${ESC}]10;rgb:cdcd/d6d6/f4f4${BEL}` +
        `some log output` +
        `${ESC}]11;rgb:1e1e/2e2e/3e3e${BEL}`;
      expect(parseOscDefaultColors(content)).toEqual({
        background: "#1e2e3e",
        foreground: "#cdd6f4",
      });
    });

    it("uses the latest reply when the same OSC code appears multiple times", () => {
      const content =
        `${ESC}]11;rgb:0000/0000/0000${BEL}` +
        `${ESC}]11;rgb:1e1e/2e2e/3e3e${BEL}`;
      expect(parseOscDefaultColors(content).background).toBe("#1e2e3e");
    });
  });

  describe("content without OSC replies", () => {
    it("returns null for both when content has no OSC 10/11 replies", () => {
      expect(parseOscDefaultColors("plain text, no escapes")).toEqual({
        background: null,
        foreground: null,
      });
    });

    it("returns null when content has OSC replies for unrelated codes (e.g. OSC 0 title)", () => {
      const content = `${ESC}]0;window title${BEL}${ESC}]52;c;clipboard${BEL}`;
      expect(parseOscDefaultColors(content)).toEqual({
        background: null,
        foreground: null,
      });
    });

    it("returns null for empty content", () => {
      expect(parseOscDefaultColors("")).toEqual({ background: null, foreground: null });
    });

    it("returns null for malformed rgb (missing channels)", () => {
      const content = `${ESC}]11;rgb:1e1e/2e2e${BEL}`;
      expect(parseOscDefaultColors(content).background).toBeNull();
    });

    it("returns null for non-rgb OSC 11 reply (e.g. '?' query echo)", () => {
      const content = `${ESC}]11;?${BEL}`;
      expect(parseOscDefaultColors(content).background).toBeNull();
    });
  });
});
