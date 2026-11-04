import React, { useMemo, useRef } from "react";
import { useQuery } from "@tanstack/react-query";
import Markdown, {
  MarkdownIt,
  renderRules,
  type ASTNode,
  type RenderRules,
} from "react-native-markdown-display";
import FitImage from "react-native-fit-image";
import { RefreshCw, SquarePen, TextSelect } from "lucide-react-native";
import {
  ActivityIndicator,
  Image as RNImage,
  Pressable,
  ScrollView as RNScrollView,
  Text,
  View,
} from "react-native";
import { StyleSheet, UnistylesRuntime, useUnistyles } from "react-native-unistyles";
import { useIsCompactFormFactor } from "@/constants/layout";
import { useAppSettings } from "@/hooks/use-settings";
import { useSessionStore, type ExplorerFile } from "@/stores/session-store";
import { useWebScrollViewScrollbar } from "@/components/use-web-scrollbar";
import { useWebScrollbarStyle } from "@/hooks/use-web-scrollbar-style";
import { highlightCode, darkHighlightColors, lightHighlightColors } from "@getsolo/highlight";
import { CodeLine } from "@/components/code-line";
import { lineNumberGutterWidth } from "@/components/code-insets";
import { isRenderedMarkdownFile } from "@/components/file-pane-render-mode";
import { FilePreviewScrollView } from "@/components/file-preview-scroll-view";
import { MarkdownFileImage, isWorkspaceImageReference } from "@/components/markdown-file-image";
import { MermaidPreview } from "@/components/mermaid-preview";
import { PlantUmlPreview } from "@/components/plantuml-preview";
import { MathText } from "@/components/math-text";
import { mathPlugin } from "@/components/markdown-math-plugin";
import {
  createSelectableMarkdownTextRules,
  renderSelectableFence,
} from "@/components/markdown-selectable-rules";
import { SvgPreview } from "@/components/svg-preview";
import { isSvgContent } from "@/components/svg-preview-utils";
import { isWeb } from "@/constants/platform";
import { createMarkdownStyles } from "@/styles/markdown-styles";
import { MermaidSvgServiceHost } from "@/components/mermaid-svg-service";
import { splitMarkdownBlocks } from "@/utils/split-markdown-blocks";
import { chunkMarkdownBlocks } from "@/utils/chunk-markdown-blocks";
import { perfLog, perfNow } from "@/utils/perf-log";
import type { AttachmentMetadata } from "@/attachments/types";
import { useAttachmentPreviewUrl } from "@/attachments/use-attachment-preview-url";
import { persistAttachmentFromBase64 } from "@/attachments/service";
import { createPreviewAttachmentId, getFileNameFromPath } from "@/attachments/utils";

interface FilePreviewBodyProps {
  preview: ExplorerFile | null;
  isLoading: boolean;
  showDesktopWebScrollbar: boolean;
  isMobile: boolean;
  filePath: string;
  imagePreviewUri: string | null;
  serverId: string;
  workspaceRoot: string;
  onNewAgent?: (filePath: string) => void;
  onRefresh?: () => void;
}

function trimNonEmpty(value: string | null | undefined): string | null {
  if (typeof value !== "string") {
    return null;
  }
  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed : null;
}

function formatFileSize({ size }: { size: number }): string {
  if (size < 1024) {
    return `${size} B`;
  }
  if (size < 1024 * 1024) {
    return `${(size / 1024).toFixed(1)} KB`;
  }
  return `${(size / (1024 * 1024)).toFixed(1)} MB`;
}

async function createFilePanePreview(file: ExplorerFile | null): Promise<{
  file: ExplorerFile | null;
  imageAttachment: AttachmentMetadata | null;
}> {
  if (!file || file.kind !== "image" || !file.content) {
    return { file, imageAttachment: null };
  }

  const { content: _content, ...imageFile } = file;
  const imageAttachment = await persistAttachmentFromBase64({
    id: createPreviewAttachmentId({
      mimeType: file.mimeType ?? "image/png",
      path: file.path,
      size: file.size,
      modifiedAt: file.modifiedAt,
      contentLength: file.content.length,
    }),
    base64: file.content,
    mimeType: file.mimeType,
    fileName: getFileNameFromPath(file.path),
  });

  return {
    file: imageFile,
    imageAttachment,
  };
}

