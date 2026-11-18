/**
 * @vitest-environment jsdom
 */
import React from "react";
import { fireEvent, render, screen, cleanup } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { FilePane } from "@/components/file-pane";

const exploreFileSystemMock = vi.hoisted(() => vi.fn());

vi.mock("react-native-unistyles", () => {
  const mockTheme = {
    colorScheme: "light",
    colors: {
      surface0: "#fff",
      surface1: "#eee",
      border: "#ccc",
      foreground: "#111",
      foregroundMuted: "#666",
      background: "#fff",
      primary: "#00f",
    },
  };
  return {
    StyleSheet: {
      create: (factory: unknown) =>
        typeof factory === "function"
          ? factory({
              colors: {
                surface0: "#fff",
                surface1: "#eee",
                border: "#ccc",
                foreground: "#111",
                foregroundMuted: "#666",
                destructive: "#f00",
                background: "#fff",
                primary: "#00f",
              },
              spacing: { 2: 8, 3: 12, 4: 16 },
              fontSize: { sm: 13 },
            })
          : factory,
    },
    UnistylesRuntime: {
      getTheme: () => mockTheme,
    },
    useUnistyles: () => ({ theme: mockTheme }),
  };
});

vi.mock("react-native-fit-image", () => ({
  default: () => null,
}));

vi.mock("lucide-react-native", () => ({
  TextSelect: () => null,
  SquarePen: () => null,
  SquareTerminal: () => null,
  RefreshCw: () => null,
}));

vi.mock("@/components/mermaid-preview", () => ({ MermaidPreview: () => null }));
vi.mock("@/components/mermaid-svg-service", () => ({ MermaidSvgServiceHost: () => null }));
vi.mock("@/components/plantuml-preview", () => ({ PlantUmlPreview: () => null }));
vi.mock("@/components/math-text", () => ({ MathText: () => null }));
vi.mock("@/components/svg-preview", () => ({ SvgPreview: () => null }));
// Pulls in the gesture/reanimated lightbox chain, which cannot load in the
// node test environment; covered by markdown-file-image.test.tsx.
vi.mock("@/components/markdown-file-image", () => ({
  MarkdownFileImage: () => null,
  isWorkspaceImageReference: () => false,
}));
vi.mock("@/attachments/service", () => ({ persistAttachmentFromBase64: vi.fn() }));

vi.mock("react-native-markdown-display", () => ({
  default: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  MarkdownIt: () => ({ use: () => ({}) }),
  renderRules: {},
}));

vi.mock("@/stores/session-store", () => ({
  useSessionStore: (selector: (state: unknown) => unknown) =>
    selector({
      sessions: {
        server: {
          client: { exploreFileSystem: exploreFileSystemMock },
        },
      },
    }),
}));

vi.mock("@/hooks/use-settings", () => ({
  useAppSettings: () => ({ settings: { filePreviewPageButtons: false } }),
}));

vi.mock("@/constants/layout", () => ({
  useIsCompactFormFactor: () => false,
}));

vi.mock("@/constants/platform", () => ({
  isWeb: true,
  isNative: false,
  isDev: false,
}));

vi.mock("@/components/file-preview-scroll-view", () => ({
  FilePreviewScrollView: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
}));

vi.mock("@/components/use-web-scrollbar", () => ({
  useWebScrollViewScrollbar: () => ({}),
}));

vi.mock("@/hooks/use-web-scrollbar-style", () => ({
  useWebScrollbarStyle: () => ({}),
}));

vi.mock("@/styles/markdown-styles", () => ({
  createMarkdownStyles: () => ({}),
}));

vi.mock("@/attachments/use-attachment-preview-url", () => ({
  useAttachmentPreviewUrl: () => null,
}));

function renderFilePane(filePath: string, onNewAgent = vi.fn(), onNewTmux = vi.fn()) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const result = render(
    <QueryClientProvider client={queryClient}>
      <FilePane
        serverId="server"
        workspaceRoot="/repo"
        filePath={filePath}
        onNewAgent={onNewAgent}
        onNewTmux={onNewTmux}
      />
    </QueryClientProvider>,
  );
  return { ...result, onNewAgent, onNewTmux, queryClient };
}

describe("FilePane markdown agent action", () => {
  afterEach(() => {
    cleanup();
  });

  beforeEach(() => {
    exploreFileSystemMock.mockReset();
    exploreFileSystemMock.mockResolvedValue({
      file: {
        kind: "text",
        path: "docs/guide.md",
        content: "# Guide",
        size: 7,
        modifiedAt: 1,
      },
    });
  });

  it("opens a new agent with the previewed markdown file", async () => {
    const { onNewAgent, queryClient } = renderFilePane("docs/guide.md");

    fireEvent.click(await screen.findByTestId("markdown-new-agent"));

    expect(onNewAgent).toHaveBeenCalledWith("docs/guide.md");
    queryClient.clear();
  });

  it("opens the tmux agent picker with the previewed markdown file", async () => {
    const { onNewTmux, queryClient } = renderFilePane("docs/guide.md");

    fireEvent.click(await screen.findByTestId("markdown-new-tmux"));

    expect(onNewTmux).toHaveBeenCalledWith("docs/guide.md");
    queryClient.clear();
  });

  it("hides the tmux button when no handler is provided", async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={queryClient}>
        <FilePane serverId="server" workspaceRoot="/repo" filePath="docs/guide.md" />
      </QueryClientProvider>,
    );

    await screen.findByTestId("markdown-refresh");
    expect(screen.queryByTestId("markdown-new-tmux")).toBeNull();
    queryClient.clear();
  });

  it("refetches the file when the refresh button is pressed", async () => {
    const { queryClient } = renderFilePane("docs/guide.md");

    fireEvent.click(await screen.findByTestId("markdown-refresh"));

    await vi.waitFor(() => {
      expect(exploreFileSystemMock).toHaveBeenCalledTimes(2);
    });
    queryClient.clear();
  });
});
