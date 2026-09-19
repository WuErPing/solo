import { useEffect } from "react";
import { AppState } from "react-native";
import { getHostRuntimeStore } from "@/runtime/host-runtime";

// OS-suspended sockets survive foregrounding as half-dead TCP connections: no
// close event fires, so the reconnect backoff never starts and the next RPC
// stalls until its own timeout. Probing on resume collapses that window.
export function useForegroundReconnect(): void {
  useEffect(() => {
    const subscription = AppState.addEventListener("change", (nextState) => {
      if (nextState !== "active") {
        return;
      }
      getHostRuntimeStore().ensureConnectedAll();
    });
    return () => {
      subscription.remove();
    };
  }, []);
}
