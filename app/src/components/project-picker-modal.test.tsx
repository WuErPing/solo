/**
 * @vitest-environment jsdom
 */
import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ProjectPickerModal } from "./project-picker-modal";

const { theme, picker, platform, serverPaths, openProject, submitHandlers, setProjectPickerOpen } =
  vi.hoisted(() => ({
    theme: {
      spacing: { 1: 4, 2: 8, 3: 12, 4: 16, 6: 24, 12: 48 },
      fontSize: { xs: 12, sm: 14, base: 16, lg: 18, "2xl": 24 },
      fontWeight: { normal: "400", medium: "500" },
      colors: {
        surface0: "#111",
        surface1: "#222",
        surface2: "#333",
        foreground: "#fff",
        foregroundMuted: "#999",
        border: "#555",
        accent: "#0a84ff",
        destructive: "#ef4444",
      },
      borderRadius: { md: 6, lg: 8, xl: 12, full: 9999 },
      borderWidth: { 1: 1 },
      opacity: { 50: 0.5 },
      shadow: { lg: {} },
    },
    picker: { current: { open: true } },
    platform: { current: { isNative: true } },
    serverPaths: { current: [] as string[] },
    openProject: vi.fn(),
    submitHandlers: { current: {} as Record<string, (() => void) | undefined> },
    setProjectPickerOpen: vi.fn(),
  }));

vi.mock("react-native", () => ({
  Modal: ({ children, visible }: React.PropsWithChildren<{ visible?: boolean }>) =>
    visible ? React.createElement("div", { "data-testid": "modal" }, children) : null,
  View: ({
    children,
    testID,
    ...props
  }: React.PropsWithChildren<{ testID?: string } & Record<string, unknown>>) =>
    React.createElement("div", { "data-testid": testID, ...props }, children),
  Text: ({
    children,
    testID,
    ...props
  }: React.PropsWithChildren<{ testID?: string } & Record<string, unknown>>) =>
    React.createElement("span", { "data-testid": testID, ...props }, children),
  Pressable: ({
    children,
    onPress,
    disabled,
    testID,
    ...props
  }: React.PropsWithChildren<{
    onPress?: () => void;
    disabled?: boolean;
    testID?: string;
  }>) =>
    React.createElement(
      "button",
      { type: "button", onClick: onPress, disabled, "data-testid": testID },
      children,
    ),
  ScrollView: ({
    children,
    testID,
    ...props
  }: React.PropsWithChildren<{ testID?: string } & Record<string, unknown>>) =>
    React.createElement("div", { "data-testid": testID, ...props }, children),
  TextInput: React.forwardRef<
    HTMLInputElement,
    {
      testID?: string;
      value?: string;
      returnKeyType?: string;
      onChangeText?: (text: string) => void;
      onSubmitEditing?: () => void;
    }
  >(({ testID, value, returnKeyType, onChangeText, onSubmitEditing, ...props }, ref) => {
    if (testID) {
      submitHandlers.current[testID] = onSubmitEditing;
    }
    return React.createElement("input", {
      ref,
      "data-testid": testID,
      "data-return-key-type": returnKeyType,
      value: value ?? "",
      onChange: (event: React.ChangeEvent<HTMLInputElement>) =>
        onChangeText?.(event.target.value),
      ...props,
    });
  }),
  Platform: { OS: "web" },
  KeyboardAvoidingView: ({
    children,
    ...props
  }: React.PropsWithChildren<Record<string, unknown>>) =>
    React.createElement("div", { "data-testid": "keyboard-avoiding-view", ...props }, children),
}));

vi.mock("react-native-unistyles", () => ({
  StyleSheet: {
    create: (factory: unknown) => (typeof factory === "function" ? factory(theme) : factory),
  },
  useUnistyles: () => ({ theme }),
}));

vi.mock("@tanstack/react-query", () => ({
  useQuery: () => ({ data: serverPaths.current }),
}));

vi.mock("@/constants/platform", () => ({
  get isNative() {
    return platform.current.isNative;
  },
}));

vi.mock("@/stores/keyboard-shortcuts-store", () => ({
  useKeyboardShortcutsStore: (
    selector: (state: {
      projectPickerOpen: boolean;
      setProjectPickerOpen: (open: boolean) => void;
    }) => unknown,
  ) =>
    selector({
      projectPickerOpen: picker.current.open,
      setProjectPickerOpen,
    }),
}));

vi.mock("@/stores/session-store-hooks", () => ({
  useRecommendedProjectPaths: () => ["/Users/andy/code/solo", "/Users/andy/code/web-api"],
}));

vi.mock("@/hooks/use-active-server-id", () => ({
  useActiveServerId: () => "server-1",
}));

vi.mock("@/runtime/host-runtime", () => ({
  useHostRuntimeClient: () => ({
    getDirectorySuggestions: vi.fn(async () => ({ entries: [] })),
    openProject: vi.fn(),
  }),
  useHostRuntimeIsConnected: () => true,
}));

vi.mock("@/hooks/use-open-project", () => ({
  useOpenProject: () => openProject,
}));

vi.mock("@/components/ui/button", () => ({
  Button: ({
    children,
    onPress,
    disabled,
    loading,
    testID,
  }: React.PropsWithChildren<{
    onPress?: () => void;
    disabled?: boolean;
    loading?: boolean;
    testID?: string;
  }>) =>
    React.createElement(
      "button",
      {
        type: "button",
        onClick: onPress,
        disabled: disabled || loading,
        "data-loading": loading ? "true" : "false",
        "data-testid": testID,
      },
      children,
    ),
}));

vi.mock("lucide-react-native", () => ({
  Folder: () => React.createElement("span", { "data-icon": "Folder" }),
  ArrowRight: () => React.createElement("span", { "data-icon": "ArrowRight" }),
  X: () => React.createElement("span", { "data-icon": "X" }),
}));