const INITIAL_MARKDOWN_BLOCKS = 24;
const MARKDOWN_BLOCKS_PER_TICK = 24;

interface MemoizedMarkdownBlockProps {
  text: string;
  rules: RenderRules;
  parser: MarkdownIt;
  style: ReturnType<typeof createMarkdownStyles>;
}

const MemoizedMarkdownBlock = React.memo(function MemoizedMarkdownBlock({
  text,
  rules,
  parser,
  style,
}: MemoizedMarkdownBlockProps) {
  return (
    <Markdown style={style} markdownit={parser} rules={rules}>
      {text}
    </Markdown>
  );
});

interface MarkdownFilePreviewProps
  extends Pick<
    React.ComponentProps<typeof FilePreviewScrollView>,
    "scrollRef" | "scrollbar" | "pagerEnabled" | "showsVerticalScrollIndicator"
  > {
  content: string;
  serverId: string;
  filePath: string;
  workspaceRoot: string;
  onNewAgent?: (filePath: string) => void;
  onRefresh?: () => void;
}

/**
 * Rendered-markdown file preview. The document is split into
 * blank-line-separated blocks (fences stay intact) and each block gets its own
 * memoized <Markdown>: a content change re-parses only the blocks that
 * actually changed, and per-block AST keys (unstable across parses by design)
 * never remount untouched siblings — mermaid SVGs and math formulas included.
 * A single shared service renders every mermaid diagram to SVG.
 */
