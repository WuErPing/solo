/**
 * @vitest-environment jsdom
 */
import React from "react";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

const { mockPush, mockTheme } = vi.hoisted(() => ({
  mockPush: vi.fn(),
  mockTheme: {
    colors: {
      background: "#000",
      foreground: "#fff",
      foregroundMuted: "#888",
      surface0: "#111",
      surface1: "#222",
      surface2: "#333",
      primary: "#0af",
      border: "#333",
      destructive: "#f00",
      statusWarning: "#fa0",
      statusSuccess: "#0a0",
      statusDanger: "#f00",
      diffAddition: "#0f0",
      diffDeletion: "#f00",
    },
  },
}));

let mockGroupId: string | undefined;

vi.mock("expo-router", () => ({
  router: { push: mockPush, back: vi.fn(), replace: vi.fn(), navigate: vi.fn() },
  useLocalSearchParams: () => ({ groupId: mockGroupId }),
}));

vi.mock("react-native-unistyles", () => ({
  StyleSheet: {
    create: (factory: unknown) => {
      if (typeof factory === "function") {
        return factory(mockTheme);
      }
      return factory;
    },
  },
  useUnistyles: () => ({ theme: mockTheme, rt: {}, breakpoint: undefined }),
  UnistylesRuntime: { setTheme: vi.fn(), themeName: "light" },
}));

vi.mock("lucide-react-native", () => {
  const icon = (name: string) => {
    const Component = (props: Record<string, unknown>) =>
      React.createElement("span", { "data-icon": name, ...props });
    Component.displayName = `Icon(${name})`;
    return Component;
  };
  return {
    ArrowDown: icon("ArrowDown"),
    ArrowRight: icon("ArrowRight"),
    Check: icon("Check"),
    FileText: icon("FileText"),
    GitBranch: icon("GitBranch"),
    GitMerge: icon("GitMerge"),
    Layers: icon("Layers"),
    Send: icon("Send"),
    Trash2: icon("Trash2"),
    Users: icon("Users"),
  };
});

vi.mock("@/components/headers/back-header", () => ({
  BackHeader: ({
    title,
    titleAccessory,
    rightContent,
  }: {
    title: string;
    titleAccessory?: React.ReactNode;
    rightContent?: React.ReactNode;
  }) =>
    React.createElement(
      "div",
      { "data-testid": "back-header" },
      title,
      titleAccessory,
      rightContent,
    ),
}));

vi.mock("@/components/provider-icons", () => ({
  getProviderIcon: () => {
    const Component = (props: Record<string, unknown>) =>
      React.createElement("span", { "data-icon": "provider", ...props });
    Component.displayName = "ProviderIcon";
    return Component;
  },
}));

vi.mock("@/constants/layout", () => ({
  useIsCompactFormFactor: () => false,
}));

import {
  MOCK_TASK_GROUPS,
  MOCK_TASK_GROUP_DETAILS,
  getTaskGroupById,
  getTaskGroupDetails,
} from "./mock-data";
import { TaskGroupsScreen } from "./task-groups-screen";
import { TaskGroupDetailScreen } from "./task-group-detail-screen";

describe("task-groups mock data", () => {
  it("has unique group ids and non-empty members", () => {
    const ids = MOCK_TASK_GROUPS.map((group) => group.id);
    expect(new Set(ids).size).toBe(ids.length);
    for (const group of MOCK_TASK_GROUPS) {
      expect(group.members.length).toBeGreaterThan(0);
    }
  });

  it("keeps task counts and money fields sane", () => {
    for (const group of MOCK_TASK_GROUPS) {
      expect(group.tasksDone).toBeLessThanOrEqual(group.tasksTotal);
      expect(group.tasksTotal).toBeGreaterThan(0);
      expect(group.costUSD).toBeGreaterThan(0);
      expect(group.tokensK).toBeGreaterThan(0);
      expect(group.pendingReviewCount).toBeGreaterThanOrEqual(0);
      expect(Number.isNaN(Date.parse(group.updatedAt))).toBe(false);
    }
  });

  it("includes a running pipeline, a finished group needing review, and an error member", () => {
    const pipeline = MOCK_TASK_GROUPS.find(
      (group) => group.mode === "pipeline" && group.tasksDone < group.tasksTotal,
    );
    expect(pipeline).toBeDefined();
    expect(pipeline!.members.some((member) => member.status === "waiting")).toBe(true);

    const finished = MOCK_TASK_GROUPS.find(
      (group) => group.tasksDone === group.tasksTotal && group.pendingReviewCount > 0,
    );
    expect(finished).toBeDefined();

    const withError = MOCK_TASK_GROUPS.find((group) =>
      group.members.some((member) => member.status === "error"),
    );
    expect(withError).toBeDefined();

    const parallel = MOCK_TASK_GROUPS.find((group) => group.mode === "parallel");
    expect(parallel).toBeDefined();
  });

  it("has detail extras for every group and no orphan extras", () => {
    for (const group of MOCK_TASK_GROUPS) {
      const details = MOCK_TASK_GROUP_DETAILS[group.id];
      expect(details).toBeDefined();
      expect(details!.activity.length).toBeGreaterThan(0);
      expect(details!.chat.length).toBeGreaterThan(0);
    }
    for (const key of Object.keys(MOCK_TASK_GROUP_DETAILS)) {
      expect(getTaskGroupById(key)).toBeDefined();
    }
  });

  it("returns empty extras for unknown groups", () => {
    const details = getTaskGroupDetails("does-not-exist");
    expect(details.activity).toEqual([]);
    expect(details.chat).toEqual([]);
    expect(details.sharedFiles).toEqual([]);
  });
});