const SUBMIT = '[data-testid="project-picker-submit"]';
const INPUT = '[data-testid="project-picker-input"]';
const ERROR = '[data-testid="project-picker-error"]';

function querySelector<T extends HTMLElement>(selector: string): T | null {
  return (container?.querySelector(selector) ?? null) as T | null;
}

function typePath(path: string) {
  const input = querySelector<HTMLInputElement>(INPUT);
  if (!input) {
    throw new Error(`Path input not found: ${INPUT}`);
  }
  const setValue = Object.getOwnPropertyDescriptor(
    window.HTMLInputElement.prototype,
    "value",
  )?.set;
  act(() => {
    setValue?.call(input, path);
    input.dispatchEvent(new Event("change", { bubbles: true }));
  });
}

async function pressSubmit() {
  await act(async () => {
    querySelector<HTMLElement>(SUBMIT)?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    await Promise.resolve();
  });
}

async function pressKeyboardGo() {
  await act(async () => {
    submitHandlers.current["project-picker-input"]?.();
    await Promise.resolve();
  });
}

async function pressWebEnter() {
  await act(async () => {
    window.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", bubbles: true }));
    await Promise.resolve();
  });
}

let container: HTMLElement | null = null;
let root: Root | null = null;

beforeEach(() => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);

  picker.current = { open: true };
  platform.current = { isNative: true };
  serverPaths.current = [];
  submitHandlers.current = {};
  openProject.mockReset().mockResolvedValue(true);
  setProjectPickerOpen.mockReset();
});

afterEach(() => {
  if (root) {
    act(() => {
      root?.unmount();
    });
  }
  root = null;
  container?.remove();
  container = null;
  vi.unstubAllGlobals();
});

function renderModal() {
  act(() => {
    root?.render(<ProjectPickerModal />);
  });
}

describe("ProjectPickerModal", () => {
  it("exposes a primary submit button on native so the path step is actionable", () => {
    renderModal();

    expect(querySelector<HTMLButtonElement>(SUBMIT)).not.toBeNull();
    // Nothing typed yet — the action must be present but inert.
    expect(querySelector<HTMLButtonElement>(SUBMIT)?.disabled).toBe(true);
  });

  it("enables the submit button once a path is typed", () => {
    renderModal();

    typePath("/Users/andy/code/new-repo");

    expect(querySelector<HTMLButtonElement>(SUBMIT)?.disabled).toBe(false);
  });

  it("opens the exact typed path when the submit button is pressed", async () => {
    renderModal();

    typePath("/Users/andy/code/new-repo");
    await pressSubmit();

    expect(openProject).toHaveBeenCalledWith("/Users/andy/code/new-repo");
    await vi.waitFor(() => {
      expect(setProjectPickerOpen).toHaveBeenCalledWith(false);
    });
  });

  it("wires the software keyboard Go key to submit the typed path", async () => {
    renderModal();

    typePath(" /Users/andy/code/solo ");
    expect(querySelector<HTMLInputElement>(INPUT)?.dataset.returnKeyType).toBe("go");

    await pressKeyboardGo();

    expect(openProject).toHaveBeenCalledWith("/Users/andy/code/solo");
  });

  it("offers the raw typed path as a row when nothing matches", () => {
    renderModal();

    typePath("/Users/andy/code/brand-new");

    expect(container?.textContent).toContain("/Users/andy/code/brand-new");
  });

  it("surfaces why opening a path failed instead of silently doing nothing", async () => {
    openProject.mockResolvedValueOnce(false);
    renderModal();

    typePath("/nope/missing");
    await pressSubmit();

    const error = querySelector<HTMLElement>(ERROR);
    expect(error).not.toBeNull();
    expect(error?.textContent).toContain("/nope/missing");
    expect(setProjectPickerOpen).not.toHaveBeenCalledWith(false);
  });

  it("surfaces a rejected open request without leaking an unhandled rejection", async () => {
    const unhandled = vi.fn();
    process.on("unhandledRejection", unhandled);
    openProject.mockRejectedValueOnce(new Error("connection reset"));
    renderModal();

    typePath("/Users/andy/code/solo");
    await act(async () => {
      querySelector<HTMLElement>(SUBMIT)?.dispatchEvent(
        new MouseEvent("click", { bubbles: true }),
      );
      await Promise.resolve();
      await Promise.resolve();
    });

    expect(querySelector<HTMLElement>(ERROR)?.textContent).toContain("connection reset");
    expect(querySelector<HTMLButtonElement>(SUBMIT)?.disabled).toBe(false);
    process.off("unhandledRejection", unhandled);
    expect(unhandled).not.toHaveBeenCalled();
  });

  it("renders an explicit close control so the backdrop is not the only exit", () => {
    renderModal();

    expect(querySelector<HTMLElement>('[data-testid="project-picker-close"]')).not.toBeNull();
  });

  it("keeps the web Enter selection path working", async () => {
    platform.current = { isNative: false };
    serverPaths.current = ["/Users/andy/code/solo-mobile"];
    renderModal();

    typePath("solo");
    await pressWebEnter();

    // A bare substring is not a confirmable path, so Enter commits the
    // highlighted suggestion at activeIndex 0.
    expect(openProject).toHaveBeenCalledWith("/Users/andy/code/solo");
  });

  it("clears a previous error when the query changes", async () => {
    openProject.mockResolvedValueOnce(false);
    renderModal();

    typePath("/bad/one");
    await pressSubmit();
    expect(querySelector<HTMLElement>(ERROR)).not.toBeNull();

    typePath("/bad/two");
    expect(querySelector<HTMLElement>(ERROR)).toBeNull();
  });
});