function MarkdownFilePreview({
  content,
  serverId,
  filePath,
  workspaceRoot,
  onNewAgent,
  onRefresh,
  scrollRef,
  scrollbar,
  pagerEnabled,
  showsVerticalScrollIndicator,
}: MarkdownFilePreviewProps) {
  const { theme } = useUnistyles();
  const isDark = theme.colorScheme === "dark";
  // Select mode (native only): swaps the text-carrying rules for selectable
  // variants so long-press starts the platform selection UI. Kept behind a
  // toggle because selectable text swallows link taps on Android/iOS.
  const [selectMode, setSelectMode] = React.useState(false);
  // `useUnistyles()` returns a fresh Proxy each render, so keying the style
  // memo (and createMarkdownStyles' WeakMap cache) on it recreates the style
  // object every render. That makes each memoized block re-parse, and since
  // AST node keys are unstable across parses, every embedded preview (mermaid
  // iframe, math) remounts — an infinite flicker loop once a preview reports a
  // new height. Resolve the stable registered theme instead; `useUnistyles()`
  // above still subscribes this component to real theme changes.
  const stableTheme = UnistylesRuntime.getTheme();
  const markdownStyles = useMemo(() => createMarkdownStyles(stableTheme), [stableTheme]);
  const markdownParser = useMemo(
    () => MarkdownIt({ typographer: true, linkify: true }).use(mathPlugin),
    [],
  );
  const markdownRules = useMemo(
    () => ({
      ...(selectMode ? createSelectableMarkdownTextRules() : null),
      fence: (
        node: ASTNode,
        children: React.ReactNode[],
        parentNodes: ASTNode[],
        styles: Record<string, unknown>,
        inheritedStyles = {},
      ) => {
        const sourceInfo = (node as ASTNode & { sourceInfo?: string }).sourceInfo;
        const language = sourceInfo?.trim().split(/\s+/)[0].toLowerCase();
        if (language === "mermaid") {
          return (
            <MermaidPreview key={node.key} source={node.content} isDark={isDark} />
          );
        }
        if (language === "plantuml" || language === "puml") {
          return (
            <PlantUmlPreview
              key={node.key}
              source={node.content}
              isDark={isDark}
              serverId={serverId}
            />
          );
        }
        return selectMode
          ? renderSelectableFence(node, styles, inheritedStyles)
          : renderRules.fence!(node, children, parentNodes, styles, inheritedStyles);
      },
      // Mirror the library's default image rule but pass `key` directly to JSX.
      // react-native-markdown-display spreads a props object containing `key`
      // into <FitImage {...props} />, which React 19 rejects with a console error.
      image: (
        node: ASTNode,
        children: React.ReactNode[],
        parentNodes: ASTNode[],
        styles: Record<string, unknown>,
        allowedImageHandlers: string[],
        defaultImageHandler: string,
      ) => {
        const { src, alt } = node.attributes as { src?: string; alt?: string };
        const uri = src ?? "";
        const show =
          allowedImageHandlers.filter((value) =>
            uri.toLowerCase().startsWith(value.toLowerCase()),
          ).length > 0;
        // The library's defaultImageHandler is `https://`, so a workspace-relative
        // reference (e.g. `imgs/architecture.svg`) would otherwise be requested as
        // a bogus URL. Hand those to the daemon-backed renderer instead.
        if (isWorkspaceImageReference(filePath, uri, workspaceRoot, allowedImageHandlers)) {
          return (
            <MarkdownFileImage
              key={node.key}
              src={uri}
              alt={alt}
              serverId={serverId}
              workspaceRoot={workspaceRoot}
              filePath={filePath}
            />
          );
        }
        if (!show && !defaultImageHandler) {
          return null;
        }
        return (
          <FitImage
            key={node.key}
            indicator
            style={styles._VIEW_SAFE_image as React.ComponentProps<typeof FitImage>["style"]}
            source={{ uri: show ? uri : `${defaultImageHandler}${uri}` }}
            accessible={alt ? true : undefined}
            accessibilityLabel={alt ?? undefined}
          />
        );
      },
      math_inline: (node: ASTNode) => (
        <MathText key={node.key} tex={node.content} display={false} isDark={isDark} />
      ),
      math_block: (node: ASTNode) => (
        <MathText key={node.key} tex={node.content} display isDark={isDark} />
      ),
    }),
    [isDark, selectMode, serverId, workspaceRoot, filePath],
  );

  const markdownBlocks = useMemo(() => {
    const splitStart = perfNow();
    const blocks = chunkMarkdownBlocks(splitMarkdownBlocks(content));
    perfLog("filepane.markdown.split", {
      ms: Math.round(perfNow() - splitStart),
      blocks: blocks.length,
    });
    return blocks;
  }, [content]);

  // Progressive mount: the first batch paints immediately; the rest append in
  // event-loop slices so a large document never monopolizes the JS thread for
  // one multi-second commit. Mermaid previews mount (and enqueue their SVG
  // renders) only when their block mounts.
  const [mountedCount, setMountedCount] = React.useState(INITIAL_MARKDOWN_BLOCKS);
  const [prevContent, setPrevContent] = React.useState(content);
  if (prevContent !== content) {
    setPrevContent(content);
    setMountedCount(INITIAL_MARKDOWN_BLOCKS);
    setSelectMode(false);
  }
  React.useEffect(() => {
    if (mountedCount >= markdownBlocks.length) {
      return;
    }
    const timer = setTimeout(
      () =>
        setMountedCount((count) =>
          Math.min(count + MARKDOWN_BLOCKS_PER_TICK, markdownBlocks.length),
        ),
      0,
    );
    return () => clearTimeout(timer);
  }, [mountedCount, markdownBlocks.length]);

  // The service WebView carries a multi-MB inline mermaid bundle — create it
  // after the first batch commits, off the critical first-paint path.
  const hasMermaid = useMemo(() => content.includes("```mermaid"), [content]);
  const [serviceMounted, setServiceMounted] = React.useState(false);
  React.useEffect(() => {
    if (!hasMermaid) {
      return;
    }
    const timer = setTimeout(() => setServiceMounted(true), 0);
    return () => clearTimeout(timer);
  }, [hasMermaid]);

  // Dev-only commit marker: re-created when content changes so the effect
  // below can report render→commit latency per document version.
  const commitMark = useMemo(() => ({ content, at: perfNow() }), [content]);
  React.useEffect(() => {
    if (!commitMark.content) {
      return;
    }
    perfLog("filepane.markdown.commit", {
      bytes: commitMark.content.length,
      blocks: markdownBlocks.length,
      mounted: mountedCount,
      mermaid: commitMark.content.split("```mermaid").length - 1,
      ms: Math.round(perfNow() - commitMark.at),
    });
  }, [commitMark, markdownBlocks, mountedCount]);

  return (
    <View style={styles.markdownPreviewContainer}>
      <FilePreviewScrollView
        scrollRef={scrollRef}
        scrollbar={scrollbar}
        pagerEnabled={pagerEnabled}
        showsVerticalScrollIndicator={showsVerticalScrollIndicator}
        contentContainerStyle={styles.previewMarkdownScrollContent}
      >
        {serviceMounted ? <MermaidSvgServiceHost /> : null}
        {markdownBlocks.slice(0, mountedCount).map((block, index) => (
          <MemoizedMarkdownBlock
            key={`${index}:${block.slice(0, 32)}`}
            text={block}
            rules={markdownRules}
            parser={markdownParser}
            style={markdownStyles}
          />
        ))}
      </FilePreviewScrollView>
      <View style={styles.markdownPreviewActions}>
        {onRefresh ? (
          <Pressable
            testID="markdown-refresh"
            accessibilityRole="button"
            accessibilityLabel="Refresh preview"
            onPress={onRefresh}
            style={({ pressed }) => [styles.selectToggleButton, pressed ? { opacity: 0.7 } : null]}
          >
            <RefreshCw size={16} color={theme.colors.foregroundMuted} />
          </Pressable>
        ) : null}
        {onNewAgent ? (
          <Pressable
            testID="markdown-new-agent"
            accessibilityRole="button"
            accessibilityLabel="New agent with this file"
            onPress={() => onNewAgent(filePath)}
            style={({ pressed }) => [styles.newAgentButton, pressed ? { opacity: 0.7 } : null]}
          >
            <SquarePen size={16} color={theme.colors.foregroundMuted} />
            <Text style={styles.newAgentButtonText}>New Agent</Text>
          </Pressable>
        ) : null}
        {isWeb ? null : (
          <Pressable
            testID="markdown-select-toggle"
            accessibilityRole="button"
            accessibilityLabel="Toggle text selection"
            accessibilityState={{ selected: selectMode }}
            onPress={() => setSelectMode((value) => !value)}
            style={({ pressed }) => [
              styles.selectToggleButton,
              selectMode ? styles.selectToggleButtonActive : null,
              pressed ? { opacity: 0.7 } : null,
            ]}
          >
            <TextSelect
              size={16}
              color={selectMode ? theme.colors.background : theme.colors.foregroundMuted}
            />
          </Pressable>
        )}
      </View>
    </View>
  );
}

