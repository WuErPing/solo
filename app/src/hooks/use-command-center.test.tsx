/**
 * @vitest-environment jsdom
 */
import React from "react";
import { act } from "@testing-library/react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const { newAgentActionMock } = vi.hoisted(() => ({
  newAgentActionMock: vi.fn(),
}));

vi.mock("expo-router", () => ({
  router: { navigate: vi.fn(), push: vi.fn() },
  usePathname: () => "/h/srv/workspace/workspace-1",
}));

vi.mock("@/constants/layout", () => ({
  getIsElectronRuntime: () => true,
  getIsElectronRuntimeMac: () => true,
}));

vi.mock("@/hooks/use-active-server-id", () => ({
  useActiveServerId: () => "srv",
}));

vi.mock("@/hooks/use-all-agents-list", () => ({
  useAllAgentsList: () => ({ agents: [] }),
}));

vi.mock("@/hooks/use-keyboard-shortcut-overrides", () => ({
  useKeyboardShortcutOverrides: () => ({ overrides: {} }),
}));

vi.mock("@/hooks/use-new-agent-action", () => ({
  useNewAgentAction: () => newAgentActionMock,
}));

vi.mock("@/hooks/use-open-project-picker", () => ({
  useOpenProjectPicker: () => vi.fn(),
}));

vi.mock("@/utils/command-center-focus-restore", () => ({
  clearCommandCenterFocusRestoreElement: vi.fn(),
  takeCommandCenterFocusRestoreElement: () => null,
}));

vi.mock("@/utils/web-focus", () => ({
  focusWithRetries: () => () => {},
}));

import { useKeyboardShortcutsStore } from "@/stores/keyboard-shortcuts-store";
import { useCommandCenter, type CommandCenterItem } from "./use-command-center";

function Probe({
  onResult,
}: {
  onResult: (result: ReturnType<typeof useCommandCenter>) => void;
}) {
  onResult(useCommandCenter());
  return null;
}

describe("useCommandCenter", () => {
  let root: Root | null = null;
  let container: HTMLElement | null = null;
  let result: ReturnType<typeof useCommandCenter> | null = null;

  beforeEach(() => {
    newAgentActionMock.mockReset();
    newAgentActionMock.mockResolvedValue(undefined);
    useKeyboardShortcutsStore.setState({ commandCenterOpen: true });
    result = null;
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
  });

  afterEach(() => {
    if (root) {
      act(() => root?.unmount());
    }
    useKeyboardShortcutsStore.setState({ commandCenterOpen: false });
    root = null;
    container?.remove();
    container = null;
  });

  it("starts a new agent from the New agent command", async () => {
    await act(async () => {
      root?.render(<Probe onResult={(nextResult) => (result = nextResult)} />);
    });

    const newAgentItem = result?.items.find(
      (item): item is Extract<CommandCenterItem, { kind: "action" }> =>
        item.kind === "action" && item.action.id === "new-agent",
    );
    expect(newAgentItem?.action.title).toBe("New agent");

    await act(async () => {
      if (newAgentItem) result?.handleSelectItem(newAgentItem);
    });

    expect(newAgentActionMock).toHaveBeenCalledTimes(1);
  });
});
