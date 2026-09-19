/**
 * @vitest-environment jsdom
 */
import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const { listeners, ensureConnectedAll } = vi.hoisted(() => ({
  listeners: new Set<(state: string) => void>(),
  ensureConnectedAll: vi.fn(),
}));

vi.mock("react-native", () => ({
  AppState: {
    currentState: "active",
    addEventListener: (_event: string, cb: (state: string) => void) => {
      listeners.add(cb);
      return { remove: () => listeners.delete(cb) };
    },
  },
}));

vi.mock("@/runtime/host-runtime", () => ({
  getHostRuntimeStore: () => ({ ensureConnectedAll }),
}));

import { useForegroundReconnect } from "./use-foreground-reconnect";

function Harness() {
  useForegroundReconnect();
  return null;
}

function emit(state: string) {
  act(() => {
    for (const cb of Array.from(listeners)) {
      cb(state);
    }
  });
}

describe("useForegroundReconnect", () => {
  let container: HTMLElement;
  let root: Root | null = null;

  beforeEach(() => {
    vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
    container = document.createElement("div");
    document.body.appendChild(container);
    listeners.clear();
    ensureConnectedAll.mockClear();
  });

  afterEach(() => {
    act(() => {
      root?.unmount();
    });
    root = null;
    container.remove();
  });

  it("probes all hosts when the app returns to the foreground", () => {
    act(() => {
      root = createRoot(container);
      root.render(<Harness />);
    });
    emit("active");
    expect(ensureConnectedAll).toHaveBeenCalledTimes(1);
  });

  it("ignores background and inactive transitions", () => {
    act(() => {
      root = createRoot(container);
      root.render(<Harness />);
    });
    emit("background");
    emit("inactive");
    expect(ensureConnectedAll).not.toHaveBeenCalled();
  });

  it("stops probing after unmount", () => {
    act(() => {
      root = createRoot(container);
      root.render(<Harness />);
    });
    act(() => {
      root?.unmount();
    });
    root = null;
    emit("active");
    expect(ensureConnectedAll).not.toHaveBeenCalled();
  });
});
