// @ts-nocheck
import { describe, expect, it } from "vitest";
import {
  getVoiceReadinessState,
  resolveVoiceUnavailableMessage,
} from "./server-info-capabilities";

describe("server-info-capabilities", () => {
  it("always returns voice as unavailable in solo", () => {
    expect(getVoiceReadinessState({ mode: "dictation" })).toEqual({
      enabled: false,
      reason: "Voice not available",
    });
    expect(getVoiceReadinessState({ mode: "voice" })).toEqual({
      enabled: false,
      reason: "Voice not available",
    });
  });

  it("always returns unavailable message in solo", () => {
    expect(resolveVoiceUnavailableMessage({ mode: "dictation" })).toBe(
      "Voice not available",
    );
    expect(resolveVoiceUnavailableMessage({ mode: "voice" })).toBe(
      "Voice not available",
    );
  });
});
