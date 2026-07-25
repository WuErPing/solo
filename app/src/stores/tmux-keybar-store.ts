import AsyncStorage from "@react-native-async-storage/async-storage";
import { create } from "zustand";
import { createJSONStorage, persist } from "zustand/middleware";

interface TmuxKeyBarState {
  expanded: boolean;
  toggleExpanded: () => void;
}

export const useTmuxKeyBarStore = create<TmuxKeyBarState>()(
  persist(
    (set) => ({
      expanded: false,
      toggleExpanded: () => set((state) => ({ expanded: !state.expanded })),
    }),
    {
      name: "tmux-keybar-state",
      version: 1,
      storage: createJSONStorage(() => AsyncStorage),
      partialize: (state) => ({ expanded: state.expanded }),
    },
  ),
);