function FilePreviewBody({
  preview,
  isLoading,
  showDesktopWebScrollbar,
  isMobile,
  filePath,
  imagePreviewUri,
  serverId,
  workspaceRoot,
  onNewAgent,
  onRefresh,
}: FilePreviewBodyProps) {
  const { theme } = useUnistyles();
  const { settings: appSettings } = useAppSettings();
  const showPagerButtons = isMobile && appSettings.filePreviewPageButtons;
  const isDark = theme.colorScheme === "dark";
  const colorMap = isDark ? darkHighlightColors : lightHighlightColors;
  const baseColor = isDark ? "#c9d1d9" : "#24292f";
  const isMarkdownFile = preview?.kind === "text" && isRenderedMarkdownFile(filePath);

  const previewScrollRef = useRef<RNScrollView>(null);
  const webScrollbarStyle = useWebScrollbarStyle();
  const scrollbar = useWebScrollViewScrollbar(previewScrollRef, {
    enabled: showDesktopWebScrollbar,
  });

  const highlightedLines = useMemo(() => {
    if (!preview || preview.kind !== "text" || isMarkdownFile) {
      return null;
    }

    return highlightCode(preview.content ?? "", filePath);
  }, [isMarkdownFile, preview, filePath]);

  const gutterWidth = useMemo(() => {
    if (!highlightedLines) return 0;
    return lineNumberGutterWidth(highlightedLines.length);
  }, [highlightedLines]);

  const imageSource = useMemo(
    () => (imagePreviewUri ? { uri: imagePreviewUri } : null),
    [imagePreviewUri],
  );

  if (isLoading && !preview) {
    return (
      <View style={styles.centerState}>
        <ActivityIndicator size="small" />
        <Text style={styles.loadingText}>Loading file…</Text>
      </View>
    );
  }

  if (!preview) {
    return (
      <View style={styles.centerState}>
        <Text style={styles.emptyText}>No preview available</Text>
      </View>
    );
  }

  if (preview.kind === "text") {
    if (isMarkdownFile) {
      return (
        <MarkdownFilePreview
          content={preview.content ?? ""}
          serverId={serverId}
          filePath={filePath}
          workspaceRoot={workspaceRoot}
          onNewAgent={onNewAgent}
          onRefresh={onRefresh}
          scrollRef={previewScrollRef}
          scrollbar={scrollbar}
          pagerEnabled={showPagerButtons}
          showsVerticalScrollIndicator={!showDesktopWebScrollbar}
        />
      );
    }

    if (preview.content && isSvgContent(preview.content)) {
      return (
        <View style={styles.previewScrollContainer}>
          <SvgPreview source={preview.content} />
        </View>
      );
    }

    const lines = highlightedLines ?? [[{ text: preview.content ?? "", style: null }]];
    const keyedLines = lines.map((tokens, index) => ({
      key: `line-${index}`,
      tokens,
      lineNumber: index + 1,
    }));
    const codeLines = (
      <View>
        {keyedLines.map(({ key, tokens, lineNumber }) => (
          <CodeLine
            key={key}
            tokens={tokens}
            lineNumber={lineNumber}
            gutterWidth={gutterWidth}
            colorMap={colorMap}
            baseColor={baseColor}
          />
        ))}
      </View>
    );

    return (
      <FilePreviewScrollView
        scrollRef={previewScrollRef}
        scrollbar={scrollbar}
        pagerEnabled={showPagerButtons}
        showsVerticalScrollIndicator={!showDesktopWebScrollbar}
      >
        {isMobile ? (
          <View style={styles.previewCodeScrollContent}>{codeLines}</View>
        ) : (
          <RNScrollView
            horizontal
            nestedScrollEnabled
            showsHorizontalScrollIndicator
            style={webScrollbarStyle}
            contentContainerStyle={styles.previewCodeScrollContent}
          >
            {codeLines}
          </RNScrollView>
        )}
      </FilePreviewScrollView>
    );
  }

  if (preview.kind === "image") {
    if (!imagePreviewUri) {
      return (
        <View style={styles.centerState}>
          <ActivityIndicator size="small" />
          <Text style={styles.loadingText}>Loading file…</Text>
        </View>
      );
    }

    return (
      <View style={styles.previewScrollContainer}>
        <RNScrollView
          ref={previewScrollRef}
          style={styles.previewContent}
          contentContainerStyle={styles.previewImageScrollContent}
          onLayout={scrollbar.onLayout}
          onScroll={scrollbar.onScroll}
          onContentSizeChange={scrollbar.onContentSizeChange}
          scrollEventThrottle={16}
          showsVerticalScrollIndicator={!showDesktopWebScrollbar}
        >
          <RNImage
            source={imageSource ?? undefined}
            style={styles.previewImage}
            resizeMode="contain"
          />
        </RNScrollView>
        {scrollbar.overlay}
      </View>
    );
  }

  return (
    <View style={styles.centerState}>
      <Text style={styles.emptyText}>Binary preview unavailable</Text>
      <Text style={styles.binaryMetaText}>{formatFileSize({ size: preview.size })}</Text>
    </View>
  );
}

