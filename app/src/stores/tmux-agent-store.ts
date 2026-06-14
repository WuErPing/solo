import { create } from "zustand";
import type { TmuxAgent, TmuxPane } from "@/hooks/use-tmux-agents";

type TmuxAgentOrPane = TmuxAgent | TmuxPane;

interface TmuxAgentStore {
  selectedAgent: TmuxAgentOrPane | null;
  setSelectedAgent: (agent: TmuxAgentOrPane | null) => void;
}

export const useTmuxAgentStore = create<TmuxAgentStore>((set) => ({
  selectedAgent: null,
  setSelectedAgent: (agent) => set({ selectedAgent: agent }),
}));

export type { TmuxAgentOrPane };
