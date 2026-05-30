/**
 * Absence-verification test for the Voice / Dictation removal.
 *
 * These tests prove that the voice/dictation feature surface has been fully
 * removed from the web app entry points. They assert on the *source* (not
 * behavior) so they are robust to the underlying components being deleted.
 *
 * Run with: npm test -- voice-removal-absence
 */
import { describe, expect, it } from "vitest";
import fs from "node:fs";
import path from "node:path";

const APP_SRC = path.resolve(__dirname);
const APP_ROOT = path.resolve(__dirname, "..");

const readSource = (relPath: string): string | null => {
  const isOutsideSrc = relPath.startsWith("../");
  const base = isOutsideSrc ? APP_ROOT : APP_SRC;
  const rel = isOutsideSrc ? relPath.slice(3) : relPath;
  const abs = path.resolve(base, rel);
  if (!fs.existsSync(abs)) return null;
  return fs.readFileSync(abs, "utf8");
};

const mustNotExist = (relPath: string) => {
  it(`deleted: ${relPath}`, () => {
    expect(readSource(relPath), `${relPath} should be deleted`).toBeNull();
  });
};

const mustNotReference = (
  relPath: string,
  forbidden: RegExp[],
) => {
  const src = readSource(relPath);
  if (src === null) {
    it(`${relPath} deleted or contains no voice/dictation references`, () => {
      expect(true).toBe(true);
    });
    return;
  }
  it(`${relPath} contains no voice/dictation references`, () => {
    const hits: string[] = [];
    for (const re of forbidden) {
      const match = src.match(re);
      if (match) hits.push(`${re.source} -> ${match[0]}`);
    }
    expect(hits).toEqual([]);
  });
};

describe("voice/dictation stub files are removed", () => {
  mustNotExist("hooks/use-dictation.ts");
  mustNotExist("hooks/use-is-dictation-ready.ts");
  mustNotExist("hooks/use-dictation-audio-source.types.ts");
  mustNotExist("hooks/use-audio-recorder.web.ts");
  mustNotExist("hooks/use-audio-recorder.d.ts");
  mustNotExist("components/dictation-controls.tsx");
  mustNotExist("components/realtime-voice-overlay.tsx");
  mustNotExist("contexts/voice-context.tsx");
  mustNotExist("voice/audio-engine-types.ts");
});

describe("entry-point components no longer reference voice/dictation", () => {
  const forbidden = [
    /\buseDictation\b/,
    /\buseIsDictationReady\b/,
    /\bDictationOverlay\b/,
    /\bRealtimeVoiceOverlay\b/,
    /\bVoiceProvider\b/,
    /\buseVoiceOptional\b/,
    /\buseVoiceRuntimeOptional\b/,
    /\buseVoiceAudioEngineOptional\b/,
    /\bComposerVoiceModeButton\b/,
    /\battemptStartRealtimeVoice\b/,
    /\bSpeakMessage\b/,
    /\bdictation-(toggle|cancel|confirm)\b/,
    /\bvoice-(toggle|mute-toggle)\b/,
    /@\/hooks\/use-dictation/,
    /@\/contexts\/voice-context/,
    /\.\/dictation-controls/,
    /\.\/realtime-voice-overlay/,
  ];

  mustNotReference("components/message-input.tsx", forbidden);
  mustNotReference("components/composer.tsx", forbidden);
  mustNotReference("components/message.tsx", forbidden);
  mustNotReference("app/_layout.tsx", forbidden);
  mustNotReference("screens/settings-screen.tsx", forbidden);
});
describe("session context no longer handles voice/STT events", () => {
  const forbidden = [
    /transcription_result/,
    /voice_input_state/,
    /onTranscriptionResult/,
    /onServerSpeechStateChanged/,
    /useVoiceRuntimeOptional/,
    /useVoiceAudioEngineOptional/,
    /@\/contexts\/voice-context/,
  ];
  mustNotReference("contexts/session-context.tsx", forbidden);
});

describe("keyboard layer has no voice/dictation actions", () => {
  const forbidden = [
    /\bdictation-(toggle|cancel|confirm)\b/,
    /\bvoice-(toggle|mute-toggle)\b/,
    /DictationToggle/,
    /VoiceToggle/,
    /VoiceMuteToggle/,
  ];
  mustNotReference("keyboard/actions.ts", forbidden);
  mustNotReference("keyboard/keyboard-action-dispatcher.ts", forbidden);
  mustNotReference("keyboard/keyboard-shortcuts.ts", forbidden);
});

describe("desktop permissions no longer expose microphone", () => {
  const forbidden = [
    /getMicrophonePermissionStatus/,
    /requestMicrophonePermissionStatus/,
    /"microphone"/,
    /'microphone'/,
  ];
  mustNotReference("desktop/permissions/desktop-permissions.ts", forbidden);
});

describe("voice readiness helpers are removed", () => {
  const forbidden = [
    /getVoiceReadinessState/,
    /resolveVoiceUnavailableMessage/,
  ];
  mustNotReference("utils/server-info-capabilities.ts", forbidden);
});

describe("app config no longer requests microphone / audio recording", () => {
  const forbidden = [
    /NSMicrophoneUsageDescription/,
    /RECORD_AUDIO/,
    /MODIFY_AUDIO_SETTINGS/,
    /expo-audio/,
  ];
  mustNotReference("../app.config.js", forbidden);
  mustNotReference("../package.json", [/expo-audio/]);
});

describe("e2e voice fixtures removed", () => {
  mustNotExist("../e2e/fixtures/recording.wav");
  mustNotExist("../e2e/fixtures/recording.webm");
  mustNotExist("../e2e/fixtures/recording.baseline.txt");
});