export function FilePane({
  serverId,
  workspaceRoot,
  filePath,
  onNewAgent,
}: {
  serverId: string;
  workspaceRoot: string;
  filePath: string;
  onNewAgent?: (filePath: string) => void;
}) {
  const isMobile = useIsCompactFormFactor();
  const showDesktopWebScrollbar = isWeb && !isMobile;

  const client = useSessionStore((state) => state.sessions[serverId]?.client ?? null);
  const normalizedWorkspaceRoot = useMemo(() => workspaceRoot.trim(), [workspaceRoot]);
  const normalizedFilePath = useMemo(() => trimNonEmpty(filePath), [filePath]);

  const query = useQuery({
    queryKey: ["workspaceFile", serverId, normalizedWorkspaceRoot, normalizedFilePath],
    enabled: Boolean(client && normalizedWorkspaceRoot && normalizedFilePath),
    queryFn: async () => {
      if (!client || !normalizedWorkspaceRoot || !normalizedFilePath) {
        return { file: null as ExplorerFile | null, error: "Host is not connected" };
      }
      const payload = await client.exploreFileSystem(
        normalizedWorkspaceRoot,
        normalizedFilePath,
        "file",
      );
      const preview = await createFilePanePreview(payload.file ?? null);
      return {
        file: preview.file,
        imageAttachment: preview.imageAttachment,
        error: payload.error ?? null,
      };
    },
    staleTime: 5_000,
    refetchOnMount: true,
  });
  const imagePreviewUri = useAttachmentPreviewUrl(query.data?.imageAttachment ?? null);

  return (
    <View style={styles.container} testID="workspace-file-pane">
      {query.data?.error ? (
        <View style={styles.centerState}>
          <Text style={styles.errorText}>{query.data.error}</Text>
        </View>
      ) : null}

      <FilePreviewBody
        preview={query.data?.file ?? null}
        isLoading={query.isFetching}
        showDesktopWebScrollbar={showDesktopWebScrollbar}
        isMobile={isMobile}
        filePath={filePath}
        imagePreviewUri={imagePreviewUri}
        serverId={serverId}
        workspaceRoot={normalizedWorkspaceRoot}
        onNewAgent={onNewAgent}
        onRefresh={() => {
          void query.refetch();
        }}
      />
    </View>
  );
}

