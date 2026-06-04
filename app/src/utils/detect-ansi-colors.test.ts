import { describe, expect, it } from "vitest";
import { detectColorsFromAnsi } from "./detect-ansi-colors";

describe("detectColorsFromAnsi", () => {
  it("returns null colors for empty content", () => {
    const result = detectColorsFromAnsi("");
    expect(result.background).toBeNull();
    expect(result.foreground).toBeNull();
  });

  it("detects background from OSC 11 sequence", () => {
    const content = "\x1b]11;rgb:1a1a/2b2b/3c3c\x1b\\";
    const result = detectColorsFromAnsi(content);
    expect(result.background).toBe("#1a1a2b2b3c3c");
  });

  it("detects foreground from OSC 10 sequence", () => {
    const content = "\x1b]10;rgb:ff/ff/ff\x07";
    const result = detectColorsFromAnsi(content);
    expect(result.foreground).toBe("#ffffff");
  });

  it("detects background from SGR true color sequence", () => {
    const content = "Hello \x1b[48;2;26;26;44m World";
    const result = detectColorsFromAnsi(content);
    expect(result.background).toBe("#1a1a2c");
  });

  it("detects foreground from SGR true color sequence", () => {
    const content = "Hello \x1b[38;2;205;214;244m World";
    const result = detectColorsFromAnsi(content);
    expect(result.foreground).toBe("#cdd6f4");
  });

  it("prefers OSC over SGR when both present", () => {
    const content = "\x1b]11;rgb:aa/bb/cc\x1b\\ \x1b[48;2;0;0;0m";
    const result = detectColorsFromAnsi(content);
    expect(result.background).toBe("#aabbcc");
  });

  it("handles opencode-style content with embedded colors", () => {
    const content = "\x1b[48;2;24;24;37m\x1b[38;2;205;214;244m  opencode> _";
    const result = detectColorsFromAnsi(content);
    expect(result.background).toBe("#181825");
    expect(result.foreground).toBe("#cdd6f4");
  });

  it("detects background from 256-color palette", () => {
    const content = "Hello \x1b[48;5;235m World";
    const result = detectColorsFromAnsi(content);
    expect(result.background).toBe("#262626");
  });

  it("detects foreground from 256-color palette", () => {
    const content = "Hello \x1b[38;5;78m World";
    const result = detectColorsFromAnsi(content);
    expect(result.foreground).toBe("#5fd787");
  });

  it("handles qodercli-style content with 256-color palette", () => {
    const content = "\x1b[38;5;78m  Qoder CLI\x1b[0m\x1b[38;5;145m v1.0.13\x1b[39m";
    const result = detectColorsFromAnsi(content);
    expect(result.foreground).toBe("#5fd787");
  });
});
