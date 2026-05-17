// SOLO-TODO: Voice/Dictation removed - simplified
export function getVoiceReadinessState(_params: { capabilities?: unknown; mode: string }) {
  return { enabled: false, reason: "Voice not available" };
}

export function resolveVoiceUnavailableMessage(_params: { capabilities?: unknown; mode: string }) {
  return "Voice not available";
}
