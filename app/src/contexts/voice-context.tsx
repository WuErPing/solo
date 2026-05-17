import { useCallback, useRef } from "react";

// SOLO-TODO: Voice/Dictation removed - stub context

export interface VoiceSessionAdapter {
  serverId: string;
  setVoiceMode(enabled: boolean, agentId?: string): Promise<void>;
  sendVoiceAudioChunk(audioData: string, mimeType: string): Promise<void>;
  audioPlayed(chunkId: string): Promise<void>;
  abortRequest(): Promise<void>;
  setAssistantAudioPlaying(isPlaying: boolean): void;
}

export interface VoiceRuntimeSnapshot {
  phase: "disabled";
  isVoiceMode: boolean;
  isVoiceSwitching: boolean;
  isMuted: boolean;
  activeServerId: string | null;
  activeAgentId: string | null;
}

export type VoiceContextValue = VoiceRuntimeSnapshot & {
  startVoice: (serverId: string, agentId: string) => Promise<void>;
  stopVoice: () => Promise<void>;
  isVoiceModeForAgent: (serverId: string, agentId: string) => boolean;
  toggleMute: () => void;
};

export interface VoiceRuntime {
  subscribe(listener: () => void): () => void;
  getSnapshot(): VoiceRuntimeSnapshot;
  subscribeTelemetry(listener: () => void): () => void;
  getTelemetrySnapshot(): { volume: number; isSpeaking: boolean; segmentDuration: number };
  registerSession(adapter: VoiceSessionAdapter): () => void;
  updateSessionConnection(serverId: string, connected: boolean): void;
  handleCapturePcm(chunk: Uint8Array): void;
  handleCaptureVolume(level: number): void;
  handleAudioOutput(serverId: string, payload: unknown): void;
  startVoice(serverId: string, agentId: string): Promise<void>;
  stopVoice(): Promise<void>;
  destroy(): Promise<void>;
  toggleMute(): void;
  isVoiceModeForAgent(serverId: string, agentId: string): boolean;
  shouldPlayVoiceAudio(serverId: string): boolean;
  onAssistantAudioStarted(serverId: string): void;
  onAssistantAudioFinished(serverId: string): void;
  onTranscriptionResult(serverId: string, text: string): void;
  onServerSpeechStateChanged(serverId: string, isSpeaking: boolean): void;
  onTurnEvent(serverId: string, agentId: string, eventType: string): void;
}

export interface AudioEngine {
  initialize(): Promise<void>;
  startRecording(): Promise<void>;
  stopRecording(): Promise<void>;
  play(source: unknown): Promise<void>;
  stop(): void;
}

const EMPTY_SNAPSHOT: VoiceRuntimeSnapshot = {
  phase: "disabled",
  isVoiceMode: false,
  isVoiceSwitching: false,
  isMuted: false,
  activeServerId: null,
  activeAgentId: null,
};

function createNoopVoiceRuntime(): VoiceRuntime {
  return {
    subscribe: () => () => {},
    getSnapshot: () => EMPTY_SNAPSHOT,
    subscribeTelemetry: () => () => {},
    getTelemetrySnapshot: () => ({ volume: 0, isSpeaking: false, segmentDuration: 0 }),
    registerSession: () => () => {},
    updateSessionConnection: () => {},
    handleCapturePcm: () => {},
    handleCaptureVolume: () => {},
    handleAudioOutput: () => {},
    startVoice: async () => {},
    stopVoice: async () => {},
    destroy: async () => {},
    toggleMute: () => {},
    isVoiceModeForAgent: () => false,
    shouldPlayVoiceAudio: () => false,
    onAssistantAudioStarted: () => {},
    onAssistantAudioFinished: () => {},
    onTranscriptionResult: () => {},
    onServerSpeechStateChanged: () => {},
    onTurnEvent: () => {},
  };
}

export function VoiceProvider({ children }: { children: React.ReactNode }) {
  return <>{children}</>;
}

export function useVoiceOptional(): VoiceContextValue | null {
  return {
    phase: "disabled",
    isVoiceMode: false,
    isVoiceSwitching: false,
    isMuted: false,
    activeServerId: null,
    activeAgentId: null,
    startVoice: async () => {},
    stopVoice: async () => {},
    isVoiceModeForAgent: () => false,
    toggleMute: () => {},
  };
}

export function useVoiceAudioEngineOptional(): AudioEngine | null {
  return null;
}

export function useVoiceRuntimeOptional(): VoiceRuntime | null {
  return createNoopVoiceRuntime();
}
