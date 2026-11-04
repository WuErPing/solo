/**
 * @vitest-environment jsdom
 */
import React from "react";
import { Buffer } from "buffer";
import { cleanup, render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  MarkdownFileImage,
  isWorkspaceImageReference,
  resolveMarkdownImagePath,
} from "@/components/markdown-file-image";

const exploreFileSystemMock = vi.hoisted(() => vi.fn());
const persistAttachmentMock = vi.hoisted(() => vi.fn());
const attachmentUriMock = vi.hoisted(() => vi.fn());

vi.mock("react-native-unistyles", () => ({
  StyleSheet: {
    create: (factory: unknown) =>
      typeof factory === "function"
        ? factory({
            colors: { surface0: "#fff", surface2: "#ddd", border: "#ccc", foregroundMuted: "#666" },
            spacing: { 3: 12, 4: 16 },
            fontSize: { sm: 13 },
          })
        : factory,
  },
  useUnistyles: () => ({
    theme: {
      colorScheme: "light",
      colors: { surface0: "#fff", surface2: "#ddd", border: "#ccc", foregroundMuted: "#666" },
      spacing: { 3: 12, 4: 16 },
      fontSize: { sm: 13 },
    },
  }),
}));

vi.mock("react-native-svg", () => ({
  SvgXml: ({ xml, width, height }: { xml: string; width: number; height: number }) => (
    <div data-testid="svg-xml" data-width={width} data-height={height}>
      {xml}
    </div>
  ),
}));

vi.mock("react-native-fit-image", () => ({
  default: ({ source }: { source: { uri: string } }) => (
    <img data-testid="fit-image" src={source.uri} alt="" />
  ),
}));

vi.mock("lucide-react-native", () => ({ Maximize2: () => null }));
vi.mock("@/components/diagram-lightbox", () => ({ DiagramLightbox: () => null }));

vi.mock("@/stores/session-store", () => ({
  useSessionStore: (selector: (state: unknown) => unknown) =>
    selector({ sessions: { server: { client: { exploreFileSystem: exploreFileSystemMock } } } }),
}));

vi.mock("@/attachments/service", () => ({
  persistAttachmentFromBase64: persistAttachmentMock,
}));

vi.mock("@/attachments/use-attachment-preview-url", () => ({
  useAttachmentPreviewUrl: attachmentUriMock,
}));

const SVG_SOURCE =
  '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 960 830" font-family="PingFang SC"><rect width="960" height="830"/></svg>';

function renderImage(props: Partial<React.ComponentProps<typeof MarkdownFileImage>> = {}) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const result = render(
    <QueryClientProvider client={queryClient}>
      <MarkdownFileImage
        src="imgs/architecture.svg"
        alt="Architecture"
        serverId="server"
        workspaceRoot="/repo"
        filePath="docs/guide.md"
        {...props}
      />
    </QueryClientProvider>,
  );
  return { ...result, queryClient };
}

describe("resolveMarkdownImagePath", () => {
  it("resolves against the document's directory", () => {
    expect(resolveMarkdownImagePath("docs/guide.md", "imgs/a.svg", "/repo")).toBe(
      "docs/imgs/a.svg",
    );
    expect(resolveMarkdownImagePath("readme.md", "imgs/a.svg", "/repo")).toBe("imgs/a.svg");
  });

  it("collapses parent-directory segments inside the workspace", () => {
    expect(resolveMarkdownImagePath("docs/nested/guide.md", "../imgs/a.svg", "/repo")).toBe(
      "docs/imgs/a.svg",
    );
    expect(resolveMarkdownImagePath("docs/guide.md", "./imgs/a.svg", "/repo")).toBe(
      "docs/imgs/a.svg",
    );
  });

  it("rejects references that escape the workspace", () => {
    expect(resolveMarkdownImagePath("docs/guide.md", "../../secret.svg", "/repo")).toBeNull();
    expect(resolveMarkdownImagePath("guide.md", "../outside.svg", "/repo")).toBeNull();
  });

  it("accepts absolute paths inside the workspace and rejects ones outside", () => {
    expect(resolveMarkdownImagePath("docs/guide.md", "/repo/imgs/a.svg", "/repo")).toBe(
      "imgs/a.svg",
    );
    expect(resolveMarkdownImagePath("docs/guide.md", "/etc/passwd", "/repo")).toBeNull();
  });

  it("ignores URL schemes", () => {
    expect(resolveMarkdownImagePath("docs/guide.md", "https://x.dev/a.svg", "/repo")).toBeNull();
    expect(resolveMarkdownImagePath("docs/guide.md", "data:image/png;base64,AA", "/repo")).toBeNull();
  });

  it("decodes percent-encoding and strips query and fragment suffixes", () => {
    expect(resolveMarkdownImagePath("docs/guide.md", "imgs/my%20diagram.svg", "/repo")).toBe(
      "docs/imgs/my diagram.svg",
    );
    expect(resolveMarkdownImagePath("docs/guide.md", "imgs/a.svg?v=2#frag", "/repo")).toBe(
      "docs/imgs/a.svg",
    );
  });

  it("resolves relative to an absolute document path", () => {
    expect(resolveMarkdownImagePath("/repo/docs/guide.md", "imgs/a.svg", "/repo")).toBe(
      "docs/imgs/a.svg",
    );
  });
});

