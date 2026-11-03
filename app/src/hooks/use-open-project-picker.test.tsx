/**
 * @vitest-environment jsdom
 */
import React from "react";
import { act } from "@testing-library/react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const { runtime, openProjectMock, pickDirectoryMock, setProjectPickerOpenMock } = vi.hoisted(() => ({
  runtime: { isLocalDaemon: true, isWeb: true },
  openProjectMock: vi.fn(),
  pickDirectoryMock: vi.fn(),
  setProjectPickerOpenMock: vi.fn(),
}));

vi.mock("@/desktop/pick-directory", () => ({
  pickDirectory: pickDirectoryMock,
}));

vi.mock("@/constants/platform", () => ({
  get isWeb() {
    return runtime.isWeb;
  },
}));

vi.mock("@/stores/keyboard-shortcuts-store", () => ({
  useKeyboardShortcutsStore: (
    selector: (state: { setProjectPickerOpen: (open: boolean) => void }) => unknown,
  ) => selector({ setProjectPickerOpen: setProjectPickerOpenMock }),
}));

vi.mock("@/hooks/use-is-local-daemon", () => ({
  useIsLocalDaemon: () => runtime.isLocalDaemon,
}));

vi.mock("@/hooks/use-open-project", () => ({
  useOpenProject: () => openProjectMock,
}));

import { useOpenProjectPicker } from "./use-open-project-picker";

function Probe({ onAction }: { onAction: (action: () => Promise<void>) => void }) {
  onAction(useOpenProjectPicker("srv"));
  return null;
}

describe("useOpenProjectPicker", () => {
  let root: Root | null = null;
  let container: HTMLElement | null = null;
  let action: (() => Promise<void>) | null = null;

  beforeEach(() => {
    runtime.isLocalDaemon = true;
    runtime.isWeb = true;
    openProjectMock.mockReset().mockResolvedValue(true);
    pickDirectoryMock.mockReset().mockResolvedValue(null);
    setProjectPickerOpenMock.mockReset();
    action = null;
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
  });

  afterEach(() => {
    if (root) {
      act(() => root?.unmount());
    }
    root = null;
    container?.remove();
    container = null;
  });

  async function renderPicker() {
    await act(async () => {
      root?.render(<Probe onAction={(nextAction) => (action = nextAction)} />);
    });
  }

  it.each([true, false])(
    "opens the recent-directory modal on web when isLocalDaemon is %s",
    async (isLocalDaemon) => {
      runtime.isLocalDaemon = isLocalDaemon;
      await renderPicker();

      await act(async () => action?.());

      expect(setProjectPickerOpenMock).toHaveBeenCalledWith(true);
      expect(pickDirectoryMock).not.toHaveBeenCalled();
    },
  );

  it("keeps the native directory picker outside web", async () => {
    runtime.isWeb = false;
    pickDirectoryMock.mockResolvedValue("/repo/solo");
    await renderPicker();

    await act(async () => action?.());

    expect(pickDirectoryMock).toHaveBeenCalledTimes(1);
    expect(openProjectMock).toHaveBeenCalledWith("/repo/solo");
    expect(setProjectPickerOpenMock).not.toHaveBeenCalled();
  });
});