describe("TaskGroupsScreen", () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it("renders a card per mock group with goal and project name", () => {
    render(<TaskGroupsScreen />);
    for (const group of MOCK_TASK_GROUPS) {
      expect(screen.getByText(group.goal)).toBeDefined();
      expect(screen.getByTestId(`task-group-card-${group.id}`)).toBeDefined();
    }
    expect(screen.getAllByText("solo").length).toBeGreaterThanOrEqual(2);
  });

  it("marks the screen as a Beta prototype", () => {
    render(<TaskGroupsScreen />);
    expect(screen.getByText("Beta")).toBeDefined();
  });

  it("shows the review-needed badge only for groups with pending reviews", () => {
    render(<TaskGroupsScreen />);
    expect(screen.getByTestId("review-badge-tg-billing-migration")).toBeDefined();
    expect(screen.getByText("待验收 Review ×2")).toBeDefined();
    expect(screen.queryByTestId("review-badge-tg-auth-refresh")).toBeNull();
  });

  it("renders member chips with status and progress per card", () => {
    render(<TaskGroupsScreen />);
    expect(screen.getByTestId("member-chip-kimi-tokens")).toBeDefined();
    expect(screen.getByText("5/8 tasks")).toBeDefined();
    expect(screen.getByText("12/12 tasks")).toBeDefined();
    expect(screen.getByText("$3.42")).toBeDefined();
  });

  it("navigates to the detail route on card press", () => {
    render(<TaskGroupsScreen />);
    fireEvent.click(screen.getByTestId("task-group-card-tg-auth-refresh"));
    expect(mockPush).toHaveBeenCalledWith("/task-groups/tg-auth-refresh");
  });
});

describe("TaskGroupDetailScreen", () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
    mockGroupId = undefined;
  });

  it("renders members, mock chat, shared context, changes and activity", () => {
    mockGroupId = "tg-auth-refresh";
    render(<TaskGroupDetailScreen />);

    // Members in pipeline order
    expect(screen.getByTestId("member-row-claude-auth")).toBeDefined();
    expect(screen.getByTestId("member-row-kimi-tokens")).toBeDefined();
    expect(screen.getByTestId("member-row-opencode-review")).toBeDefined();

    // Chat pane with disabled prototype input
    expect(screen.getByText(/token test suite/)).toBeDefined();
    expect(screen.getByTestId("chat-input")).toBeDefined();
    expect(screen.getByText(/static mock data/)).toBeDefined();

    // Shared context + changes + activity
    expect(screen.getByText("tokens/refresh.ts")).toBeDefined();
    // Branch appears in both the member row and the changes list
    expect(screen.getAllByText("tg/auth-refresh/auth").length).toBeGreaterThanOrEqual(2);
    expect(screen.getByText(/Handoff: claude-auth → kimi-tokens/)).toBeDefined();
  });

  it("shows an error state for unknown group ids", () => {
    mockGroupId = "nope";
    render(<TaskGroupDetailScreen />);
    expect(screen.getByText(/Task group not found/)).toBeDefined();
  });

  it("toggles change rows to merged/dropped locally", () => {
    mockGroupId = "tg-auth-refresh";
    render(<TaskGroupDetailScreen />);

    fireEvent.click(screen.getByTestId("merge-button-claude-auth"));
    expect(screen.getByText("Merged")).toBeDefined();

    fireEvent.click(screen.getByTestId("drop-button-kimi-tokens"));
    expect(screen.getByText("Dropped")).toBeDefined();
  });

  it("toggles the diff placeholder preview", () => {
    mockGroupId = "tg-auth-refresh";
    render(<TaskGroupDetailScreen />);

    expect(screen.queryByText(/diff rendering is a prototype placeholder/)).toBeNull();
    fireEvent.click(screen.getByTestId("diff-button-claude-auth"));
    expect(screen.getByText(/diff rendering is a prototype placeholder/)).toBeDefined();
  });
});