describe("isWorkspaceImageReference", () => {
  // react-native-markdown-display's defaults.
  const ALLOWED = [
    "data:image/png;base64",
    "data:image/gif;base64",
    "data:image/jpeg;base64",
    "https://",
    "http://",
  ];

  const check = (src: string) =>
    isWorkspaceImageReference("docs/guide.md", src, "/repo", ALLOWED);

  it("claims references the markdown renderer cannot load itself", () => {
    expect(check("imgs/architecture.svg")).toBe(true);
    expect(check("./imgs/architecture.svg")).toBe(true);
    expect(check("/repo/imgs/architecture.svg")).toBe(true);
  });

  it("leaves loadable URLs to the markdown renderer", () => {
    expect(check("https://example.dev/a.svg")).toBe(false);
    expect(check("http://example.dev/a.svg")).toBe(false);
    expect(check("data:image/png;base64,AAAA")).toBe(false);
  });

  it("rejects references that escape the workspace", () => {
    expect(check("../../secret.svg")).toBe(false);
    expect(check("/etc/passwd")).toBe(false);
  });
});

describe("MarkdownFileImage", () => {
  beforeEach(() => {
    exploreFileSystemMock.mockReset();
    persistAttachmentMock.mockReset();
    attachmentUriMock.mockReset();
    attachmentUriMock.mockReturnValue(null);
  });

  afterEach(() => {
    cleanup();
  });

  it("paints a base64-encoded SVG file with react-native-svg", async () => {
    exploreFileSystemMock.mockResolvedValue({
      file: {
        path: "docs/imgs/architecture.svg",
        kind: "image",
        encoding: "base64",
        mimeType: "image/svg+xml",
        content: Buffer.from(SVG_SOURCE, "utf8").toString("base64"),
        size: SVG_SOURCE.length,
        modifiedAt: "2026-01-01T00:00:00Z",
      },
      error: null,
    });

    const { queryClient } = renderImage();

    const svg = await screen.findByTestId("svg-xml");
    expect(exploreFileSystemMock).toHaveBeenCalledWith("/repo", "docs/imgs/architecture.svg", "file");
    expect(svg.getAttribute("data-width")).not.toBe("0");
    expect(svg.textContent).toContain('font-family="PingFang SC"');
    expect(persistAttachmentMock).not.toHaveBeenCalled();
    queryClient.clear();
  });

  it("routes raster images through the attachment store", async () => {
    exploreFileSystemMock.mockResolvedValue({
      file: {
        path: "docs/imgs/shot.png",
        kind: "image",
        encoding: "base64",
        mimeType: "image/png",
        content: "aGVsbG8=",
        size: 5,
        modifiedAt: "2026-01-01T00:00:00Z",
      },
      error: null,
    });
    persistAttachmentMock.mockResolvedValue({ id: "att_1", mimeType: "image/png" });
    attachmentUriMock.mockReturnValue("blob:preview");

    const { queryClient } = renderImage({ src: "imgs/shot.png" });

    expect((await screen.findByTestId("fit-image")).getAttribute("src")).toBe("blob:preview");
    expect(persistAttachmentMock).toHaveBeenCalledWith(
      expect.objectContaining({ base64: "aGVsbG8=", mimeType: "image/png" }),
    );
    queryClient.clear();
  });

  it("shows the alt text when the file cannot be read", async () => {
    exploreFileSystemMock.mockResolvedValue({ file: null, error: "open: no such file" });

    const { queryClient } = renderImage();

    const missing = await screen.findByTestId("markdown-file-image-missing");
    expect(missing.textContent).toBe("Architecture");
    queryClient.clear();
  });

  it("does not query the daemon for references outside the workspace", async () => {
    const { queryClient } = renderImage({ src: "../../secret.svg" });

    expect(await screen.findByTestId("markdown-file-image-missing")).toBeTruthy();
    expect(exploreFileSystemMock).not.toHaveBeenCalled();
    queryClient.clear();
  });
});
