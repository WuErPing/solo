/**
 * @vitest-environment jsdom
 */
import React from "react";
import { act } from "@testing-library/react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const { activeWorkspaceState, openProjectPickerMock, prepareWorkspaceTabMock } = vi.hoisted(() => ({
  activeWorkspaceState: {
    selection: null as { serverId: string; workspaceId: string } | null,
  },
  openProjectPickerMock: vi.fn(),
  prepareWorkspaceTabMock: vi.fn(),
}));

vi.mock("@/stores/navigation-active-workspace-store", () => ({
  useNavigationActiveWorkspaceSelection: () => activeWorkspaceState.selection,
}));

vi.mock("@/hooks/use-open-project-picker", () => ({
  useOpenProjectPicker: () => openProjectPickerMock,
}));

vi.mock("@/utils/workspace-navigation", () => ({
  prepareWorkspaceTab: prepareWorkspaceTabMock,
}));

import { useNewAgentAction } from "./use-new-agent-action";

function Probe({
  serverId,
  onAction,
}: {
  serverId: string | null;
  onAction: (action: () => Promise<void>) => void;
}) {
  onAction(useNewAgentAction(serverId));
  return null;
}

describe("useNewAgentAction", () => {
  let root: Root | null = null;
  let container: HTMLElement | null = null;
  let action: (() => Promise<void>) | null = null;

  beforeEach(() => {
    activeWorkspaceState.selection = null;
    openProjectPickerMock.mockReset();
    openProjectPickerMock.mockResolvedValue(undefined);
    prepareWorkspaceTabMock.mockReset();
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

  async function renderProbe(serverId: string | null = "srv") {
    await act(async () => {
      root?.render(<Probe serverId={serverId} onAction={(nextAction) => (action = nextAction)} />);
    });
  }

  it("opens a new draft directly in the current workspace", async () => {
    activeWorkspaceState.selection = { serverId: "srv", workspaceId: "workspace-1" };
    await renderProbe();

    await act(async () => action?.());

    expect(prepareWorkspaceTabMock).toHaveBeenCalledWith({
      serverId: "srv",
      workspaceId: "workspace-1",
      target: { kind: "draft", draftId: "new" },
    });
    expect(openProjectPickerMock).not.toHaveBeenCalled();
  });

  it("opens the project picker when no workspace is active", async () => {
    await renderProbe();

    await act(async () => action?.());

    expect(openProjectPickerMock).toHaveBeenCalledTimes(1);
    expect(prepareWorkspaceTabMock).not.toHaveBeenCalled();
  });

  it("does not open a draft on a different host", async () => {
    activeWorkspaceState.selection = { serverId: "other-host", workspaceId: "workspace-1" };
    await renderProbe("srv");

    await act(async () => action?.());

    expect(openProjectPickerMock).toHaveBeenCalledTimes(1);
    expect(prepareWorkspaceTabMock).not.toHaveBeenCalled();
  });
});
