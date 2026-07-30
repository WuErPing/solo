/**
 * @vitest-environment jsdom
 */
import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { UsageResult } from "@/hooks/use-usage";
import type { UsageQuotaSnapshot } from "@server/client/usage-rpc";
import { UsageScreen } from "@/screens/usage/usage-screen";

const { usageResult } = vi.hoisted(() => ({
  usageResult: {
    current: null as UsageResult | null,
  },
}));

vi.mock("react-native", () => ({
  Platform: { OS: "web" },
  View: ({ children, ...props }: React.PropsWithChildren<Record<string, unknown>>) =>
    React.createElement("div", props, children),
  Text: ({ children, ...props }: React.PropsWithChildren<Record<string, unknown>>) =>
    React.createElement("span", props, children),
  Pressable: ({ children, onPress, ...props }: React.PropsWithChildren<{ onPress?: () => void } & Record<string, unknown>>) =>
    React.createElement("button", { type: "button", onClick: onPress, ...props }, children),
  ScrollView: ({ children, refreshControl, ...props }: React.PropsWithChildren<{ refreshControl?: React.ReactNode } & Record<string, unknown>>) =>
    React.createElement("div", { ...props, "data-has-refresh": refreshControl ? "true" : "false" }, refreshControl, children),
  RefreshControl: ({ refreshing }: { refreshing?: boolean }) =>
    React.createElement("div", { "data-refreshing": refreshing ? "true" : "false", "data-testid": "refresh-control" }),
}));

vi.mock("react-native-unistyles", () => {
  const theme = {
    spacing: { 2: 8, 3: 12, 4: 16, 6: 24 },
    fontSize: { sm: 14, base: 15, lg: 18 },
    fontWeight: { medium: "500", semibold: "600" },
    colors: {
      surface0: "#111",
      surface1: "#222",
      surface2: "#333",
      foreground: "#fff",
      foregroundMuted: "#999",
      border: "#444",
      palette: {
        green: { 400: "#4ade80" },
        amber: { 500: "#f59e0b" },
        red: { 500: "#ef4444" },
      },
    },
    borderRadius: { md: 6, lg: 8, xl: 12, full: 9999 },
    iconSize: { md: 20 },
    borderWidth: { 1: 1 },
  };

  return {
    StyleSheet: {
      create: (factory: unknown) => (typeof factory === "function" ? factory(theme) : factory),
    },
    useUnistyles: () => ({ theme }),
  };
});

vi.mock("expo-router", () => ({
  router: { navigate: vi.fn() },
}));

vi.mock("@/hooks/use-is-focused", () => ({
  useIsFocused: () => true,
}));

vi.mock("lucide-react-native", () => ({
  AlertTriangle: () => React.createElement("span", { "data-icon": "AlertTriangle" }),
  ChevronLeft: () => React.createElement("span", { "data-icon": "ChevronLeft" }),
  Gauge: () => React.createElement("span", { "data-icon": "Gauge" }),
  RefreshCw: () => React.createElement("span", { "data-icon": "RefreshCw" }),
  Server: () => React.createElement("span", { "data-icon": "Server" }),
}));

vi.mock("@/components/headers/back-header", () => ({
  BackHeader: ({
    title,
    rightContent,
  }: {
    title: string;
    rightContent?: React.ReactNode;
  }) =>
    React.createElement("header", { "data-testid": "back-header" }, title, rightContent),
}));

vi.mock("@/components/ui/button", () => ({
  Button: ({
    children,
    onPress,
    testID,
  }: React.PropsWithChildren<{ onPress?: () => void; testID?: string }>) =>
    React.createElement("button", { type: "button", onClick: onPress, "data-testid": testID }, children),
}));

vi.mock("@/components/ui/loading-spinner", () => ({
  LoadingSpinner: () => React.createElement("div", { "data-testid": "loading-spinner" }),
}));

