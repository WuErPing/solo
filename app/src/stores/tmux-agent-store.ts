import { create } from "zustand";
import type { TmuxAgent } from "@/hooks/use-tmux-agents";

interface TmuxAgentStore {
  selectedAgent: TmuxAgent | null;
  setSelectedAgent: (agent: TmuxAgent | null) => void;
}

export const useTmuxAgentStore = create<TmuxAgentStore>((set) => ({
  selectedAgent: null,
  setSelectedAgent: (agent) => set({ selectedAgent: agent }),
}));