const styles = StyleSheet.create((theme) => ({
  container: {
    flex: 1,
    minHeight: 0,
    backgroundColor: theme.colors.surface0,
  },
  centerState: {
    flex: 1,
    alignItems: "center",
    justifyContent: "center",
    padding: theme.spacing[4],
  },
  loadingText: {
    marginTop: theme.spacing[2],
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.sm,
  },
  errorText: {
    color: theme.colors.destructive,
    fontSize: theme.fontSize.sm,
    textAlign: "center",
  },
  emptyText: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.sm,
    textAlign: "center",
  },
  binaryMetaText: {
    marginTop: theme.spacing[2],
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.sm,
  },
  previewScrollContainer: {
    flex: 1,
    minHeight: 0,
  },
  previewContent: {
    flex: 1,
    minHeight: 0,
  },
  previewCodeScrollContent: {
    padding: theme.spacing[4],
  },
  previewMarkdownScrollContent: {
    padding: theme.spacing[4],
  },
  markdownPreviewContainer: {
    flex: 1,
    minHeight: 0,
  },
  markdownPreviewActions: {
    position: "absolute",
    top: theme.spacing[2],
    right: theme.spacing[2],
    flexDirection: "row",
    alignItems: "center",
    gap: theme.spacing[2],
  },
  newAgentButton: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "center",
    gap: theme.spacing[2],
    paddingHorizontal: theme.spacing[2],
    paddingVertical: 4,
    borderRadius: 6,
    borderWidth: 1,
    borderColor: theme.colors.border,
    backgroundColor: theme.colors.surface1,
  },
  newAgentButtonText: {
    color: theme.colors.foreground,
    fontSize: theme.fontSize.sm,
  },
  selectToggleButton: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "center",
    paddingHorizontal: 6,
    paddingVertical: 4,
    borderRadius: 6,
    borderWidth: 1,
    borderColor: theme.colors.border,
    backgroundColor: theme.colors.surface1,
  },
  selectToggleButtonActive: {
    backgroundColor: theme.colors.primary,
    borderColor: theme.colors.primary,
  },
  previewImageScrollContent: {
    flexGrow: 1,
    padding: theme.spacing[4],
    alignItems: "center",
    justifyContent: "center",
  },
  previewImage: {
    width: "100%",
    height: 420,
  },
}));