vi.mock("@/hooks/use-usage", () => ({
  useUsage: () => {
    if (!usageResult.current) {
      throw new Error("Expected usage result");
    }
    return usageResult.current;
  },
}));

vi.mock("@/utils/host-routes", () => ({
  buildHostOpenProjectRoute: (serverId: string) => `/h/${serverId}/open-project`,
}));

function makeUsageQuotaSnapshot(overrides: Partial<UsageQuotaSnapshot> = {}): UsageQuotaSnapshot {
  return {
    provider: "anthropic",
    plan: { name: "Pro", tier: "pro" },
    quotas: [
      {
        name: "5h",
        label: "5h",
        used: 10,
        limit: 100,
        usedPct: 10,
        unit: "requests",
        windowStart: null,
        resetAt: null,
        resetIn: "2h 30m",
      },
    ],
    fetchedAt: new Date().toISOString(),
    ...overrides,
  };
}

function makeUsageResult(overrides: Partial<UsageResult> = {}): UsageResult {
  return {
    snapshots: [],
    errors: {},
    cachedAt: null,
    isLoading: false,
    isInitialLoad: false,
    isRevalidating: false,
    error: null,
    refreshAll: vi.fn(),
    ...overrides,
  };
}

describe("UsageScreen", () => {
  let root: Root | null = null;
  let container: HTMLElement | null = null;

  beforeEach(() => {
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
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
    usageResult.current = null;
  });

  it("renders provider cards for loaded snapshots", async () => {
    usageResult.current = makeUsageResult({
      snapshots: [makeUsageQuotaSnapshot()],
    });

    await act(async () => {
      root?.render(<UsageScreen serverId="server-1" />);
    });

    expect(container?.textContent).toContain("Usage");
    expect(container?.textContent).toContain("Anthropic");
    expect(container?.textContent).toContain("Pro · pro");
    expect(container?.textContent).toContain("10/100 requests");
    expect(container?.textContent).toContain("Resets in 2h 30m");
    expect(container?.textContent).not.toContain("elapsed");
  });

  it("shows the elapsed window percentage when the quota has a reset window", async () => {
    const now = Date.now();
    usageResult.current = makeUsageResult({
      snapshots: [
        makeUsageQuotaSnapshot({
          quotas: [
            {
              name: "weekly_usage",
              label: "Weekly Usage",
              used: 62,
              limit: 100,
              usedPct: 62,
              unit: "requests",
              windowStart: new Date(now - 3 * 60 * 60 * 1000).toISOString(),
              resetAt: new Date(now + 3 * 60 * 60 * 1000).toISOString(),
              resetIn: "2h 59m",
            },
          ],
        }),
      ],
    });

    await act(async () => {
      root?.render(<UsageScreen serverId="server-1" />);
    });

    expect(container?.textContent).toContain("50% elapsed · Resets in 2h 59m");
  });

  it("renders the empty state when no providers are configured", async () => {
    usageResult.current = makeUsageResult();

    await act(async () => {
      root?.render(<UsageScreen serverId="server-1" />);
    });

    expect(container?.textContent).toContain("No usage providers configured");
    expect(container?.textContent).toContain("solo-usage init");
    expect(container?.textContent).toContain("~/.solo/usage.json");
  });

  it("renders a warning card for per-provider errors", async () => {
    usageResult.current = makeUsageResult({
      errors: { openai: "rate limited" },
    });

    await act(async () => {
      root?.render(<UsageScreen serverId="server-1" />);
    });

    expect(container?.textContent).toContain("Openai");
    expect(container?.textContent).toContain("rate limited");
  });

  it("shows the loading spinner during the initial load", async () => {
    usageResult.current = makeUsageResult({ isLoading: true, isInitialLoad: true });

    await act(async () => {
      root?.render(<UsageScreen serverId="server-1" />);
    });

    expect(container?.querySelector('[data-testid="loading-spinner"]')).not.toBeNull();
  });
});
