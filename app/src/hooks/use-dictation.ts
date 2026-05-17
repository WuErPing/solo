import { useCallback, useState } from "react";

// SOLO-TODO: Voice/Dictation removed - stub hook

export type DictationStatus = "idle" | "recording" | "uploading" | "failed";

export interface UseDictationOptions {
  client: unknown;
  onTranscript: (text: string, meta: { requestId: string }) => void;
  onPartialTranscript?: (text: string, meta: { requestId: string }) => void;
  onError?: (error: Error) => void;
  onPermanentFailure?: (error: Error, context: { requestId: string }) => void;
  canStart?: () => boolean;
  canConfirm?: () => boolean;
  autoStopWhenHidden?: { isVisible: boolean };
  enableDuration?: boolean;
}

export interface UseDictationResult {
  isRecording: boolean;
  isProcessing: boolean;
  partialTranscript: string;
  volume: number;
  duration: number;
  error: string | null;
  status: DictationStatus;
  startDictation: () => Promise<void>;
  cancelDictation: () => Promise<void>;
  confirmDictation: () => Promise<void>;
  retryFailedDictation: () => Promise<void>;
  discardFailedDictation: () => void;
  reset: () => void;
}

export function useDictation(_options: UseDictationOptions): UseDictationResult {
  const [status, setStatus] = useState<DictationStatus>("idle");

  return {
    isRecording: false,
    isProcessing: false,
    partialTranscript: "",
    volume: 0,
    duration: 0,
    error: null,
    status,
    startDictation: useCallback(async () => { setStatus("recording"); }, []),
    cancelDictation: useCallback(async () => { setStatus("idle"); }, []),
    confirmDictation: useCallback(async () => { setStatus("idle"); }, []),
    retryFailedDictation: useCallback(async () => { setStatus("idle"); }, []),
    discardFailedDictation: useCallback(() => { setStatus("idle"); }, []),
    reset: useCallback(() => { setStatus("idle"); }, []),
  };
}
